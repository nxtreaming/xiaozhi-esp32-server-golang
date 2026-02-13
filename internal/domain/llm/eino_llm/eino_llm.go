package eino_llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	log "xiaozhi-esp32-server-golang/logger"
)

// EinoLLMProvider 基于Eino框架的LLM提供者
// 直接使用Eino的ChatModel接口和类型，支持openai和ollama
type EinoLLMProvider struct {
	chatModel    model.ToolCallingChatModel
	modelName    string
	maxTokens    int
	streamable   bool
	config       map[string]interface{}
	providerType string // "openai" 或 "ollama"
}

// EinoConfig Eino LLM配置
type EinoConfig struct {
	Type       string                 `json:"type"` // "openai" 或 "ollama"
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Streamable bool                   `json:"streamable,omitempty"`
}

// 连接池配置
const (
	maxIdleConns          = 200
	maxIdleConnsPerHost   = 50
	idleConnTimeout       = 90 * time.Second
	dialTimeout           = 30 * time.Second
	keepAliveTimeout      = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 60 * time.Second
)

// 全局HTTP客户端，用于所有OpenAI请求
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getHTTPClient 返回配置了连接池的HTTP客户端
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAliveTimeout,
			}).DialContext,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
		}

		httpClient = &http.Client{
			Transport: transport,
			// 流式输出场景不要用 http.Client.Timeout 截断整个连接，改由 ctx 控制请求生命周期。
			Timeout: 0,
		}
	})

	return httpClient
}

// NewEinoLLMProvider 创建新的Eino LLM提供者，根据type支持openai和ollama
func NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error) {
	//log.Debugf("NewEinoLLMProvider config: %+v", config)
	providerType, _ := config["type"].(string)
	if providerType == "" {
		return nil, fmt.Errorf("type不能为空，必须是 'openai' 或 'ollama'")
	}

	modelName, _ := config["model_name"].(string)
	if modelName == "" {
		return nil, fmt.Errorf("model_name不能为空")
	}

	maxTokens := 500
	if mt, ok := config["max_tokens"].(int); ok {
		maxTokens = mt
	}

	streamable := true
	if s, ok := config["streamable"].(bool); ok {
		streamable = s
	}

	var chatModel model.ToolCallingChatModel
	var err error

	// 根据类型创建不同的ChatModel实现
	switch providerType {
	case "openai":
		chatModel, err = createOpenAIChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("创建OpenAI ChatModel失败: %v", err)
		}
	case "ollama":
		chatModel, err = createOllamaChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("创建Ollama ChatModel失败: %v", err)
		}
	default:
		return nil, fmt.Errorf("不支持的模型类型: %s", providerType)
	}

	provider := &EinoLLMProvider{
		chatModel:    chatModel,
		modelName:    modelName,
		maxTokens:    maxTokens,
		streamable:   streamable,
		config:       config,
		providerType: providerType,
	}

	return provider, nil
}

// createOpenAIChatModel 创建OpenAI的ChatModel实现
func createOpenAIChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
	}

	apiKey, _ := config["api_key"].(string)
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	baseURL, _ := config["base_url"].(string)

	// 创建OpenAI ChatModel配置
	openaiConfig := &openai.ChatModelConfig{
		Model:      modelName,
		APIKey:     apiKey,
		HTTPClient: getHTTPClient(),
	}

	if baseURL != "" {
		openaiConfig.BaseURL = baseURL
	}

	log.Debugf("openaiConfig: %+v", openaiConfig)

	// 使用eino-ext官方OpenAI实现
	chatModel, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建OpenAI ChatModel失败: %v", err)
	}

	log.Infof("成功创建OpenAI ChatModel，模型: %s", modelName)
	return chatModel, nil
}

// createOllamaChatModel 创建Ollama的ChatModel实现
func createOllamaChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	baseURL, _ := config["base_url"].(string)

	if modelName == "" || baseURL == "" {
		log.Warnf("model_name和base_url不能为空，使用默认模型: %s", modelName)
		return nil, fmt.Errorf("model_name和base_url不能为空")
	}

	// 创建Ollama ChatModel配置
	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
	}

	// 使用eino-ext官方Ollama实现
	chatModel, err := ollama.NewChatModel(ctx, ollamaConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Ollama ChatModel失败: %v", err)
	}

	log.Infof("成功创建Ollama ChatModel，模型: %s", modelName)
	return chatModel, nil
}

// GetModelInfo 获取模型信息
func (p *EinoLLMProvider) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model_name":      p.modelName,
		"max_tokens":      p.maxTokens,
		"streamable":      p.streamable,
		"type":            "eino",
		"provider_type":   p.providerType,
		"framework":       "eino",
		"adapter_version": "3.0.0",
		"base_url":        p.config["base_url"],
	}
}

// ResponseWithFunctions 带函数调用的响应，使用Eino原生工具类型，直接调用EinoResponseWithTools
func (p *EinoLLMProvider) ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan *schema.Message {

	log.Infof("[Eino-LLM] 开始处理带工具的请求 - SessionID: %s, Type: %s", sessionID, p.providerType)

	logMessages(dialogue)
	// 直接调用EinoResponseWithTools获取Eino原生响应
	einoResponseChan := p.EinoResponseWithTools(ctx, sessionID, dialogue, functions)

	log.Infof("[Eino-LLM] 工具调用请求处理完成 - SessionID: %s", sessionID)

	return einoResponseChan
}

