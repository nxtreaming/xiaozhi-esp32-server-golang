package controllers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	DB                  *gorm.DB
	WebSocketController interface {
		RequestMcpToolsFromClient(ctx context.Context, agentID string) ([]string, error)
		InjectMessageToDevice(ctx context.Context, deviceID, message string, skipLlm bool) error
	}
}

// 注入消息到设备
func (uc *UserController) InjectMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
		Message  string `json:"message" binding:"required"`
		SkipLlm  bool   `json:"skip_llm"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证设备是否属于当前用户
	var device models.Device

	if err := uc.DB.Where("device_name = ? AND user_id = ?", req.DeviceID, userID).First(&device).Error; err != nil {
		log.Printf("[InjectMessage] 设备查询失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "设备不存在或不属于当前用户"})
		return
	}

	// 通过WebSocket发送消息注入请求到主服务器
	ctx := context.Background()
	err := uc.WebSocketController.InjectMessageToDevice(ctx, device.DeviceName, req.Message, req.SkipLlm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "消息注入失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "消息注入请求已发送",
		"data": gin.H{
			"device_id": req.DeviceID,
			"message":   req.Message,
			"skip_llm":  req.SkipLlm,
		},
	})
}

// 用户直接创建设备（无需验证码）
func (uc *UserController) CreateDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		DeviceName string `json:"device_name" binding:"required,min=2,max=50"`
		AgentID    uint   `json:"agent_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "智能体不存在或不属于当前用户"})
		return
	}

	// 生成6位随机设备代码，确保不重复
	var deviceCode string
	for i := 0; i < 10; i++ { // 最多尝试10次
		code := generateRandomCode()

		// 检查代码是否已存在
		var count int64
		if err := uc.DB.Model(&models.Device{}).Where("device_code = ?", code).Count(&count).Error; err == nil && count == 0 {
			deviceCode = code
			break
		}
	}

	// 如果10次都重复，使用时间戳生成
	if deviceCode == "" {
		deviceCode = fmt.Sprintf("%06d", time.Now().Unix()%1000000)
	}

	// 创建设备
	device := models.Device{
		UserID:     userID.(uint),
		AgentID:    req.AgentID,
		DeviceCode: deviceCode,
		DeviceName: req.DeviceName,
		Activated:  true, // 新创建的设备默认未激活
	}

	if err := uc.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建设备失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "设备创建成功",
		"data": gin.H{
			"device_code": deviceCode,
			"device":      device,
		},
	})
}

// 生成6位随机数字代码
func generateRandomCode() string {
	// 生成6位随机数字
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	return code
}

