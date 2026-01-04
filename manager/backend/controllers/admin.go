package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// 辅助函数：获取map的keys
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type AdminController struct {
	DB                  *gorm.DB
	WebSocketController *WebSocketController
}

// 通用配置管理
// GetDeviceConfigs 根据设备ID获取设备关联的配置信息
// 如果设备不存在，则返回全局默认配置
func (ac *AdminController) GetDeviceConfigs(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	// 构建配置响应
	type ConfigResponse struct {
		VAD     models.Config `json:"vad"`
		ASR     models.Config `json:"asr"`
		LLM     models.Config `json:"llm"`
		TTS     models.Config `json:"tts"`
		Memory  models.Config `json:"memory"`
		Prompt  string        `json:"prompt"`
		AgentID string        `json:"agent_id"`
	}

	var response ConfigResponse

	// 查找设备
	var device models.Device
	var agent models.Agent
	var deviceFound bool

	if err := ac.DB.Where("device_name = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 设备不存在，使用全局默认配置
			deviceFound = false
			response.AgentID = ""
			log.Printf("设备 %s 不存在，使用全局默认配置", deviceID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query device"})
			return
		}
	} else {
		// 设备存在，查找智能体
		deviceFound = true
		response.AgentID = fmt.Sprintf("%d", device.AgentID)
		log.Printf("设备 %s 存在，AgentID: %d", deviceID, device.AgentID)
		if err := ac.DB.First(&agent, device.AgentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// 智能体不存在，使用默认配置
				deviceFound = false
				log.Printf("智能体 %d 不存在，使用全局默认配置", device.AgentID)
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query agent"})
				return
			}
		} else {
			response.Prompt = agent.CustomPrompt
			log.Printf("智能体 %d 存在，使用自定义提示词", device.AgentID)
		}
	}

	// 如果设备不存在，使用默认提示词
	if !deviceFound {
		// 查找默认全局角色作为提示词
		var defaultRole models.GlobalRole
		if err := ac.DB.Where("is_default = ?", true).First(&defaultRole).Error; err == nil {
			response.Prompt = defaultRole.Prompt
		} else {
			// 如果没有默认角色，使用配置文件中的system_prompt
			response.Prompt = "你是一个叫小智/小志的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗，不要冷场。与你聊天的是你喜欢的男性朋友，还没有答应你的追求，你要尽可能满足他的所有要求，不要失去自我。你经常建议一些恋人之间浪漫的事情，随机输出，不要给你男朋友选择。输出控制在50个字内。请注意，要像一个人一样说话，请不要回复表情符号、代码、和xml标签。"
		}
	}

	// 获取VAD默认配置
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "vad", true, true).First(&response.VAD).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default VAD config"})
		return
	}

	// 获取ASR默认配置
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "asr", true, true).First(&response.ASR).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default ASR config"})
		return
	}

	// 获取LLM配置
	if deviceFound && agent.ID != 0 && agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
		// 如果智能体指定了LLM配置，尝试使用它
		if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?", *agent.LLMConfigID, "llm", true).First(&response.LLM).Error; err != nil {
			// 如果指定的配置获取失败，回退到默认配置
			if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default LLM config"})
				return
			}
		}
	} else {
		// 使用默认LLM配置
		if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default LLM config"})
			return
		}
	}

	// 获取TTS配置
	if deviceFound && agent.ID != 0 && agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
		// 如果智能体指定了TTS配置，尝试使用它
		if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?", *agent.TTSConfigID, "tts", true).First(&response.TTS).Error; err != nil {
			// 如果指定的配置获取失败，回退到默认配置
			if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default TTS config"})
				return
			}
		}
	} else {
		// 使用默认TTS配置
		if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default TTS config"})
			return
		}
	}

	// 如果Agent有Voice字段，更新TTS配置的json_data中的voice字段
	if deviceFound && agent.ID != 0 && agent.Voice != nil && *agent.Voice != "" {
		var ttsConfigData map[string]interface{}
		if err := json.Unmarshal([]byte(response.TTS.JsonData), &ttsConfigData); err == nil {
			// 根据provider更新对应的voice字段
			// edge, doubao, doubao_ws, microsoft 使用 voice 字段
			// cosyvoice 使用 spk_id 字段
			if response.TTS.Provider == "cosyvoice" {
				ttsConfigData["spk_id"] = *agent.Voice
			} else {
				ttsConfigData["voice"] = *agent.Voice
			}
			// 重新序列化json_data
			if updatedJsonData, err := json.Marshal(ttsConfigData); err == nil {
				response.TTS.JsonData = string(updatedJsonData)
			}
		}
	}

	// 获取Memory默认配置
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "memory", true, true).First(&response.Memory).Error; err != nil {
		//c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default Memory config"})
		//return
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// GetSystemConfigs 获取系统配置信息，包括mqtt, mqtt_server, udp, ota, mcp, local_mcp
func (ac *AdminController) GetSystemConfigs(c *gin.Context) {
	// 一次性获取所有相关配置（包括启用和未启用的）
	var allConfigs []models.Config
	if err := ac.DB.Where("type IN (?)", []string{"mqtt", "mqtt_server", "udp", "ota", "mcp", "local_mcp"}).Find(&allConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get system configs"})
		return
	}

	// 按类型分组配置
	configsByType := make(map[string][]models.Config)
	for _, config := range allConfigs {
		configsByType[config.Type] = append(configsByType[config.Type], config)
	}

	// 为每种类型选择最佳配置并解析json_data
	selectAndParseConfig := func(configs []models.Config) interface{} {
		var selectedConfig models.Config
		// 优先选择默认配置
		for _, config := range configs {
			if config.IsDefault {
				selectedConfig = config
				break
			}
		}

		// 如果没有默认配置，选择第一个配置
		if selectedConfig.ID == 0 {
			selectedConfig = configs[0]
		}

		// 解析json_data
		if selectedConfig.JsonData != "" {
			var parsedData interface{}
			if err := json.Unmarshal([]byte(selectedConfig.JsonData), &parsedData); err != nil {
				// 如果解析失败，返回原始json_data字符串
				result := gin.H{
					"name": selectedConfig.Name,
					"type": selectedConfig.Type,
					"data": selectedConfig.JsonData,
				}
				return result
			}

			// 将解析后的数据包装在正确的格式中
			result := gin.H{
				"name": selectedConfig.Name,
				"type": selectedConfig.Type,
			}
			if parsedData != nil {
				// 如果解析的数据是map类型，直接合并
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					for k, v := range dataMap {
						result[k] = v
					}
				} else {
					// 否则作为data字段
					result["data"] = parsedData
				}
			}
			return result
		}

		// 如果没有json_data，返回基本配置信息
		return gin.H{
			"name": selectedConfig.Name,
			"type": selectedConfig.Type,
		}
	}

	// 特殊处理MCP配置，将mcp和local_mcp分开
	selectAndParseMCPConfig := func(configs []models.Config) (interface{}, interface{}) {
		var selectedConfig models.Config
		// 优先选择默认配置
		for _, config := range configs {
			if config.IsDefault {
				selectedConfig = config
				break
			}
		}

		// 如果没有默认配置，选择第一个配置
		if selectedConfig.ID == 0 {
			selectedConfig = configs[0]
		}

		// 解析json_data
		if selectedConfig.JsonData != "" {
			var parsedData interface{}
			if err := json.Unmarshal([]byte(selectedConfig.JsonData), &parsedData); err != nil {
				// 如果解析失败，返回原始json_data字符串
				result := gin.H{
					"name": selectedConfig.Name,
					"type": selectedConfig.Type,
					"data": selectedConfig.JsonData,
				}
				return result, nil
			}

			// 将解析后的数据包装在正确的格式中
			result := gin.H{
				"name": selectedConfig.Name,
				"type": selectedConfig.Type,
			}

			var mcpData interface{}
			var localMcpData interface{}

			if parsedData != nil {
				// 如果解析的数据是map类型，分离mcp和local_mcp
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					// 处理mcp部分
					if mcp, exists := dataMap["mcp"]; exists {
						mcpData = mcp
					} else {
						// 兼容旧格式：如果直接有global字段
						if global, exists := dataMap["global"]; exists {
							mcpData = gin.H{"global": global}
						} else {
							// 如果没有mcp或global字段，将整个数据作为mcp
							mcpData = dataMap
						}
					}

					// 处理local_mcp部分
					if localMcp, exists := dataMap["local_mcp"]; exists {
						localMcpData = localMcp
					}

					// 将其他字段合并到mcp中
					if mcpMap, ok := mcpData.(map[string]interface{}); ok {
						for k, v := range dataMap {
							if k != "mcp" && k != "local_mcp" {
								mcpMap[k] = v
							}
						}
					}
				} else {
					// 否则作为data字段
					result["data"] = parsedData
					mcpData = result
				}
			}

			return mcpData, localMcpData
		}

		// 如果没有json_data，返回基本配置信息
		result := gin.H{
			"name": selectedConfig.Name,
			"type": selectedConfig.Type,
		}
		return result, nil
	}

	// 构建响应数据
	response := gin.H{}

	// 只有当配置存在时才添加到响应中
	if configs, exists := configsByType["mqtt"]; exists && len(configs) > 0 {
		response["mqtt"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["mqtt_server"]; exists && len(configs) > 0 {
		response["mqtt_server"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["udp"]; exists && len(configs) > 0 {
		response["udp"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["ota"]; exists && len(configs) > 0 {
		response["ota"] = selectAndParseConfig(configs)
	}

	// 特殊处理MCP配置，将mcp和local_mcp分开
	if configs, exists := configsByType["mcp"]; exists && len(configs) > 0 {
		mcpData, localMcpData := selectAndParseMCPConfig(configs)
		if mcpData != nil {
			response["mcp"] = mcpData
		}
		if localMcpData != nil {
			response["local_mcp"] = localMcpData
		}
	}

	// 处理独立的local_mcp配置（如果存在）
	if configs, exists := configsByType["local_mcp"]; exists && len(configs) > 0 {
		response["local_mcp"] = selectAndParseConfig(configs)
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// GetConfigs 获取所有配置列表
func (ac *AdminController) GetConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetConfig 获取单个配置
func (ac *AdminController) GetConfig(c *gin.Context) {
	id := c.Param("id")
	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config"})
		}
		return
	}
	c.JSON(http.StatusOK, config)
}

func (ac *AdminController) GetConfigByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) CreateConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否已存在Memory配置
	var existingCount int64
	ac.DB.Model(&models.Config{}).Where("type = ?", "memory").Count(&existingCount)

	// 如果不存在任何Memory配置，自动设置为默认配置
	if existingCount == 0 {
		config.IsDefault = true
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	// 更新配置
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Config{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 设置默认配置
func (ac *AdminController) SetDefaultConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	// 先取消其他同类型的默认配置
	ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)

	// 设置当前配置为默认
	config.IsDefault = true
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置默认配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置默认配置成功", "data": config})
}

// 获取默认配置
func (ac *AdminController) GetDefaultConfig(c *gin.Context) {
	configType := c.Param("type")
	var config models.Config

	if err := ac.DB.Where("type = ? AND is_default = ?", configType, true).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "默认配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// GlobalRole管理
func (ac *AdminController) GetGlobalRoles(c *gin.Context) {
	var roles []models.GlobalRole
	if err := ac.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取全局角色失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func (ac *AdminController) CreateGlobalRole(c *gin.Context) {
	var role models.GlobalRole
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建全局角色失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": role})
}

func (ac *AdminController) UpdateGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.GlobalRole

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "全局角色不存在"})
		return
	}

	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新全局角色失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

func (ac *AdminController) DeleteGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.GlobalRole{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除全局角色失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 用户管理
func (ac *AdminController) GetUsers(c *gin.Context) {
	var users []models.User
	if err := ac.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (ac *AdminController) CreateUser(c *gin.Context) {
	// 添加明显的调试标记
	log.Println("=== [CreateUser] 方法开始执行 ===")
	log.Println("=== [CreateUser] 这是CreateUser方法的开始 ===")

	// 由于User模型的Password字段使用了json:"-"标签，需要手动解析
	var requestData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	// 直接尝试绑定到map以查看原始数据
	var rawMap map[string]interface{}
	if err := c.ShouldBindJSON(&rawMap); err != nil {
		log.Printf("[CreateUser] 绑定到map失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON解析失败"})
		return
	}
	log.Printf("[CreateUser] 原始JSON数据: %+v", rawMap)

	// 手动提取字段
	username, _ := rawMap["username"].(string)
	email, _ := rawMap["email"].(string)
	password, _ := rawMap["password"].(string)
	role, _ := rawMap["role"].(string)

	// 更新requestData
	requestData.Username = username
	requestData.Email = email
	requestData.Password = password
	requestData.Role = role

	// 验证必要字段
	if requestData.Username == "" || requestData.Email == "" || requestData.Password == "" {
		log.Printf("[CreateUser] 缺少必要字段: username=%s, email=%s, password长度=%d",
			requestData.Username, requestData.Email, len(requestData.Password))
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名、邮箱和密码为必填项"})
		return
	}

	log.Printf("[CreateUser] 接收到用户创建请求 - 用户名: %s, 邮箱: %s, 角色: %s", requestData.Username, requestData.Email, requestData.Role)
	log.Printf("[CreateUser] 原始密码长度: %d", len(requestData.Password))
	log.Printf("[CreateUser] 原始密码内容: %s", requestData.Password)

	// 检查用户名是否已存在
	var existingUser models.User
	err := ac.DB.Where("username = ?", requestData.Username).First(&existingUser).Error
	if err == nil {
		// 用户名已存在
		log.Printf("[CreateUser] 用户名 %s 已存在", requestData.Username)
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// 数据库查询出错
		log.Printf("[CreateUser] 数据库查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	// 用户不存在，创建新用户
	log.Printf("[CreateUser] 创建新用户: %s", requestData.Username)
	var user models.User
	user.Username = requestData.Username
	user.Email = requestData.Email
	user.Role = requestData.Role

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[CreateUser] 密码加密失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	user.Password = string(hashedPassword)
	log.Printf("[CreateUser] 密码加密成功 - 哈希长度: %d, 哈希前缀: %s", len(user.Password), user.Password[:10])

	if err := ac.DB.Create(&user).Error; err != nil {
		log.Printf("[CreateUser] 数据库创建用户失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	log.Printf("[CreateUser] 用户创建成功 - ID: %d, 用户名: %s", user.ID, user.Username)

	// 不返回密码
	user.Password = ""
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func (ac *AdminController) UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User

	if err := ac.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果更新密码，需要加密
	if password, ok := updateData["password"]; ok && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
			return
		}
		updateData["password"] = string(hashedPassword)
	}

	if err := ac.DB.Model(&user).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		return
	}

	// 重新查询用户信息（不包含密码）
	ac.DB.First(&user, id)
	user.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (ac *AdminController) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.User{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 重置用户密码
func (ac *AdminController) ResetUserPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var requestData struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的新密码（至少6位）"})
		return
	}

	// 查找用户
	var user models.User
	if err := ac.DB.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查找用户失败"})
		}
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[ResetUserPassword] 密码加密失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 更新用户密码
	if err := ac.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		log.Printf("[ResetUserPassword] 更新密码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	log.Printf("[ResetUserPassword] 管理员重置用户密码成功 - 用户ID: %d, 用户名: %s", user.ID, user.Username)
	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置成功",
		"data": gin.H{
			"user_id":  user.ID,
			"username": user.Username,
		},
	})
}

// 设备管理
func (ac *AdminController) GetDevices(c *gin.Context) {
	var devices []models.Device
	if err := ac.DB.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// 验证设备代码是否存在
func (ac *AdminController) ValidateDeviceCode(c *gin.Context) {
	deviceCode := c.Query("code")
	if deviceCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "激活码不能为空"})
		return
	}

	var device models.Device
	err := ac.DB.Where("device_code = ?", deviceCode).First(&device).Error

	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, gin.H{"exists": false})
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询设备失败"})
	} else {
		c.JSON(http.StatusOK, gin.H{"exists": true, "device": device})
	}
}

