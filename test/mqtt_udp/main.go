package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"xiaozhi-esp32-server-golang/internal/domain/tts"
)

var sendAudioEndTs int64
var firstTts bool
var firstAudio bool
var opusData [][]byte

var audioRate = 16000
var frameDuration = 60

var allowChat = make(chan struct{}, 1)

// ServerMessage 表示服务器消息
type ServerMessage struct {
	Type        string      `json:"type"`
	Text        string      `json:"text,omitempty"`
	SessionID   string      `json:"session_id,omitempty"`
	Version     int         `json:"version"`
	State       string      `json:"state,omitempty"`
	Transport   string      `json:"transport,omitempty"`
	AudioFormat AudioFormat `json:"audio_params,omitempty"`
	Emotion     string      `json:"emotion,omitempty"`
}

type AudioFormat struct {
	Format        string `json:"format,omitempty"`
	SampleRate    int    `json:"sample_rate,omitempty"`
	Channels      int    `json:"channels,omitempty"`
	FrameDuration int    `json:"frame_duration,omitempty"`
}

// UDPConfig represents the UDP configuration structure
type UDPConfig struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	Transport string `json:"transport"`
	UDP       struct {
		Server     string `json:"server"`
		Port       int    `json:"port"`
		Encryption string `json:"encryption"`
		Key        string `json:"key"`
		Nonce      string `json:"nonce"`
	} `json:"udp"`
	AudioParams struct {
		Format        string `json:"format"`
		SampleRate    int    `json:"sample_rate"`
		Channels      int    `json:"channels"`
		FrameDuration int    `json:"frame_duration"`
	} `json:"audio_params"`
}

var globalChannel chan *UDPConfig
var serverConfig *ServerResponse

func test_aes_encrypt(plainText string) []byte {
	md5Data := md5.Sum([]byte(plainText))
	md5Str := hex.EncodeToString(md5Data[:])
	fmt.Println("加密前 md5Str:", md5Str)

	// 32字节的密钥 (256位)
	key, _ := hex.DecodeString("7f99ed0bf6647d38666628c322bc6a49")
	// 16字节的IV (128位)
	iv, _ := hex.DecodeString("010000003c2075c40000000000000000")

	//md5 iv
	ivMd5 := md5.Sum(iv)
	ivMd5Str := hex.EncodeToString(ivMd5[:])
	fmt.Println("ivMd5Str:", ivMd5Str)

	encryptedData, err := AesCTREncrypt(key, iv, []byte(plainText))
	if err != nil {
		fmt.Println("加密失败:", err)
		return nil
	}

	//计算md5
	md5Data = md5.Sum(encryptedData)

	fmt.Println("加密后的md5:", hex.EncodeToString(md5Data[:]))
	return encryptedData
}

func test_aes_decrypt(data []byte) []byte {
	md5Data := md5.Sum(data)
	md5Str := hex.EncodeToString(md5Data[:])
	fmt.Println("解密前 md5Str:", md5Str)

	// 32字节的密钥 (256位)
	key, _ := hex.DecodeString("7f99ed0bf6647d38666628c322bc6a49")
	// 16字节的IV (128位)
	iv, _ := hex.DecodeString("010000003c2075c40000000000000000")

	decryptedData, err := AesCTRDecrypt(key, iv, data)
	if err != nil {
		fmt.Println("加密失败:", err)
		return nil
	}

	//计算md5
	md5Data = md5.Sum(decryptedData)

	fmt.Println("解密后 md5:", hex.EncodeToString(md5Data[:]))
	return decryptedData
}

func main1() {
	plainText := "12345"
	fmt.Println("加密前数据:", plainText)
	enc_data := test_aes_encrypt(plainText)
	dec_data := test_aes_decrypt(enc_data)
	fmt.Println("解密后的数据:", string(dec_data))
}

