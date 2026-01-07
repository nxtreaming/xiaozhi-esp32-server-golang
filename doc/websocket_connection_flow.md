# WebSocket连接流程说明

## 概述

本文档描述了 `internal/domain/config/manager/websocket_client.go` 和 `websocket.go` 之间的WebSocket连接和通信流程。

## 架构设计

### 角色定义

1. **`internal/domain/config/manager/websocket_client.go`** - 主服务器WebSocket客户端
   - 作为客户端连接到Manager Backend
   - 可以发送请求和接收响应
   - 支持双向通信

2. **`websocket.go`** - Manager Backend WebSocket服务器
   - 作为服务端接收来自主服务器的WebSocket连接
   - 处理主服务器发送的请求
   - **只保留最后一个有效连接**（新连接会断开旧连接）
   - 支持主动推送消息

### 连接流程

```
主服务器 (internal/domain/config/manager/websocket_client.go)  →  Manager Backend (websocket.go)
        客户端                          服务端（单连接）
```

## 详细流程

### 1. 建立连接

#### Manager Backend启动WebSocket服务器
```go
// 在 websocket.go 中
controller := NewWebSocketController(db)
// 在路由中注册
router.GET("/ws", controller.HandleWebSocket)
```

#### 主服务器连接Manager Backend
```go
// 在 internal/domain/config/manager/websocket_client.go 中
client := manager.NewWebSocketClient()
err := client.Connect(ctx)
```

连接URL格式：
- 如果配置为 `http://localhost:8080`
- 实际连接 `ws://localhost:8080/ws`

**重要**：如果有新的连接请求，Manager Backend会自动断开现有连接，只保留最新的连接。

### 2. 请求工具列表流程

#### 主服务器请求MCP工具列表
```go
// 在 internal/domain/config/manager/websocket_client.go 中
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend处理请求
```go
// 在 websocket.go 中
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)
    
    // 获取工具列表逻辑
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }
    
    client.sendResponse(request.ID, 200, response, "")
}
```

### 3. 双向通信支持

### 客户端 → 服务器（原有功能）
#### 主服务器请求MCP工具列表
```go
// 在 internal/domain/config/manager/websocket_client.go 中
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend处理请求
```go
// 在 websocket.go 中
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)
    
    // 获取工具列表逻辑
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }
    
    client.sendResponse(request.ID, 200, response, "")
}
```

### 服务器 → 客户端（新增功能）
#### Manager Backend主动请求客户端
```go
// 在 websocket.go 中
func (ctrl *WebSocketController) RequestMcpToolsFromClient(ctx context.Context, agentID string) (*WebSocketResponse, error) {
    body := map[string]interface{}{
        "agent_id": agentID,
    }
    return ctrl.SendRequestToClient(ctx, "GET", "/api/mcp/tools", body)
}

// 请求客户端服务器信息
func (ctrl *WebSocketController) RequestServerInfoFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/info", nil)
}

// 请求客户端ping
func (ctrl *WebSocketController) RequestPingFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/ping", nil)
}
```

#### 客户端处理服务器请求
```go
// 在 internal/domain/config/manager/websocket_client.go 中
client.SetRequestHandler(func(request *WebSocketRequest) {
    // 处理收到的请求
    switch request.Path {
    case "/api/mcp/tools":
        // 处理MCP工具列表请求
        c.handleMcpToolListRequest(request)
    case "/api/server/info":
        // 处理服务器信息请求
        c.handleServerInfoRequest(request)
    case "/api/server/ping":
        // 处理ping请求
        c.handlePingRequest(request)
    }
})
```

### 完整的双向通信示例
```go
// 1. 客户端连接到服务器
client := manager.NewWebSocketClient()
err := client.Connect(ctx)

// 2. 客户端设置请求处理器
client.SetRequestHandler(func(request *WebSocketRequest) {
    // 处理来自服务器的请求
    // 并发送响应
})

// 3. 客户端主动请求服务器
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "agent_123",
})

// 4. 服务器主动请求客户端
serverResponse, err := websocketController.RequestMcpToolsFromClient(ctx, "agent_456")

// 5. 双向通信完成
```

## 消息格式

### 请求消息 (WebSocketRequest)
```json
{
    "id": "uuid-string",
    "method": "GET",
    "path": "/api/mcp/tools",
    "body": {
        "agent_id": "agent_123"
    }
}
```

### 响应消息 (WebSocketResponse)
```json
{
    "id": "uuid-string",
    "status": 200,
    "body": {
        "agent_id": "agent_123",
        "tools": ["tool1", "tool2", "tool3"],
        "count": 3
    },
    "error": ""
}
```

### Ping/Pong消息
```json
// Ping
{"ping": 1640995200}

// Pong
{"pong": 1640995200}
```

## 连接管理

### 单连接策略
- **只保留最后一个有效连接**
- 新连接会自动断开现有连接
- 简化了连接管理逻辑
- 适合一对一的通信场景

### 连接状态监控
```go
// 检查是否有连接的客户端
func (ctrl *WebSocketController) HasConnectedClient() bool

// 获取当前连接的客户端
func (ctrl *WebSocketController) GetCurrentClient() *WebSocketClient
```