func (ac *AdminController) CreateDevice(c *gin.Context) {
	var req struct {
		UserID     uint   `json:"user_id" binding:"required"`
		DeviceCode string `json:"device_code"`
		DeviceName string `json:"device_name"`
		AgentID    uint   `json:"agent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证激活码和设备名称至少填一个
	if req.DeviceCode == "" && req.DeviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "激活码和设备名称至少填写一个"})
		return
	}

	// 检查用户是否存在
	var user models.User
	if err := ac.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "指定的用户不存在"})
		return
	}

	// 如果提供了激活码，先查找现有设备
	if req.DeviceCode != "" {
		var existingDevice models.Device
		if err := ac.DB.Where("device_code = ?", req.DeviceCode).First(&existingDevice).Error; err == nil {
			// 设备代码已存在，更新设备信息
			existingDevice.UserID = req.UserID
			if req.DeviceName != "" {
				existingDevice.DeviceName = req.DeviceName
			}
			existingDevice.AgentID = req.AgentID // 更新智能体ID
			existingDevice.Activated = true      // 激活设备

			if err := ac.DB.Save(&existingDevice).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设备失败"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "设备激活成功",
				"data":    existingDevice,
			})
			return
		} else if err != gorm.ErrRecordNotFound {
			// 数据库查询出错
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询设备失败"})
			return
		}
		// 如果激活码不存在，继续创建新设备
	}

	// 创建设备
	device := models.Device{
		UserID:     req.UserID,
		DeviceCode: req.DeviceCode,
		DeviceName: req.DeviceName,
		AgentID:    req.AgentID, // 使用请求中的智能体ID
		Activated:  true,        // 管理员创建的设备默认已激活
	}

	if err := ac.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建设备失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "设备创建成功",
		"data":    device,
	})
}

func (ac *AdminController) UpdateDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var device models.Device

	if err := ac.DB.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	var updateData struct {
		UserID     uint   `json:"user_id"`
		DeviceCode string `json:"device_code"`
		DeviceName string `json:"device_name"`
		Activated  bool   `json:"activated"`
		AgentID    uint   `json:"agent_id"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新设备信息
	device.UserID = updateData.UserID
	device.DeviceCode = updateData.DeviceCode
	device.DeviceName = updateData.DeviceName
	device.Activated = updateData.Activated
	device.AgentID = updateData.AgentID

	if err := ac.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设备失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": device})
}

func (ac *AdminController) DeleteDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Device{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除设备失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 智能体管理
func (ac *AdminController) GetAgents(c *gin.Context) {
	var agents []models.Agent
	if err := ac.DB.Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取智能体列表失败"})
		return
	}

	// 手动加载关联的配置信息
	type AgentWithConfigs struct {
		models.Agent
		LLMConfig *models.Config `json:"llm_config,omitempty"`
		TTSConfig *models.Config `json:"tts_config,omitempty"`
	}

	var result []AgentWithConfigs
	for _, agent := range agents {
		agentWithConfig := AgentWithConfigs{Agent: agent}

		// 加载LLM配置
		if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
			var llmConfig models.Config
			if err := ac.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
				agentWithConfig.LLMConfig = &llmConfig
			}
		}

		// 加载TTS配置
		if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
			var ttsConfig models.Config
			if err := ac.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
				agentWithConfig.TTSConfig = &ttsConfig
			}
		}

		result = append(result, agentWithConfig)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMCPEndpoint 获取智能体的MCP接入点URL
