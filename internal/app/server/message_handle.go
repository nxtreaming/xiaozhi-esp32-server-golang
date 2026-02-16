package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"runtime"
	"sync"
	"time"

	data_client "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/data/history"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/memory/llm_memory"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

var (
	// MessageWorkerNum 消息处理worker数量（基于CPU核心数，统一配置，用于Redis+History处理）
	// 必须是2的幂次以便hash分布
	MessageWorkerNum = getMessageWorkerNum()
)

// getMessageWorkerNum 根据CPU核心数计算worker数量，向上取到最近的2的幂次
// 最小值为4，最大值为64
func getMessageWorkerNum() int {
	cpuNum := runtime.NumCPU()

	// 最小值为4，最大值为64
	if cpuNum < 4 {
		return 4
	}
	if cpuNum > 64 {
		return 64
	}

	// 向上取到最近的2的幂次
	power := 1
	for power < cpuNum {
		power <<= 1
	}
	return power
}

// MessageWorker 消息处理器
// 使用固定数量的goroutine池，按SessionID的hash值路由，保证同一会话的消息顺序处理
// 统一处理Redis、MemoryProvider和History消息
type MessageWorker struct {
	client  *history.HistoryClient
	workers []chan *eventbus.AddMessageEvent // 每个worker的channel
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMessageWorker 创建消息处理器
func NewMessageWorker(cfg history.HistoryClientConfig) *MessageWorker {
	client := history.NewHistoryClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	worker := &MessageWorker{
		client:  client,
		workers: make([]chan *eventbus.AddMessageEvent, MessageWorkerNum),
		ctx:     ctx,
		cancel:  cancel,
	}

	// 初始化每个worker的channel并启动goroutine
	for i := 0; i < MessageWorkerNum; i++ {
		worker.workers[i] = make(chan *eventbus.AddMessageEvent, 100) // 缓冲100个消息
		worker.wg.Add(1)
		go worker.workerLoop(i)
	}

	worker.subscribeEvents()
	log.Infof("MessageWorker初始化完成，启动 %d 个worker goroutine（统一处理Redis+MemoryProvider+History）", MessageWorkerNum)
	return worker
}

// workerLoop 每个worker的处理循环（保证顺序处理）
func (w *MessageWorker) workerLoop(index int) {
	defer w.wg.Done()
	defer log.Infof("MessageWorker worker %d 退出", index)

	ch := w.workers[index]
	for {
		select {
		case <-w.ctx.Done():
			// 清理channel中的剩余消息
			for {
				select {
				case event := <-ch:
					if event != nil {
						w.processMessage(event)
					}
				default:
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				// channel已关闭
				return
			}
			if event != nil {
				w.processMessage(event)
			}
		}
	}
}

// processMessage 处理消息（在worker goroutine中顺序执行）
// 统一处理Redis、MemoryProvider和History，保证同一设备/会话的消息顺序处理
func (w *MessageWorker) processMessage(event *eventbus.AddMessageEvent) {
	// 1. 处理 History（所有消息）
	// 使用独立的 context，不受 event.ClientState.Ctx 影响，确保历史消息保存不受对话取消影响
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 判断是新增还是更新
	if event.IsUpdate {
		// 第二阶段：更新音频
		w.updateMessageAudio(ctx, event)
	} else {
		// 第一阶段：保存文本消息（包含Redis处理）
		w.saveMessageText(ctx, event)
	}

	// 2. 处理 MemoryProvider（仅!IsUpdate时，独立于redis和manager）
	// 长期记忆体（memobase/mem0）处理，不管是redis还是manager场景都需要
	if !event.IsUpdate {
		w.processMemoryProvider(event)
	}
}

// processMemoryProvider 处理长期记忆体（memobase/mem0）
// 独立于redis和manager，不管是redis还是manager场景都需要处理
func (w *MessageWorker) processMemoryProvider(event *eventbus.AddMessageEvent) {
	clientState := event.ClientState
	if clientState.MemoryProvider == nil {
		return
	}
	if clientState.GetMemoryMode() != data_client.MemoryModeLong {
		return
	}

	err := clientState.MemoryProvider.AddMessage(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID(),
		event.Msg)
	if err != nil {
		log.Errorf("add message to memory provider failed: %v", err)
	}
}

// hashSessionID 计算SessionID的hash值，返回worker索引
func (w *MessageWorker) hashSessionID(sessionID string) int {
	if sessionID == "" {
		return 0 // 如果SessionID为空，使用第一个worker
	}

	// 使用FNV-1a哈希函数
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	hash := h.Sum32()
	return int(hash) % MessageWorkerNum
}

// subscribeEvents 订阅EventBus事件
func (w *MessageWorker) subscribeEvents() {
	bus := eventbus.Get()
	// 订阅统一的消息添加事件（与 EventHandle 监听同一个 Topic）
	bus.Subscribe(eventbus.TopicAddMessage, w.handleAddMessage)
}

// handleAddMessage 统一处理消息添加事件（路由到对应的worker）
func (w *MessageWorker) handleAddMessage(event *eventbus.AddMessageEvent) {
	if event == nil || event.ClientState == nil {
		return
	}

	// 确定用于路由的key：优先使用SessionID，如果为空则使用DeviceID
	key := event.ClientState.SessionID
	if key == "" {
		key = event.ClientState.DeviceID
	}
	if key == "" {
		log.Warnf("SessionID和DeviceID都为空，无法路由消息")
		return
	}

	// 计算hash值，路由到对应的worker
	workerIndex := w.hashSessionID(key)

	// 非阻塞发送到对应的worker channel
	select {
	case w.workers[workerIndex] <- event:
		// 成功发送
	default:
		// channel已满，记录警告（通常不会发生，因为channel有缓冲）
		log.Warnf("worker %d 的channel已满，丢弃消息, session_id: %s, device_id: %s",
			workerIndex, event.ClientState.SessionID, event.ClientState.DeviceID)
	}
}

// saveMessageText 保存文本消息（第一阶段，或一次性保存文本+音频）
// 包含Redis处理（当config_provider.type为redis时）
func (w *MessageWorker) saveMessageText(ctx context.Context, event *eventbus.AddMessageEvent) {
	// 处理 Redis（仅当config_provider.type为redis时）
	// 添加到 Redis 消息列表（用于 LLM 上下文）
	providerType := viper.GetString("config_provider.type")
	if providerType == "redis" {
		clientState := event.ClientState
		llm_memory.Get().AddMessage(
			clientState.Ctx,
			clientState.DeviceID,
			clientState.AgentID,
			event.Msg)
		return
	}

	// 确定消息角色
	var role history.MessageType
	switch event.Msg.Role {
	case schema.User:
		role = history.MessageTypeUser
	case schema.Assistant:
		role = history.MessageTypeAssistant
	case schema.Tool:
		role = history.MessageTypeTool
	case schema.System:
		role = history.MessageTypeSystem
	default:
		log.Warnf("不支持的消息角色: %s", event.Msg.Role)
		return
	}

	// 转换音频格式（如果存在）
	var audioBase64 string
	var audioFormat string
	var audioSize int

	if len(event.AudioData) > 0 {
		// ASR 消息：文本和音频同时获取，一次性保存
		var wavData []byte
		var err error

		// 根据消息角色选择不同的音频转换方法
		if event.Msg.Role == schema.User {
			// User 消息（ASR）：PCM float32 格式
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User 消息只有一个元素
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant 消息（TTS）：Opus 格式（理论上不应该在这里，因为 Assistant 是两阶段保存）
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("音频转换失败, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// 降级处理：直接拼接所有帧
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
			audioFormat = "raw" // 降级处理使用原始格式
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
			audioFormat = "wav"
		}
	}

	// 构建 Metadata（只保存时间戳）
	metadata := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}

	// 准备工具调用相关字段
	var toolCallID string
	var toolCallsJSON *string

	// Tool 角色：保存 tool_call_id
	if event.Msg.Role == schema.Tool && event.Msg.ToolCallID != "" {
		toolCallID = event.Msg.ToolCallID
	}

	// Assistant 角色：保存 ToolCalls（如果有）
	if event.Msg.Role == schema.Assistant && len(event.Msg.ToolCalls) > 0 {
		// 序列化 ToolCalls 为 JSON 字符串
		toolCallsBytes, err := json.Marshal(event.Msg.ToolCalls)
		if err != nil {
			log.Warnf("序列化 ToolCalls 失败, device_id: %s, message_id: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, err)
		} else {
			jsonStr := string(toolCallsBytes)
			toolCallsJSON = &jsonStr
		}
	}

	req := &history.SaveMessageRequest{
		MessageID:     event.MessageID,
		DeviceID:      event.ClientState.DeviceID,
		AgentID:       event.ClientState.AgentID,
		SessionID:     event.ClientState.SessionID,
		Role:          role,
		Content:       event.Msg.Content,
		ToolCallID:    toolCallID,
		ToolCallsJSON: toolCallsJSON,
		AudioData:     audioBase64,
		AudioFormat:   audioFormat,
		AudioSize:     audioSize,
		Metadata:      metadata,
	}

	if err := w.client.SaveMessage(ctx, req); err != nil {
		log.Errorf("保存消息失败, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}

// updateMessageAudio 更新消息音频（第二阶段）
func (w *MessageWorker) updateMessageAudio(ctx context.Context, event *eventbus.AddMessageEvent) {
	// 转换音频格式
	var audioBase64 string
	var audioSize int

	if len(event.AudioData) > 0 {
		var wavData []byte
		var err error

		// 根据消息角色选择不同的音频转换方法
		// User 消息（ASR）：PCM float32 格式，使用 PCMFloat32BytesToWav
		// Assistant 消息（TTS）：Opus 格式，使用 OpusFramesToWav
		if event.Msg.Role == schema.User {
			// User 消息：PCM float32 格式
			// event.AudioData 是 [][]byte，但 User 消息只有一个元素（完整的 PCM float32 字节数组）
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User 消息只有一个元素
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant 消息：Opus 格式
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("音频转换失败, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// 降级处理：直接拼接所有帧
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
		}
	}

	// 构建更新请求
	req := &history.UpdateMessageAudioRequest{
		MessageID:   event.MessageID,
		AudioData:   audioBase64,
		AudioFormat: "wav",
		AudioSize:   audioSize,
		Metadata: map[string]interface{}{
			"tts_duration": event.TTSDuration,
		},
	}

	// 调用更新接口
	if err := w.client.UpdateMessageAudio(ctx, req); err != nil {
		log.Errorf("更新消息音频失败, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}
