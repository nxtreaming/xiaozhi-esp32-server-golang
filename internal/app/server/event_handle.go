package server

import (
	"fmt"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/memory/llm_memory"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/panjf2000/ants/v2"
)

type EventHandle struct {
	addMessagePool *ants.Pool
	sessionEndPool *ants.Pool
}

func NewEventHandle() (*EventHandle, error) {
	// 创建 AddMessage 处理池（10 个 worker）
	addMessagePool, err := ants.NewPool(10)
	if err != nil {
		return nil, fmt.Errorf("创建 AddMessage 处理池失败: %w", err)
	}

	// 创建 SessionEnd 处理池（10 个 worker）
	sessionEndPool, err := ants.NewPool(10)
	if err != nil {
		addMessagePool.Release()
		return nil, fmt.Errorf("创建 SessionEnd 处理池失败: %w", err)
	}

	return &EventHandle{
		addMessagePool: addMessagePool,
		sessionEndPool: sessionEndPool,
	}, nil
}

func (s *EventHandle) Start() error {
	go s.HandleAddMessage()
	go s.HandleSessionEnd()
	return nil
}

func (s *EventHandle) HandleAddMessage() {
	// 订阅统一的事件（与 HistoryWorker 监听同一个 Topic）
	eventbus.Get().Subscribe(eventbus.TopicAddMessage, func(event *eventbus.AddMessageEvent) {
		// 使用闭包捕获 event，避免并发问题
		eventCopy := event
		err := s.addMessagePool.Submit(func() {
			clientState := eventCopy.ClientState

			// 只处理第一阶段：保存到 Redis 记忆体（仅文本）
			// 第二阶段（IsUpdate=true）不需要更新 Redis，因为 Redis 记忆体不需要音频
			if !eventCopy.IsUpdate {
				// 添加到 Redis 消息列表（用于 LLM 上下文）
				llm_memory.Get().AddMessage(
					clientState.Ctx,
					clientState.DeviceID,
					clientState.AgentID,
					eventCopy.Msg)

				// 将消息加到长期记忆体（memobase/mem0）
				if clientState.MemoryProvider != nil {
					err := clientState.MemoryProvider.AddMessage(
						clientState.Ctx,
						clientState.GetDeviceIDOrAgentID(),
						eventCopy.Msg)
					if err != nil {
						log.Errorf("add message to memory provider failed: %v", err)
					}
				}
			}
			// 第二阶段（IsUpdate=true）：不更新 Redis 记忆体，因为不需要音频
		})
		if err != nil {
			log.Errorf("提交 AddMessage 任务失败: %v", err)
		}
	})
}

func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}
		if clientState.MemoryProvider == nil {
			return
		}
		log.Infof("HandleSessionEnd: deviceId: %s", clientState.DeviceID)

		// 使用闭包捕获 clientState，避免并发问题
		clientStateCopy := clientState
		err := s.sessionEndPool.Submit(func() {
			// 将消息加到长期记忆体中
			if clientStateCopy.MemoryProvider != nil {
				err := clientStateCopy.MemoryProvider.Flush(
					clientStateCopy.Ctx,
					clientStateCopy.GetDeviceIDOrAgentID())
				if err != nil {
					log.Errorf("flush message to memory provider failed: %v", err)
				}
			}
		})
		if err != nil {
			log.Errorf("提交 SessionEnd 任务失败: %v", err)
		}
	})
	return nil
}
