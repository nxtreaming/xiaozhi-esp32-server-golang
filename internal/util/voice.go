package util

import (
	"bytes"
	"encoding/binary"
	"math"
)

// PCM16BytesToFloat32 将16位PCM小端字节流转换为float32切片（范围-1.0~1.0）
func PCM16BytesToFloat32(pcm []byte) []float32 {
	n := len(pcm) / 2
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		// 取两个字节，按小端序转为int16
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		out[i] = float32(sample) / float32(math.MaxInt16)
	}
	return out
}

// float32ToPCMBytes 将 float32 数组转换为 16-bit PCM 字节数组
func Float32ToPCMBytes(samples []float32, pcmBytes []byte) {
	for i, sample := range samples {
		// 将 float32 (-1.0 到 1.0) 转换为 int16 (-32768 到 32767)
		intSample := float32ToInt16(sample)

		// 小端序写入字节数组
		binary.LittleEndian.PutUint16(pcmBytes[i*2:], uint16(intSample))
	}

	return
}

// Float32ToInt16 将float32值转换为int16值（范围-1.0~1.0转换为-32768~32767）
func float32ToInt16(sample float32) int16 {
	if sample > 1.0 {
		return 32767
	} else if sample < -1.0 {
		return -32768
	} else {
		return int16(sample * 32767)
	}
}

// Float32SliceToInt16Slice 将float32切片转换为int16切片
func Float32SliceToInt16Slice(samples []float32) []int16 {
	result := make([]int16, len(samples))
	for i, sample := range samples {
		result[i] = float32ToInt16(sample)
	}
	return result
}

// int16SliceToBytes 将int16切片转换为[]byte（小端序）
func Int16SliceToBytes(samples []int16) []byte {
	buf := new(bytes.Buffer)
	for _, s := range samples {
		buf.WriteByte(byte(s))
		buf.WriteByte(byte(s >> 8))
	}
	return buf.Bytes()
}

func ResampleLinearFloat32(input []float32, inRate, outRate int) []float32 {
	ratio := float64(outRate) / float64(inRate)
	outLen := int(float64(len(input)) * ratio)
	output := make([]float32, outLen)

	for i := 0; i < outLen; i++ {
		pos := float64(i) / ratio
		index := int(pos)
		if index >= len(input)-1 {
			output[i] = input[len(input)-1]
		} else {
			frac := float32(pos - float64(index))
			output[i] = input[index]*(1-frac) + input[index+1]*frac
		}
	}
	return output
}

// Float32SliceToBytes 将 float32 数组转换为字节数组（小端序，每个float32占4字节）
func Float32SliceToBytes(data []float32) []byte {
	if len(data) == 0 {
		return nil
	}
	bytes := make([]byte, len(data)*4)
	for i, v := range data {
		binary.LittleEndian.PutUint32(bytes[i*4:i*4+4], math.Float32bits(v))
	}
	return bytes
}
