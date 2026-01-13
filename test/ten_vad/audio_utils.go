package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"gopkg.in/hraban/opus.v2"
)

// WavToOpus 将WAV音频数据转换为标准Opus格式
// 返回Opus帧的切片集合，每个切片是一个Opus编码帧
func WavToOpus(wavData []byte, sampleRate int, channels int, bitRate int) ([][]byte, error) {
	// 创建WAV解码器
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("无效的WAV文件")
	}

	// 读取WAV文件信息
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()
	wavSampleRate := int(format.SampleRate)
	wavChannels := int(format.NumChannels)

	// 如果提供的参数与文件参数不一致，使用文件中的参数
	if sampleRate == 0 {
		sampleRate = wavSampleRate
	}
	if channels == 0 {
		channels = wavChannels
	}

	//打印wavDecoder信息
	fmt.Println("WAV格式:", format)

	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("创建Opus编码器失败: %v", err)
	}

	// 设置比特率
	if bitRate > 0 {
		if err := enc.SetBitrate(bitRate); err != nil {
			return nil, fmt.Errorf("设置比特率失败: %v", err)
		}
	}

	// 创建输出帧切片数组
	opusFrames := make([][]byte, 0)

	perFrameDuration := 60
	// PCM缓冲区 - Opus帧大小(60ms)
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)
	opusBuffer := make([]byte, 1000) // 足够大的缓冲区存储编码后的数据

	// 读取音频缓冲区
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("开始转换...")
	for {
		// 读取WAV数据
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取WAV数据失败: %v", err)
		}

		// 将int转换为int16
		for i := 0; i < len(audioBuf.Data); i++ {
			if i < len(pcmBuffer) {
				pcmBuffer[i] = int16(audioBuf.Data[i])
			}
		}

		// 编码为Opus格式
		n, err = enc.Encode(pcmBuffer, opusBuffer)
		if err != nil {
			return nil, fmt.Errorf("编码失败: %v", err)
		}

		// 将当前帧复制到新的切片中并添加到帧数组
		frameData := make([]byte, n)
		copy(frameData, opusBuffer[:n])
		opusFrames = append(opusFrames, frameData)
	}

	return opusFrames, nil
}

func OpusToWav(opusData [][]byte, sampleRate int, channels int, fileName string) ([][]int16, error) {
	opusDecoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("创建Opus解码器失败: %v", err)
	}

	wavOut, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("创建WAV文件失败: %v", err)
	}

	pcmDataList := make([][]int16, 0)
	pcmBuffer := make([]int16, 8192)

	wavEncoder := wav.NewEncoder(wavOut, sampleRate, 16, channels, 1)
	wavBuffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels, // 使用传入的通道数
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           make([]int, 8192),
	}

	for _, frame := range opusData {
		n, err := opusDecoder.Decode(frame, pcmBuffer)
		if err != nil {
			return nil, fmt.Errorf("解码失败: %v", err)
		}
		copyData := make([]int16, len(pcmBuffer[:n]))
		copy(copyData, pcmBuffer[:n])
		//fmt.Println("decode pcmData len: ", len(copyData))
		pcmDataList = append(pcmDataList, copyData)

		//fmt.Println("pcmData len: ", len(copyData))

		// 将PCM数据转换为int格式
		for i := 0; i < len(copyData); i++ {
			wavBuffer.Data = append(wavBuffer.Data, int(copyData[i]))
		}
	}

	// 写入WAV文件
	err = wavEncoder.Write(&wavBuffer)
	if err != nil {
		return nil, fmt.Errorf("写入WAV文件失败: %v", err)
	}

	wavEncoder.Close()

	return pcmDataList, nil
}

func Wav2Pcm(wavData []byte, sampleRate int, channels int) ([][]float32, [][]byte, error) {
	// 创建WAV解码器
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, nil, fmt.Errorf("无效的WAV文件")
	}

	// 读取WAV文件信息
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()

	fmt.Println("WAV格式:", format)

	perFrameDuration := 20
	// PCM缓冲区 - 20ms帧大小
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)

	// 读取音频缓冲区
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("开始转换...")
	resultFloat32 := make([][]float32, 0)
	result := make([][]byte, 0)
	for {
		// 读取WAV数据
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("读取WAV数据失败: %v", err)
		}

		// 将int转换为int16
		for i := 0; i < len(audioBuf.Data); i++ {
			if i < len(pcmBuffer) {
				pcmBuffer[i] = int16(audioBuf.Data[i])
			}
		}

		float32Data := audioBuf.AsFloat32Buffer()
		resultFloat32 = append(resultFloat32, float32Data.Data)

		// 将int16数组转换为字节数组
		frameBytes := PcmInt16ToByte(pcmBuffer)

		result = append(result, frameBytes)
	}

	return resultFloat32, result, nil
}

func PcmInt16ToByte(pcmData []int16) []byte {
	byteData := make([]byte, len(pcmData)*2)
	for i := 0; i < len(pcmData); i++ {
		byteData[i*2] = byte(pcmData[i] & 0xFF)
		byteData[i*2+1] = byte((pcmData[i] >> 8) & 0xFF)
	}
	return byteData
}
