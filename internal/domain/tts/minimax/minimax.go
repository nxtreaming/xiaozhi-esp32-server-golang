package minimax

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// 常量定义
const (
	wsURL = "wss://api.minimaxi.com/ws/v1/t2a_v2"
)

// 全局WebSocket Dialer
var wsDialer = websocket.Dialer{
	ReadBufferSize:   16384, // 16KB 读取缓冲区
	WriteBufferSize:  16384, // 16KB 写入缓冲区
	HandshakeTimeout: 45 * time.Second,
}

// MinimaxTTSProvider Minimax TTS提供者
type MinimaxTTSProvider struct {
	APIKey     string
	Model      string
	Voice      string
	Speed      float64
	Volume     float64
	Pitch      int
	SampleRate int
	Bitrate    int
	Format     string
	Channel    int

	// 连接管理
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// 发送锁，确保同一时间只有一个请求在使用连接
	sendMutex sync.Mutex
}

// WebSocket 消息结构
type minimaxMessage struct {
	Event           string        `json:"event,omitempty"`
	Model           string        `json:"model,omitempty"`
	VoiceSetting    *voiceSetting `json:"voice_setting,omitempty"`
	AudioSetting    *audioSetting `json:"audio_setting,omitempty"`
	ContinuousSound bool          `json:"continuous_sound,omitempty"`
	Text            string        `json:"text,omitempty"`
}

type minimaxResp struct {
	SessionId string            `json:"session_id,omitempty"`
	Event     string            `json:"event,omitempty"`
	TraceId   string            `json:"trace_id,omitempty"`
	Data      *minimaxData      `json:"data,omitempty"`
	IsFinal   bool              `json:"is_final,omitempty"`
	BaseResp  *minimaxBaseResp  `json:"base_resp,omitempty"`
	ExtraInfo *minimaxExtraInfo `json:"extra_info,omitempty"`
}

type minimaxExtraInfo struct {
	AudioLength     int    `json:"audio_length"`
	AudioSampleRate int    `json:"audio_sample_rate"`
	AudioDuration   int    `json:"audio_duration"`
	AudioSize       int    `json:"audio_size"`
	Bitrate         int    `json:"bitrate"`
	AudioFormat     string `json:"audio_format"`
	AudioChannel    int    `json:"audio_channel"`

	UsageCharacters int `json:"usage_characters"`
	WordCount       int `json:"word_count"`
}

type minimaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type voiceSetting struct {
	VoiceID              string  `json:"voice_id"`
	Speed                float64 `json:"speed"`
	Vol                  float64 `json:"vol"`
	Pitch                int     `json:"pitch"`
	EnglishNormalization bool    `json:"english_normalization"`
}

type audioSetting struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

type minimaxData struct {
	Audio string `json:"audio"`
}

// NewMinimaxTTSProvider 创建新的Minimax TTS提供者
func NewMinimaxTTSProvider(config map[string]interface{}) *MinimaxTTSProvider {
	apiKey, _ := config["api_key"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["vol"].(float64)
	if volume == 0 {
		volume, _ = config["volume"].(float64)
	}
	pitch, _ := config["pitch"].(float64)
	sampleRate, _ := config["sample_rate"].(float64)
	bitrate, _ := config["bitrate"].(float64)
	format, _ := config["format"].(string)
	channel, _ := config["channel"].(float64)

	// 设置默认值
	if model == "" {
		model = "speech-2.8-hd"
	}
	if voice == "" {
		voice = "male-qn-qingse"
	}
	if speed == 0 {
		speed = 1.0
	}
	if volume == 0 {
		volume = 1.0
	}
	if sampleRate == 0 {
		sampleRate = 32000
	}
	if bitrate == 0 {
		bitrate = 128000
	}
	if format == "" {
		format = "mp3"
	}
	if channel == 0 {
		channel = 1
	}

	return &MinimaxTTSProvider{
		APIKey:     apiKey,
		Model:      model,
		Voice:      voice,
		Speed:      speed,
		Volume:     volume,
		Pitch:      int(pitch),
		SampleRate: int(sampleRate),
		Bitrate:    int(bitrate),
		Format:     format,
		Channel:    int(channel),
	}
}

// TextToSpeech 一次性合成（暂不支持，使用流式实现）
func (p *MinimaxTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// Minimax 主要支持流式，这里可以收集流式数据后返回
	outputChan, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}

	var frames [][]byte
	for frame := range outputChan {
		frames = append(frames, frame)
	}

	return frames, nil
}