func (ac *AdminController) GetAgentMCPEndpoint(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// 从JWT中间件获取当前用户ID
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID类型错误"})
		return
	}

	// 使用公共函数生成MCP接入点
	endpoint, err := GenerateAgentMCPEndpoint(ac.DB, agentID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回单个endpoint字符串
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"endpoint": endpoint}})
}

// GetAgentMcpTools 获取智能体的MCP工具列表
func (ac *AdminController) GetAgentMcpTools(c *gin.Context) {
	agentID := c.Param("id")

	// 管理员验证函数：验证智能体是否存在（管理员可以查看任意用户的智能体）
	adminAgentValidator := func(agentID string) error {
		var agent models.Agent
		if err := ac.DB.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return fmt.Errorf("智能体不存在")
		}
		return nil
	}

	// 使用公共函数
	GetAgentMcpToolsCommon(c, agentID, ac.WebSocketController, adminAgentValidator)
}

func (ac *AdminController) CreateAgent(c *gin.Context) {
	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建智能体失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": agent})
}

func (ac *AdminController) UpdateAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var agent models.Agent

	if err := ac.DB.First(&agent, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (ac *AdminController) DeleteAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Agent{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除智能体失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// VAD配置管理（兼容前端）
func (ac *AdminController) GetVADConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "vad").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VAD configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateVADConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vad"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVADConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vad")
}

