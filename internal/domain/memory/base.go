package memory

import (
	"context"
	"fmt"

	"xiaozhi-esp32-server-golang/internal/domain/memory/mem0"
	"xiaozhi-esp32-server-golang/internal/domain/memory/memobase"
	"xiaozhi-esp32-server-golang/internal/domain/memory/nomemo"

	"github.com/cloudwego/eino/schema"
)

// MemoryProvider 记忆提供者接口
// 定义所有记忆提供者都需要实现的核心方法
type MemoryProvider interface {
	// AddMessage 添加一条消息到记忆
	AddMessage(ctx context.Context, agentID string, msg schema.Message) error

	// GetMessages 获取用户的历史消息
	GetMessages(ctx context.Context, agentId string, count int) ([]*schema.Message, error)

	// GetContext 获取用户的上下文信息，用于增强 LLM prompt
	GetContext(ctx context.Context, agentId string, maxToken int) (string, error)

	// Search 搜索用户的记忆
	Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error)

	// Flush 刷新用户的记忆
	Flush(ctx context.Context, agentId string) error

	// ResetMemory 重置用户的记忆
	ResetMemory(ctx context.Context, agentId string) error
}

// MemoryType 记忆类型
type MemoryType string

const (
	MemoryTypeNone     MemoryType = "nomemo"
	MemoryTypeMemobase MemoryType = "memobase" // Memobase 长期记忆
	MemoryTypeMem0     MemoryType = "mem0"     // Mem0 记忆服务
)

// GetProvider 获取指定类型的记忆提供者
func GetProvider(memoryType MemoryType, config map[string]interface{}) (MemoryProvider, error) {
	return GetProviderByType(memoryType, config)
}

// GetProviderByType 根据类型获取记忆提供者
func GetProviderByType(memoryType MemoryType, config map[string]interface{}) (MemoryProvider, error) {
	if memoryType == "" {
		memoryType = MemoryTypeNone
	}
	switch memoryType {
	case MemoryTypeNone:
		return nomemo.Get(), nil
	case MemoryTypeMemobase:
		return memobase.GetWithConfig(config)
	case MemoryTypeMem0:
		return mem0.GetMem0ClientWithConfig(config)
	default:
		return nil, fmt.Errorf("unsupported memory type: %v", memoryType)
	}
}
