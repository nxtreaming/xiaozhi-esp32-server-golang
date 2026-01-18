package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/memory/llm_memory"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// redisWorkerNum Redis消息处理worker数量，必须是2的幂次以便hash分布
	redisWorkerNum = 16
	// sessionEndWorkerNum SessionEnd处理worker数量，必须是2的幂次以便hash分布
	sessionEndWorkerNum = 16
)

type EventHandle struct {
	// Redis消息处理workers（按DeviceID分片，保证顺序）
	redisWorkers []chan *eventbus.AddMessageEvent
	redisCtx     context.Context
	redisCancel  context.CancelFunc
	redisWg      sync.WaitGroup

	// SessionEnd处理workers（按DeviceID分片，保证顺序）
	sessionEndWorkers []chan *ClientState
	sessionEndCtx     context.Context
	sessionEndCancel  context.CancelFunc
	sessionEndWg      sync.WaitGroup
}

func NewEventHandle() (*EventHandle, error) {
	redisCtx, redisCancel := context.WithCancel(context.Background())
	sessionEndCtx, sessionEndCancel := context.WithCancel(context.Background())

	handle := &EventHandle{
		redisWorkers:      make([]chan *eventbus.AddMessageEvent, redisWorkerNum),
		redisCtx:          redisCtx,
		redisCancel:       redisCancel,
		sessionEndWorkers: make([]chan *ClientState, sessionEndWorkerNum),
		sessionEndCtx:     sessionEndCtx,
		sessionEndCancel:  sessionEndCancel,
	}

	// 初始化每个Redis worker的channel并启动goroutine
	for i := 0; i < redisWorkerNum; i++ {
		handle.redisWorkers[i] = make(chan *eventbus.AddMessageEvent, 100) // 缓冲100个消息
		handle.redisWg.Add(1)
		go handle.redisWorkerLoop(i)
	}

	// 初始化每个SessionEnd worker的channel并启动goroutine
	for i := 0; i < sessionEndWorkerNum; i++ {
		handle.sessionEndWorkers[i] = make(chan *ClientState, 100) // 缓冲100个消息
		handle.sessionEndWg.Add(1)
		go handle.sessionEndWorkerLoop(i)
	}

	log.Infof("EventHandle初始化完成，启动 %d 个Redis worker goroutine, %d 个SessionEnd worker goroutine", redisWorkerNum, sessionEndWorkerNum)
	return handle, nil
}

func (s *EventHandle) Start() error {
	go s.HandleAddMessage()
	go s.HandleSessionEnd()
	return nil
}

