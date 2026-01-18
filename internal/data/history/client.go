package history

import (
	"context"
	"fmt"
	"time"

	"xiaozhi-esp32-server-golang/internal/components/http"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeTool      MessageType = "tool"   // 工具调用结果
	MessageTypeSystem    MessageType = "system" // 系统消息（如果使用）
)

// HistoryClientConfig 客户端配置
type HistoryClientConfig struct {
	BaseURL   string        // Manager后端地址
	AuthToken string        // 认证Token
	Timeout   time.Duration // 请求超时
	Enabled   bool          // 是否启用
}

// HistoryClient 聊天历史HTTP客户端
type HistoryClient struct {
	client  *http.ManagerClient
	enabled bool
}

// NewHistoryClient 创建聊天历史客户端
func NewHistoryClient(cfg HistoryClientConfig) *HistoryClient {
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    cfg.BaseURL,
		AuthToken:  cfg.AuthToken,
		Timeout:    cfg.Timeout,
		MaxRetries: 3, // 默认重试3次
	})

	return &HistoryClient{
		client:  managerClient,
		enabled: cfg.Enabled,
	}
}

// SaveMessageRequest 保存消息请求
type SaveMessageRequest struct {
	MessageID     string                 `json:"message_id"`
	DeviceID      string                 `json:"device_id"`
	AgentID       string                 `json:"agent_id"`
	SessionID     string                 `json:"session_id,omitempty"`
	Role          MessageType            `json:"role"`
	Content       string                 `json:"content"`
	ToolCallID    string  `json:"tool_call_id,omitempty"`    // 工具调用ID（Tool角色使用）
	ToolCallsJSON *string `json:"tool_calls_json,omitempty"` // 工具调用列表JSON（Assistant角色使用），nil 表示 NULL
	AudioData     string                 `json:"audio_data,omitempty"`      // base64编码
	AudioFormat   string                 `json:"audio_format,omitempty"`
	AudioDuration int                    `json:"audio_duration,omitempty"`
	AudioSize     int                    `json:"audio_size,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SaveMessage 保存消息
func (c *HistoryClient) SaveMessage(ctx context.Context, req *SaveMessageRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   "/api/internal/history/messages",
		Body:   req,
	})
}

// UpdateMessageAudioRequest 更新消息音频请求
type UpdateMessageAudioRequest struct {
	MessageID   string                 `json:"message_id"`
	AudioData   string                 `json:"audio_data"` // base64编码
	AudioFormat string                 `json:"audio_format"`
	AudioSize   int                    `json:"audio_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageAudio 更新消息音频
func (c *HistoryClient) UpdateMessageAudio(ctx context.Context, req *UpdateMessageAudioRequest) error {
	if !c.enabled {
		return nil
	}
	return c.client.DoRequest(ctx, http.RequestOptions{
		Method: "PUT",
		Path:   "/api/internal/history/messages/" + req.MessageID + "/audio",
		Body:   req,
	})
}

// GetMessagesRequest 获取消息请求
type GetMessagesRequest struct {
	DeviceID  string `json:"device_id"`
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id,omitempty"`
	Limit     int    `json:"limit"` // 限制数量
}

// GetMessagesResponse 获取消息响应
type GetMessagesResponse struct {
	Messages []MessageItem `json:"messages"`
}

// MessageItem 消息项（用于初始化加载，不包含音频）
type MessageItem struct {
	MessageID  string `json:"message_id"`
	Role       string `json:"role"` // user/assistant/tool/system
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"` // Tool 角色使用
	CreatedAt  string `json:"created_at"`
}

// GetMessages 从 Manager 数据库获取消息（用于初始化加载）
func (c *HistoryClient) GetMessages(ctx context.Context, req *GetMessagesRequest) (*GetMessagesResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("history client is disabled")
	}

	// 构建查询参数
	queryParams := map[string]string{
		"device_id": req.DeviceID,
		"agent_id":  req.AgentID,
		"limit":     fmt.Sprintf("%d", req.Limit),
	}
	if req.SessionID != "" {
		queryParams["session_id"] = req.SessionID
	}

	var resp GetMessagesResponse
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:      "GET",
		Path:        "/api/internal/history/messages",
		QueryParams: queryParams,
		Response:    &resp,
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
