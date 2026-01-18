package client

import (
	"bytes"
	"context"
	"sync"
	asr_types "xiaozhi-esp32-server-golang/internal/domain/asr/types"
	log "xiaozhi-esp32-server-golang/logger"
)

type Asr struct {
	lock sync.RWMutex
	// ASR 上下文和通道
	Ctx              context.Context
	Cancel           context.CancelFunc
	AsrEnd           chan bool
	AsrAudioChannel  chan []float32                 //流式音频输入的channel
	AsrResultChannel chan asr_types.StreamingResult //流式输出asr识别到的结果片断
	AsrResult        bytes.Buffer                   //保存此次识别到的最终文本
	Statue           int                            //0:初始化 1:识别中 2:识别结束
	AutoEnd          bool                           //auto_end是指使用asr自动判断结束，不再使用vad模块

	// ASR 类型和模式
	AsrType string // ASR 类型，如 "funasr", "doubao"
	Mode    string // ASR 模式，如 "online", "offline"

	// ClientState 引用，用于回调通知
	ClientState *ClientState

	// 聊天历史音频缓存：持续累积发送到ASR的音频数据
	HistoryAudioBuffer []float32
}

func (a *Asr) Reset() {
	a.AsrResult.Reset()
}

func (a *Asr) RetireAsrResult(ctx context.Context) (string, bool, error) {
	defer func() {
		a.Reset()
	}()

	log.Log().Debugf("asr type: %s, mode: %s", a.AsrType, a.Mode)

	// 使用局部变量跟踪是否已发送首次字符事件
	firstTextSent := false

	for {
		select {
		case <-ctx.Done():
			return "", false, nil
		case result, ok := <-a.AsrResultChannel:
			log.Debugf("asr result: %s, ok: %+v, isFinal: %+v, error: %+v", result.Text, ok, result.IsFinal, result.Error)
			if result.Error != nil {
				return "", false, result.Error
			}

			// 检测首次返回字符（文本不为空且未发送过）
			if result.Text != "" && !firstTextSent && a.ClientState != nil && a.ClientState.OnAsrFirstTextCallback != nil {
				firstTextSent = true
				// 调用回调函数通知首次字符
				a.ClientState.OnAsrFirstTextCallback(result.Text, result.IsFinal)
			}

			// 如果是 funasr 的流式模式（online），直接返回 IsFinal 中的文字
			if a.AsrType == "funasr" && a.Mode == "2pass" && result.IsFinal {
				return result.Text, true, nil
			}

			// 其他情况按原有逻辑执行
			a.AsrResult.WriteString(result.Text)
			if a.AutoEnd || result.IsFinal {
				text := a.AsrResult.String()
				return text, true, nil
			}
			if !ok {
				log.Debugf("asr result channel closed")
				return "", true, nil
			}
		}
	}
}

func (a *Asr) Stop() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.AsrAudioChannel != nil {
		log.Debugf("停止asr")
		close(a.AsrAudioChannel) //close掉asr输入音频的channel，通知asr停止, 返回结果
		a.AsrAudioChannel = nil  //由于已经close，所以需要置空
	}
}

func (a *Asr) AddAudioData(pcmFrameData []float32) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.AsrAudioChannel != nil {
		a.AsrAudioChannel <- pcmFrameData

		// 同步缓存音频数据用于聊天历史记录
		a.HistoryAudioBuffer = append(a.HistoryAudioBuffer, pcmFrameData...)
	}
	return nil
}

// GetHistoryAudio 获取历史音频缓存（返回副本，不清空原始数据）
func (a *Asr) GetHistoryAudio() []float32 {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.HistoryAudioBuffer) == 0 {
		return nil
	}
	// 返回副本，避免外部修改影响原始数据
	result := make([]float32, len(a.HistoryAudioBuffer))
	copy(result, a.HistoryAudioBuffer)
	return result
}

// ClearHistoryAudio 清空历史音频缓存
func (a *Asr) ClearHistoryAudio() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.HistoryAudioBuffer = nil
}

type AsrAudioBuffer struct {
	PcmData          []float32
	AudioBufferMutex sync.RWMutex
}

func (a *AsrAudioBuffer) AddAsrAudioData(pcmFrameData []float32) error {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = append(a.PcmData, pcmFrameData...)
	return nil
}

func (a *AsrAudioBuffer) GetAsrDataSize() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData)
}

// GetFrameCount 获取帧数（需要传入帧大小用于计算）
func (a *AsrAudioBuffer) GetFrameCount(frameSize int) int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	if frameSize == 0 {
		return 0
	}
	return len(a.PcmData) / frameSize
}

func (a *AsrAudioBuffer) GetAndClearAllData() []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmData := make([]float32, len(a.PcmData))
	copy(pcmData, a.PcmData)
	a.PcmData = []float32{}
	return pcmData
}

// GetAsrData 滑动窗口进行取数据（需要传入帧大小用于计算）
func (a *AsrAudioBuffer) GetAsrData(frameCount int, frameSize int) []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmDataLen := len(a.PcmData)
	retSize := frameCount * frameSize
	if pcmDataLen < retSize {
		retSize = pcmDataLen
	}
	pcmData := make([]float32, retSize)
	copy(pcmData, a.PcmData[pcmDataLen-retSize:])
	return pcmData
}

// RemoveAsrAudioData 移除指定帧数的音频数据（需要传入帧大小用于计算）
func (a *AsrAudioBuffer) RemoveAsrAudioData(frameCount int, frameSize int) {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	removeSize := frameCount * frameSize
	if removeSize > len(a.PcmData) {
		removeSize = len(a.PcmData)
	}
	a.PcmData = a.PcmData[removeSize:]
}

func (a *AsrAudioBuffer) ClearAsrAudioData() {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = nil
}