func main() {
	otaUrl := flag.String("ota", "https://api.tenclass.net/xiaozhi/ota/", "OTA服务器地址")
	deviceID := flag.String("device", "ba:8f:17:de:94:94", "设备ID")
	flag.Parse()

	clientID := "e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d"
	boardName := "lc-esp32-s3"

	// Get device configuration
	deviceInfo := CreateDefaultDeviceInfo(clientID, *deviceID, boardName)

	// 生成序列号和HMAC密钥
	uuid1 := strings.ReplaceAll(uuid.New().String(), "-", "")
	uuid2 := strings.ReplaceAll(uuid.New().String(), "-", "")
	serialNumber := fmt.Sprintf("SN-%s-%s", strings.ToUpper(uuid1[:8]), uuid2[:12])

	// 生成HMAC密钥 (32字节的十六进制字符串)
	//hmacKey := strings.ReplaceAll(uuid.New().String(), "-", "")
	hmacKey := "b05df1f583419f4a088c812533b4774b97d3ff5e22d5735d3aab8dff160ebef6"

	fmt.Printf("生成的序列号: %s\n", serialNumber)
	fmt.Printf("生成的HMAC密钥: %s\n", hmacKey)

	config, err := GetDeviceConfig(deviceInfo, *deviceID, clientID, *otaUrl)
	if err != nil {
		fmt.Println("获取设备配置失败:", err)
		os.Exit(1)
	}
	serverConfig = config

	if config.Activation.Code != "" {
		fmt.Println("设备激活中, 验证码: ", config.Activation.Code)
		// 进行激活请求
		_, err := activateDevice(*deviceID, clientID, serialNumber, hmacKey, config.Activation.Challenge, *otaUrl)
		if err != nil {
			fmt.Println("设备激活失败:", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("设备已激活")
	}

	globalChannel = make(chan *UDPConfig, 1)

	// v3.1.1
	mqttClient, ok := connectMQTT(config)
	if !ok {
		fmt.Println("❌ MQTT 连接失败")
		os.Exit(1)
	}

	var udpConfig *UDPConfig
	select {
	case udpConfig = <-globalChannel:
		fmt.Println("收到UDP消息")
	case <-time.After(10 * time.Second):
		fmt.Println("等待hello消息超时")
		return
	}

	connectUdqAndSendAudio(udpConfig, mqttClient)

	// 保持程序运行
	select {}
}

func connectMQTT(config *ServerResponse) (mqtt.Client, bool) {
	// Setup MQTT client with configuration from server
	opts := mqtt.NewClientOptions()

	endpoint := config.MQTT.Endpoint
	port := "8883"
	protocol := "tls"
	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		endpoint = parts[0]
		port = parts[1]
	}
	if port != "8883" {
		protocol = "tcp"
	}
	brokerUrl := fmt.Sprintf("%s://%s:%s", protocol, endpoint, port)

	// 设置 TLS 配置
	tlsConfig := &tls.Config{
		ServerName: endpoint,
		//InsecureSkipVerify: true, // 跳过证书验证，仅用于测试环境
	}
	if protocol == "tls" {
		opts.SetTLSConfig(tlsConfig)
	}
	opts.AddBroker(brokerUrl)
	opts.SetClientID(config.MQTT.ClientID)
	opts.SetUsername(config.MQTT.Username)
	opts.SetPassword(config.MQTT.Password)

	opts.SetKeepAlive(60 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Minute)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetCleanSession(true)

	// 设置连接回调
	/*
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			version := "v3.1.1"
			if useV5 {
				version = "v5.0"
			}
			fmt.Printf("✅ MQTT %s 连接成功\n", version)
		})*/

	// 设置断开连接回调
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		fmt.Printf("⚠️ MQTT 连接断开: %v\n", err)
	})

	// 设置重连回调
	opts.SetReconnectingHandler(func(client mqtt.Client, opts *mqtt.ClientOptions) {
		fmt.Println("🔄 正在重新连接 MQTT 服务器...")
	})

	// 设置默认消息处理函数
	opts.SetDefaultPublishHandler(onMessage)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("❌ 连接失败:", token.Error())
		return nil, false
	}

	// 发布一条测试消息
	err := publicHello(config.MQTT.PublishTopic, client)
	if err != nil {
		fmt.Println("❌ 发布消息失败:", err)
		return nil, false
	}

	return client, true
}

