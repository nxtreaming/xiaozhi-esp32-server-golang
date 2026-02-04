package edge_offline

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	"github.com/gorilla/websocket"
)

// EdgeOfflineTTSProvider WebSocket TTS 提供者
type EdgeOfflineTTSProvider struct {
	ServerURL        string
	Timeout          time.Duration
	HandshakeTimeout time.Duration

	// 连接管理
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// 发送锁，确保同一时间只有一个请求在使用连接
	sendMutex sync.Mutex
}

// NewEdgeOfflineTTSProvider 创建新的 Edge Offline TTS 提供者
func NewEdgeOfflineTTSProvider(config map[string]interface{}) *EdgeOfflineTTSProvider {
	serverURL, _ := config["server_url"].(string)
	timeout, _ := config["timeout"].(float64)
	handshakeTimeout, _ := config["handshake_timeout"].(float64)

	// 设置默认值
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tts"
	}
	if timeout == 0 {
		timeout = 30 // 默认30秒超时
	}
	if handshakeTimeout == 0 {
		handshakeTimeout = 10 // 默认10秒握手超时
	}

	return &EdgeOfflineTTSProvider{
		ServerURL:        serverURL,
		Timeout:          time.Duration(timeout) * time.Second,
		HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
	}
}

// getConnection 获取连接，如果不存在则创建
func (p *EdgeOfflineTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
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

	// 创建新连接
	dialer := &websocket.Dialer{
		HandshakeTimeout: p.HandshakeTimeout,
	}
	conn, _, err := dialer.DialContext(ctx, p.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket连接失败: %v", err)
	}

	p.conn = conn
	log.Infof("WebSocket 连接已建立")
	return conn, nil
}

// clearConnection 清空连接（用于断线重连）
func (p *EdgeOfflineTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("WebSocket 连接已清空，等待下次重连")
	}
}

// writeMessage 安全地向 WebSocket 连接写入消息
func (p *EdgeOfflineTTSProvider) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	// 使用读锁保护连接写入操作，防止并发写入导致数据混乱
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	// 检查连接是否有效
	if conn == nil {
		return fmt.Errorf("连接已关闭")
	}

	return conn.WriteMessage(messageType, data)
}

// TextToSpeech 将文本转换为语音，返回音频帧数据
func (p *EdgeOfflineTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	var frames [][]byte

	// 使用发送锁保护，确保同一时间只有一个请求在使用连接
	p.sendMutex.Lock()
	// 注意：不在函数返回时释放锁，而是在 goroutine 完成时释放

	// 获取连接（复用或创建）
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock() // 获取连接失败时立即释放锁
		return nil, err
	}

	// 发送文本（使用受保护的写入方法）
	err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
	if err != nil {
		// 发送失败，清空连接，下次使用时自动重连
		log.Errorf("发送文本失败: %v，清空连接", err)
		p.clearConnection()
		p.sendMutex.Unlock() // 发送失败时立即释放锁
		return nil, fmt.Errorf("发送文本失败: %v", err)
	}

	// 创建管道用于音频数据传输
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	startTs := time.Now().UnixMilli()

	// 创建音频解码器
	audioDecoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		pipeReader.Close()
		p.sendMutex.Unlock() // 创建解码器失败时立即释放锁
		return nil, fmt.Errorf("创建音频解码器失败: %v", err)
	}

	// 启动解码器
	go func() {
		if err := audioDecoder.Run(startTs); err != nil {
			log.Errorf("音频解码失败: %v", err)
		}
	}()

	// 使用 WaitGroup 等待读取 goroutine 完成
	var wg sync.WaitGroup
	wg.Add(1)

	// 接收WebSocket数据并写入管道；锁在此 goroutine 内统一由 defer 释放，确保无论正常结束、错误或 panic 都会释放
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		defer p.sendMutex.Unlock()
		defer close(done)
		defer pipeWriter.Close()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				log.Errorf("读取WebSocket消息失败: %v，清空连接", err)
				// 连接断开，清空连接，下次使用时自动重连
				p.clearConnection()
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := pipeWriter.Write(data); err != nil {
					log.Errorf("写入音频数据失败: %v", err)
					return
				}
			}
		}
	}()

	// 收集所有的Opus帧
	go func() {
		for frame := range outputChan {
			frames = append(frames, frame)
		}
	}()

	// 等待完成或超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("TTS合成超时或被取消")
	case <-done:
		close(outputChan)
		return frames, nil
	}
}

// TextToSpeechStream 流式语音合成
func (p *EdgeOfflineTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 100)

	go func() {
		// 使用发送锁保护，确保同一时间只有一个请求在使用连接
		p.sendMutex.Lock()

		// 获取连接（复用或创建）
		conn, err := p.getConnection(ctx)
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("获取WebSocket连接失败: %v", err)
			return
		}

		// 发送文本（使用受保护的写入方法）
		err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("发送文本失败: %v，清空连接", err)
			// 发送失败，清空连接，下次使用时自动重连
			p.clearConnection()
			return
		}

		// 创建管道用于音频数据传输
		pipeReader, pipeWriter := io.Pipe()
		defer func() {
			pipeWriter.Close()
			// 读取完成后释放锁
			log.Debugf("TextToSpeechStream read completed, release sendMutex")
			p.sendMutex.Unlock()
		}()

		// 启动解码器（解码器会在 defer 中自动关闭 outputChan）
		go func() {

			startTs := time.Now().UnixMilli()
			// 创建音频解码器
			audioDecoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, "pcm", sampleRate)
			if err != nil {
				log.Errorf("创建音频解码器失败: %v", err)
				return
			}

			audioDecoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(24000),
				NumChannels: channels,
				Precision:   2,
			})

			// 解码器会在 defer 中自动关闭 outputChan
			if err := audioDecoder.Run(startTs); err != nil {
				log.Errorf("音频解码失败: %v", err)
			}
		}()

		// 接收WebSocket数据并写入管道（读取过程中持有锁，确保串行化）
		for {
			select {
			case <-ctx.Done():
				log.Debugf("TextToSpeechStream context done, exit")
				// 关闭 pipeWriter，让解码器自然结束并关闭 channel
				return
			default:
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					// 关闭 pipeWriter，让解码器自然结束并关闭 channel
					pipeWriter.Close()
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return
					}
					log.Errorf("读取WebSocket消息失败: %v，清空连接", err)
					// 连接断开，清空连接，下次使用时自动重连
					p.clearConnection()
					return
				}

				if messageType == websocket.BinaryMessage {
					if _, err := pipeWriter.Write(data); err != nil {
						log.Errorf("写入音频数据失败: %v", err)
						return
					}
					return
				}
			}
		}
	}()

	return outputChan, nil
}

// SetVoice 设置音色参数（EdgeOffline 不支持动态设置音色，但不报错）
func (p *EdgeOfflineTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	// EdgeOffline 通过 WebSocket 连接，音色由服务端控制，不支持客户端动态设置
	// 返回 nil 表示操作成功（虽然实际上不执行任何操作）
	return nil
}

// Close 关闭资源，释放连接
func (p *EdgeOfflineTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid 检查资源是否有效
func (p *EdgeOfflineTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	// 检查连接是否存在
	return conn != nil
}
