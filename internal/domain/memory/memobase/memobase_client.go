package memobase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/memodb-io/memobase/src/client/memobase-go/blob"
	"github.com/memodb-io/memobase/src/client/memobase-go/core"
)

var (
	clientInstance *MemobaseClient
	once           sync.Once
	configOnce     sync.Once
	// 使用固定的命名空间UUID，用于生成设备ID的UUID v5
	// 这样同一个设备ID总是映射到相同的UUID
	deviceNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // DNS命名空间
)

// MemobaseClient Memobase客户端管理器
type MemobaseClient struct {
	client *core.MemoBaseClient
	users  sync.Map // 缓存用户对象
	sync.RWMutex
	EnableSearch    bool
	SearchThreshold float64
	SearchTopk      int
}

// GetWithConfig 使用配置获取Memobase客户端实例（单例模式）
func GetWithConfig(config map[string]interface{}) (*MemobaseClient, error) {
	var initErr error
	configOnce.Do(func() {
		iClient := &MemobaseClient{
			users: sync.Map{},
		}
		// 从配置中读取 memobase 相关配置		// 读取必要的配置项
		projectUrlInterface, ok := config["base_url"]
		if !ok {
			initErr = fmt.Errorf("memobase.base_url 配置缺失")
			return
		}
		baseUrl, ok := projectUrlInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.base_url 必须是字符串")
			return
		}

		apiKeyInterface, ok := config["api_key"]
		if !ok {
			initErr = fmt.Errorf("memobase.api_key 配置缺失")
			return
		}
		apiKey, ok := apiKeyInterface.(string)
		if !ok {
			initErr = fmt.Errorf("memobase.api_key 必须是字符串")
			return
		}

		if baseUrl == "" || apiKey == "" {
			initErr = fmt.Errorf("Memobase 配置不完整: base_url 或 api_key 为空")
			log.Log().Errorf("Memobase 初始化失败: %v", initErr)
			return
		}

		// 读取可选的搜索配置
		enableSearchInterface, ok := config["enable_search"]
		if ok {
			enableSearch, ok := enableSearchInterface.(bool)
			if ok {
				iClient.EnableSearch = enableSearch
			}
		}

		thresholdInterface, ok := config["search_threshold"]
		if ok {
			threshold, ok := thresholdInterface.(float64)
			if ok {
				iClient.SearchThreshold = threshold
			}
		}

		topKInterface, ok := config["search_topk"]
		if ok {
			topK, ok := topKInterface.(int)
			if ok {
				iClient.SearchTopk = topK
			}
		}

		// 创建客户端
		client, err := core.NewMemoBaseClient(baseUrl, apiKey)
		if err != nil {
			initErr = fmt.Errorf("创建 Memobase 客户端失败: %v", err)
			log.Log().Errorf("Memobase 初始化失败: %v", initErr)
			return
		}

		iClient.client = client
		clientInstance = iClient

		log.Log().Infof("Memobase 客户端初始化成功, project_url: %s", baseUrl)
	})

	if initErr != nil {
		return nil, initErr
	}
	return clientInstance, nil
}

// deviceIDToUUID 将设备ID转换为UUID v5格式
// 使用UUID v5确保同一个设备ID总是生成相同的UUID
func deviceIDToUUID(deviceID string) string {
	return uuid.NewSHA1(deviceNamespace, []byte(deviceID)).String()
}

func IsEnableSearch() bool {
	return clientInstance.EnableSearch
}

// AddMessage 添加消息到Memobase
func (m *MemobaseClient) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	memobaseUserID := deviceIDToUUID(agentID)
	// 构建消息
	messages := []blob.OpenAICompatibleMessage{
		{
			Role:    string(msg.Role),
			Content: msg.Content,
		},
	}

	// 如果有工具调用，添加到消息中
	if len(msg.ToolCalls) > 0 {
		for _, toolCall := range msg.ToolCalls {
			messages = append(messages, blob.OpenAICompatibleMessage{
				Role:    "tool",
				Content: fmt.Sprintf("Tool: %s, Args: %v", toolCall.Function.Name, toolCall.Function.Arguments),
			})
		}
	}

	// 创建ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: messages,
	}

	// 获取或创建用户实例（使用UUID格式的userID）
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("获取或创建用户失败, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("获取或创建用户失败: %v", err)
	}

	// 插入消息（异步）
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("添加消息到Memobase失败, deviceID: %s, error: %v", agentID, err)
		return fmt.Errorf("添加消息到Memobase失败: %v", err)
	}

	//user.Flush(blob.ChatType, false)

	log.Log().Debugf("成功添加消息到Memobase, deviceID: %s, blobID: %s", agentID, blobID)
	return nil
}

func (m *MemobaseClient) Flush(ctx context.Context, agentID string) error {
	memobaseUserID := deviceIDToUUID(agentID)
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("刷新用户记忆失败, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return fmt.Errorf("刷新用户记忆失败: %v", err)
	}
	user.Flush(blob.ChatType, false)
	return nil
}

