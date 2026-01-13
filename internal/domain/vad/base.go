package vad

import (
	"errors"
	"fmt"
	"xiaozhi-esp32-server-golang/constants"
	"xiaozhi-esp32-server-golang/internal/domain/vad/inter"
	"xiaozhi-esp32-server-golang/internal/domain/vad/silero_vad"
	"xiaozhi-esp32-server-golang/internal/domain/vad/ten_vad"
	"xiaozhi-esp32-server-golang/internal/domain/vad/webrtc_vad"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

func AcquireVAD(provider string, config map[string]interface{}) (inter.VAD, error) {
	switch provider {
	case constants.VadTypeSileroVad:
		return silero_vad.AcquireVAD(config)
	case constants.VadTypeWebRTCVad:
		return webrtc_vad.AcquireVAD(config)
	case constants.VadTypeTenVad:
		return ten_vad.AcquireVAD(config)
	default:
		return nil, errors.New("invalid vad provider")
	}
}

func ReleaseVAD(vad inter.VAD) error {
	//根据vad的类型，调用对应的ReleaseVAD方法
	switch vad.(type) {
	case *webrtc_vad.WebRTCVAD:
		return webrtc_vad.ReleaseVAD(vad)
	case *silero_vad.SileroVAD:
		return silero_vad.ReleaseVAD(vad)
	case *ten_vad.TenVAD:
		return ten_vad.ReleaseVAD(vad)
	default:
		return errors.New("invalid vad type")
	}
	return nil
}

// InitVAD 从全局配置初始化VAD资源池
func InitVAD() error {
	log.Infof("开始初始化 VAD 资源池...")

	vadProvider := viper.GetString("vad.provider")
	if vadProvider == "" {
		err := fmt.Errorf("vad.provider 配置未设置")
		log.Errorf("VAD 初始化失败: %v", err)
		return err
	}

	log.Infof("检测到 VAD 提供商: %s", vadProvider)

	switch vadProvider {
	case constants.VadTypeSileroVad:
		log.Infof("初始化 Silero VAD 资源池...")
		vadConfig := viper.GetStringMap("vad.silero_vad")
		silero_vad.InitVadPool(vadConfig)
		log.Infof("Silero VAD 资源池初始化完成")
		return nil
	case constants.VadTypeTenVad:
		log.Infof("初始化 TEN-VAD 资源池...")
		vadConfig := viper.GetStringMap("vad.ten_vad")
		ten_vad.InitVadPool(vadConfig)
		log.Infof("TEN-VAD 资源池初始化完成")
		return nil
	case constants.VadTypeWebRTCVad:
		log.Infof("WebRTC VAD 使用懒加载模式，将在首次使用时自动初始化")
		// WebRTC VAD 使用懒加载，在 AcquireVAD 时自动初始化，这里不需要显式初始化
		return nil
	default:
		err := fmt.Errorf("不支持的 VAD 提供商: %s", vadProvider)
		log.Errorf("VAD 初始化失败: %v", err)
		return err
	}
}