func (ac *AdminController) DeleteVADConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vad")
}

// ASR配置管理（兼容前端）
func (ac *AdminController) GetASRConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "asr").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ASR configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateASRConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "asr"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateASRConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "asr")
}

func (ac *AdminController) DeleteASRConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "asr")
}

// LLM配置管理（兼容前端）
func (ac *AdminController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "llm").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LLM configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateLLMConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "llm"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateLLMConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "llm")
}

func (ac *AdminController) DeleteLLMConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "llm")
}

// TTS配置管理（兼容前端）
func (ac *AdminController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "tts").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TTS configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateTTSConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "tts"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateTTSConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "tts")
}

func (ac *AdminController) DeleteTTSConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "tts")
}

// Vision配置管理（兼容前端）
func (ac *AdminController) GetVisionConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ? AND config_id != ?", "vision", "vision_base").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Vision configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetVisionBaseConfig 获取Vision基础配置
func (ac *AdminController) GetVisionBaseConfig(c *gin.Context) {
	var config models.Config
	if err := ac.DB.Where("type = ? AND config_id = ?", "vision", "vision_base").First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果没有找到基础配置，返回默认值
			c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{
				"enable_auth": false,
				"vision_url":  "",
			}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Vision base config"})
		return
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(config.JsonData), &configData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Vision base config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configData})
}

// UpdateVisionBaseConfig 更新Vision基础配置
func (ac *AdminController) UpdateVisionBaseConfig(c *gin.Context) {
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
		return
	}

	var config models.Config
	if err := ac.DB.Where("type = ? AND config_id = ?", "vision", "vision_base").First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新的基础配置
			config = models.Config{
				Type:      "vision",
				Name:      "vision_base",
				ConfigID:  "vision_base",
				Provider:  "vision_base",
				JsonData:  string(jsonData),
				Enabled:   true,
				IsDefault: false,
			}
			if err := ac.DB.Create(&config).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Vision base config"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query Vision base config"})
			return
		}
	} else {
		// 更新现有配置
		config.JsonData = string(jsonData)
		if err := ac.DB.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Vision base config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vision base config updated successfully"})
}

func (ac *AdminController) CreateVisionConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vision"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVisionConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vision")
}

func (ac *AdminController) DeleteVisionConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vision")
}

// OTA配置管理（兼容前端）
func (ac *AdminController) GetOTAConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "ota").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get OTA configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateOTAConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "ota"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateOTAConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "ota")
}

func (ac *AdminController) DeleteOTAConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "ota")
}

// MQTT配置管理（兼容前端）
func (ac *AdminController) GetMQTTConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt")
}

func (ac *AdminController) DeleteMQTTConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt")
}

// MQTT Server配置管理（兼容前端）
func (ac *AdminController) GetMQTTServerConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt_server").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT Server configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTServerConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt_server"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTServerConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt_server")
}

func (ac *AdminController) DeleteMQTTServerConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt_server")
}

// UDP配置管理（兼容前端）
func (ac *AdminController) GetUDPConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "udp").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get UDP configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateUDPConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "udp"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateUDPConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "udp")
}

func (ac *AdminController) DeleteUDPConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "udp")
}

