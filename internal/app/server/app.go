package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/mqtt_server"
	"xiaozhi-esp32-server-golang/internal/app/server/chat"
	"xiaozhi-esp32-server-golang/internal/app/server/mqtt_udp"
	"xiaozhi-esp32-server-golang/internal/app/server/types"
	"xiaozhi-esp32-server-golang/internal/app/server/websocket"
	"xiaozhi-esp32-server-golang/internal/data/history"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"
	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spf13/viper"
)

// App 统一管理所有协议服务和 ChatManager

type App struct {
	wsServer       *websocket.WebSocketServer
	mqttUdpAdapter *mqtt_udp.MqttUdpAdapter

	// ChatManager管理 - 使用concurrent map
	chatManagers cmap.ConcurrentMap[string, *chat.ChatManager]
}

func NewApp() *App {
	var err error
	app := &App{
		chatManagers: cmap.New[*chat.ChatManager](),
	}
	app.wsServer = app.newWebSocketServer()
	app.mqttUdpAdapter, err = app.newMqttUdpAdapter()
	if err != nil {
		log.Errorf("newMqttUdpAdapter err: %+v", err)
		return nil
	}
	return app
}

func (a *App) Run() {
	go a.wsServer.Start()
	if viper.GetBool("mqtt_server.enable") {
		go func() {
			err := a.startMqttServer()
			if err != nil {
				log.Errorf("startMqttServer err: %+v", err)
			}
		}()
	}
	if a.mqttUdpAdapter != nil {
		time.Sleep(1 * time.Second)
		go a.mqttUdpAdapter.Start()
	}

	// 注册聊天相关的本地MCP工具
	a.registerChatMCPTools()

	a.registerHandler()

	a.initEventHandle()

	select {} // 阻塞主线程
}

func (app *App) initEventHandle() {
	eventHandle, err := NewEventHandle()
	if err != nil {
		log.Errorf("初始化 EventHandle 失败: %v", err)
		return
	}
	if err := eventHandle.Start(); err != nil {
		log.Errorf("启动 EventHandle 失败: %v", err)
		return
	}

	// 初始化聊天历史记录处理器
	if viper.GetBool("manager.history_enabled") {
		historyCfg := history.HistoryClientConfig{
			BaseURL:     viper.GetString("manager.backend_url"),
			AuthToken:   viper.GetString("manager.history_auth_token"),
			Timeout:     viper.GetDuration("manager.history_timeout"),
			Enabled:     viper.GetBool("manager.history_enabled"),
			EnableAudio: viper.GetBool("manager.history_enable_audio"),
		}
		NewHistoryWorker(historyCfg)
		log.Info("聊天历史记录处理器已初始化")
	} else {
		log.Info("聊天历史记录功能已禁用")
	}
}

func (app *App) newMqttUdpAdapter() (*mqtt_udp.MqttUdpAdapter, error) {
	isEnableUdp := viper.GetBool("mqtt.enable")
	if !isEnableUdp {
		return nil, nil
	}
	mqttConfig := mqtt_udp.MqttConfig{
		Broker:   viper.GetString("mqtt.broker"),
		Type:     viper.GetString("mqtt.type"),
		Port:     viper.GetInt("mqtt.port"),
		ClientID: viper.GetString("mqtt.client_id"),
		Username: viper.GetString("mqtt.username"),
		Password: viper.GetString("mqtt.password"),
	}

	udpServer, err := app.newUdpServer()
	if err != nil {
		return nil, err
	}

	return mqtt_udp.NewMqttUdpAdapter(
		&mqttConfig,
		mqtt_udp.WithUdpServer(udpServer),
		mqtt_udp.WithOnNewConnection(app.OnNewConnection),
	), nil
}

func (app *App) newUdpServer() (*mqtt_udp.UdpServer, error) {
	udpPort := viper.GetInt("udp.listen_port")
	externalHost := viper.GetString("udp.external_host")
	externalPort := viper.GetInt("udp.external_port")

	udpServer := mqtt_udp.NewUDPServer(udpPort, externalHost, externalPort)
	err := udpServer.Start()
	if err != nil {
		log.Fatalf("udpServer.Start err: %+v", err)
		return nil, err
	}
	return udpServer, nil
}

func (app *App) newWebSocketServer() *websocket.WebSocketServer {
	port := viper.GetInt("websocket.port")
	return websocket.NewWebSocketServer(port, websocket.WithOnNewConnection(app.OnNewConnection))
}

func (app *App) startMqttServer() error {
	return mqtt_server.StartMqttServer()
}