### 连接切换逻辑
```go
// 在HandleWebSocket中
if ctrl.currentClient != nil && ctrl.currentClient.isConnected {
    log.Printf("断开现有连接: %s", ctrl.currentClient.ID)
    ctrl.currentClient.conn.Close()
    ctrl.currentClient.isConnected = false
}

// 设置新连接为当前客户端
ctrl.currentClient = client
```

## 错误处理

### 连接错误
- 自动心跳检测
- 连接超时自动断开
- 连接异常自动清理
- 新连接自动替换旧连接

### 消息错误
- 消息格式验证
- 错误响应返回
- 日志记录

## 配置要求

### 主服务器配置
```yaml
manager:
  backend_url: "http://localhost:8080"
```

### Manager Backend配置
```go
// 在路由中注册WebSocket端点
router.GET("/ws", websocketController.HandleWebSocket)
```

## 测试建议

1. **连接测试**
   - 验证WebSocket连接建立
   - 测试新连接断开旧连接
   - 测试连接断开重连

2. **功能测试**
   - 测试MCP工具列表请求
   - 验证双向通信
   - 测试消息推送

3. **错误测试**
   - 网络断开重连
   - 无效消息处理
   - 超时处理
   - 心跳超时
   - 连接切换

## 注意事项

1. **单连接限制**
   - 同时只能有一个活跃连接
   - 新连接会强制断开旧连接
   - 适合主从架构，不适合多客户端场景

2. **并发安全**
   - 使用读写锁保护当前客户端引用
   - 安全的客户端切换
   - 线程安全的消息发送

3. **资源管理**
   - 及时清理断开的连接
   - 正确关闭WebSocket连接
   - 避免内存泄漏

4. **心跳机制**
   - 30秒发送一次ping
   - 60秒无响应自动断开
   - 支持ping/pong消息

5. **日志记录**
   - 记录连接状态变化
   - 记录连接切换
   - 记录请求和响应信息
   - 记录错误和异常情况

## 完整使用示例

### 双向通信测试代码
```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "xiaozhi-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()
    
    // 1. 创建客户端并连接
    client := manager.NewWebSocketClient()
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer client.Disconnect()
    
    // 2. 设置请求处理器（处理来自服务器的请求）
    client.SetRequestHandler(func(request *manager.WebSocketRequest) {
        log.Printf("收到服务器请求: %s %s", request.Method, request.Path)
        
        switch request.Path {
        case "/api/mcp/tools":
            // 处理MCP工具列表请求
            agentID := ""
            if request.Body != nil {
                if id, ok := request.Body["agent_id"].(string); ok {
                    agentID = id
                }
            }
            
            response := map[string]interface{}{
                "agent_id": agentID,
                "tools":    []string{"client_tool_1", "client_tool_2"},
                "count":    2,
            }
            
            client.SendResponse(request.ID, 200, response, "")
            
        case "/api/server/info":
            response := map[string]interface{}{
                "server_name": "xiaozhi-client",
                "version":     "1.0.0",
                "uptime":      time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")
            
        case "/api/server/ping":
            response := map[string]interface{}{
                "message": "pong from client",
                "time":    time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")
        }
    })
    
    // 3. 客户端主动请求服务器
    fmt.Println("=== 客户端请求服务器 ===")
    response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
        "agent_id": "client_agent_123",
    })
    if err != nil {
        log.Printf("客户端请求失败: %v", err)
    } else {
        fmt.Printf("服务器响应: %+v\n", response)
    }
    
    // 4. 等待一段时间，让服务器有机会发送请求
    fmt.Println("等待服务器请求...")
    time.Sleep(5 * time.Second)
    
    fmt.Println("双向通信测试完成！")
}
```

### 服务器端测试代码
```go
// 在Manager Backend中
func testBidirectionalCommunication() {
    ctx := context.Background()
    
    // 1. 检查客户端连接状态
    status := websocketController.GetClientConnectionStatus()
    fmt.Printf("客户端状态: %+v\n", status)
    
    // 2. 服务器主动请求客户端
    fmt.Println("=== 服务器请求客户端 ===")
    
    // 请求MCP工具列表
    response, err := websocketController.RequestMcpToolsFromClient(ctx, "server_agent_456")
    if err != nil {
        log.Printf("请求MCP工具列表失败: %v", err)
    } else {
        fmt.Printf("客户端MCP工具响应: %+v\n", response)
    }
    
    // 请求服务器信息
    infoResponse, err := websocketController.RequestServerInfoFromClient(ctx)
    if err != nil {
        log.Printf("请求服务器信息失败: %v", err)
    } else {
        fmt.Printf("客户端服务器信息: %+v\n", infoResponse)
    }
    
    // 请求ping
    pingResponse, err := websocketController.RequestPingFromClient(ctx)
    if err != nil {
        log.Printf("请求ping失败: %v", err)
    } else {
        fmt.Printf("客户端ping响应: %+v\n", pingResponse)
    }
}
```

## 注意事项

1. **双向通信要求**
   - 客户端必须设置请求处理器
   - 服务器和客户端都必须实现相应的请求处理方法
   - 请求ID必须匹配，确保响应正确路由

2. **错误处理**
   - 网络断开时双向通信会失败
   - 超时处理很重要
   - 连接状态检查必不可少

3. **性能考虑**
   - 避免频繁的双向请求
   - 合理设置超时时间
   - 监控连接状态
