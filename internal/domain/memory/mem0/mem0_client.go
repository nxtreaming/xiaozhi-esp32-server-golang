package mem0

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/hackers365/mem0-go/client"
	"github.com/hackers365/mem0-go/types"

	log "xiaozhi-esp32-server-golang/logger"
)

// Mem0Client 实现 MemoryProvider 和 EnhancedMemoryProvider 接口
type Mem0Client struct {
	client          *client.MemoryClient
	config          Mem0Config
	mu              sync.RWMutex
	EnableSearch    bool    `mapstructure:"enable_search"`
	SearchThreshold float64 `mapstructure:"search_threshold"`
	SearchTopk      int     `mapstructure:"search_topk"`
}

// Mem0Config 配置结构
type Mem0Config struct {
	APIKey           string `mapstructure:"api_key"`
	BaseUrl          string `mapstructure:"base_url"`
	OrganizationName string `mapstructure:"organization_name"`
	ProjectName      string `mapstructure:"project_name"`
	OrganizationID   string `mapstructure:"organization_id"`
	ProjectID        string `mapstructure:"project_id"`
}

var (
	mem0Instance *Mem0Client
	mem0Once     sync.Once
	configOnce   sync.Once
)

// GetMem0ClientWithConfig 使用配置获取 Mem0 客户端单例
func GetMem0ClientWithConfig(config map[string]interface{}) (*Mem0Client, error) {
	var err error
	configOnce.Do(func() {
		var enableSearch bool = true
		var searchThreshold float64 = 0.5
		var searchTopk int = 3
		// 解析配置到结构体
		var mem0Cfg Mem0Config

		if enableSearchInterface, exists := config["enable_search"]; exists {
			if iEnableSearch, ok := enableSearchInterface.(bool); ok {
				enableSearch = iEnableSearch
			}
		}

		if searchThresholdInterface, exists := config["search_threshold"]; exists {
			if iSearchThreshold, ok := searchThresholdInterface.(float64); ok {
				searchThreshold = iSearchThreshold
			}
		}

		if searchTopkInterface, exists := config["search_topk"]; exists {
			if iSearchTopk, ok := searchTopkInterface.(int); ok {
				searchTopk = iSearchTopk
			}
		}

		// 读取 API Key
		if apiKeyInterface, exists := config["api_key"]; exists {
			if apiKey, ok := apiKeyInterface.(string); ok {
				mem0Cfg.APIKey = apiKey
			} else {
				err = fmt.Errorf("mem0.api_key 必须是字符串")
				return
			}
		}

		// 读取 Host
		if hostInterface, exists := config["base_url"]; exists {
			if host, ok := hostInterface.(string); ok {
				mem0Cfg.BaseUrl = host
			} else {
				err = fmt.Errorf("mem0.host 必须是字符串")
				return
			}
		}

		// 验证必要配置
		if mem0Cfg.APIKey == "" {
			err = fmt.Errorf("mem0.api_key 配置缺失或为空")
			return
		}

		// 设置默认值
		if mem0Cfg.BaseUrl == "" {
			mem0Cfg.BaseUrl = "https://api.mem0.ai"
		}

		// 创建 mem0 客户端
		clientOptions := client.ClientOptions{
			APIKey: mem0Cfg.APIKey,
			/*Host:             mem0Cfg.Host,
			OrganizationName: mem0Cfg.OrganizationName,
			ProjectName:      mem0Cfg.ProjectName,
			OrganizationID:   mem0Cfg.OrganizationID,
			ProjectID:        mem0Cfg.ProjectID,*/
		}

		mem0Client, clientErr := client.NewMemoryClient(clientOptions)
		if clientErr != nil {
			err = fmt.Errorf("failed to create mem0 client: %w", clientErr)
			return
		}

		mem0Instance = &Mem0Client{
			client:          mem0Client,
			config:          mem0Cfg,
			EnableSearch:    enableSearch,
			SearchThreshold: searchThreshold,
			SearchTopk:      searchTopk,
		}

		log.Log().Infof("Mem0 客户端初始化成功, base_url: %s", mem0Cfg.BaseUrl)
	})

	return mem0Instance, err
}

// Init 初始化客户端
func (m *Mem0Client) Init() error {
	// 客户端已在创建时初始化
	log.Log().Info("Mem0 client initialized successfully")
	return nil
}