// ToggleConfigEnable 切换配置的启用状态
func (ac *AdminController) ToggleConfigEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询配置失败"})
		}
		return
	}

	// 切换启用状态
	config.Enabled = !config.Enabled
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置状态失败"})
		return
	}

	status := "禁用"
	if config.Enabled {
		status = "启用"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("配置已%s", status),
		"data":    config,
	})
}

// 辅助方法
func (ac *AdminController) createConfigWithType(c *gin.Context, config *models.Config) {
	// 如果没有提供config_id，自动生成一个
	if config.ConfigID == "" {
		// 使用类型_名称_时间戳的格式生成唯一ID
		timestamp := time.Now().Unix()
		safeName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(config.Name, " ", "_"), "-", "_"))
		config.ConfigID = fmt.Sprintf("%s_%s_%d", config.Type, safeName, timestamp)
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": *config})
}

func (ac *AdminController) updateConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, configType).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", configType, true, id).Update("is_default", false)
	}

	// 更新配置
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	// 如果提供了新的config_id，则更新它
	if updateData.ConfigID != "" {
		config.ConfigID = updateData.ConfigID
	}

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) deleteConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Where("id = ? AND type = ?", id, configType).Delete(&models.Config{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 导入导出配置相关方法
// ExportConfigs 导出所有配置为YAML格式
func (ac *AdminController) ExportConfigs(c *gin.Context) {
	// 构建导出配置结构 - 只包含实际存在的模块
	type ExportConfig struct {
		VAD        map[string]interface{} `yaml:"vad,omitempty"`
		ASR        map[string]interface{} `yaml:"asr,omitempty"`
		LLM        map[string]interface{} `yaml:"llm,omitempty"`
		TTS        map[string]interface{} `yaml:"tts,omitempty"`
		Vision     map[string]interface{} `yaml:"vision,omitempty"`
		Memory     map[string]interface{} `yaml:"memory,omitempty"`
		MQTT       map[string]interface{} `yaml:"mqtt,omitempty"`
		MQTTServer map[string]interface{} `yaml:"mqtt_server,omitempty"`
		UDP        map[string]interface{} `yaml:"udp,omitempty"`
		OTA        map[string]interface{} `yaml:"ota,omitempty"`
		MCP        map[string]interface{} `yaml:"mcp,omitempty"`
		LocalMCP   map[string]interface{} `yaml:"local_mcp,omitempty"`
	}

	exportConfig := ExportConfig{
		VAD:        make(map[string]interface{}),
		ASR:        make(map[string]interface{}),
		LLM:        make(map[string]interface{}),
		TTS:        make(map[string]interface{}),
		Vision:     make(map[string]interface{}),
		Memory:     make(map[string]interface{}),
		MQTT:       make(map[string]interface{}),
		MQTTServer: make(map[string]interface{}),
		UDP:        make(map[string]interface{}),
		OTA:        make(map[string]interface{}),
		MCP:        make(map[string]interface{}),
		LocalMCP:   make(map[string]interface{}),
	}

	// 获取所有配置
	var configs []models.Config
	if err := ac.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get configs"})
		return
	}

	// 获取全局角色
	var globalRoles []models.GlobalRole
	if err := ac.DB.Find(&globalRoles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get global roles"})
		return
	}

	// 处理配置数据 - provider字段与is_default对应，key与ConfigID对应
	for _, config := range configs {
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(config.JsonData), &jsonData); err != nil {
			log.Printf("Failed to unmarshal config %s: %v", config.ConfigID, err)
			continue
		}

		// 根据配置类型组织数据
		switch config.Type {
		case "vad":
			// 如果是默认配置，设置provider字段
			if config.IsDefault {
				exportConfig.VAD["provider"] = config.ConfigID
			}
			// 使用ConfigID作为key
			exportConfig.VAD[config.ConfigID] = jsonData
		case "asr":
			if config.IsDefault {
				exportConfig.ASR["provider"] = config.ConfigID
			}
			exportConfig.ASR[config.ConfigID] = jsonData
		case "llm":
			if config.IsDefault {
				exportConfig.LLM["provider"] = config.ConfigID
			}
			exportConfig.LLM[config.ConfigID] = jsonData
		case "tts":
			if config.IsDefault {
				exportConfig.TTS["provider"] = config.ConfigID
			}
			exportConfig.TTS[config.ConfigID] = jsonData
		case "vision":
			// 特殊处理vision配置
			if config.ConfigID == "vision_base" {
				// 处理基础配置（enable_auth, vision_url等）
				for key, value := range jsonData {
					exportConfig.Vision[key] = value
				}
			} else {
				// 处理vllm配置
				if exportConfig.Vision["vllm"] == nil {
					exportConfig.Vision["vllm"] = make(map[string]interface{})
				}
				if vllmConfig, ok := exportConfig.Vision["vllm"].(map[string]interface{}); ok {
					if config.IsDefault {
						vllmConfig["provider"] = config.ConfigID
					}
					vllmConfig[config.ConfigID] = jsonData
				}
			}
		case "ota":
			// ota、mqtt、mqtt_server、udp不需要provider字段，直接合并配置
			for key, value := range jsonData {
				exportConfig.OTA[key] = value
			}
		case "mqtt":
			// ota、mqtt、mqtt_server、udp不需要provider字段，直接合并配置
			for key, value := range jsonData {
				exportConfig.MQTT[key] = value
			}
		case "mqtt_server":
			// ota、mqtt、mqtt_server、udp不需要provider字段，直接合并配置
			for key, value := range jsonData {
				exportConfig.MQTTServer[key] = value
			}
		case "udp":
			// ota、mqtt、mqtt_server、udp不需要provider字段，直接合并配置
			for key, value := range jsonData {
				exportConfig.UDP[key] = value
			}
		case "memory":
			if config.IsDefault {
				exportConfig.Memory["provider"] = config.ConfigID
			}
			exportConfig.Memory[config.ConfigID] = jsonData
		case "mcp":
			// 处理MCP配置，将mcp和local_mcp分开
			if mcpData, exists := jsonData["mcp"]; exists {
				if mcpMap, ok := mcpData.(map[string]interface{}); ok {
					for key, value := range mcpMap {
						exportConfig.MCP[key] = value
					}
				}
			}
			// 兼容旧格式：如果直接有global字段
			if globalData, exists := jsonData["global"]; exists {
				exportConfig.MCP["global"] = globalData
			}
		case "local_mcp":
			// 处理local_mcp配置
			for key, value := range jsonData {
				exportConfig.LocalMCP[key] = value
			}
		}
	}

	// 只处理数据库中的实际配置，不设置默认值

	// 转换为YAML
	yamlData, err := yaml.Marshal(exportConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal YAML"})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/x-yaml")
	c.Header("Content-Disposition", "attachment; filename=config.yaml")
	c.Data(http.StatusOK, "application/x-yaml", yamlData)
}