// GetContext 获取用户上下文
func (m *MemobaseClient) GetContext(ctx context.Context, agentID string, maxToken int) (string, error) {

	// 将设备ID转换为UUID格式（Memobase要求）
	memobaseUserID := deviceIDToUUID(agentID)

	// 获取用户实例（不执行 HTTP GET 请求，只创建实例）
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("获取用户实例失败, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("获取用户实例失败: %v", err)
	}

	// 获取上下文，使用默认选项
	context, err := user.Context(&core.ContextOptions{
		MaxTokenSize: maxToken,
	})
	if err != nil {
		log.Log().Errorf("从Memobase获取上下文失败, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("从Memobase获取上下文失败: %v", err)
	}

	log.Log().Debugf("成功从Memobase获取上下文, agentID: %s, context长度: %d", agentID, len(context))
	return context, nil
}

func (m *MemobaseClient) Search(ctx context.Context, agentID string, query string, topK int, timeRangeDays int64) (string, error) {
	if !m.EnableSearch {
		return "", nil
	}
	topK = m.SearchTopk
	// 将设备ID转换为UUID格式（Memobase要求）
	memobaseUserID := deviceIDToUUID(agentID)

	// 获取用户实例（不执行 HTTP GET 请求，只创建实例）
	user, err := m.getUser(memobaseUserID)
	if err != nil {
		log.Log().Errorf("获取用户实例失败, agentID: %s, memobaseUserID: %s, error: %v", agentID, memobaseUserID, err)
		return "", fmt.Errorf("获取用户实例失败: %v", err)
	}

	topK = 2

	// 搜索event
	userEventList, err := user.SearchEvent(query, topK, 0.2, int(timeRangeDays))
	if err != nil {
		log.Log().Errorf("从Memobase搜索事件失败, agentID: %s, error: %v", agentID, err)
		return "", fmt.Errorf("从Memobase搜索事件失败: %v", err)
	}

	var eventList []string
	for _, event := range userEventList {
		eventList = append(eventList, fmt.Sprintf("- %s: %s", event.CreatedAt, event.EventData.EventTip))
	}

	// 转换为字符串
	userEventStr := strings.Join(eventList, "\n")

	log.Log().Debugf("成功从Memobase搜索事件, agentID: %s, event数量: %d", agentID, len(eventList))
	return userEventStr, nil
}

// AddBatchMessages 批量添加消息到Memobase
func (m *MemobaseClient) AddBatchMessages(ctx context.Context, userID string, messages []schema.Message) error {
	m.Lock()
	defer m.Unlock()

	if len(messages) == 0 {
		return nil
	}

	// 转换消息格式
	blobMessages := make([]blob.OpenAICompatibleMessage, 0, len(messages))
	for _, msg := range messages {
		blobMessages = append(blobMessages, blob.OpenAICompatibleMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// 创建ChatBlob
	chatBlob := &blob.ChatBlob{
		BaseBlob: blob.BaseBlob{
			Type: blob.ChatType,
		},
		Messages: blobMessages,
	}

	// 将设备ID转换为UUID格式（Memobase要求）
	memobaseUserID := deviceIDToUUID(userID)

	// 获取或创建用户实例（使用UUID格式的userID）
	user, err := m.getUser(userID)
	if err != nil {
		log.Log().Errorf("批量添加消息: 获取或创建用户失败, deviceID: %s, memobaseUserID: %s, error: %v", userID, memobaseUserID, err)
		return fmt.Errorf("获取或创建用户失败: %v", err)
	}

	// 插入消息（异步）
	blobID, err := user.Insert(chatBlob, false)
	if err != nil {
		log.Log().Errorf("批量添加消息到Memobase失败, deviceID: %s, error: %v", userID, err)
		return fmt.Errorf("批量添加消息到Memobase失败: %v", err)
	}

	log.Log().Debugf("成功批量添加 %d 条消息到Memobase, deviceID: %s, blobID: %s", len(messages), userID, blobID)
	return nil
}

// GetMessages 获取用户的历史消息
// 实现 BaseMemoryProvider 接口
// 注意：Memobase 主要用于长期记忆和上下文增强，不提供历史消息检索功能
func (m *MemobaseClient) GetMessages(ctx context.Context, agentID string, count int) ([]*schema.Message, error) {
	return []*schema.Message{}, nil
}

// ResetMemory 重置用户的记忆
// 实现 MemoryProvider 接口
// 注意：Memobase 的记忆重置需要通过 API 删除用户数据
func (m *MemobaseClient) ResetMemory(ctx context.Context, userID string) error {
	// TODO: 如果 Memobase SDK 提供了删除用户数据的接口，在这里调用
	// 目前返回 nil 表示操作成功（即使没有实际删除）
	log.Log().Infof("Memobase 重置记忆请求: userID=%s (注意: Memobase 不支持直接重置)", userID)
	return nil
}

// Close 关闭客户端（如果需要）
func (m *MemobaseClient) Close() error {
	log.Log().Info("Memobase 客户端已关闭")
	return nil
}

// todo 加user对象缓存
func (m *MemobaseClient) getUser(userID string) (*core.User, error) {
	if user, ok := m.users.Load(userID); ok {
		return user.(*core.User), nil
	}

	memobaseUserID := deviceIDToUUID(userID)
	user, err := m.client.GetOrCreateUser(memobaseUserID)
	if err != nil {
		return nil, fmt.Errorf("获取用户实例失败: %v", err)
	}

	m.users.Store(userID, user)
	return user, nil
}
