package chat

import "context"

// ChatSessionOperator 定义 local mcp tool 需要的 ChatSession 操作接口
// 这个接口用于解耦 LLMManager 和 ChatSession，避免循环依赖
type ChatSessionOperator interface {
	// LocalMcpCloseChat 关闭聊天会话
	LocalMcpCloseChat() error

	// LocalMcpClearHistory 清空历史对话
	LocalMcpClearHistory() error

	// LocalMcpPlayMusic 播放音乐
	LocalMcpPlayMusic(ctx context.Context, params *PlayMusicParams) error

	// LocalMcpSwitchDeviceRole 按角色名称切换设备角色（支持模糊匹配）
	LocalMcpSwitchDeviceRole(ctx context.Context, roleName string) (string, error)

	// LocalMcpRestoreDeviceDefaultRole 恢复设备默认角色
	LocalMcpRestoreDeviceDefaultRole(ctx context.Context) error

	// 未来可以根据需要添加其他操作
	// GetDeviceID() string
	// IsActive() bool
}