// 获取用户所有设备概览（只读）
func (uc *UserController) GetMyDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")

	type DeviceOverview struct {
		ID           uint       `json:"id"`
		DeviceName   string     `json:"device_name"`
		DeviceCode   string     `json:"device_code"`
		AgentID      uint       `json:"agent_id"`
		AgentName    string     `json:"agent_name,omitempty"`
		Activated    bool       `json:"activated"`
		LastActiveAt *time.Time `json:"last_active_at"`
		CreatedAt    time.Time  `json:"created_at"`
	}

	var devices []models.Device
	if err := uc.DB.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}

	// 构建设备概览信息
	var result []DeviceOverview
	for _, device := range devices {
		overview := DeviceOverview{
			ID:           device.ID,
			DeviceName:   device.DeviceName,
			DeviceCode:   device.DeviceCode,
			AgentID:      device.AgentID,
			Activated:    device.Activated,
			LastActiveAt: device.LastActiveAt,
			CreatedAt:    device.CreatedAt,
		}

		// 如果设备绑定了智能体，获取智能体名称
		if device.AgentID > 0 {
			var agent models.Agent
			if err := uc.DB.Where("id = ? AND user_id = ?", device.AgentID, userID).First(&agent).Error; err == nil {
				overview.AgentName = agent.Name
			}
		}

		result = append(result, overview)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// 智能体管理
func (uc *UserController) GetAgents(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var agents []models.Agent
	if err := uc.DB.Where("user_id = ?", userID).Find(&agents).Error; err != nil {
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
			if err := uc.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
				agentWithConfig.LLMConfig = &llmConfig
			}
		}

		// 加载TTS配置
		if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
			var ttsConfig models.Config
			if err := uc.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
				agentWithConfig.TTSConfig = &ttsConfig
			}
		}

		result = append(result, agentWithConfig)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) CreateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Name         string  `json:"name" binding:"required,min=2,max=50"`
		CustomPrompt string  `json:"custom_prompt"`
		LLMConfigID  *string `json:"llm_config_id"`
		TTSConfigID  *string `json:"tts_config_id"`
		Voice        *string `json:"voice"`
		ASRSpeed     string  `json:"asr_speed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 设置默认值
	if req.ASRSpeed == "" {
		req.ASRSpeed = "normal"
	}

	agent := models.Agent{
		UserID:       userID.(uint),
		Name:         req.Name,
		CustomPrompt: req.CustomPrompt,
		LLMConfigID:  req.LLMConfigID,
		TTSConfigID:  req.TTSConfigID,
		Voice:        req.Voice,
		ASRSpeed:     req.ASRSpeed,
		Status:       "active",
	}

	if err := uc.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建智能体失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": agent})
}

func (uc *UserController) GetAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	// 手动加载关联的配置信息
	type AgentWithConfigs struct {
		models.Agent
		LLMConfig *models.Config `json:"llm_config,omitempty"`
		TTSConfig *models.Config `json:"tts_config,omitempty"`
	}

	result := AgentWithConfigs{Agent: agent}

	// 加载LLM配置
	if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
		var llmConfig models.Config
		if err := uc.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
			result.LLMConfig = &llmConfig
		}
	}

	// 加载TTS配置
	if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
		var ttsConfig models.Config
		if err := uc.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
			result.TTSConfig = &ttsConfig
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) UpdateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	var req struct {
		Name         string  `json:"name" binding:"required,min=2,max=50"`
		CustomPrompt string  `json:"custom_prompt"`
		LLMConfigID  *string `json:"llm_config_id"`
		TTSConfigID  *string `json:"tts_config_id"`
		Voice        *string `json:"voice"`
		ASRSpeed     string  `json:"asr_speed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 更新字段
	agent.Name = req.Name
	agent.CustomPrompt = req.CustomPrompt
	agent.LLMConfigID = req.LLMConfigID
	agent.TTSConfigID = req.TTSConfigID
	agent.Voice = req.Voice

	if req.ASRSpeed != "" {
		agent.ASRSpeed = req.ASRSpeed
	} else {
		agent.ASRSpeed = "normal"
	}

	if err := uc.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (uc *UserController) DeleteAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	if err := uc.DB.Delete(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 获取智能体关联的设备
func (uc *UserController) GetAgentDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// 首先验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	// 获取属于该智能体的设备
	var devices []models.Device
	if err := uc.DB.Where("user_id = ? AND agent_id = ?", userID, agentID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// 将设备添加到智能体
func (uc *UserController) AddDeviceToAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	var req struct {
		Code string `json:"code" binding:"required,len=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码格式错误"})
		return
	}

	// 首先验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	// 验证设备验证码（user_id为0表示设备未绑定用户）
	var device models.Device
	if err := uc.DB.Where("device_code = ? AND user_id = 0", req.Code).First(&device).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码无效或设备已被绑定"})
		return
	}

	// 绑定设备到用户和智能体
	device.UserID = userID.(uint)

	// 转换agentID字符串为uint
	agentIDInt, err := strconv.Atoi(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的智能体ID"})
		return
	}
	device.AgentID = uint(agentIDInt)

	// 自动激活设备
	device.Activated = true

	if err := uc.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设备绑定失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": device})
}

// 从智能体移除设备
func (uc *UserController) RemoveDeviceFromAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	deviceID := c.Param("device_id")

	// 首先验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	// 查找设备并验证所有权
	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ? AND agent_id = ?", deviceID, userID, agentID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在或不属于此智能体"})
		return
	}

	// 将设备从智能体中移除（设置agent_id为0，但保持用户绑定）
	device.AgentID = 0
	if err := uc.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除设备失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "设备移除成功"})
}

// 获取角色模板
func (uc *UserController) GetRoleTemplates(c *gin.Context) {
	var roles []models.GlobalRole
	if err := uc.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取角色模板失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

// 获取音色选项
func (uc *UserController) GetVoiceOptions(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider参数必填"})
		return
	}

	// 根据provider获取音色列表
	voices := GetVoiceOptionsByProvider(provider)
	c.JSON(http.StatusOK, gin.H{"data": voices})
}

// 获取LLM配置列表
func (uc *UserController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	// 从全局配置中获取所有启用的LLM配置，默认配置排在前面
	if err := uc.DB.Where("type = ? AND enabled = ?", "llm", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取LLM配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// 获取TTS配置列表
func (uc *UserController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	// 从全局配置中获取所有启用的TTS配置，默认配置排在前面
	if err := uc.DB.Where("type = ? AND enabled = ?", "tts", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取TTS配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetAgentMCPEndpoint 获取智能体的MCP接入点URL（用户版本）
func (uc *UserController) GetAgentMCPEndpoint(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// 验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在或不属于当前用户"})
		return
	}

	// 使用公共函数生成MCP接入点
	endpoint, err := GenerateAgentMCPEndpoint(uc.DB, agentID, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回单个endpoint字符串
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"endpoint": endpoint}})
}

// GetAgentMcpTools 获取智能体的MCP工具列表（用户版本）
func (uc *UserController) GetAgentMcpTools(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// 用户验证函数：验证智能体是否存在且属于当前用户
	userAgentValidator := func(agentID string) error {
		var agent models.Agent
		if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
			return fmt.Errorf("智能体不存在或不属于当前用户")
		}
		return nil
	}

	// 使用公共函数
	GetAgentMcpToolsCommon(c, agentID, uc.WebSocketController, userAgentValidator)
}

// 获取仪表板统计数据
func (uc *UserController) GetDashboardStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("role")

	type DashboardStats struct {
		TotalUsers    int64 `json:"totalUsers"`
		TotalDevices  int64 `json:"totalDevices"`
		TotalAgents   int64 `json:"totalAgents"`
		OnlineDevices int64 `json:"onlineDevices"`
	}

	stats := DashboardStats{}

	if userRole == "admin" {
		// 管理员查看全部数据
		uc.DB.Model(&models.User{}).Count(&stats.TotalUsers)
		uc.DB.Model(&models.Device{}).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Count(&stats.TotalAgents)
		// 在线设备：最近5分钟内活跃的设备
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("last_active_at > ?", fiveMinutesAgo).Count(&stats.OnlineDevices)
	} else {
		// 普通用户只查看自己的数据
		stats.TotalUsers = 0 // 普通用户不显示用户数
		uc.DB.Model(&models.Device{}).Where("user_id = ?", userID).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Where("user_id = ?", userID).Count(&stats.TotalAgents)
		// 在线设备：用户自己的最近5分钟内活跃的设备
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("user_id = ? AND last_active_at > ?", userID, fiveMinutesAgo).Count(&stats.OnlineDevices)
	}

	c.JSON(http.StatusOK, stats)
}