// TextToSpeechStream 流式语音合成实现
func (p *MinimaxTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// 使用发送锁保护，确保同一时间只有一个请求在使用连接
	p.sendMutex.Lock()
	// 注意：不在函数返回时释放锁，而是在 goroutine 完成时释放

	// 获取连接（复用或创建）
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock()
		return nil, fmt.Errorf("获取WebSocket连接失败: %v", err)
	}

	// 创建输出通道
	outputChan = make(chan []byte, 100)

	// 创建管道用于音频解码
	pipeReader, pipeWriter := io.Pipe()

	// 启动音频解码器 goroutine
	go func() {
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, p.Format, sampleRate)
		if err != nil {
			log.Errorf("创建音频解码器失败: %v", err)
			pipeReader.Close()
			close(outputChan)
			return
		}

		if err := decoder.Run(startTs); err != nil {
			log.Errorf("音频解码失败: %v", err)
		}
	}()

	// 使用 WaitGroup 等待读取 goroutine 完成
	var wg sync.WaitGroup
	wg.Add(1)

	// 启动读取和处理 goroutine
	go func() {
		defer wg.Done()
		defer func() {
			pipeWriter.Close()
			pipeReader.Close()
		}()

		p.processStreamTTS(ctx, conn, text, pipeWriter)
	}()

	// 在后台等待 goroutine 完成并释放锁
	go func() {
		wg.Wait()
		log.Debugf("Minimax TTS流式合成完成，耗时: %d ms", time.Now().UnixMilli()-startTs)
	}()

	return outputChan, nil
}

// processStreamTTS 处理流式TTS合成流程
func (p *MinimaxTTSProvider) processStreamTTS(ctx context.Context, conn *websocket.Conn, text string, pipeWriter *io.PipeWriter) {
	// 发送任务开始消息
	startMsg := minimaxMessage{
		Event: "task_start",
		Model: p.Model,
		VoiceSetting: &voiceSetting{
			VoiceID:              p.Voice,
			Speed:                p.Speed,
			Vol:                  p.Volume,
			Pitch:                p.Pitch,
			EnglishNormalization: false,
		},
		AudioSetting: &audioSetting{
			SampleRate: p.SampleRate,
			Bitrate:    p.Bitrate,
			Format:     p.Format,
			Channel:    p.Channel,
		},
		ContinuousSound: false,
	}

	log.Debugf("minimax 发送任务开始消息: model=%s, voice=%s, format=%s", p.Model, p.Voice, p.Format)
	if err := p.sendMessage(conn, startMsg); err != nil {
		log.Errorf("发送任务开始消息失败: %v", err)
		p.clearConnection()
		p.sendMutex.Unlock()
		return
	}

	// 等待任务开始确认
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := p.readMessage(conn)
	if err != nil {
		// 检查是否是超时错误
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			log.Errorf("读取任务开始确认超时（10秒内未收到响应）")
		} else {
			log.Errorf("读取任务开始确认失败: %v", err)
		}
		p.clearConnection()
		p.sendMutex.Unlock()
		return
	}

	log.Debugf("收到任务开始确认消息: %+v", msg)

	if msg.Event != "task_started" {
		log.Errorf("任务开始失败，期望 'task_started'，收到: event=%s, 完整消息=%+v", msg.Event, msg)
		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("错误详情: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}
		p.clearConnection()
		p.sendMutex.Unlock()
		return
	}
	// 重置读取超时
	conn.SetReadDeadline(time.Time{})

	log.Debugf("任务开始确认成功")

	// 发送文本消息
	continueMsg := minimaxMessage{
		Event: "task_continue",
		Text:  text,
	}

	if err := p.sendMessage(conn, continueMsg); err != nil {
		log.Errorf("发送文本消息失败: %v", err)
		p.clearConnection()
		p.sendMutex.Unlock()
		return
	}

	// 读取音频数据
	chunkCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Debugf("Minimax TTS流式合成取消, 文本: %s", text)
			// 发送任务结束消息
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// 根据文档，服务器收到 task_finish 后会关闭 WebSocket 连接
			// 尝试读取 task_finished 响应（如果服务器发送的话）
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if finishResp, err := p.readMessage(conn); err == nil {
				log.Debugf("收到任务结束确认: event=%s, 完整消息=%+v", finishResp.Event, finishResp)
			} else {
				// 连接可能已经关闭，这是正常行为
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugf("服务器已关闭连接（正常行为）")
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("关闭帧详情: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				} else {
					log.Debugf("读取任务结束确认失败: %v", err)
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("关闭帧详情: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				}
			}

			// 清空连接状态，因为服务器已经关闭了连接
			p.clearConnection()
			p.sendMutex.Unlock()
			return
		default:
		}

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		msg, err := p.readMessage(conn)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("读取WebSocket消息失败: %v", err)
				// 尝试获取关闭帧信息
				if closeErr, ok := err.(*websocket.CloseError); ok {
					log.Errorf("WebSocket关闭帧详情: code=%d, text=%s", closeErr.Code, closeErr.Text)
				}
				p.clearConnection()
				p.sendMutex.Unlock()
				return
			}
			// 正常关闭或读取错误
			log.Debugf("WebSocket连接关闭或读取错误: %v", err)
			if closeErr, ok := err.(*websocket.CloseError); ok {
				log.Debugf("WebSocket关闭帧详情: code=%d, text=%s", closeErr.Code, closeErr.Text)
			}
			p.sendMutex.Unlock()
			return
		}

		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("BaseResp: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}

		// 检查是否有错误消息
		if msg.Event == "error" || msg.Event == "task_error" {
			log.Errorf("收到错误消息: %+v", msg)
			if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
				log.Errorf("错误详情: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
			}
			p.clearConnection()
			p.sendMutex.Unlock()
			return
		}

		// 处理音频数据
		if msg.Data != nil && msg.Data.Audio != "" {
			chunkCount++

			// 将 hex 编码的音频数据转换为二进制
			audioBytes, err := hex.DecodeString(msg.Data.Audio)
			if err != nil {
				log.Errorf("解码音频数据失败: %v", err)
				continue
			}

			// 写入管道供解码器处理
			if _, err := pipeWriter.Write(audioBytes); err != nil {
				log.Errorf("写入音频数据到管道失败: %v", err)
				return
			}
		}

		// 检查是否完成
		if msg.IsFinal {
			log.Debugf("收到最后一个音频片段，共%d个片段", chunkCount)
			// 发送任务结束消息
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// 清空连接状态，因为服务器已经关闭了连接
			// 下次使用时需要创建新连接
			p.clearConnection()
			p.sendMutex.Unlock()
			return
		}
	}
}