// redisWorkerLoop 每个Redis worker的处理循环（保证顺序处理）
func (s *EventHandle) redisWorkerLoop(index int) {
	defer s.redisWg.Done()
	defer log.Infof("EventHandle Redis worker %d 退出", index)

	ch := s.redisWorkers[index]
	for {
		select {
		case <-s.redisCtx.Done():
			// 清理channel中的剩余消息
			for {
				select {
				case event := <-ch:
					if event != nil {
						s.processRedisMessage(event)
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
				s.processRedisMessage(event)
			}
		}
	}
}

// hashDeviceID 计算DeviceID的hash值，返回worker索引
func (s *EventHandle) hashDeviceID(deviceID string) int {
	if deviceID == "" {
		return 0 // 如果DeviceID为空，使用第一个worker
	}

	// 使用FNV-1a哈希函数
	h := fnv.New32a()
	h.Write([]byte(deviceID))
	hash := h.Sum32()
	return int(hash) % redisWorkerNum
}

// processRedisMessage 处理Redis消息（在worker goroutine中顺序执行）
func (s *EventHandle) processRedisMessage(event *eventbus.AddMessageEvent) {
	clientState := event.ClientState

	// 只处理第一阶段：保存到 Redis 记忆体（仅文本）
	// 第二阶段（IsUpdate=true）不需要更新 Redis，因为 Redis 记忆体不需要音频
	if !event.IsUpdate {
		// 添加到 Redis 消息列表（用于 LLM 上下文）
		llm_memory.Get().AddMessage(
			clientState.Ctx,
			clientState.DeviceID,
			clientState.AgentID,
			event.Msg)

		// 将消息加到长期记忆体（memobase/mem0）
		if clientState.MemoryProvider != nil {
			err := clientState.MemoryProvider.AddMessage(
				clientState.Ctx,
				clientState.GetDeviceIDOrAgentID(),
				event.Msg)
			if err != nil {
				log.Errorf("add message to memory provider failed: %v", err)
			}
		}
	}
	// 第二阶段（IsUpdate=true）：不更新 Redis 记忆体，因为不需要音频
}

func (s *EventHandle) HandleAddMessage() {
	// 订阅统一的事件（与 HistoryWorker 监听同一个 Topic）
	eventbus.Get().Subscribe(eventbus.TopicAddMessage, func(event *eventbus.AddMessageEvent) {
		if event == nil || event.ClientState == nil {
			return
		}

		// 只处理需要保存到Redis的消息（第一阶段，非更新）
		if !event.IsUpdate {
			// 使用DeviceID进行hash路由，保证同一设备的消息顺序处理
			deviceID := event.ClientState.DeviceID
			if deviceID == "" {
				log.Warnf("DeviceID为空，无法路由消息到Redis worker")
				return
			}

			// 计算hash值，路由到对应的worker
			workerIndex := s.hashDeviceID(deviceID)

			// 非阻塞发送到对应的worker channel
			select {
			case s.redisWorkers[workerIndex] <- event:
				// 成功发送
			default:
				// channel已满，记录警告（通常不会发生，因为channel有缓冲）
				log.Warnf("Redis worker %d 的channel已满，丢弃消息, device_id: %s",
					workerIndex, deviceID)
			}
		}
	})
}

// sessionEndWorkerLoop 每个SessionEnd worker的处理循环（保证顺序处理）
func (s *EventHandle) sessionEndWorkerLoop(index int) {
	defer s.sessionEndWg.Done()
	defer log.Infof("EventHandle SessionEnd worker %d 退出", index)

	ch := s.sessionEndWorkers[index]
	for {
		select {
		case <-s.sessionEndCtx.Done():
			// 清理channel中的剩余消息
			for {
				select {
				case clientState := <-ch:
					if clientState != nil {
						s.processSessionEnd(clientState)
					}
				default:
					return
				}
			}
		case clientState, ok := <-ch:
			if !ok {
				// channel已关闭
				return
			}
			if clientState != nil {
				s.processSessionEnd(clientState)
			}
		}
	}
}

// hashDeviceIDForSessionEnd 计算DeviceID的hash值，返回worker索引（用于SessionEnd）
func (s *EventHandle) hashDeviceIDForSessionEnd(deviceID string) int {
	if deviceID == "" {
		return 0 // 如果DeviceID为空，使用第一个worker
	}

	// 使用FNV-1a哈希函数
	h := fnv.New32a()
	h.Write([]byte(deviceID))
	hash := h.Sum32()
	return int(hash) % sessionEndWorkerNum
}

// processSessionEnd 处理SessionEnd消息（在worker goroutine中顺序执行）
func (s *EventHandle) processSessionEnd(clientState *ClientState) {
	if clientState.MemoryProvider == nil {
		return
	}

	log.Infof("HandleSessionEnd: deviceId: %s", clientState.DeviceID)

	// 将消息加到长期记忆体中
	err := clientState.MemoryProvider.Flush(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID())
	if err != nil {
		log.Errorf("flush message to memory provider failed: %v", err)
	}
}

func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}

		// 使用DeviceID进行hash路由，保证同一设备的消息顺序处理
		deviceID := clientState.DeviceID
		if deviceID == "" {
			log.Warnf("DeviceID为空，无法路由SessionEnd消息到worker")
			return
		}

		// 计算hash值，路由到对应的worker
		workerIndex := s.hashDeviceIDForSessionEnd(deviceID)

		// 非阻塞发送到对应的worker channel
		select {
		case s.sessionEndWorkers[workerIndex] <- clientState:
			// 成功发送
		default:
			// channel已满，记录警告（通常不会发生，因为channel有缓冲）
			log.Warnf("SessionEnd worker %d 的channel已满，丢弃消息, device_id: %s",
				workerIndex, deviceID)
		}
	})
	return nil
}

// Close 关闭EventHandle，优雅关闭所有worker
func (s *EventHandle) Close() {
	// 关闭Redis workers
	s.redisCancel()
	s.redisWg.Wait()

	// 关闭所有Redis worker channels
	for i := 0; i < redisWorkerNum; i++ {
		close(s.redisWorkers[i])
	}

	// 关闭SessionEnd workers
	s.sessionEndCancel()
	s.sessionEndWg.Wait()

	// 关闭所有SessionEnd worker channels
	for i := 0; i < sessionEndWorkerNum; i++ {
		close(s.sessionEndWorkers[i])
	}

	log.Info("EventHandle已关闭")
}
