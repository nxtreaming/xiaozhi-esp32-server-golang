package client

import (
	"bytes"
	"context"
	"sync"
	"xiaozhi-esp32-server-golang/internal/domain/asr"
	asr_types "xiaozhi-esp32-server-golang/internal/domain/asr/types"
	log "xiaozhi-esp32-server-golang/logger"
)

type Asr struct {
	lock sync.RWMutex
	// ASR 提供者
	Ctx              context.Context
	Cancel           context.CancelFunc
	AsrProvider      asr.AsrProvider
	AsrEnd           chan bool
	AsrAudioChannel  chan []float32                 //流式音频输入的channel
	AsrResultChannel chan asr_types.StreamingResult //流式输出asr识别到的结果片断
	AsrResult        bytes.Buffer                   //保存此次识别到的最终文本
	Statue           int                            //0:初始化 1:识别中 2:识别结束
	AutoEnd          bool                           //auto_end是指使用asr自动判断结束，不再使用vad模块
}

func (a *Asr) Reset() {
	a.AsrResult.Reset()
}

func (a *Asr) RetireAsrResult(ctx context.Context) (string, bool, error) {
	defer func() {
		a.Reset()
	}()
	for {
		select {
		case <-ctx.Done():
			return "", false, nil
		case result, ok := <-a.AsrResultChannel:
			log.Debugf("asr result: %s, ok: %+v, isFinal: %+v, error: %+v", result.Text, ok, result.IsFinal, result.Error)
			if result.Error != nil {
				return "", false, result.Error
			}
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
	}
	return nil
}

type AsrAudioBuffer struct {
	PcmData          []float32
	AudioBufferMutex sync.RWMutex
	PcmFrameSize     int
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

func (a *AsrAudioBuffer) GetFrameCount() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData) / a.PcmFrameSize
}

func (a *AsrAudioBuffer) GetAndClearAllData() []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmData := make([]float32, len(a.PcmData))
	copy(pcmData, a.PcmData)
	a.PcmData = []float32{}
	return pcmData
}

// 滑动窗口进行取数据
func (a *AsrAudioBuffer) GetAsrData(frameCount int) []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmDataLen := len(a.PcmData)
	retSize := frameCount * a.PcmFrameSize
	if pcmDataLen < retSize {
		retSize = pcmDataLen
	}
	pcmData := make([]float32, retSize)
	copy(pcmData, a.PcmData[pcmDataLen-retSize:])
	return pcmData
}

func (a *AsrAudioBuffer) RemoveAsrAudioData(frameCount int) {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = a.PcmData[frameCount*a.PcmFrameSize:]
}

func (a *AsrAudioBuffer) ClearAsrAudioData() {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = nil
}
