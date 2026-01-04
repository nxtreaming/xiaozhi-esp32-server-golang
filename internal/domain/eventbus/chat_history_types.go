package eventbus

import (
	"context"
	"time"
)

// UserMessageEvent 用户消息事件
// Deprecated: 使用 AddMessageEvent 替代，统一使用 TopicAddMessage 事件
type UserMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	// ASR结果
	Text      string
	AudioData []byte  // 原始音频数据（PCM float32 转字节）
	AudioSize int     // 音频采样数

	// 音频格式信息（用于转换为WAV）
	SampleRate int // 采样率
	Channels   int // 通道数

	// 元数据
	Timestamp time.Time
}

// AssistantMessageEvent 机器人回复事件
// Deprecated: 使用 AddMessageEvent 替代，统一使用 TopicAddMessage 事件
type AssistantMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	// LLM结果
	Text string

	// TTS结果
	AudioData [][]byte // 合成音频数据（Opus格式，音频帧数组）
	AudioSize int      // 音频大小(字节)

	// 音频格式信息（用于转换为WAV）
	SampleRate int // 采样率
	Channels   int // 通道数

	// 元数据
	TTSDuration int // 毫秒
	Timestamp   time.Time
}
