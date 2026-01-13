package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/vad/ten_vad"

	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func genFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// 计算样本数
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)
	// 创建静音缓冲区
	var buf bytes.Buffer
	// 32位浮点静音值为0.0
	for i := 0; i < numSamples*channels; i++ {
		binary.Write(&buf, binary.LittleEndian, float32(0.0))
	}
	//将数据转换为float32
	float32Data := make([]float32, numSamples*channels)
	for i := 0; i < numSamples*channels; i++ {
		float32Data[i] = float32(buf.Bytes()[i])
	}
	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		result = append(result, float32Data)
	}
	return result
}

func genOpusFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// 计算样本数
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)

	audioProcesser, err := audio.GetAudioProcesser(sampleRate, channels, 20)
	if err != nil {
		fmt.Printf("获取解码器失败: %v", err)
		return nil
	}

	pcmFrame := make([]int16, numSamples)

	opusFrame := make([]byte, 1000)
	n, err := audioProcesser.Encoder(pcmFrame, opusFrame)
	if err != nil {
		fmt.Printf("解码失败: %v", err)
		return nil
	}

	//将opus数据转换为float32
	pcmFloat32 := make([]float32, n)
	for i := 0; i < n; i++ {
		pcmFloat32[i] = float32(opusFrame[i])
	}

	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		tmp := make([]float32, n)
		copy(tmp, pcmFloat32)
		result = append(result, tmp)
	}
	return result
}

