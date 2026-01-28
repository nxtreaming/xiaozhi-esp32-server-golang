package zhipu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/data/audio"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	sse "github.com/tmaxmax/go-sse"
)

// 全局HTTP客户端，实现连接池
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// 获取配置了连接池的HTTP客户端
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		}
	})
	return httpClient
}

// ZhipuTTSProvider 智谱 TTS提供者
type ZhipuTTSProvider struct {
	APIKey         string
	APIURL         string
	Model          string
	Voice          string
	ResponseFormat string
	Speed          float64
	Volume         float64
	Stream         bool
	EncodeFormat   string // 仅流式时使用：base64 或 hex
	FrameDuration  int
}

// 请求结构体（根据智谱 API 文档）
type zhipuRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Volume         float64 `json:"volume,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
	EncodeFormat   string  `json:"encode_format,omitempty"` // 仅流式时使用：base64 或 hex
}

// Event Stream 响应结构体（类似 OpenAI 格式）
type zhipuEventStreamResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason,omitempty"`
		Delta        struct {
			Role             string `json:"role,omitempty"`
			Content          string `json:"content,omitempty"` // base64 编码的音频数据
			ReturnSampleRate int    `json:"return_sample_rate,omitempty"`
			ReturnFormat     string `json:"return_format,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
}

// NewZhipuTTSProvider 创建新的智谱 TTS提供者
func NewZhipuTTSProvider(config map[string]interface{}) *ZhipuTTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	responseFormat, _ := config["response_format"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["volume"].(float64)
	stream, _ := config["stream"].(bool)
	encodeFormat, _ := config["encode_format"].(string)
	frameDuration, _ := config["frame_duration"].(float64)

	// 设置默认值
	if apiURL == "" {
		apiURL = "https://open.bigmodel.cn/api/paas/v4/audio/speech"
	}
	if model == "" {
		model = "glm-tts"
	}
	if voice == "" {
		voice = "tongtong" // 默认音色
	}
	if responseFormat == "" {
		responseFormat = "pcm" // 智谱默认 pcm，也支持 wav
	}
	if speed == 0 {
		speed = 1.0 // 0.5 到 2.0
	}
	if volume == 0 {
		volume = 1.0 // 0 到 10
	}
	if encodeFormat == "" {
		encodeFormat = "base64" // 默认 base64，也支持 hex
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &ZhipuTTSProvider{
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		Voice:          voice,
		ResponseFormat: responseFormat,
		Stream:         stream,
		Speed:          speed,
		Volume:         volume,
		EncodeFormat:   encodeFormat,
		FrameDuration:  int(frameDuration),
	}
}

// TextToSpeech 将文本转换为语音，返回音频帧数据和错误
func (p *ZhipuTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()

	// 限制文本长度（智谱 API 最大 1024 字符）
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("文本长度超过1024字符，已截断")
	}

	// 创建请求体
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: p.ResponseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         false, // 非流式
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// 使用连接池发送请求
	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 检查响应内容长度
	contentLength := resp.ContentLength
	log.Debugf("收到智谱 TTS响应，Content-Length: %d", contentLength)

	// 判断Content-Length是否合理
	if contentLength == 0 {
		log.Errorf("API返回空响应，Content-Length为0")
		return nil, fmt.Errorf("API返回空响应，Content-Length为0")
	}

	// 根据音频格式处理响应（智谱只支持 wav 和 pcm）
	if p.ResponseFormat == "wav" || p.ResponseFormat == "pcm" {
		// 创建一个通道来收集音频帧
		outputChan := make(chan []byte, 1000)

		// 创建音频解码器
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, p.ResponseFormat, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("创建音频解码器失败: %v", err)
		}

		// 启动解码过程
		go func() {
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("音频解码失败: %v", err)
			}
		}()

		// 收集所有的音频帧
		var audioFrames [][]byte
		for frame := range outputChan {
			audioFrames = append(audioFrames, frame)
		}

		log.Debugf("智谱 TTS完成，从输入到获取音频数据结束耗时: %d ms", time.Now().UnixMilli()-startTs)
		return audioFrames, nil
	}

	return nil, fmt.Errorf("不支持的音频格式: %s，智谱仅支持 wav 和 pcm", p.ResponseFormat)
}

// TextToSpeechStream 流式语音合成实现
func (p *ZhipuTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// 限制文本长度（智谱 API 最大 1024 字符）
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("文本长度超过1024字符，已截断")
	}

	// 流式时只支持 pcm和wav 格式
	responseFormat := p.ResponseFormat

	// 创建请求体
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: responseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         true,           // 流式
		EncodeFormat:   p.EncodeFormat, // 使用配置的编码格式
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// 使用连接池创建客户端
	client := getHTTPClient()

	// 创建输出通道
	outputChan = make(chan []byte, 100)

	// 启动goroutine处理流式响应
	go func() {
		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("发送智谱请求失败: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		// 检查响应状态码
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("智谱 API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// 检查 Content-Type 是否为 Event Stream
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			log.Warnf("智谱 API返回的Content-Type不是text/event-stream: %s", contentType)
		}

		// 流式时只支持 pcm 和 wav 格式
		if responseFormat == "pcm" || responseFormat == "wav" {
			// 创建管道，用于将解码后的二进制数据传递给音频解码器
			pipeReader, pipeWriter := io.Pipe()

			// 启动 goroutine 解析 Event Stream 并解码
			go func() {
				defer func() {
					if err := pipeWriter.Close(); err != nil {
						log.Debugf("关闭管道写入端失败: %v", err)
					}
				}()

				// 调用独立的解析方法
				if err := p.parseEventStream(ctx, resp.Body, pipeWriter, text); err != nil {
					log.Errorf("解析 Event Stream 失败: %v", err)
				}
			}()

			// 创建音频解码器，从管道读取解码后的二进制数据
			decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, responseFormat, sampleRate)
			if err != nil {
				log.Errorf("创建智谱音频解码器失败: %v", err)
				pipeReader.Close()
				close(outputChan)
				return
			}

			// 启动解码过程
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("智谱音频解码失败: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("智谱 TTS流式合成取消, 文本: %s", text)
				return
			default:
				log.Debugf("智谱 TTS耗时: 从输入至获取音频数据结束耗时: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("智谱流式输出仅支持 pcm 格式")
			close(outputChan)
		}
	}()

	return outputChan, nil
}

// parseEventStream 使用 go-sse 解析智谱的 Event Stream 响应，解码数据并写入管道
// ctx: 上下文，用于取消操作
// reader: 响应体读取器
// writer: 管道写入端，用于输出解码后的二进制数据
// text: 原始文本，用于日志记录
func (p *ZhipuTTSProvider) parseEventStream(ctx context.Context, reader io.Reader, writer *io.PipeWriter, text string) error {
	// 配置 go-sse 的 ReadConfig，设置更大的 MaxEventSize 以处理长 token
	// 智谱 TTS 返回的 base64 编码音频数据可能超过默认的 64KB 限制
	readConfig := &sse.ReadConfig{
		MaxEventSize: 4 * 1024 * 1024, // 4MB，足够处理大型 base64 编码的音频数据
	}

	for ev, evErr := range sse.Read(reader, readConfig) {
		if evErr != nil {
			return fmt.Errorf("读取智谱 SSE 事件失败: %w", evErr)
		}

		select {
		case <-ctx.Done():
			log.Debugf("智谱 TTS流式合成取消, 文本: %s", text)
			return ctx.Err()
		default:
		}

		// Event Stream 格式：
		// data: {"id":"...","choices":[{"delta":{"content":"base64_data"}}]}
		// data: {"choices":[{"finish_reason":"stop"}]}

		dataValue := strings.TrimSpace(ev.Data)
		if dataValue == "" {
			continue
		}

		// 解析 JSON
		var eventResp zhipuEventStreamResponse
		if err := json.Unmarshal([]byte(dataValue), &eventResp); err != nil {
			log.Warnf("解析智谱 Event Stream JSON 失败: %v, 数据: %s", err, previewString(dataValue, 200))
			continue
		}

		// 检查是否有 finish_reason，表示流结束
		for _, choice := range eventResp.Choices {
			if choice.FinishReason == "stop" {
				log.Debugf("收到 finish_reason: stop，Event Stream 结束")
				return nil
			}
		}

		// 提取每个 choice 的 content 字段并独立处理
		for _, choice := range eventResp.Choices {
			if choice.Delta.Content != "" {
				// 每个 content 独立解码并写入
				if err := p.decodeAndWriteContent(choice.Delta.Content, writer); err != nil {
					return fmt.Errorf("处理 content 失败: %v", err)
				}
			}
		}
	}

	return nil
}

// previewString 返回字符串的前 n 个字符用于日志
func previewString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// decodeAndWriteContent 解码单个 content 字段并写入管道
// content: base64 或 hex 编码的音频数据字符串
// writer: 管道写入端
func (p *ZhipuTTSProvider) decodeAndWriteContent(content string, writer *io.PipeWriter) error {
	if content == "" {
		return nil
	}

	// 清理空白字符（base64 和 hex 编码不应该包含空白字符）
	// 移除所有空白字符：空格、换行、制表符等
	content = strings.ReplaceAll(content, " ", "")
	content = strings.ReplaceAll(content, "\n", "")
	content = strings.ReplaceAll(content, "\r", "")
	content = strings.ReplaceAll(content, "\t", "")

	if content == "" {
		return nil
	}

	// 根据 encode_format 解码
	var decodedData []byte
	var decodeErr error

	switch p.EncodeFormat {
	case "base64":
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	case "hex":
		decodedData, decodeErr = hex.DecodeString(content)
	default:
		log.Warnf("未知的编码格式: %s，使用 base64", p.EncodeFormat)
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	}

	if decodeErr != nil {
		return fmt.Errorf("解码音频数据失败: %v, 数据长度: %d", decodeErr, len(content))
	}

	// 将解码后的数据写入管道
	if len(decodedData) > 0 {
		if _, err := writer.Write(decodedData); err != nil {
			return fmt.Errorf("写入管道失败: %v", err)
		}
	}

	return nil
}

// SetVoice 设置音色参数
func (p *ZhipuTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("无效的音色配置: 缺少 voice")
}

// Close 关闭资源（无状态 Provider，无需关闭）
func (p *ZhipuTTSProvider) Close() error {
	return nil
}

// IsValid 检查资源是否有效
func (p *ZhipuTTSProvider) IsValid() bool {
	return p != nil
}
