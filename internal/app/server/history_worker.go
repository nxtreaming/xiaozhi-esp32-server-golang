package server

import (
	"context"
	"encoding/base64"
	"time"

	"xiaozhi-esp32-server-golang/internal/data/history"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/panjf2000/ants/v2"
)

// HistoryWorker 聊天历史记录处理器
type HistoryWorker struct {
	client *history.HistoryClient
	pool   *ants.Pool
}

// NewHistoryWorker 创建历史记录处理器
func NewHistoryWorker(cfg history.HistoryClientConfig) *HistoryWorker {
	client := history.NewHistoryClient(cfg)
	pool, _ := ants.NewPool(50)

	worker := &HistoryWorker{
		client: client,
		pool:   pool,
	}

	worker.subscribeEvents()
	return worker
}

// subscribeEvents 订阅EventBus事件
func (w *HistoryWorker) subscribeEvents() {
	bus := eventbus.Get()
	// 订阅统一的消息添加事件（与 EventHandle 监听同一个 Topic）
	bus.Subscribe(eventbus.TopicAddMessage, w.handleAddMessage)
}

// handleAddMessage 统一处理消息添加事件
func (w *HistoryWorker) handleAddMessage(event *eventbus.AddMessageEvent) {
	w.pool.Submit(func() {
		ctx, cancel := context.WithTimeout(event.ClientState.Ctx, 5*time.Second)
		defer cancel()

		// 判断是新增还是更新
		if event.IsUpdate {
			// 第二阶段：更新音频
			w.updateMessageAudio(ctx, event)
		} else {
			// 第一阶段：保存文本消息
			w.saveMessageText(ctx, event)
		}
	})
}

// saveMessageText 保存文本消息（第一阶段，或一次性保存文本+音频）
func (w *HistoryWorker) saveMessageText(ctx context.Context, event *eventbus.AddMessageEvent) {
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

	// 构建 Metadata
	metadata := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}
	// Tool 角色需要将 ToolCallID 存储到 Metadata 中
	if event.Msg.Role == schema.Tool && event.Msg.ToolCallID != "" {
		metadata["tool_call_id"] = event.Msg.ToolCallID
	}

	req := &history.SaveMessageRequest{
		MessageID:   event.MessageID,
		DeviceID:    event.ClientState.DeviceID,
		AgentID:     event.ClientState.AgentID,
		SessionID:   event.ClientState.SessionID,
		Role:        role,
		Content:     event.Msg.Content,
		AudioData:   audioBase64,
		AudioFormat: audioFormat,
		AudioSize:   audioSize,
		Metadata:    metadata,
	}

	if err := w.client.SaveMessage(ctx, req); err != nil {
		log.Errorf("保存消息失败, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}

// updateMessageAudio 更新消息音频（第二阶段）
func (w *HistoryWorker) updateMessageAudio(ctx context.Context, event *eventbus.AddMessageEvent) {
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