func main() {
	// 检查命令行参数
	if len(os.Args) < 2 {
		log.Fatalf("用法: %s <wav文件路径> [hop_size] [threshold]\n示例: %s test.wav 512 0.3", os.Args[0], os.Args[0])
	}

	wavFilePath := os.Args[1]

	// 解析可选参数
	hopSize := 512
	threshold := 0.3
	if len(os.Args) >= 3 {
		_, err := fmt.Sscanf(os.Args[2], "%d", &hopSize)
		if err != nil {
			log.Printf("无效的 hop_size 参数，使用默认值 512")
			hopSize = 512
		}
	}
	if len(os.Args) >= 4 {
		_, err := fmt.Sscanf(os.Args[3], "%f", &threshold)
		if err != nil {
			log.Printf("无效的 threshold 参数，使用默认值 0.3")
			threshold = 0.3
		}
	}

	// 读取WAV文件
	wavFile, err := os.Open(wavFilePath)
	if err != nil {
		log.Fatalf("无法打开WAV文件: %v", err)
	}
	defer wavFile.Close()

	// 读取整个文件内容
	wavData, err := io.ReadAll(wavFile)
	if err != nil {
		log.Fatalf("无法读取WAV文件: %v", err)
	}

	fmt.Printf("成功读取WAV文件: %s (%d 字节)\n", wavFilePath, len(wavData))

	// 调用 Wav2Pcm 函数转换WAV数据为PCM数据
	// 使用TEN-VAD支持的标准参数：16000Hz采样率，单声道
	sampleRate := 16000
	channels := 1

	pcmFloat32, pcmBytes, err := Wav2Pcm(wavData, sampleRate, channels)
	if err != nil {
		log.Fatalf("WAV转PCM失败: %v", err)
	}

	_ = pcmBytes

	fmt.Printf("成功转换为PCM数据，共 %d 帧（每帧20ms）\n", len(pcmFloat32))

	// 创建TEN-VAD实例
	config := map[string]interface{}{
		"hop_size":  hopSize,
		"threshold": threshold,
	}
	vadImpl, err := ten_vad.NewTenVAD(config)
	if err != nil {
		log.Fatalf("创建TEN-VAD失败: %v", err)
	}
	defer vadImpl.Close()

	fmt.Printf("TEN-VAD创建成功 (hop_size=%d, threshold=%.2f)，开始测试...\n", hopSize, threshold)

	// 直接测试VAD是否能正常工作
	if len(pcmFloat32) == 0 {
		log.Fatalf("没有PCM数据可供处理")
	}

	// 将所有帧合并成连续的音频数据
	// 因为 TEN-VAD 需要按 hopSize 分帧，而不是按 20ms 分帧
	totalSamples := 0
	for _, frame := range pcmFloat32 {
		totalSamples += len(frame)
	}
	allPcmData := make([]float32, 0, totalSamples)
	for _, frame := range pcmFloat32 {
		allPcmData = append(allPcmData, frame...)
	}

	fmt.Printf("合并后的音频数据: %d 个样本 (%.2f 秒)\n", len(allPcmData), float64(len(allPcmData))/float64(sampleRate))

	// 检查音频数据范围（用于调试）
	if len(allPcmData) > 0 {
		minVal := allPcmData[0]
		maxVal := allPcmData[0]
		for _, v := range allPcmData {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		fmt.Printf("音频数据范围: [%.6f, %.6f]\n", minVal, maxVal)
		// 如果数据不在 [-1.0, 1.0] 范围内，可能需要归一化
		if maxVal > 1.0 || minVal < -1.0 {
			fmt.Printf("警告: 音频数据超出 [-1.0, 1.0] 范围，可能需要归一化\n")
		}
	}

	fmt.Println("开始进行语音活动检测...")

	// 按 hopSize 分帧进行检测
	detectVoice := func(pcmData []float32) {
		speechFrames := 0
		totalFrames := 0
		var speechFramesData []float32 // 收集所有有声音的帧

		// 按 hopSize 分帧处理
		for i := 0; i < len(pcmData); i += hopSize {
			end := i + hopSize
			if end > len(pcmData) {
				end = len(pcmData)
			}

			frame := pcmData[i:end]

			// 如果帧长度不足 hopSize，填充零或跳过
			if len(frame) < hopSize {
				// 填充零到 hopSize 长度
				paddedFrame := make([]float32, hopSize)
				copy(paddedFrame, frame)
				frame = paddedFrame
			}

			totalFrames++

			// 进行VAD检测
			isVoice, err := vadImpl.IsVADExt(frame, sampleRate, hopSize)
			if err != nil {
				log.Printf("第%d帧VAD检测失败: %v", totalFrames, err)
				// 如果是第一帧就失败，说明VAD未正确初始化
				if totalFrames == 1 {
					log.Fatalf("VAD初始化失败，请检查TEN-VAD配置和库文件")
				}
				continue
			}

			if isVoice {
				speechFrames++
				// 收集有声音的帧数据（使用原始帧，不包含填充）
				originalFrame := pcmData[i:end]
				speechFramesData = append(speechFramesData, originalFrame...)
				fmt.Printf("第%d帧: 检测到语音活动 (样本范围: %d-%d)\n", totalFrames, i, end-1)
			} else {
				fmt.Printf("第%d帧: 无语音活动 (样本范围: %d-%d)\n", totalFrames, i, end-1)
			}
		}

		// 输出统计结果
		speechPercentage := float64(speechFrames) / float64(totalFrames) * 100
		nonSpeechFrames := totalFrames - speechFrames
		fmt.Printf("\n=== TEN-VAD检测结果统计 ===\n")
		fmt.Printf("总帧数: %d (每帧 %d 样本, %.2f ms)\n", totalFrames, hopSize, float64(hopSize)/float64(sampleRate)*1000)
		fmt.Printf("语音帧数: %d\n", speechFrames)
		fmt.Printf("非语音帧数: %d\n", nonSpeechFrames)
		fmt.Printf("语音活动比例: %.2f%%\n", speechPercentage)

		if speechFrames > 0 {
			fmt.Println("结论: 检测到语音活动")

			// 保存有声音的帧到WAV文件
			outputFileName := generateOutputFileName(wavFilePath)
			err := saveFloat32ToWav(speechFramesData, outputFileName, sampleRate, channels)
			if err != nil {
				log.Printf("保存有声音的帧到WAV文件失败: %v", err)
			} else {
				fmt.Printf("成功将有声音的帧保存到: %s (共 %d 个样本, %.2f 秒)\n",
					outputFileName, len(speechFramesData), float64(len(speechFramesData))/float64(sampleRate))
			}
		} else {
			fmt.Println("结论: 未检测到语音活动")
		}
	}

	// 使用合并后的完整音频数据进行测试
	detectVoice(allPcmData)
}

func float32ToByte(pcmFrame []float32) []byte {
	byteData := make([]byte, len(pcmFrame)*4)
	for i, sample := range pcmFrame {
		binary.LittleEndian.PutUint32(byteData[i*4:], math.Float32bits(sample))
	}
	return byteData
}

// generateOutputFileName 生成输出文件名，在原文件名基础上添加 "_speech" 后缀
func generateOutputFileName(inputPath string) string {
	dir := filepath.Dir(inputPath)
	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	outputName := nameWithoutExt + "_speech" + ext
	return filepath.Join(dir, outputName)
}

// saveFloat32ToWav 将 float32 PCM 数据保存为 WAV 文件
func saveFloat32ToWav(pcmData []float32, fileName string, sampleRate int, channels int) error {
	// 创建输出文件
	wavFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("创建WAV文件失败: %v", err)
	}
	defer wavFile.Close()

	// 创建WAV编码器
	wavEncoder := wav.NewEncoder(wavFile, sampleRate, 16, channels, 1)

	// 将 float32 转换为 int16
	// float32 范围通常是 [-1.0, 1.0]，需要缩放到 int16 范围 [-32768, 32767]
	intData := make([]int, len(pcmData))
	for i, sample := range pcmData {
		// 限制范围到 [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		}
		if sample < -1.0 {
			sample = -1.0
		}
		// 转换为 int16 范围
		intSample := int(sample * 32767.0)
		if intSample > 32767 {
			intSample = 32767
		}
		if intSample < -32768 {
			intSample = -32768
		}
		intData[i] = intSample
	}

	// 创建音频缓冲区
	audioBuf := &goaudio.IntBuffer{
		Format: &goaudio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           intData,
	}

	// 写入WAV文件
	err = wavEncoder.Write(audioBuf)
	if err != nil {
		return fmt.Errorf("写入WAV文件失败: %v", err)
	}

	// 关闭编码器
	err = wavEncoder.Close()
	if err != nil {
		return fmt.Errorf("关闭WAV编码器失败: %v", err)
	}

	return nil
}
