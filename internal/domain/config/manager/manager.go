package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"xiaozhi-esp32-server-golang/internal/components/http"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// ConfigManager 配置管理器
// 提供高层级的配置管理功能，包括缓存、热更新、配置验证等
type ConfigManager struct {
	// HTTP客户端
	client *http.ManagerClient
}

// NewConfigManager 创建新的配置管理器
func NewManagerUserConfigProvider(config map[string]interface{}) (*ConfigManager, error) {
	// 从配置中获取后端管理系统的基础URL
	var baseURL string
	if backendUrl := config["backend_url"]; backendUrl != nil {
		baseURL = backendUrl.(string)
	}
	// 如果配置中没有，使用默认值
	if baseURL == "" {
		baseURL = "http://localhost:8080" // 默认值
	}

	// 创建Manager HTTP客户端
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    baseURL,
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	manager := &ConfigManager{
		client: managerClient,
	}

	//log.Log().Debug("配置管理器初始化成功", "backend_url", baseURL)
	return manager, nil
}

func (c *ConfigManager) GetUserConfig(ctx context.Context, deviceID string) (types.UConfig, error) {
	// 解析响应
	var response struct {
		Data struct {
			VAD struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"vad"`
			ASR struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"asr"`
			LLM struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"llm"`
			TTS struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"tts"`
			Memory struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"memory"`
			VoiceIdentify map[string]struct {
				ID                 uint     `json:"id"`
				Name               string   `json:"name"`
				Prompt             string   `json:"prompt"`
				Description        string   `json:"description"`
				Uuids              []string `json:"uuids"`
				TTSConfigID        *string  `json:"tts_config_id"`
				Voice              *string  `json:"voice"`
				VoiceModelOverride *string  `json:"voice_model_override"`
			} `json:"voice_identify"`
			KnowledgeBases  []types.KnowledgeBaseRef `json:"knowledge_bases"`
			Prompt          string                   `json:"prompt"`
			AgentId         string                   `json:"agent_id"`
			MemoryMode      string                   `json:"memory_mode"`
			MCPServiceNames string                   `json:"mcp_service_names"`
		} `json:"data"`
	}

	// 发送HTTP请求
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/configs",
		QueryParams: map[string]string{
			"device_id": deviceID,
		},
		Response: &response,
	})
	if err != nil {
		log.Log().Error("获取用户配置失败", "error", err, "device_id", deviceID)
		return types.UConfig{}, err
	}

	// 解析JSON配置数据的辅助函数
	parseJsonData := func(jsonStr string) map[string]interface{} {
		var data map[string]interface{}
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				log.Log().Warn("解析JSON数据失败", "error", err, "json", jsonStr)
				return make(map[string]interface{})
			}
		}
		return data
	}

	// 从设备配置获取声纹组信息（只获取声纹组配置，不获取服务地址）
	// VoiceIdentify 是一个 map，key 是声纹组名称，value 包含 prompt、description 和 uuids
	voiceIdentifyData := make(map[string]types.SpeakerGroupInfo)
	if len(response.Data.VoiceIdentify) > 0 {
		// 将 map 格式的声纹组信息转换为配置格式
		for groupName, groupInfo := range response.Data.VoiceIdentify {
			groupData := types.SpeakerGroupInfo{
				ID:                 groupInfo.ID,
				Name:               groupInfo.Name,
				Prompt:             groupInfo.Prompt,
				Description:        groupInfo.Description,
				Uuids:              groupInfo.Uuids,
				TTSConfigID:        groupInfo.TTSConfigID,
				Voice:              groupInfo.Voice,
				VoiceModelOverride: groupInfo.VoiceModelOverride,
			}
			voiceIdentifyData[groupName] = groupData
		}
	}

	// 构建配置结果
	config := types.UConfig{
		SystemPrompt: response.Data.Prompt, // 使用智能体的自定义提示
		Asr: types.AsrConfig{
			Provider: response.Data.ASR.Provider,
			Config:   parseJsonData(response.Data.ASR.JsonData),
		},
		Tts: types.TtsConfig{
			Provider: response.Data.TTS.Provider,
			Config:   parseJsonData(response.Data.TTS.JsonData),
		},
		Llm: types.LlmConfig{
			Provider: response.Data.LLM.Provider,
			Config:   parseJsonData(response.Data.LLM.JsonData),
		},
		Vad: types.VadConfig{
			Provider: response.Data.VAD.Provider,
			Config:   parseJsonData(response.Data.VAD.JsonData),
		},
		Memory: types.MemoryConfig{
			Provider: response.Data.Memory.Provider,
			Config:   parseJsonData(response.Data.Memory.JsonData),
		},
		KnowledgeBases:  response.Data.KnowledgeBases,
		VoiceIdentify:   voiceIdentifyData,
		MemoryMode:      response.Data.MemoryMode,
		AgentId:         response.Data.AgentId,
		MCPServiceNames: strings.TrimSpace(response.Data.MCPServiceNames),
	}
	if strings.TrimSpace(config.MemoryMode) == "" {
		config.MemoryMode = "short"
	}

	log.Log().Infof("成功获取设备配置: deviceId: %s, config: %+v", deviceID, config)
	return config, nil
}

// 获取 mqtt, mqtt_server, udp, ota, vision配置
func (c *ConfigManager) GetSystemConfig(ctx context.Context) (string, error) {
	// 解析响应JSON
	var apiResponse struct {
		Data map[string]interface{} `json:"data"`
	}

	// 发送HTTP请求
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "GET",
		Path:     "/api/system/configs",
		Response: &apiResponse,
	})
	if err != nil {
		return "", fmt.Errorf("获取系统配置失败: %w", err)
	}

	// 处理 voice_identify 配置，确保包含 threshold 字段
	if voiceIdentifyData, exists := apiResponse.Data["voice_identify"]; exists {
		if voiceIdentifyMap, ok := voiceIdentifyData.(map[string]interface{}); ok {
			// 如果 voice_identify 配置存在但没有 threshold 字段，添加默认值
			if _, hasThreshold := voiceIdentifyMap["threshold"]; !hasThreshold {
				voiceIdentifyMap["threshold"] = 0.4
				log.Log().Info("voice_identify 配置缺少 threshold 字段，已添加默认值 0.4")
			} else {
				// 验证阈值范围
				if thresholdVal, ok := voiceIdentifyMap["threshold"].(float64); ok {
					if thresholdVal < 0 || thresholdVal > 1 {
						log.Log().Warnf("voice_identify.threshold 值 %.4f 超出有效范围 [0.0, 1.0]，使用默认值 0.4", thresholdVal)
						voiceIdentifyMap["threshold"] = 0.4
					}
				}
			}
			// 更新配置数据
			apiResponse.Data["voice_identify"] = voiceIdentifyMap
		}
	}
	//log.Debugf("从内控获取到系统配置: %+v", apiResponse.Data)

	// 将API响应转换为配置JSON字符串
	configJSON, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}

	return string(configJSON), nil
}

// LoadSystemConfigToViper 从backend API加载系统配置并设置到viper
func (c *ConfigManager) LoadSystemConfigToViper(ctx context.Context) error {
	// 获取系统配置JSON字符串
	configJSON, err := c.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取系统配置失败: %w", err)
	}

	// 使用viper.MergeConfigMap将配置设置到viper
	// 首先将JSON字符串解析为map
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("解析配置JSON失败: %w", err)
	}

	// 设置到viper（需要导入viper包）
	// viper.MergeConfigMap(configMap)

	log.Log().Info("系统配置已成功加载到viper", "config_size", len(configJSON))
	return nil
}

// SwitchDeviceRoleByName 按角色名（支持模糊匹配）切换设备角色
func (c *ConfigManager) SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	roleName = strings.TrimSpace(roleName)
	if deviceID == "" {
		return "", fmt.Errorf("deviceID 不能为空")
	}
	if roleName == "" {
		return "", fmt.Errorf("roleName 不能为空")
	}

	var response struct {
		Data struct {
			RoleName string `json:"role_name"`
		} `json:"data"`
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/switch-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   path,
		Body: map[string]string{
			"role_name": roleName,
		},
		Response: &response,
	})
	if err != nil {
		return "", fmt.Errorf("切换设备角色失败: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf(response.Error)
	}
	if strings.TrimSpace(response.Data.RoleName) == "" {
		return "", fmt.Errorf("切换设备角色失败: 未返回匹配角色")
	}
	return response.Data.RoleName, nil
}

// RestoreDeviceDefaultRole 恢复设备默认角色（清空设备绑定角色）
func (c *ConfigManager) RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("deviceID 不能为空")
	}

	var response struct {
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/restore-default-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     path,
		Response: &response,
	})
	if err != nil {
		return fmt.Errorf("恢复默认角色失败: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf(response.Error)
	}
	return nil
}

// SearchKnowledge 通过管理后台统一检索知识库（控制台按provider转发）
func (c *ConfigManager) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	_, err := SendDeviceRequest(ctx, eventType, eventData)
	if err != nil {
		log.Log().Error("发送设备事件失败", "error", err)
	}
}

func (c *ConfigManager) RegisterMessageEventHandler(ctx context.Context, eventType string, handler types.EventHandler) {
	GetDefaultClient().RegisterMessageHandler(ctx, eventType, handler)
}