func publicHello(publishTopic string, client mqtt.Client) error {
	message := ServerMessage{
		Type:      "hello",
		Version:   3,
		Transport: "udp",
		AudioFormat: AudioFormat{
			Format:        "opus",
			SampleRate:    audioRate,
			Channels:      1,
			FrameDuration: frameDuration,
		},
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", publishTopic, string(jsonData))

	// 使用 MQTT v5.0 的发布选项
	token := client.Publish(publishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	fmt.Println("✅ 发布消息成功")
	allowChat <- struct{}{}
	return nil
}

func encodeHexPayload(payload []byte) string {
	return hex.EncodeToString(payload)
}

func onMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("📩 收到消息: 时间: %d, topic: [%s] %s\n", time.Now().UnixMilli(), msg.Topic(), string(msg.Payload()))

	// 解析消息
	var message map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &message); err != nil {
		fmt.Printf("❌ 消息解析错误: %v, msg: %s\n", err, string(msg.Payload()))
		return
	}

	// 根据消息类型处理
	msgType, ok := message["type"].(string)
	if !ok {
		fmt.Println("❌ 消息格式错误: 缺少type字段")
		return
	}

	switch msgType {
	case "hello":
		handleHello(client, msg)
	case "tts":
		handleTTS(client, msg)
	case "llm":
		handleLLM(client, msg)
	case "stt":
		handleStt(client, msg)
	case "goodbye":
		handleGoodbye(client, msg)
	default:
		fmt.Printf("⚠️ 未知消息类型: %s\n", msgType)
	}
}

func handleHello(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("处理 hello 消息: %s\n", string(msg.Payload()))
	//解析msg到HelloMessage
	var helloMessage UDPConfig
	if err := json.Unmarshal(msg.Payload(), &helloMessage); err != nil {
		fmt.Printf("❌ 消息解析错误: %v\n", err)
		return
	}

	globalChannel <- &helloMessage

	fmt.Printf("处理 hello 消息: %s\n", helloMessage)

}

func handleLLM(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("从发送音频结束至 LLM 消息 耗时: %d ms\n", time.Now().UnixMilli()-sendAudioEndTs)
}

func handleStt(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("从发送音频结束至 STT 消息 耗时: %d ms\n", time.Now().UnixMilli()-sendAudioEndTs)
}

func handleTTS(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("处理 TTS 消息: %s\n", string(msg.Payload()))
	type st struct {
		Type  string `json:"type"`
		State string `json:"state"`
	}
	// TODO: 实现 TTS 状态更新
	var ttsState st
	if err := json.Unmarshal(msg.Payload(), &ttsState); err != nil {
		fmt.Printf("❌ 消息解析错误: %v\n", err)
		return
	}
	fmt.Printf("处理 TTS 消息: %s\n", ttsState)
	if ttsState.Type == "tts" && !firstTts {
		if ttsState.State == "sentence_start" {
			fmt.Printf("从发送音频结束至TTS 开始 耗时: %d ms\n", time.Now().UnixMilli()-sendAudioEndTs)
			firstTts = true
		}
	}

	if ttsState.State == "stop" {
		//pcmDataList, err := OpusToWav(opusData, audioRate, 1, "output_16000.wav")
		saveOpusData()
		pcmDataList, err := OpusToWav(opusData, 24000, 1, "output_24000.wav")
		if err != nil {
			fmt.Println("转换WAV文件失败:", err)
			return
		}
		fmt.Printf("TTS 结束, 音频数据长度: %d\n", len(pcmDataList))
	}
}

func saveOpusData() error {
	f, err := os.Create("opus_udp.data")
	if err != nil {
		return err
	}
	defer f.Close()

	for _, data := range opusData {
		f.Write(data)
	}

	f.Close()

	return nil
}

func handleGoodbye(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("处理 goodbye 消息: %s\n", string(msg.Payload()))
	// TODO: 实现会话清理
}

func connectUdqAndSendAudio(udpConfig *UDPConfig, mqttClient mqtt.Client) error {
	udpInstance, err := NewUDPClient(udpConfig.UDP.Server, udpConfig.UDP.Port, udpConfig.UDP.Key, udpConfig.UDP.Nonce)
	if err != nil {
		fmt.Println(err)
		return err
	}

	sessionId := "b23a56y8" //29f15278

	sendTextToSpeech(mqttClient, sessionId, udpInstance, udpConfig)

	/*

				sendListenStart(mqttClient, sessionId)
			time.Sleep(100 * time.Millisecond)
				err = sendWavFileWithOpusEncoding(udpInstance, "test.wav")
				if err != nil {
					fmt.Println(err)
					return err
				}
			fmt.Printf("发送音频数据结束: %d\n", time.Now().UnixMilli())
		//sendListenStop(mqttClient, sessionId)
		fmt.Printf("发送停止消息结束: %d\n", time.Now().UnixMilli())
		sendAudioEndTs = time.Now().UnixMilli()
	*/

	return nil
}