// Get 获取记忆（内部方法）
func (m *Mem0Client) Get(userID string) (interface{}, error) {
	// 搜索用户的所有记忆
	results, err := m.client.Search("", &types.SearchOptions{
		MemoryOptions: types.MemoryOptions{
			UserID: userID,
		},
		Limit: 100, // 获取更多记忆
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search memories for user %s: %w", userID, err)
	}

	return results, nil
}

// AddMessage 添加消息到记忆
func (m *Mem0Client) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	message := types.Message{
		Role:    string(msg.Role),
		Content: msg.Content,
	}
	// 添加记忆
	_, err := m.client.Add([]types.Message{message}, types.MemoryOptions{
		AgentID:   agentID,
		AsyncMode: true,
	})
	if err != nil {
		return fmt.Errorf("failed to add message to mem0 for user %s: %w", agentID, err)
	}

	log.Log().Debugf("Added message to mem0 for user %s: %s", agentID, message)
	return nil
}

// GetMessages 获取用户的消息历史
func (m *Mem0Client) GetMessages(ctx context.Context, agentID string, count int) ([]*schema.Message, error) {
	var memoryOptions = types.MemoryOptions{
		AgentID: agentID,
	}

	results, err := m.client.GetAll(&types.SearchOptions{
		MemoryOptions: memoryOptions,
		Limit:         count,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages for user %s: %w", agentID, err)
	}

	// 转换为 schema.Message 格式
	var messages []*schema.Message
	for _, result := range results {
		// 从 metadata 中提取 role 和 content
		role := schema.Assistant // 默认角色
		content := result.Memory

		if result.Metadata != nil {
			if r, ok := result.Metadata["role"].(string); ok {
				switch r {
				case "user":
					role = schema.User
				case "assistant":
					role = schema.Assistant
				case "system":
					role = schema.System
				}
			}
			if c, ok := result.Metadata["content"].(string); ok {
				content = c
			}
		}

		messages = append(messages, &schema.Message{
			Role:    role,
			Content: content,
		})
	}

	return messages, nil
}

// ResetMemory 重置用户记忆
func (m *Mem0Client) ResetMemory(ctx context.Context, userID string) error {

	// 删除用户的所有记忆
	err := m.client.DeleteUser(userID)
	if err != nil {
		return fmt.Errorf("failed to reset memory for user %s: %w", userID, err)
	}

	log.Log().Infof("Reset memory for user %s", userID)
	return nil
}

// GetContext 获取上下文（实现 EnhancedMemoryProvider 接口）
func (m *Mem0Client) GetContext(ctx context.Context, agentID string, maxToken int) (string, error) {
	return "", nil
}

func (m *Mem0Client) IsEnableSearch() bool {
	return m.EnableSearch
}

func (m *Mem0Client) Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error) {
	if !m.EnableSearch {
		return "", nil
	}
	topK = m.SearchTopk
	results, err := m.actionSearch(ctx, agentId, query, topK, m.SearchThreshold)
	if err != nil {
		return "", err
	}

	// 构建上下文字符串
	var msgList []string
	for _, result := range results {
		msgList = append(msgList, fmt.Sprintf("- %s [%s]", result.Memory, result.CreatedAt))
	}

	return strings.Join(msgList, "\n"), nil
}

func (m *Mem0Client) Flush(ctx context.Context, agentID string) error {
	return nil
}

func (m *Mem0Client) actionSearch(ctx context.Context, agentID string, query string, topK int, threshold float64) ([]types.Memory, error) {
	// 搜索相关记忆
	results, err := m.client.Search(query, &types.SearchOptions{
		MemoryOptions: types.MemoryOptions{
			AgentID: agentID,
		},
		Limit:     topK,      // 获取topK条记忆
		Threshold: threshold, // 设置相似度阈值
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get context for user %s: %w", agentID, err)
	}

	log.Log().Debugf("成功从mem0获取上下文, agentID: %s, results长度: %d", agentID, len(results))
	return results, nil
}

// AddBatchMessages 批量添加消息
func (m *Mem0Client) AddBatchMessages(ctx context.Context, agentID string, messages []schema.Message) error {

	// 准备批量消息
	var batchMessages []string
	for _, msg := range messages {
		message := fmt.Sprintf("%s: %s", msg.Role, msg.Content)
		batchMessages = append(batchMessages, message)
	}

	// 逐个添加记忆（mem0-go 可能不支持批量添加）
	for _, message := range batchMessages {
		_, err := m.client.Add(message, types.MemoryOptions{
			AgentID: agentID,
			Metadata: map[string]interface{}{
				"source": "xiaozhi-esp32",
				"batch":  true,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to add batch message to mem0 for user %s: %w", agentID, err)
		}
	}

	log.Log().Debugf("Added %d batch messages to mem0 for user %s", len(messages), agentID)
	return nil
}

// Close 关闭客户端
func (m *Mem0Client) Close() error {
	// mem0-go 客户端不需要显式关闭
	log.Log().Info("Mem0 client closed")
	return nil
}

// 确保 Mem0Client 实现了所需的接口
// 注意：这里不能直接引用 memory 包，因为会造成循环导入
// 接口实现会在编译时自动检查