func logMessages(messages []*schema.Message) {
	for _, msg := range messages {
		if msg == nil {
			log.Debugf("history llm msg: <nil>")
			continue
		}
		log.Debugf("history llm msg: %s\n", msg.String())
	}
}

// llmExtraErrorKey 与 domain/llm.LLMExtraErrorKey 保持一致，失败时透传错误用（避免循环依赖）
const llmExtraErrorKey = "error"

// sendLLMError 向 channel 发送带 Extra.error 的错误消息
func sendLLMError(ch chan *schema.Message, err error) {
	ch <- &schema.Message{
		Role:  schema.System,
		Extra: map[string]any{llmExtraErrorKey: err.Error()},
	}
}

// EinoResponseWithTools 直接使用Eino类型的带工具响应
func (p *EinoLLMProvider) EinoResponseWithTools(ctx context.Context, sessionID string, messages []*schema.Message, tools []*schema.ToolInfo) chan *schema.Message {
	responseChan := make(chan *schema.Message, 200)

	var err error
	go func() {
		defer close(responseChan)

		log.Infof("[Eino-LLM] 开始处理Eino工具请求 - SessionID: %s, tools: %+v", sessionID, tools)

		// 如果有工具，需要绑定工具到ChatModel
		if len(tools) > 0 {
			p.chatModel, err = p.chatModel.WithTools(tools)
			if err != nil {
				log.Errorf("绑定工具失败: %v", err)
				sendLLMError(responseChan, err)
				return
			}
		}

		if p.streamable {
			log.Debugf("EinoLLMProvider.EinoResponseWithTools() streamable: %t", p.streamable)
			// 直接使用Eino的Stream方法
			streamReader, err := p.chatModel.Stream(ctx, messages, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino工具流式调用失败: %v", err)
				// 对于mock实现，如果Stream失败，回退到Generate
				message, genErr := p.chatModel.Generate(ctx, messages, model.WithMaxTokens(p.maxTokens))
				if genErr != nil {
					log.Errorf("Eino工具生成响应失败: %v", genErr)
					sendLLMError(responseChan, genErr)
					return
				}
				if message != nil {
					responseChan <- message
				}
				return
			}

			if streamReader != nil {
				defer streamReader.Close()

				var currentToolCall *schema.ToolCall
				var toolCallBuffer string
				var isToolCallComplete bool

				// 处理流式响应
				for {
					message, err := streamReader.Recv()
					//log.Debugf("streamReader.Recv() message: %+v", message)
					if err == io.EOF {
						// 如果有未完成的工具调用，发送最后一次
						if currentToolCall != nil {
							completeMessage := &schema.Message{
								Role:      schema.Assistant,
								ToolCalls: []schema.ToolCall{*currentToolCall},
							}
							responseChan <- completeMessage
						}
						break
					}
					if err != nil {
						log.Errorf("接收流式响应失败: %v", err)
						sendLLMError(responseChan, err)
						break
					}

					if message != nil {
						// 检查是否是工具调用的开始
						if len(message.ToolCalls) > 0 {
							toolCall := message.ToolCalls[0]

							if toolCall.Function.Name != "" {
								// 新工具调用开始
								currentToolCall = &toolCall
								toolCallBuffer = toolCall.Function.Arguments
								isToolCallComplete = false
							} else if currentToolCall != nil {
								// 累积工具调用参数
								toolCallBuffer += toolCall.Function.Arguments
								currentToolCall.Function.Arguments = toolCallBuffer

								// 检查参数是否是完整的 JSON
								if isValidJSON(toolCallBuffer) {
									isToolCallComplete = true
								}
							}

							// 如果工具调用完整，发送消息
							if isToolCallComplete {
								completeMessage := &schema.Message{
									Role:      schema.Assistant,
									ToolCalls: []schema.ToolCall{*currentToolCall},
								}
								responseChan <- completeMessage

								// 重置状态
								currentToolCall = nil
								toolCallBuffer = ""
								isToolCallComplete = false
							}
						} else if message.Content != "" {
							// 发送非工具调用的普通消息
							message.ToolCalls = nil
							responseChan <- message
						}
					}
				}
			}
		} else {
			// 直接使用Eino的Generate方法
			message, err := p.chatModel.Generate(ctx, messages, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino工具生成响应失败: %v", err)
				sendLLMError(responseChan, err)
				return
			}

			if message != nil {
				responseChan <- message
			}
		}

		log.Infof("[Eino-LLM] Eino工具请求处理完成 - SessionID: %s", sessionID)
	}()

	return responseChan
}

// isValidJSON 检查字符串是否是有效的JSON
func isValidJSON(str string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// GetChatModel 获取底层的Eino ChatModel
func (p *EinoLLMProvider) GetChatModel() model.ToolCallingChatModel {
	return p.chatModel
}

// GetProviderType 获取提供者类型
func (p *EinoLLMProvider) GetProviderType() string {
	return p.providerType
}

// WithMaxTokens 设置最大令牌数
func (p *EinoLLMProvider) WithMaxTokens(maxTokens int) *EinoLLMProvider {
	newProvider := *p
	newProvider.maxTokens = maxTokens
	return &newProvider
}

// WithStreamable 设置是否支持流式
func (p *EinoLLMProvider) WithStreamable(streamable bool) *EinoLLMProvider {
	newProvider := *p
	newProvider.streamable = streamable
	return &newProvider
}

// Close 关闭资源（无状态 Provider，无需关闭）
func (p *EinoLLMProvider) Close() error {
	return nil
}

// IsValid 检查资源是否有效
func (p *EinoLLMProvider) IsValid() bool {
	return p != nil && p.chatModel != nil
}