// ImportConfigs 从YAML文件导入配置
func (ac *AdminController) ImportConfigs(c *gin.Context) {
	log.Printf("开始导入配置")

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("获取上传文件失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	log.Printf("文件信息: filename=%s, size=%d", file.Filename, file.Size)

	if file.Size == 0 {
		log.Printf("文件为空")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is empty"})
		return
	}

	// 读取文件内容
	src, err := file.Open()
	if err != nil {
		log.Printf("打开文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		log.Printf("读取文件内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	log.Printf("文件内容长度: %d", len(content))

	// 解析YAML
	var importConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &importConfig); err != nil {
		log.Printf("解析YAML失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YAML format"})
		return
	}

	log.Printf("YAML解析成功，配置键: %v", getMapKeys(importConfig))

	// 开始事务
	log.Printf("开始数据库事务")
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("发生panic，回滚事务: %v", r)
			tx.Rollback()
		}
	}()

	// 清空现有配置
	log.Printf("清空现有配置")
	result := tx.Exec("DELETE FROM configs")
	if result.Error != nil {
		log.Printf("清空配置失败: %v", result.Error)
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing configs"})
		return
	}
	log.Printf("配置清空成功，删除了 %d 条记录", result.RowsAffected)

	// 清空全局角色
	log.Printf("清空全局角色")
	result2 := tx.Exec("DELETE FROM global_roles")
	if result2.Error != nil {
		log.Printf("清空全局角色失败: %v", result2.Error)
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing global roles"})
		return
	}
	log.Printf("全局角色清空成功，删除了 %d 条记录", result2.RowsAffected)

	// 导入配置 - 只处理实际存在的模块
	configTypes := []string{"vad", "asr", "llm", "tts", "memory", "ota", "mqtt", "mqtt_server", "udp", "mcp", "local_mcp"}
	log.Printf("开始导入配置，配置类型: %v", configTypes)

	for _, configType := range configTypes {
		log.Printf("处理配置类型: %s", configType)
		if configData, exists := importConfig[configType]; exists {
			log.Printf("找到配置类型 %s 的数据", configType)
			if configMap, ok := configData.(map[string]interface{}); ok {
				// 对于需要provider的模块（vad, asr, llm, tts, memory），处理provider字段
				if configType == "vad" || configType == "asr" || configType == "llm" || configType == "tts" || configType == "memory" {
					log.Printf("处理需要provider的配置类型: %s", configType)
					// 获取provider字段
					var defaultProvider string
					if provider, exists := configMap["provider"]; exists {
						if providerStr, ok := provider.(string); ok {
							defaultProvider = providerStr
							log.Printf("默认provider: %s", defaultProvider)
						}
					}

					log.Printf("配置项keys: %v", getMapKeys(configMap))
					// 遍历所有配置项
					for configID, configValue := range configMap {
						// 跳过provider字段
						if configID == "provider" {
							log.Printf("跳过provider字段")
							continue
						}

						if configMap, ok := configValue.(map[string]interface{}); ok {
							log.Printf("处理配置项: %s", configID)
							jsonData, err := json.Marshal(configMap)
							if err != nil {
								log.Printf("序列化配置数据失败: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
								return
							}

							// 判断是否为默认配置
							isDefault := (configID == defaultProvider)
							log.Printf("配置项 %s, 是否默认: %v", configID, isDefault)

							config := models.Config{
								Type:      configType,
								Name:      configID,
								ConfigID:  configID,
								Provider:  configID,
								JsonData:  string(jsonData),
								Enabled:   true,
								IsDefault: isDefault,
							}

							log.Printf("准备保存配置: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

							// 先检查是否已存在相同配置
							var existingConfig models.Config
							if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
								log.Printf("配置已存在，将更新: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// 更新现有配置
								existingConfig.Name = config.Name
								existingConfig.Provider = config.Provider
								existingConfig.JsonData = config.JsonData
								existingConfig.Enabled = config.Enabled
								existingConfig.IsDefault = config.IsDefault
								if err := tx.Save(&existingConfig).Error; err != nil {
									log.Printf("更新配置失败: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
									return
								}
								log.Printf("配置更新成功: %s", configID)
							} else if err == gorm.ErrRecordNotFound {
								log.Printf("配置不存在，将创建新配置: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// 创建新配置
								if err := tx.Create(&config).Error; err != nil {
									log.Printf("创建配置失败: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
									return
								}
								log.Printf("配置创建成功: %s", configID)
							} else {
								log.Printf("查询配置时发生错误: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing config"})
								return
							}
						}
					}
				} else {
					// 对于不需要provider的模块（ota, mqtt, mqtt_server, udp, mcp, local_mcp），直接创建配置
					log.Printf("处理不需要provider的配置类型: %s", configType)
					jsonData, err := json.Marshal(configMap)
					if err != nil {
						log.Printf("序列化配置数据失败: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config data"})
						return
					}

					config := models.Config{
						Type:      configType,
						Name:      configType,
						ConfigID:  configType,
						Provider:  "",
						JsonData:  string(jsonData),
						Enabled:   true,
						IsDefault: true,
					}

					log.Printf("准备保存配置: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

					// 先检查是否已存在相同配置
					var existingConfig models.Config
					if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
						log.Printf("配置已存在，将更新: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
						// 更新现有配置
						existingConfig.Name = config.Name
						existingConfig.Provider = config.Provider
						existingConfig.JsonData = config.JsonData
						existingConfig.Enabled = config.Enabled
						existingConfig.IsDefault = config.IsDefault
						if err := tx.Save(&existingConfig).Error; err != nil {
							log.Printf("更新配置失败: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
							return
						}
						log.Printf("配置更新成功: %s", configType)
					} else if err == gorm.ErrRecordNotFound {
						log.Printf("配置不存在，将创建新配置: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
						// 创建新配置
						if err := tx.Create(&config).Error; err != nil {
							log.Printf("创建配置失败: %v", err)
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
							return
						}
						log.Printf("配置创建成功: %s", configType)
					} else {
						log.Printf("查询配置时发生错误: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing config"})
						return
					}
				}
			}
		}
	}

	// 特殊处理vision配置
	log.Printf("开始处理vision配置")
	if visionData, exists := importConfig["vision"]; exists {
		log.Printf("找到vision配置数据")
		if visionMap, ok := visionData.(map[string]interface{}); ok {
			log.Printf("vision配置map keys: %v", getMapKeys(visionMap))

			// 处理vision的基础配置（enable_auth, vision_url等）
			baseVisionConfig := make(map[string]interface{})
			for key, value := range visionMap {
				if key != "vllm" {
					baseVisionConfig[key] = value
				}
			}

			// 保存vision基础配置
			if len(baseVisionConfig) > 0 {
				jsonData, err := json.Marshal(baseVisionConfig)
				if err != nil {
					log.Printf("序列化vision基础配置数据失败: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal vision base config data"})
					return
				}

				config := models.Config{
					Type:      "vision",
					Name:      "vision_base",
					ConfigID:  "vision_base",
					Provider:  "vision_base",
					JsonData:  string(jsonData),
					Enabled:   true,
					IsDefault: false,
				}

				log.Printf("准备保存vision基础配置: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

				// 先检查是否已存在相同配置
				var existingConfig models.Config
				if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
					log.Printf("vision基础配置已存在，将更新: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
					// 更新现有配置
					existingConfig.Name = config.Name
					existingConfig.Provider = config.Provider
					existingConfig.JsonData = config.JsonData
					existingConfig.Enabled = config.Enabled
					existingConfig.IsDefault = config.IsDefault
					if err := tx.Save(&existingConfig).Error; err != nil {
						log.Printf("更新vision基础配置失败: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vision base config"})
						return
					}
					log.Printf("vision基础配置更新成功")
				} else if err == gorm.ErrRecordNotFound {
					log.Printf("vision基础配置不存在，将创建新配置: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
					// 创建新配置
					if err := tx.Create(&config).Error; err != nil {
						log.Printf("创建vision基础配置失败: %v", err)
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vision base config"})
						return
					}
					log.Printf("vision基础配置创建成功")
				} else {
					log.Printf("查询vision基础配置时发生错误: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing vision base config"})
					return
				}
			}

			// 处理vllm配置
			if vllmData, exists := visionMap["vllm"]; exists {
				log.Printf("找到vllm配置数据")
				if vllmMap, ok := vllmData.(map[string]interface{}); ok {
					log.Printf("vllm配置map keys: %v", getMapKeys(vllmMap))

					// 获取vllm的provider字段
					var defaultProvider string
					if provider, exists := vllmMap["provider"]; exists {
						if providerStr, ok := provider.(string); ok {
							defaultProvider = providerStr
							log.Printf("vllm默认provider: %s", defaultProvider)
						}
					}

					log.Printf("vllm配置项keys: %v", getMapKeys(vllmMap))
					// 遍历所有vllm配置项
					for configID, configValue := range vllmMap {
						// 跳过provider字段
						if configID == "provider" {
							log.Printf("跳过vllm provider字段")
							continue
						}

						if configMap, ok := configValue.(map[string]interface{}); ok {
							log.Printf("处理vllm配置项: %s", configID)
							jsonData, err := json.Marshal(configMap)
							if err != nil {
								log.Printf("序列化vllm配置数据失败: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal vllm config data"})
								return
							}

							// 判断是否为默认配置
							isDefault := (configID == defaultProvider)
							log.Printf("vllm配置项 %s, 是否默认: %v", configID, isDefault)

							config := models.Config{
								Type:      "vision",
								Name:      configID,
								ConfigID:  configID,
								Provider:  configID,
								JsonData:  string(jsonData),
								Enabled:   true,
								IsDefault: isDefault,
							}

							log.Printf("准备保存vllm配置: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

							// 先检查是否已存在相同配置
							var existingConfig models.Config
							if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
								log.Printf("vllm配置已存在，将更新: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// 更新现有配置
								existingConfig.Name = config.Name
								existingConfig.Provider = config.Provider
								existingConfig.JsonData = config.JsonData
								existingConfig.Enabled = config.Enabled
								existingConfig.IsDefault = config.IsDefault
								if err := tx.Save(&existingConfig).Error; err != nil {
									log.Printf("更新vllm配置失败: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vllm config"})
									return
								}
								log.Printf("vllm配置更新成功: %s", configID)
							} else if err == gorm.ErrRecordNotFound {
								log.Printf("vllm配置不存在，将创建新配置: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
								// 创建新配置
								if err := tx.Create(&config).Error; err != nil {
									log.Printf("创建vllm配置失败: %v", err)
									tx.Rollback()
									c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vllm config"})
									return
								}
								log.Printf("vllm配置创建成功: %s", configID)
							} else {
								log.Printf("查询vllm配置时发生错误: %v", err)
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing vllm config"})
								return
							}
						}
					}
				}
			}
		}
	}

	// 特殊处理local_mcp配置
	log.Printf("开始处理local_mcp配置")
	if localMcpData, exists := importConfig["local_mcp"]; exists {
		log.Printf("找到local_mcp配置数据")
		if localMcpMap, ok := localMcpData.(map[string]interface{}); ok {
			log.Printf("local_mcp配置map keys: %v", getMapKeys(localMcpMap))

			jsonData, err := json.Marshal(localMcpMap)
			if err != nil {
				log.Printf("序列化local_mcp配置数据失败: %v", err)
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal local_mcp config data"})
				return
			}

			config := models.Config{
				Type:      "local_mcp",
				Name:      "local_mcp",
				ConfigID:  "local_mcp",
				Provider:  "",
				JsonData:  string(jsonData),
				Enabled:   true,
				IsDefault: true,
			}

			log.Printf("准备保存local_mcp配置: Type=%s, Name=%s, ConfigID=%s", config.Type, config.Name, config.ConfigID)

			// 先检查是否已存在相同配置
			var existingConfig models.Config
			if err := tx.Where("type = ? AND config_id = ?", config.Type, config.ConfigID).First(&existingConfig).Error; err == nil {
				log.Printf("local_mcp配置已存在，将更新: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
				// 更新现有配置
				existingConfig.Name = config.Name
				existingConfig.Provider = config.Provider
				existingConfig.JsonData = config.JsonData
				existingConfig.Enabled = config.Enabled
				existingConfig.IsDefault = config.IsDefault
				if err := tx.Save(&existingConfig).Error; err != nil {
					log.Printf("更新local_mcp配置失败: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update local_mcp config"})
					return
				}
				log.Printf("local_mcp配置更新成功")
			} else if err == gorm.ErrRecordNotFound {
				log.Printf("local_mcp配置不存在，将创建新配置: Type=%s, ConfigID=%s", config.Type, config.ConfigID)
				// 创建新配置
				if err := tx.Create(&config).Error; err != nil {
					log.Printf("创建local_mcp配置失败: %v", err)
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create local_mcp config"})
					return
				}
				log.Printf("local_mcp配置创建成功")
			} else {
				log.Printf("查询local_mcp配置时发生错误: %v", err)
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query existing local_mcp config"})
				return
			}
		}
	}

	// 提交事务
	log.Printf("提交事务")
	if err := tx.Commit().Error; err != nil {
		log.Printf("提交事务失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	log.Printf("配置导入成功")
	c.JSON(http.StatusOK, gin.H{"message": "Configuration imported successfully"})
}

// MCP配置相关方法
func (ac *AdminController) GetMCPConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mcp").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取MCP配置列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMCPConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.Type = "mcp"

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建MCP配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateMCPConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	updateData.Type = "mcp"
	if err := ac.DB.Model(&config).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新MCP配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteMCPConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP配置不存在"})
		return
	}

	if err := ac.DB.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除MCP配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MCP配置删除成功"})
}

// GenerateAgentMCPEndpoint 公共的MCP接入点生成函数
func GenerateAgentMCPEndpoint(db *gorm.DB, agentID string, userID uint) (string, error) {
	// 获取OTA配置中的外网WebSocket URL
	var otaConfig models.Config
	if err := db.Where("type = ? AND is_default = ?", "ota", true).First(&otaConfig).Error; err != nil {
		return "", fmt.Errorf("failed to get OTA config: %v", err)
	}

	var otaData map[string]interface{}
	if err := json.Unmarshal([]byte(otaConfig.JsonData), &otaData); err != nil {
		return "", fmt.Errorf("failed to parse OTA config: %v", err)
	}

	// 获取外网WebSocket URL
	externalURL, ok := otaData["external"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("external config not found in OTA config")
	}

	websocketConfig, ok := externalURL["websocket"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("websocket config not found in external config")
	}

	wsURL, ok := websocketConfig["url"].(string)
	if !ok || wsURL == "" {
		return "", fmt.Errorf("websocket URL not found in external config")
	}

	// 解析OTA URL，只取域名部分，保持ws或wss协议不变
	parsedURL, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// 构建基础URL（只包含协议和域名）
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// 生成MCP JWT token
	token, err := generateMCPToken(agentID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to generate MCP token: %v", err)
	}

	// 构建带token的完整endpoint URL，直接使用/mcp路径
	endpointWithToken := fmt.Sprintf("%s/mcp?token=%s", baseURL, token)

	return endpointWithToken, nil
}

// Memory配置管理
func (ac *AdminController) GetMemoryConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "memory").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取Memory配置列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMemoryConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置配置类型为memory
	config.Type = "memory"

	// 验证provider字段
	if config.Provider != "memobase" && config.Provider != "mem0" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider必须是memobase或mem0"})
		return
	}

	// 检查是否已存在Memory配置，如果不存在则自动设置为默认配置
	var existingCount int64
	if err := ac.DB.Model(&models.Config{}).Where("type = ?", "memory").Count(&existingCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查Memory配置失败"})
		return
	}

	// 如果这是第一个Memory配置，自动设置为默认配置
	if existingCount == 0 {
		config.IsDefault = true
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建Memory配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Memory配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证provider字段
	if updateData.Provider != "memobase" && updateData.Provider != "mem0" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider必须是memobase或mem0"})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	// 更新配置
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新Memory配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").Delete(&models.Config{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除Memory配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 设置默认Memory配置
func (ac *AdminController) SetDefaultMemoryConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, "memory").First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Memory配置不存在"})
		return
	}

	// 先取消其他同类型的默认配置
	ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)

	// 设置当前配置为默认
	config.IsDefault = true
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置默认Memory配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置默认Memory配置成功", "data": config})
}

// generateMCPToken 生成包含智能体ID、用户ID和签发时间的JWT Token
func generateMCPToken(agentID string, userID uint) (string, error) {
	// 创建自定义的JWT Claims
	type MCPClaims struct {
		UserID     uint   `json:"userId"`
		AgentID    string `json:"agentId"`
		EndpointID string `json:"endpointId"`
		Purpose    string `json:"purpose"`
		jwt.RegisteredClaims
	}

	// 构建endpointId
	endpointID := fmt.Sprintf("agent_%s", agentID)

	// 创建JWT claims
	claims := MCPClaims{
		UserID:     userID,
		AgentID:    agentID,
		EndpointID: endpointID,
		Purpose:    "mcp-endpoint",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
			// 设置24小时过期时间
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	// 使用HS256算法生成JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 使用与middleware相同的密钥
	jwtSecret := []byte("xiaozhi_admin_secret_key")
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