// 读取WAV文件并使用Opus编码发送
func sendWavFileWithOpusEncoding(udpInstance *UDPClient, filePath string) error {
	sampleRate := audioRate
	channels := 1
	// 打开WAV文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开WAV文件失败: %v", err)
	}
	defer file.Close()

	// 读取文件内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("读取文件内容失败: %v", err)
	}
	fmt.Printf("文件内容长度: %d\n", len(fileContent))
	file.Close()

	opusFrames, err := WavToOpus(fileContent, sampleRate, channels, 0)
	if err != nil {
		return fmt.Errorf("转换WAV文件失败: %v", err)
	}

	fmt.Printf("开始发送音频数据\n", len(opusFrames))

	for i, frame := range opusFrames {
		fmt.Printf("Opus帧 %d 长度: %d\n", i, len(frame))
		// 发送Opus帧
		if err := udpInstance.SendAudioData(frame); err != nil {
			return fmt.Errorf("发送Opus帧失败: %v", err)
		}
		// 控制发送速率，模拟实时音频流
		time.Sleep(60 * time.Millisecond)
	}
	fmt.Printf("总共发送: %d 个帧\n", len(opusFrames))

	//持续发送空的音频数据
	/*emptyFrame := make([]byte, 50)
	for {
		if err := conn.WriteMessage(websocket.BinaryMessage, emptyFrame); err != nil {
			return fmt.Errorf("发送空音频数据失败: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}*/

	return nil
}

