package speaker

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// StreamingClient WebSocket 流式识别客户端
type StreamingClient struct {
	wsURL      string
	conn       *websocket.Conn
	sampleRate int
	mutex      sync.Mutex
}

// NewStreamingClient 创建流式识别客户端
func NewStreamingClient(baseURL string) *StreamingClient {
	wsURL := deriveWebSocketURL(baseURL)
	return &StreamingClient{
		wsURL: wsURL,
	}
}

// deriveWebSocketURL 从 HTTP base_url 推导 WebSocket URL
func deriveWebSocketURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		log.Errorf("解析 base_url 失败: %v, 使用默认值", err)
		return "ws://localhost:8080/api/v1/speaker/identify_ws"
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	return fmt.Sprintf("%s://%s/api/v1/speaker/identify_ws", scheme, u.Host)
}

// Connect 连接到声纹识别服务的 WebSocket
func (sc *StreamingClient) Connect(sampleRate int, agentId string, threshold float32) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.sampleRate = sampleRate

	// 如果已存在连接，使用 Ping 检测连接是否仍然有效
	if sc.conn != nil {
		if sc.pingConnectionLocked() {
			// 连接有效，复用现有连接
			return nil
		}
		// 连接已断开，关闭旧连接准备重连
		log.Debugf("检测到旧连接已断开，将重新建立连接")
		sc.closeConnectionLocked()
	}

	// 构建 WebSocket URL，包含采样率、agent_id 和 threshold 参数
	wsURL := fmt.Sprintf("%s?sample_rate=%d", sc.wsURL, sampleRate)
	if agentId != "" {
		wsURL += fmt.Sprintf("&agent_id=%s", url.QueryEscape(agentId))
	}
	// 如果阈值大于 0，则传递阈值参数
	if threshold > 0 {
		wsURL += fmt.Sprintf("&threshold=%.6f", threshold)
	}

	// 连接 WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %v", err)
	}

	sc.conn = conn

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// 接收连接确认消息
	var connectionMsg map[string]interface{}
	if err := conn.ReadJSON(&connectionMsg); err != nil {
		conn.Close()
		sc.conn = nil
		return fmt.Errorf("读取连接确认消息失败: %v", err)
	}

	if msgType, ok := connectionMsg["type"].(string); !ok || msgType != "connection" {
		conn.Close()
		sc.conn = nil
		return fmt.Errorf("意外的连接消息: %v", connectionMsg)
	}

	log.Debugf("声纹识别 WebSocket 连接成功，采样率: %d Hz, agent_id: %s, 阈值: %.4f", sampleRate, agentId, threshold)
	return nil
}

// SendAudioChunk 发送音频数据块
func (sc *StreamingClient) SendAudioChunk(audioData []float32) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if sc.conn == nil {
		return fmt.Errorf("not connected")
	}

	// 将 float32 数组转换为二进制字节
	chunkBytes := float32ToBytes(audioData)

	// 发送二进制消息
	if err := sc.conn.WriteMessage(websocket.BinaryMessage, chunkBytes); err != nil {
		// 发送失败时关闭连接
		sc.closeConnectionLocked()
		return fmt.Errorf("发送音频数据失败: %v", err)
	}

	return nil
}

// FinishAndIdentify 完成输入并获取识别结果
func (sc *StreamingClient) FinishAndIdentify() (*IdentifyResult, error) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if sc.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// 发送完成命令
	finishCmd := map[string]interface{}{
		"action": "finish",
	}
	if err := sc.conn.WriteJSON(finishCmd); err != nil {
		sc.closeConnectionLocked()
		return nil, fmt.Errorf("发送完成命令失败: %v", err)
	}

	// 设置读取超时
	sc.conn.SetReadDeadline(time.Now().Add(15 * time.Second))

	// 等待识别结果
	for {
		messageType, message, err := sc.conn.ReadMessage()
		if err != nil {
			sc.closeConnectionLocked()
			return nil, fmt.Errorf("读取消息失败: %v", err)
		}

		if messageType == websocket.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Warnf("解析消息失败: %v", err)
				continue
			}

			if msgType, ok := msg["type"].(string); ok {
				switch msgType {
				case "result":
					// 连接保持打开，供下次识别复用
					if resultData, ok := msg["result"].(map[string]interface{}); ok {
						result := &IdentifyResult{
							Identified:  getBool(resultData, "identified"),
							SpeakerID:   getString(resultData, "speaker_id"),
							SpeakerName: getString(resultData, "speaker_name"),
							Confidence:  getFloat32(resultData, "confidence"),
							Threshold:   getFloat32(resultData, "threshold"),
						}
						return result, nil
					}
				case "error":
					sc.closeConnectionLocked()
					if errMsg, ok := msg["message"].(string); ok {
						return nil, fmt.Errorf("服务器错误: %s", errMsg)
					}
				}
			}
		}
	}
}

// Close 关闭连接
func (sc *StreamingClient) Close() error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.closeConnectionLocked()
}

// closeConnectionLocked 关闭连接（必须在已持有 mutex 的情况下调用）
func (sc *StreamingClient) closeConnectionLocked() error {
	if sc.conn != nil {
		err := sc.conn.Close()
		sc.conn = nil
		return err
	}
	return nil
}

// IsConnected 检查是否已连接
func (sc *StreamingClient) IsConnected() bool {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.conn != nil
}

// pingConnectionLocked 使用 Ping 检测连接是否有效（必须在已持有 mutex 的情况下调用）
func (sc *StreamingClient) pingConnectionLocked() bool {
	if sc.conn == nil {
		return false
	}

	// 使用 Ping 消息检测连接活性
	sc.conn.SetWriteDeadline(time.Now().Add(1000 * time.Millisecond))
	err := sc.conn.WriteMessage(websocket.PingMessage, nil)
	sc.conn.SetWriteDeadline(time.Time{})

	return err == nil
}

// float32ToBytes 将 float32 数组转换为二进制字节（小端序）
func float32ToBytes(samples []float32) []byte {
	buf := make([]byte, len(samples)*4)
	for i, sample := range samples {
		bits := math.Float32bits(sample)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

// 辅助函数：从 map 中安全获取值
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getFloat32(m map[string]interface{}, key string) float32 {
	if v, ok := m[key].(float64); ok {
		return float32(v)
	}
	return 0.0
}
