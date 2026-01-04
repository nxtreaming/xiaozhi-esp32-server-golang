package eventbus

const (
	TopicAddMessage = "add_message"
	TopicSessionEnd = "session_end"

	// 聊天历史相关事件（已废弃，统一使用 TopicAddMessage）
	// Deprecated: 使用 TopicAddMessage 替代
	TopicChatHistoryUserMessage      = "chat_history_user_message"      // 用户消息(ASR后) - 已废弃
	TopicChatHistoryAssistantMessage = "chat_history_assistant_message" // 机器人回复(LLM+TTS后) - 已废弃
)