// ClientMessage 表示客户端消息
type ClientMessage struct {
	Type        string   `json:"type"`
	DeviceID    string   `json:"device_id,omitempty"`
	SessionID   string   `json:"session_id"`
	Text        string   `json:"text,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	State       string   `json:"state,omitempty"`
	Token       string   `json:"token,omitempty"`
	DeviceMac   string   `json:"device_mac,omitempty"`
	Version     int      `json:"version,omitempty"`
	Transport   string   `json:"transport,omitempty"`
	Descriptors []string `json:"descriptors,omitempty"`
	States      []string `json:"states,omitempty"`
}

// ClientMessage 表示客户端消息
type IotClientMessage struct {
	Type        string   `json:"type"`
	SessionID   string   `json:"session_id"`
	Descriptors []string `json:"descriptors"`
}

// ClientMessage 表示客户端消息
type IotStatesClientMessage struct {
	Type      string   `json:"type"`
	SessionID string   `json:"session_id"`
	States    []string `json:"states"`
}

func sendListenStart(mqttClient mqtt.Client, sessionID string) error {
	//sendIotMessage(mqttClient, sessionID)
	time.Sleep(1 * time.Second)
	message := ClientMessage{
		Type:      "listen",
		State:     "start",
		Mode:      "manual",
		SessionID: sessionID,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))

	token := mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func sendListenStop(mqttClient mqtt.Client, sessionID string) error {
	message := ClientMessage{
		Type:      "listen",
		State:     "stop",
		Mode:      "manual",
		SessionID: sessionID,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))

	token := mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	allowChat <- struct{}{}
	return nil
}

func sendListenDetect(mqttClient mqtt.Client, sessionID string, text string) error {
	message := ClientMessage{
		Type:      "listen",
		State:     "detect",
		Text:      text,
		Mode:      "manual",
		SessionID: sessionID,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))

	token := mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func sendIotMessage(mqttClient mqtt.Client, sessionID string) error {
	message := IotClientMessage{
		Type:        "iot",
		SessionID:   sessionID,
		Descriptors: []string{},
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))

	token := mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	messageStates := IotStatesClientMessage{
		Type:      "iot",
		SessionID: sessionID,
		States:    []string{},
	}
	jsonData, err = json.Marshal(messageStates)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))

	token = mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func sendAbort(mqttClient mqtt.Client, sessionID string) error {
	message := ClientMessage{
		Type:      "abort",
		SessionID: sessionID,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	fmt.Println("📤 发布消息to topic:", "", string(jsonData))
	token := mqttClient.Publish(serverConfig.MQTT.PublishTopic, byte(0), false, jsonData)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// 调用tts服务生成语音, 并编码至opus发送至服务端
func sendTextToSpeech(mqttClient mqtt.Client, sessionID string, udpInstance *UDPClient, udpConfig *UDPConfig) error {
	cosyVoiceConfig := map[string]interface{}{
		"api_url":        "https://tts.linkerai.cn/tts",
		"spk_id":         "OUeAo1mhq6IBExi",
		"frame_duration": frameDuration,
		"target_sr":      audioRate,
		"audio_format":   "mp3",
		"instruct_text":  "你好",
	}
	_ = cosyVoiceConfig
	/**
		    "edge": {
	      "voice": "zh-CN-XiaoxiaoNeural",
	      "rate": "+0%",
	      "volume": "+0%",
	      "pitch": "+0Hz",
	      "connect_timeout": 10,
	      "receive_timeout": 60
	    }
	*/
	edgeConfig := map[string]interface{}{
		"voice":           "zh-CN-XiaoxiaoNeural",
		"rate":            "+0%",
		"volume":          "+0%",
		"pitch":           "+0Hz",
		"connect_timeout": 10,
		"receive_timeout": 60,
	}
	_ = edgeConfig
	//调用tts服务生成语音
	//ttsProvider, err := tts.GetTTSProvider("cosyvoice", cosyVoiceConfig)
	ttsProvider, err := tts.GetTTSProvider("edge", edgeConfig)
	if err != nil {
		return fmt.Errorf("获取tts服务失败: %v", err)
	}

	/*
		audioData, err := ttsProvider.TextToSpeech(context.Background(), "你叫什么名字?")
		if err != nil {
			fmt.Printf("生成语音失败: %v\n", err)
			return fmt.Errorf("生成语音失败: %v", err)
		}
	*/

	hexKey, _ := hex.DecodeString(udpConfig.UDP.Key)
	opusData = make([][]byte, 0)

	var isStart bool
	var startTs int64

	udpInstance.ReceiveAudioData(hexKey, func(key []byte, audioData []byte) {
		decryptedData, err := udpInstance.decryptAudioData(key, audioData)
		if err != nil {
			fmt.Println("解密失败:", err)
			return
		}
		if isStart {
			fmt.Printf("发送音频结束至收到首帧耗时: %d ms\n", time.Now().UnixMilli()-startTs)
			isStart = false
			os.WriteFile("mqtt_output_first_frame.wav", decryptedData, 0644)
		}

		//fmt.Printf("收到音频数据, 长度: %d\n", len(decryptedData))
		opusData = append(opusData, decryptedData)
		//fmt.Println("收到音频数据", len(decryptedData))
	})

	var audioCtx context.Context
	var audioCancel context.CancelFunc

	genAndSendAudio := func(ctx context.Context, msg string, count int) error {
		sendListenStart(mqttClient, sessionID)
		defer func() {
			isStart = true
			sendListenStop(mqttClient, sessionID)
			startTs = time.Now().UnixMilli()
		}()
		audioChan, err := ttsProvider.TextToSpeechStream(context.Background(), msg, 16000, 1, 60)
		if err != nil {
			//fmt.Printf("生成语音失败: %v\n", err)
			return fmt.Errorf("生成语音失败: %v", err)
		}

		for audioData := range audioChan {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			fmt.Printf("生成语音数据长度: %d\n", len(audioData))
			udpInstance.SendAudioData(audioData)
			time.Sleep(60 * time.Millisecond)
		}

		/*
			emptyFrame := make([]byte, 50)
			for i := 0; i <= count; i++ {
				udpInstance.SendAudioData(emptyFrame)
				time.Sleep(60 * time.Millisecond)
			}*/
		return nil
	}

	// 新增：等待用户输入文本
	reader := bufio.NewReader(os.Stdin)

	f := func() bool {
		fmt.Print("请输入要合成的文本（回车发送，直接回车退出）：")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取输入失败: %v\n", err)
			return false
		}
		input = strings.TrimSpace(input)
		if input == "" {
			sendAbort(mqttClient, sessionID)
			audioCancel()
			return false
		}

		audioCtx, audioCancel = context.WithCancel(context.Background())
		genAndSendAudio(audioCtx, input, 50)
		return true
	}
	for {
		_ = <-allowChat
		for {
			if f() {
				break
			}
		}
	}

	//genAndSendAudio("你好", 100)
	//time.Sleep(30 * time.Second)
	/*genAndSendAudio("再来一个", 20)
	time.Sleep(30 * time.Second)
	genAndSendAudio("你今天穿的衣服真好看", 20)
	time.Sleep(30 * time.Second)
	genAndSendAudio("明天准备穿什么", 20)
	time.Sleep(30 * time.Second)*/

	return nil
}
