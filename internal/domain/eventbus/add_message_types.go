package eventbus

import (
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"

	"github.com/cloudwego/eino/schema"
)

// AddMessageEvent 统一的消息添加事件
type AddMessageEvent struct {
	// 客户端状态
	ClientState *ClientState

	// 消息内容（统一使用 schema.Message）
	// schema.Message 是标准的 LLM 消息格式，包含：
	// - Role: 消息角色（User/Assistant/System/Tool）
	// - Content: 消息文本内容
	// - ToolCalls: 工具调用列表（可选）
	// - ToolCallID: 工具调用ID（Tool 角色使用）
	Msg schema.Message

	// 消息ID（用于关联两阶段保存）
	MessageID string

	// 音频数据（可选，不属于 schema.Message 标准格式）
	// 第一阶段：AudioData = nil（仅保存文本）
	// 第二阶段：AudioData != nil（更新音频）
	AudioData [][]byte // TTS/ASR 音频帧数组（Opus格式或PCM格式）
	AudioSize int      // 音频大小（字节）

	// 音频格式信息（不属于 schema.Message 标准格式）
	SampleRate int // 采样率
	Channels   int // 通道数

	// 元数据（不属于 schema.Message 标准格式）
	Timestamp   time.Time
	TTSDuration int // TTS 耗时（毫秒）

	// 阶段标识
	IsUpdate bool // true=更新音频，false=新增消息
}