// 所有协议新连接都走这里
func (a *App) OnNewConnection(transport types.IConn) {
	deviceID := transport.GetDeviceID()

	// 检查是否已存在该设备的ChatManager
	if existingManager, exists := a.chatManagers.Get(deviceID); exists {
		log.Infof("设备 %s 已存在ChatManager，先关闭旧的连接", deviceID)
		// 关闭旧的ChatManager
		existingManager.Close()
		a.chatManagers.Remove(deviceID)
	}

	// 创建新的ChatManager
	chatManager, err := chat.NewChatManager(deviceID, transport)
	if err != nil {
		log.Errorf("创建chatManager失败: %v", err)
		return
	}

	// 存储ChatManager
	a.chatManagers.Set(deviceID, chatManager)

	a.DeviceOnline(deviceID)

	log.Infof("设备 %s 的ChatManager已创建并存储", deviceID)

	// 启动ChatManager
	go func() {
		defer func() {
			// ChatManager结束时，从映射中移除
			if storedManager, exists := a.chatManagers.Get(deviceID); exists && storedManager == chatManager {
				a.chatManagers.Remove(deviceID)
				log.Infof("设备 %s 的ChatManager已从映射中移除", deviceID)
				a.DeviceOffline(deviceID)
			}
		}()

		if err := chatManager.Start(); err != nil {
			log.Errorf("ChatManager启动失败: %v", err)
		}
	}()
}

// GetChatManager 获取指定设备的ChatManager
func (a *App) GetChatManager(deviceID string) (*chat.ChatManager, bool) {
	return a.chatManagers.Get(deviceID)
}

// CloseChatManager 关闭指定设备的ChatManager
func (a *App) CloseChatManager(deviceID string) bool {
	if manager, exists := a.chatManagers.Get(deviceID); exists {
		manager.Close()
		a.chatManagers.Remove(deviceID)
		log.Infof("设备 %s 的ChatManager已关闭并移除", deviceID)
		return true
	}
	return false
}

// GetAllChatManagers 获取所有ChatManager的副本
func (a *App) GetAllChatManagers() map[string]*chat.ChatManager {
	// 返回副本以避免并发访问问题
	managers := make(map[string]*chat.ChatManager)
	for tuple := range a.chatManagers.IterBuffered() {
		managers[tuple.Key] = tuple.Val
	}
	return managers
}

// GetChatManagerCount 获取当前活跃的ChatManager数量
func (a *App) GetChatManagerCount() int {
	return a.chatManagers.Count()
}

// CloseAllChatManagers 关闭所有ChatManager
func (a *App) CloseAllChatManagers() {
	for tuple := range a.chatManagers.IterBuffered() {
		tuple.Val.Close()
		log.Infof("设备 %s 的ChatManager已关闭", tuple.Key)
	}

	// 清空映射
	a.chatManagers.Clear()
	log.Info("所有ChatManager已关闭")
}

// registerChatMCPTools 注册聊天相关的本地MCP工具
func (s *App) registerChatMCPTools() {
	// 调用chat包的注册函数
	chat.RegisterChatMCPTools()

	log.Info("聊天相关的本地MCP工具注册完成")
}

func (s *App) DeviceOnline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOnline, eventData)
}

func (s *App) DeviceOffline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOffline, eventData)
}

func (a *App) registerHandler() {
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.RegisterMessageEventHandler(context.Background(), config_types.EventHandleMessageInject, a.HandleInjectMsg)
}

// 向客户端注入消息
func (a *App) HandleInjectMsg(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error) {
	type InjectMsg struct {
		SkipLlm  bool   `json:"skip_llm"`
		DeviceId string `json:"device_id"`
		Message  string `json:"message"`
	}
	bodyBytes, _ := json.Marshal(eventData)
	var msg InjectMsg
	err := json.Unmarshal(bodyBytes, &msg)
	if err != nil {
		log.Errorf("HandleInjectMsg error: %+v", err)
		return "", fmt.Errorf("HandleInjectMsg error")
	}

	// 验证必要参数
	if msg.DeviceId == "" {
		log.Errorf("HandleInjectMsg: device_id is required")
		return "", fmt.Errorf("device_id is required")
	}
	if msg.Message == "" {
		log.Errorf("HandleInjectMsg: message is required")
		return "", fmt.Errorf("message is required")
	}

	// 获取指定设备的ChatManager
	chatManager, exists := a.GetChatManager(msg.DeviceId)
	if !exists {
		log.Errorf("HandleInjectMsg: device %s not found or offline", msg.DeviceId)
		return "", fmt.Errorf("device %s not found or offline", msg.DeviceId)
	}

	log.Debugf("HandleInjectMsg: injecting message to device %s, skip_llm: %v, message: %s",
		msg.DeviceId, msg.SkipLlm, msg.Message)

	// 使用ChatManager的公开方法注入消息
	err = chatManager.InjectMessage(msg.Message, msg.SkipLlm)
	if err != nil {
		log.Errorf("HandleInjectMsg: failed to inject message to device %s: %v", msg.DeviceId, err)
		return "", fmt.Errorf("failed to inject message: %v", err)
	}

	return "message injected successfully", nil
}
