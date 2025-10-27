package router

import (
	"xiaozhi/manager/backend/controllers"
	"xiaozhi/manager/backend/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// CORS配置
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// 初始化控制器
	authController := &controllers.AuthController{DB: db}
	webSocketController := controllers.NewWebSocketController(db)
	adminController := &controllers.AdminController{DB: db, WebSocketController: webSocketController}
	userController := &controllers.UserController{DB: db, WebSocketController: webSocketController}
	deviceActivationController := &controllers.DeviceActivationController{DB: db}
	setupController := &controllers.SetupController{DB: db}

	// API路由组
	api := r.Group("/api")
	{
		// 公开路由（无需认证）
		api.POST("/login", authController.Login)
		api.POST("/register", authController.Register)

		// 数据库初始化相关路由（无需认证）
		api.GET("/setup/status", setupController.CheckSetupStatus)
		api.POST("/setup/initialize", setupController.InitializeDatabase)

		// 设备激活相关公开接口（无需认证）
		api.GET("/public/device/check-activation", deviceActivationController.CheckDeviceActivation)
		api.GET("/public/device/activation-info", deviceActivationController.GetActivationInfo)
		api.POST("/public/device/activate", deviceActivationController.ActivateDevice)

		// 内部服务接口（无需认证）
		api.GET("/configs", adminController.GetDeviceConfigs)
		api.GET("/system/configs", adminController.GetSystemConfigs)

		// 需要认证的路由
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/profile", authController.GetProfile)
			// 通用接口，获取系统中的设备信息
			auth.GET("/dashboard/stats", userController.GetDashboardStats)

			// 用户路由
			user := auth.Group("/user")
			{
				// 设备管理
				user.GET("/devices", userController.GetMyDevices)
				user.POST("/devices", userController.CreateDevice)

				// 智能体管理
				user.GET("/agents", userController.GetAgents)
				user.POST("/agents", userController.CreateAgent)
				user.GET("/agents/:id", userController.GetAgent)
				user.PUT("/agents/:id", userController.UpdateAgent)
				user.DELETE("/agents/:id", userController.DeleteAgent)
				user.GET("/agents/:id/devices", userController.GetAgentDevices)
				user.POST("/agents/:id/devices", userController.AddDeviceToAgent)
				user.DELETE("/agents/:id/devices/:device_id", userController.RemoveDeviceFromAgent)

				// 角色模板和音色选项
				user.GET("/role-templates", userController.GetRoleTemplates)
				user.GET("/voice-options", userController.GetVoiceOptions)

				// 配置列表
				user.GET("/llm-configs", userController.GetLLMConfigs)
				user.GET("/tts-configs", userController.GetTTSConfigs)

				// MCP接入点
				user.GET("/agents/:id/mcp-endpoint", userController.GetAgentMCPEndpoint)
				user.GET("/agents/:id/mcp-tools", userController.GetAgentMcpTools)

				// 消息注入
				user.POST("/devices/inject-message", userController.InjectMessage)
			}

			// 管理员路由
			admin := auth.Group("/admin")
			admin.Use(middleware.AdminAuth())
			{
				// 通用配置管理
				admin.GET("/configs", adminController.GetConfigs)
				admin.POST("/configs", adminController.CreateConfig)
				admin.GET("/configs/:id", adminController.GetConfig)
				admin.PUT("/configs/:id", adminController.UpdateConfig)
				admin.DELETE("/configs/:id", adminController.DeleteConfig)
				admin.POST("/configs/:id/toggle", adminController.ToggleConfigEnable)

				// 具体配置类型路由（兼容前端）
				admin.GET("/vad-configs", adminController.GetVADConfigs)
				admin.POST("/vad-configs", adminController.CreateVADConfig)
				admin.PUT("/vad-configs/:id", adminController.UpdateVADConfig)
				admin.DELETE("/vad-configs/:id", adminController.DeleteVADConfig)

				admin.GET("/asr-configs", adminController.GetASRConfigs)
				admin.POST("/asr-configs", adminController.CreateASRConfig)
				admin.PUT("/asr-configs/:id", adminController.UpdateASRConfig)
				admin.DELETE("/asr-configs/:id", adminController.DeleteASRConfig)

				admin.GET("/llm-configs", adminController.GetLLMConfigs)
				admin.POST("/llm-configs", adminController.CreateLLMConfig)
				admin.PUT("/llm-configs/:id", adminController.UpdateLLMConfig)
				admin.DELETE("/llm-configs/:id", adminController.DeleteLLMConfig)

				admin.GET("/tts-configs", adminController.GetTTSConfigs)
				admin.POST("/tts-configs", adminController.CreateTTSConfig)
				admin.PUT("/tts-configs/:id", adminController.UpdateTTSConfig)
				admin.DELETE("/tts-configs/:id", adminController.DeleteTTSConfig)

				admin.GET("/vision-configs", adminController.GetVisionConfigs)
				admin.POST("/vision-configs", adminController.CreateVisionConfig)
				admin.PUT("/vision-configs/:id", adminController.UpdateVisionConfig)
				admin.DELETE("/vision-configs/:id", adminController.DeleteVisionConfig)

				// Vision基础配置
				admin.GET("/vision-base-config", adminController.GetVisionBaseConfig)
				admin.PUT("/vision-base-config", adminController.UpdateVisionBaseConfig)

				admin.GET("/ota-configs", adminController.GetOTAConfigs)
				admin.POST("/ota-configs", adminController.CreateOTAConfig)
				admin.PUT("/ota-configs/:id", adminController.UpdateOTAConfig)
				admin.DELETE("/ota-configs/:id", adminController.DeleteOTAConfig)

				admin.GET("/mqtt-configs", adminController.GetMQTTConfigs)
				admin.POST("/mqtt-configs", adminController.CreateMQTTConfig)
				admin.PUT("/mqtt-configs/:id", adminController.UpdateMQTTConfig)
				admin.DELETE("/mqtt-configs/:id", adminController.DeleteMQTTConfig)

				admin.GET("/mqtt-server-configs", adminController.GetMQTTServerConfigs)
				admin.POST("/mqtt-server-configs", adminController.CreateMQTTServerConfig)
				admin.PUT("/mqtt-server-configs/:id", adminController.UpdateMQTTServerConfig)
				admin.DELETE("/mqtt-server-configs/:id", adminController.DeleteMQTTServerConfig)

				admin.GET("/udp-configs", adminController.GetUDPConfigs)
				admin.POST("/udp-configs", adminController.CreateUDPConfig)
				admin.PUT("/udp-configs/:id", adminController.UpdateUDPConfig)
				admin.DELETE("/udp-configs/:id", adminController.DeleteUDPConfig)

				admin.GET("/mcp-configs", adminController.GetMCPConfigs)
				admin.POST("/mcp-configs", adminController.CreateMCPConfig)
				admin.PUT("/mcp-configs/:id", adminController.UpdateMCPConfig)
				admin.DELETE("/mcp-configs/:id", adminController.DeleteMCPConfig)

				// Memory配置管理
				admin.GET("/memory-configs", adminController.GetMemoryConfigs)
				admin.POST("/memory-configs", adminController.CreateMemoryConfig)
				admin.PUT("/memory-configs/:id", adminController.UpdateMemoryConfig)
				admin.DELETE("/memory-configs/:id", adminController.DeleteMemoryConfig)
				admin.POST("/memory-configs/:id/set-default", adminController.SetDefaultMemoryConfig)

				// 全局角色管理
				admin.GET("/global-roles", adminController.GetGlobalRoles)
				admin.POST("/global-roles", adminController.CreateGlobalRole)
				admin.PUT("/global-roles/:id", adminController.UpdateGlobalRole)
				admin.DELETE("/global-roles/:id", adminController.DeleteGlobalRole)

				// 设备管理
				admin.GET("/devices", adminController.GetDevices)
				admin.GET("/devices/validate-code", adminController.ValidateDeviceCode)
				admin.POST("/devices", adminController.CreateDevice)
				admin.PUT("/devices/:id", adminController.UpdateDevice)
				admin.DELETE("/devices/:id", adminController.DeleteDevice)

				// 智能体管理
				admin.GET("/agents", adminController.GetAgents)
				admin.POST("/agents", adminController.CreateAgent)
				admin.PUT("/agents/:id", adminController.UpdateAgent)
				admin.DELETE("/agents/:id", adminController.DeleteAgent)
				admin.GET("/agents/:id/mcp-endpoint", adminController.GetAgentMCPEndpoint)
				admin.GET("/agents/:id/mcp-tools", adminController.GetAgentMcpTools)

				// 用户管理
				admin.GET("/users", adminController.GetUsers)
				admin.POST("/users", adminController.CreateUser)
				admin.PUT("/users/:id", adminController.UpdateUser)
				admin.DELETE("/users/:id", adminController.DeleteUser)
				admin.POST("/users/:id/reset-password", adminController.ResetUserPassword)

				// 配置导入导出
				admin.GET("/configs/export", adminController.ExportConfigs)
				admin.POST("/configs/import", adminController.ImportConfigs)
			}
		}
	}

	// WebSocket路由
	r.GET("/ws", webSocketController.HandleWebSocket)

	return r
}
