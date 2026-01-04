package history

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
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
	BaseURL     string        // Manager后端地址
	AuthToken   string        // 认证Token
	Timeout     time.Duration // 请求超时
	Enabled     bool          // 是否启用
	EnableAudio bool          // 是否保存音频
}

// HistoryClient 聊天历史HTTP客户端
type HistoryClient struct {
	httpClient  *http.Client
	baseURL     string
	authToken   string
	enabled     bool
	enableAudio bool // 是否启用音频保存
}

// NewHistoryClient 创建聊天历史客户端
func NewHistoryClient(cfg HistoryClientConfig) *HistoryClient {
	return &HistoryClient{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:     cfg.BaseURL,
		authToken:   cfg.AuthToken,
		enabled:     cfg.Enabled,
		enableAudio: cfg.EnableAudio,
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
	AudioData     string                 `json:"audio_data,omitempty"` // base64编码
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
	return c.doRequest(ctx, "POST", "/api/internal/history/messages", req, nil)
}

// doRequest 执行HTTP请求，带重试机制
func (c *HistoryClient) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	operation := func() error {
		var bodyReader io.Reader
		if reqBody != nil {
			data, err := json.Marshal(reqBody)
			if err != nil {
				return err
			}
			bodyReader = bytes.NewReader(data)
		}

		url := c.baseURL + path
		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		if respBody != nil {
			return json.NewDecoder(resp.Body).Decode(respBody)
		}
		return nil
	}

	backoffCfg := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3)
	return backoff.Retry(operation, backoffCfg)
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
	// 检查是否启用音频保存
	if !c.enableAudio {
		return nil
	}
	return c.doRequest(ctx, "PUT", "/api/internal/history/messages/"+req.MessageID+"/audio", req, nil)
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

	// GET 请求需要将参数放在 URL 中（使用 url.Values 进行编码）
	params := url.Values{}
	params.Set("device_id", req.DeviceID)
	params.Set("agent_id", req.AgentID)
	params.Set("limit", fmt.Sprintf("%d", req.Limit))
	if req.SessionID != "" {
		params.Set("session_id", req.SessionID)
	}
	path := "/api/internal/history/messages?" + params.Encode()

	var resp GetMessagesResponse
	err := c.doRequest(ctx, "GET", path, nil, &resp)
	return &resp, err
}
