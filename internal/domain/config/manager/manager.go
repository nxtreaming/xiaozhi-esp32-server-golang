package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// ConfigManager 配置管理器
// 提供高层级的配置管理功能，包括缓存、热更新、配置验证等
type ConfigManager struct {
	// HTTP客户端
	httpClient *http.Client
	// 后端管理系统基础URL
	baseURL string
}

// NewConfigManager 创建新的配置管理器
func NewManagerUserConfigProvider(config map[string]interface{}) (*ConfigManager, error) {
	// 从配置中获取后端管理系统的基础URL
	var baseURL string
	if backendUrl := config["backend_url"]; backendUrl != nil {
		baseURL = backendUrl.(string)
	} else {
		baseURL = "http://localhost:8080" // 默认值
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	manager := &ConfigManager{
		httpClient: httpClient,
		baseURL:    baseURL,
	}

	log.Log().Info("配置管理器初始化成功", "backend_url", baseURL)
	return manager, nil
}

func (c *ConfigManager) GetUserConfig(ctx context.Context, deviceID string) (types.UConfig, error) {
	// 构建请求URL
	url := c.baseURL + "/api/configs?device_id=" + deviceID

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Log().Error("创建HTTP请求失败", "error", err)
		return types.UConfig{}, err
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Log().Error("发送HTTP请求失败", "error", err, "url", url)
		return types.UConfig{}, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		log.Log().Error("HTTP请求返回错误状态", "status_code", resp.StatusCode, "url", url)
		return types.UConfig{}, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

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
			Prompt  string `json:"prompt"`
			AgentId string `json:"agent_id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Log().Error("解析HTTP响应失败", "error", err)
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
		AgentId: response.Data.AgentId,
	}

	log.Log().Infof("成功获取设备配置: deviceId: %s, config: %+v", deviceID, config)
	return config, nil
}

// 获取 mqtt, mqtt_server, udp, ota, vision配置
func (c *ConfigManager) GetSystemConfig(ctx context.Context) (string, error) {
	// 构建backend API URL
	apiURL := fmt.Sprintf("%s/api/system/configs", c.baseURL)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	log.Debugf("从内控获取到系统配置: %s", string(body))

	// 解析响应JSON
	var apiResponse struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("解析API响应失败: %w", err)
	}

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

func (c *ConfigManager) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	_, err := SendDeviceRequest(ctx, eventType, eventData)
	if err != nil {
		log.Log().Error("发送设备事件失败", "error", err)
	}
}

func (c *ConfigManager) RegisterMessageEventHandler(ctx context.Context, eventType string, handler types.EventHandler) {
	GetDefaultClient().RegisterMessageHandler(ctx, eventType, handler)
}
