package mcp

import (
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

func GetToolByName(deviceId string, agentId string, toolName string) (tool.InvokableTool, bool) {
	// 优先从本地管理器获取
	localManager := GetLocalMCPManager()
	tool, ok := localManager.GetToolByName(toolName)
	if ok {
		return tool, ok
	}

	// 其次从全局管理器获取
	tool, ok = globalManager.GetToolByName(toolName)
	if ok {
		return tool, ok
	}

	// 最后从设备MCP客户端池获取
	tool, ok = mcpClientPool.GetToolByDeviceId(deviceId, toolName)
	if ok {
		return tool, true
	}
	// 兼容 AgentID 上报的 MCP 工具
	if agentId != "" && agentId != deviceId {
		tool, ok = mcpClientPool.GetToolByDeviceId(agentId, toolName)
		if ok {
			return tool, true
		}
	}
	return nil, false
}

func GetDeviceMcpClient(deviceId string) *DeviceMcpSession {
	return mcpClientPool.GetMcpClient(deviceId)
}

func AddDeviceMcpClient(deviceId string, mcpClient *DeviceMcpSession) error {
	mcpClientPool.AddMcpClient(deviceId, mcpClient)
	return nil
}

func RemoveDeviceMcpClient(deviceId string) error {
	mcpClientPool.RemoveMcpClient(deviceId)
	return nil
}

func GetToolsByDeviceId(deviceId string, agentId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)

	// 优先从本地管理器获取
	localManager := GetLocalMCPManager()
	localTools := localManager.GetAllTools()
	for toolName, tool := range localTools {
		retTools[toolName] = tool
	}
	log.Infof("从本地管理器获取到 %d 个工具", len(localTools))

	// 其次从全局管理器获取
	globalTools := globalManager.GetAllTools()
	for toolName, tool := range globalTools {
		// 本地工具优先，如果已存在同名工具则不覆盖
		if _, exists := retTools[toolName]; !exists {
			retTools[toolName] = tool
		}
	}
	log.Infof("从全局管理器获取到 %d 个工具", len(globalTools))

	// 最后从MCP客户端池获取
	deviceTools, err := mcpClientPool.GetAllToolsByDeviceIdAndAgentId(deviceId, agentId)
	if err != nil {
		log.Errorf("获取设备 %s 的工具失败: %v", deviceId, err)
		return retTools, nil
	}
	for toolName, tool := range deviceTools {
		// 本地工具和全局工具优先，如果已存在同名工具则不覆盖
		if _, exists := retTools[toolName]; !exists {
			retTools[toolName] = tool
		}
	}
	log.Infof("从设备 %s 获取到 %d 个工具", deviceId, len(deviceTools))
	log.Infof("设备 %s 总共获取到 %d 个工具", deviceId, len(retTools))

	return retTools, nil
}

func GetWsEndpointMcpTools(agentId string) (map[string]tool.InvokableTool, error) {
	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

// GetReportedToolsByDeviceID 仅获取设备上报的MCP工具
func GetReportedToolsByDeviceID(deviceId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if deviceId == "" {
		return retTools, nil
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return retTools, nil
	}

	for toolName, invokable := range client.GetTools() {
		retTools[toolName] = invokable
	}

	return retTools, nil
}

// GetReportedToolsByAgentID 仅获取智能体(WebSocket端点)上报的MCP工具
func GetReportedToolsByAgentID(agentId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if agentId == "" {
		return retTools, nil
	}

	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

// GetReportedToolByDeviceIDAndName 仅在设备上报工具中查找
func GetReportedToolByDeviceIDAndName(deviceId, toolName string) (tool.InvokableTool, bool) {
	reportedTools, err := GetReportedToolsByDeviceID(deviceId)
	if err != nil {
		log.Errorf("获取设备上报MCP工具失败: device=%s err=%v", deviceId, err)
		return nil, false
	}

	invokable, ok := reportedTools[toolName]
	return invokable, ok
}

// GetReportedToolByAgentIDAndName 仅在智能体上报工具中查找
func GetReportedToolByAgentIDAndName(agentId, toolName string) (tool.InvokableTool, bool) {
	reportedTools, err := GetReportedToolsByAgentID(agentId)
	if err != nil {
		log.Errorf("获取智能体上报MCP工具失败: agent=%s err=%v", agentId, err)
		return nil, false
	}

	invokable, ok := reportedTools[toolName]
	return invokable, ok
}

// GetReportedToolsByDeviceIdAndAgentId 兼容方法：明确分流设备/智能体查询，不再混用
func GetReportedToolsByDeviceIdAndAgentId(deviceId string, agentId string) (map[string]tool.InvokableTool, error) {
	if deviceId != "" {
		return GetReportedToolsByDeviceID(deviceId)
	}
	if agentId != "" {
		return GetReportedToolsByAgentID(agentId)
	}
	return make(map[string]tool.InvokableTool), nil
}

// GetReportedToolByName 兼容方法：按维度分流，不再混用
func GetReportedToolByName(deviceId string, agentId string, toolName string) (tool.InvokableTool, bool) {
	if deviceId != "" {
		return GetReportedToolByDeviceIDAndName(deviceId, toolName)
	}
	if agentId != "" {
		return GetReportedToolByAgentIDAndName(agentId, toolName)
	}
	return nil, false
}

func GetAudioResourceByTool(tool McpTool, resourceLink mcp_go.ResourceLink) (mcp_go.ReadResourceResult, error) {
	/*client := tool.GetClient()
	resourceRequest := mcp_go.ReadResourceRequest{
		Request: mcp_go.Request{
			Params: mcp_go.ReadResourceParams{
				URI: resourceLink.URL,
			},
		},
	}
	client.ReadResource(context.Background(), resourceRequest)*/
	return mcp_go.ReadResourceResult{}, nil
}
