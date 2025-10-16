package user_config

import (
	"context"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"

	"xiaozhi-esp32-server-golang/internal/domain/config/manager"
	"xiaozhi-esp32-server-golang/internal/domain/config/memory"
	redis_config "xiaozhi-esp32-server-golang/internal/domain/config/redis"

	"github.com/spf13/viper"
)

// InitConfigSystem 初始化配置系统
// 根据config_provider.type的值调用对应配置包的Init方法
func InitConfigSystem(ctx context.Context) error {
	// 获取配置提供者类型
	providerType := viper.GetString("config_provider.type")
	if providerType == "" {
		providerType = "redis" // 默认使用redis
		log.Infof("config_provider.type not set, using default: redis")
	}

	log.Infof("Initializing config system with provider: %s", providerType)

	// 根据配置提供者类型调用对应的Init方法
	switch providerType {
	case "manager":
		return manager.Init(ctx)
	case "redis":
		return redis_config.Init(ctx)
	case "memory":
		return memory.Init(ctx)
	default:
		return fmt.Errorf("unsupported config provider type: %s", providerType)
	}
}
