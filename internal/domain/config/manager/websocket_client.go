package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

type MessageHandleFunc func(*WebSocketRequest) (string, error)

type WebSocketClient struct {
	conn           *websocket.Conn
	baseURL        string
	requestTimeout time.Duration
	responseChans  map[string]chan *WebSocketResponse
	callbacks      map[string]func(*WebSocketResponse)
	requestHandler func(*WebSocketRequest) // 处理收到的请求
	mu             sync.RWMutex
	writeMu        sync.Mutex // 保护WebSocket写入操作，防止并发写入
	isConnected    bool
	connectMu      sync.Mutex
	messageQueue   chan *WebSocketRequest
	workers        sync.WaitGroup

	messageHandle cmap.ConcurrentMap[string, MessageHandleFunc]
	uuid          string
}

type WebSocketRequest struct {
	ID      string                 `json:"id"`
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type WebSocketResponse struct {
	ID      string                 `json:"id"`
	Status  int                    `json:"status"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

var (
	defaultClient *WebSocketClient
	clientOnce    sync.Once
)

func GetDefaultClient() *WebSocketClient {
	clientOnce.Do(func() {
		defaultClient = NewWebSocketClient()
	})
	return defaultClient
}

func NewWebSocketClient() *WebSocketClient {
	baseURL := viper.GetString("manager.backend_url")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &WebSocketClient{
		baseURL:        baseURL,
		requestTimeout: 30 * time.Second,
		responseChans:  make(map[string]chan *WebSocketResponse),
		callbacks:      make(map[string]func(*WebSocketResponse)),
		messageQueue:   make(chan *WebSocketRequest, 100),
		messageHandle:  cmap.New[MessageHandleFunc](),
		uuid:           uuid.New().String(),
	}
}

func NewWebSocketClientWithHandler(requestHandler func(*WebSocketRequest)) *WebSocketClient {
	client := NewWebSocketClient()
	client.requestHandler = requestHandler
	return client
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.isConnected {
		return nil
	}

	// 将HTTP URL转换为WebSocket URL
	wsURL := "ws://" + c.baseURL[7:] + "/ws" // 去掉 "http://" 并添加 "/ws"

	// 建立WebSocket连接
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{c.baseURL},
		"UUID":   []string{c.uuid},
	})
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	// 启动消息处理循环
	go c.handleMessages()

	// 启动消息发送工作线程
	c.startWorkers()

	log.Debugf("WebSocket客户端已连接到: %s", wsURL)
	return nil
}

func (c *WebSocketClient) Disconnect() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if !c.isConnected {
		return nil
	}

	if err := c.conn.Close(); err != nil {
		return err
	}

	c.isConnected = false
	c.mu.Lock()
	// 关闭所有响应通道
	for _, ch := range c.responseChans {
		close(ch)
	}
	c.responseChans = make(map[string]chan *WebSocketResponse)
	c.callbacks = make(map[string]func(*WebSocketResponse))
	c.mu.Unlock()

	// 停止工作线程
	close(c.messageQueue)
	c.workers.Wait()
	// 重新创建消息队列
	c.messageQueue = make(chan *WebSocketRequest, 100)

	log.Debugf("WebSocket连接已断开")
	return nil
}

func (c *WebSocketClient) IsConnected() bool {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	return c.isConnected
}

func (c *WebSocketClient) SendRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return nil, fmt.Errorf("连接失败: %v", err)
		}
	}

	// 生成UUID作为请求ID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// 创建响应通道
	responseChan := make(chan *WebSocketResponse, 1)
	c.mu.Lock()
	c.responseChans[requestID] = responseChan
	c.mu.Unlock()

	// 清理响应通道
	defer func() {
		c.mu.Lock()
		delete(c.responseChans, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	// 发送请求（使用写入锁保护）
	c.writeMu.Lock()
	err := c.conn.WriteJSON(request)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	// 等待响应
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(c.requestTimeout):
		return nil, fmt.Errorf("请求超时")
	case <-ctx.Done():
		return nil, fmt.Errorf("上下文取消")
	}
}

// 便捷方法 - 使用WebSocket原生ping
func (c *WebSocketClient) Ping() error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocket未连接")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
}

func (c *WebSocketClient) GetStatus(ctx context.Context) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "GET", "/api/ws/status", nil)
}

func (c *WebSocketClient) Echo(ctx context.Context, message string) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "POST", "/api/ws/echo", map[string]interface{}{
		"message": message,
	})
}

// 全局便捷方法
func ConnectManagerWebSocket(ctx context.Context) error {
	return GetDefaultClient().Connect(ctx)
}

func DisconnectManagerWebSocket() error {
	return GetDefaultClient().Disconnect()
}

func SendManagerRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, method, path, body)
}

func ManagerWebSocketPing(ctx context.Context) error {
	return GetDefaultClient().Ping()
}

func ManagerWebSocketStatus(ctx context.Context) (*WebSocketResponse, error) {
	return GetDefaultClient().GetStatus(ctx)
}

func ManagerWebSocketEcho(ctx context.Context, message string) (*WebSocketResponse, error) {
	return GetDefaultClient().Echo(ctx, message)
}

func IsManagerWebSocketConnected() bool {
	return GetDefaultClient().IsConnected()
}

func SendDeviceRequest(ctx context.Context, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, "POST", path, body)
}

// startWorkers 启动消息发送工作线程
func (c *WebSocketClient) startWorkers() {
	workerCount := 3 // 启动3个工作线程

	for i := 0; i < workerCount; i++ {
		c.workers.Add(1)
		go func(workerID int) {
			defer c.workers.Done()

			log.Debugf("Manager WebSocket工作线程 %d 已启动", workerID)

			for request := range c.messageQueue {
				if !c.IsConnected() {
					log.Debugf("工作线程 %d: WebSocket未连接，丢弃请求", workerID)
					continue
				}

				// 发送请求（使用写入锁保护）
				c.writeMu.Lock()
				err := c.conn.WriteJSON(request)
				c.writeMu.Unlock()
				if err != nil {
					log.Debugf("工作线程 %d: 发送请求失败: %v", workerID, err)
					// 连接可能已断开，触发重连
					go c.handleConnectionError()
					continue
				}

				log.Debugf("工作线程 %d: 已发送请求 %s", workerID, request.ID)
			}

			log.Debugf("Manager WebSocket工作线程 %d 已停止", workerID)
		}(i)
	}
}

// handleConnectionError 处理连接错误
func (c *WebSocketClient) handleConnectionError() {
	if c.IsConnected() {
		log.Warn("检测到WebSocket连接错误，正在断开连接...")
		c.Disconnect()
	}
}

// SendRequestWithCallback 发送请求并使用回调处理响应
func (c *WebSocketClient) SendRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("连接失败: %v", err)
		}
	}

	// 生成UUID作为请求ID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// 注册回调
	c.mu.Lock()
	c.callbacks[requestID] = callback
	c.mu.Unlock()

	// 清理回调
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
	}()

	// 将请求放入队列
	select {
	case c.messageQueue <- &request:
		log.Debugf("请求 %s 已加入队列", requestID)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("消息队列已满，请求超时")
	case <-ctx.Done():
		return fmt.Errorf("上下文取消")
	}
}

// SendRequestAsync 异步发送请求
func (c *WebSocketClient) SendRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return "", fmt.Errorf("连接失败: %v", err)
		}
	}

	// 生成UUID作为请求ID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// 将请求放入队列
	select {
	case c.messageQueue <- &request:
		log.Debugf("异步请求 %s 已加入队列", requestID)
		return requestID, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("消息队列已满，请求超时")
	case <-ctx.Done():
		return "", fmt.Errorf("上下文取消")
	}
}

// GetResponse 获取指定请求ID的响应（用于异步请求）
func (c *WebSocketClient) GetResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	responseChan := make(chan *WebSocketResponse, 1)

	// 注册临时回调
	c.mu.Lock()
	c.callbacks[requestID] = func(response *WebSocketResponse) {
		responseChan <- response
	}
	c.mu.Unlock()

	// 清理回调
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("等待响应超时")
	}
}

// handleMessages 处理接收到的WebSocket消息
func (c *WebSocketClient) handleMessages() {
	for {
		if !c.isConnected {
			return
		}

		// 读取消息类型
		messageType, reader, err := c.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debugf("WebSocket读取错误: %v", err)
			}
			c.Disconnect()
			return
		}

		// 处理不同类型的消息
		switch messageType {
		case websocket.TextMessage:
			// 处理JSON消息
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Errorf("解析JSON消息失败: %v", err)
				continue
			}

			// 根据消息类型判断是请求还是响应
			if method, exists := rawMessage["method"]; exists && method != nil {
				// 这是收到的请求
				c.handleIncomingRequest(rawMessage)
			} else if status, exists := rawMessage["status"]; exists && status != nil {
				// 这是收到的响应
				c.handleIncomingResponse(rawMessage)
			} else {
				log.Warnf("收到无法识别的WebSocket消息: %+v", rawMessage)
			}

		case websocket.PingMessage:
			// 处理ping消息，自动回复pong（使用写入锁保护）
			log.Debugf("收到ping消息，自动回复pong")
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()
			if err != nil {
				log.Errorf("发送pong失败: %v", err)
			}

		case websocket.PongMessage:
			// 处理pong消息
			log.Debugf("收到pong消息")

		case websocket.CloseMessage:
			// 处理关闭消息
			log.Debugf("收到关闭消息")
			c.Disconnect()
			return

		default:
			log.Warnf("收到未知类型的WebSocket消息: %d", messageType)
		}
	}
}

// handleIncomingRequest 处理收到的请求
func (c *WebSocketClient) handleIncomingRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Errorf("解析WebSocket请求失败: %v", err)
		return
	}

	log.Debugf("收到请求: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// 如果有注册的请求处理器，调用它
	if c.requestHandler != nil {
		go c.requestHandler(&request)
	} else {
		// 如果没有注册处理器，使用默认处理器处理已知路径
		c.handleDefaultRequest(&request)
	}
}

func (c *WebSocketClient) RegisterMessageHandler(ctx context.Context, path string, handler types.EventHandler) {
	f := func(request *WebSocketRequest) (string, error) {
		return handler(ctx, request.Path, request.Body)
	}
	c.messageHandle.Set(path, f)
}

// handleDefaultRequest 默认请求处理器
func (c *WebSocketClient) handleDefaultRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/mcp/tools":
		// 处理MCP工具列表请求
		c.handleMcpToolListRequest(request)

	case "/api/server/info":
		// 返回服务器信息
		response := map[string]interface{}{
			"server_name": "xiaozhi-server",
			"version":     "1.0.0",
			"uptime":      time.Now().Format(time.RFC3339),
			"request_id":  request.ID,
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("发送服务器信息响应失败: %v", err)
		}

	case "/api/server/ping":
		// 简单的ping响应
		response := map[string]interface{}{
			"message": "pong from server",
			"time":    time.Now().Format(time.RFC3339),
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("发送ping响应失败: %v", err)
		}
	default:
		handler, exists := c.messageHandle.Get(request.Path)
		if exists {
			// 调用处理器并处理返回值
			result, err := handler(request)
			if err != nil {
				log.Errorf("处理请求 %s 失败: %v", request.Path, err)
				// 发送错误响应
				if err := c.SendResponse(request.ID, 500, nil, err.Error()); err != nil {
					log.Errorf("发送错误响应失败: %v", err)
				}
			} else {
				// 发送成功响应
				response := map[string]interface{}{
					"result": result,
				}
				if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
					log.Errorf("发送成功响应失败: %v", err)
				}
			}
		} else {
			log.Warnf("收到未知的WebSocket请求路径: %s, ID: %s", request.Path, request.ID)

			// 发送404响应
			if err := c.SendResponse(request.ID, 404, nil, "Unknown endpoint"); err != nil {
				log.Errorf("发送错误响应失败: %v", err)
			}
		}
	}
}

// handleIncomingResponse 处理收到的响应
func (c *WebSocketClient) handleIncomingResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Errorf("解析WebSocket响应失败: %v", err)
		return
	}

	log.Debugf("收到响应: ID=%s, Status=%d", response.ID, response.Status)

	// 查找对应的响应通道和回调
	c.mu.RLock()
	responseChan, exists := c.responseChans[response.ID]
	callback, callbackExists := c.callbacks[response.ID]
	c.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Debugf("响应通道已满，丢弃响应: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Debugf("收到未知的响应ID: %s", response.ID)
	}
}

// SendResponse 发送响应给收到的请求
func (c *WebSocketClient) SendResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocket未连接")
	}

	response := WebSocketResponse{
		ID:     requestID,
		Status: status,
		Body:   body,
		Error:  errorMsg,
	}

	// 使用写入锁保护
	c.writeMu.Lock()
	err := c.conn.WriteJSON(response)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	log.Debugf("已发送响应: ID=%s, Status=%d", requestID, status)
	return nil
}

// SetRequestHandler 设置请求处理器
func (c *WebSocketClient) SetRequestHandler(handler func(*WebSocketRequest)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestHandler = handler
}

// mapToStruct 辅助函数：将map转换为struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// GetMcpToolsByAgentID 根据agent_id获取MCP工具列表
func GetMcpToolsByAgentID(agentID string) ([]string, error) {
	nameList := make([]string, 0)

	allTools, err := mcp.GetWsEndpointMcpTools(agentID)
	if err != nil {
		log.Errorf("获取MCP工具列表失败: %v", err)
		return nameList, err
	}

	// 转换工具列表格式
	for name, _ := range allTools {
		nameList = append(nameList, name)
	}

	log.Infof("为agent_id %s 获取到 %d 个MCP工具", agentID, len(nameList))
	return nameList, nil
}

// handleMcpToolListRequest 处理MCP工具列表请求
func (c *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
	// 从请求体中获取agent_id
	agentID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = id
		}
	}

	if agentID == "" {
		log.Warnf("收到MCP工具列表请求，但缺少agent_id")
		if err := c.SendResponse(request.ID, 400, nil, "缺少agent_id参数"); err != nil {
			log.Errorf("发送错误响应失败: %v", err)
		}
		return
	}

	log.Infof("处理MCP工具列表请求，agent_id: %s", agentID)

	// 获取工具列表
	nameList, err := GetMcpToolsByAgentID(agentID)
	if err != nil {
		log.Errorf("获取MCP工具列表失败: %v", err)
		if err := c.SendResponse(request.ID, 500, nil, fmt.Sprintf("获取工具列表失败: %v", err)); err != nil {
			log.Errorf("发送错误响应失败: %v", err)
		}
		return
	}

	// 构造响应
	response := map[string]interface{}{
		"agent_id": agentID,
		"tools":    nameList,
		"count":    len(nameList),
	}

	// 发送响应
	if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
		log.Errorf("发送MCP工具列表响应失败: %v", err)
	}
}

// 全局便捷方法（异步版本）
func SendManagerRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	return GetDefaultClient().SendRequestAsync(ctx, method, path, body)
}

func SendManagerRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	return GetDefaultClient().SendRequestWithCallback(ctx, method, path, body, callback)
}

func GetManagerResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	return GetDefaultClient().GetResponse(requestID, timeout)
}

// 双向通信支持方法
func SetManagerRequestHandler(handler func(*WebSocketRequest)) {
	GetDefaultClient().SetRequestHandler(handler)
}

func SendManagerResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	return GetDefaultClient().SendResponse(requestID, status, body, errorMsg)
}

// 创建带有请求处理器的客户端
func NewManagerClientWithHandler(handler func(*WebSocketRequest)) *WebSocketClient {
	return NewWebSocketClientWithHandler(handler)
}

// SendMcpToolListRequest 发送MCP工具列表请求
func SendMcpToolListRequest(ctx context.Context, agentID string) (*WebSocketResponse, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequest(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestAsync 异步发送MCP工具列表请求
func SendMcpToolListRequestAsync(ctx context.Context, agentID string) (string, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestAsync(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestWithCallback 使用回调发送MCP工具列表请求
func SendMcpToolListRequestWithCallback(ctx context.Context, agentID string, callback func(*WebSocketResponse)) error {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestWithCallback(ctx, "GET", "/api/mcp/tools", body, callback)
}

// Init 初始化Manager配置提供者
// 包括WebSocket连接的初始化
func Init(ctx context.Context) error {
	log.Infof("Initializing Manager config provider with WebSocket client")

	// 创建WebSocket客户端
	client := GetDefaultClient()

	// 连接到WebSocket服务器
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %v", err)
	}

	log.Infof("Manager config provider initialized successfully")
	return nil
}

// Close 关闭Manager配置提供者，清理资源
func Close() error {
	log.Infof("Closing Manager config provider")
	return DisconnectManagerWebSocket()
}

// IsConnected 检查Manager配置提供者是否已连接
func IsConnected() bool {
	return IsManagerWebSocketConnected()
}
