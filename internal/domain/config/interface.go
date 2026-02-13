package user_config

import (
	"context"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// UserConfigProvider 用户配置提供者接口
// 这是一个扩展的接口，支持更多操作，区别于原有的UserConfig接口
type UserConfigProvider interface {
	//auth
	//根据deviceId和clientId获取激活信息
	IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error)
	GetActivationInfo(ctx context.Context, deviceId string, clientId string) (int, string, string, int)
	VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error)

	//llm memory

	// GetUserConfig 获取用户配置（兼容原有接口）
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)

	// SwitchDeviceRoleByName 按角色名（支持模糊匹配）切换设备角色
	SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error)

	// RestoreDeviceDefaultRole 恢复设备默认角色（清空设备绑定角色）
	RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error

	// 获取 mqtt, mqtt_server, udp, ota, vision配置
	GetSystemConfig(ctx context.Context) (string, error)

	//注册上行事件处理函数(比如设备上下线等)
	NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{})
	//注册下行事件处理函数(比如消息注入等)
	RegisterMessageEventHandler(ctx context.Context, eventType string, eventHandler types.EventHandler)
}