// getConnection 获取连接，如果不存在则创建
func (p *MinimaxTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// 先尝试读取现有连接
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	if conn != nil {
		return conn, nil
	}

	// 需要创建新连接
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// 双重检查，可能其他 goroutine 已经创建了连接
	if p.conn != nil {
		return p.conn, nil
	}

	// 创建HTTP头
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// 创建新连接
	conn, resp, err := wsDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			log.Errorf("WebSocket连接失败，状态码: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("WebSocket连接失败: %v", err)
	}

	// 设置消息读取限制
	conn.SetReadLimit(1024 * 1024) // 1MB 最大消息大小

	// 设置保持连接
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(1*time.Second))
	})

	// 等待连接成功消息
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取连接确认消息失败: %v", err)
	}

	log.Debugf("收到连接确认消息（原始）: %s", string(message))

	var connectMsg minimaxResp
	if err := json.Unmarshal(message, &connectMsg); err != nil {
		conn.Close()
		log.Errorf("解析连接确认消息失败，原始消息: %s, 错误: %v", string(message), err)
		return nil, fmt.Errorf("解析连接确认消息失败: %v", err)
	}

	log.Debugf("收到连接确认消息（解析后）: %+v", connectMsg)

	if connectMsg.Event != "connected_success" {
		conn.Close()
		log.Errorf("连接失败，期望 'connected_success'，收到: %+v", connectMsg)
		return nil, fmt.Errorf("连接失败，收到: %+v", connectMsg)
	}

	p.conn = conn
	log.Infof("Minimax WebSocket 连接已建立")
	return conn, nil
}

// clearConnection 清空连接（用于断线重连）
func (p *MinimaxTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("Minimax WebSocket 连接已清空，等待下次重连")
	}
}

// sendMessage 发送JSON消息
func (p *MinimaxTTSProvider) sendMessage(conn *websocket.Conn, msg minimaxMessage) error {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("连接已关闭")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	log.Debugf("minimax 发送消息: %s", string(data))

	return conn.WriteMessage(websocket.TextMessage, data)
}

// readMessage 读取JSON消息
func (p *MinimaxTTSProvider) readMessage(conn *websocket.Conn) (*minimaxResp, error) {
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	_ = messageType
	//log.Debugf("minimax 读取到WebSocket消息: type=%d, 原始内容长度=%d, 内容=%s", messageType, len(message), string(message))

	var msg minimaxResp
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("解析消息失败，原始消息: %s, 错误: %v", string(message), err)
		return nil, fmt.Errorf("解析消息失败: %v", err)
	}

	return &msg, nil
}

// SetVoice 设置音色参数
func (p *MinimaxTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return nil
}

// Close 关闭资源，释放连接
func (p *MinimaxTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid 检查资源是否有效
func (p *MinimaxTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	return conn != nil
}
