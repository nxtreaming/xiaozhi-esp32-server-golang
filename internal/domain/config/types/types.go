package types

type AsrConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type TtsConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type MemoryConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type LlmConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type VadConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

type ConfigItem struct {
	Provider string                 `json:"provider"`
	JsonData map[string]interface{} `json:"json_data"`
}

type SpeakerGroupInfo struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Prompt      string   `json:"prompt"`
	Description string   `json:"description"`
	Uuids       []string `json:"uuids"`
	TTSConfigID *string  `json:"tts_config_id"`
	Voice       *string  `json:"voice"`
	// 当音色来自复刻时，运行时用于覆盖 TTS 模型。
	VoiceModelOverride *string `json:"voice_model_override,omitempty"`
}

type KnowledgeBaseRef struct {
	ID                 uint     `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Provider           string   `json:"provider"`
	ExternalKBID       string   `json:"external_kb_id"`
	ExternalDocID      string   `json:"external_doc_id"`
	RetrievalThreshold *float64 `json:"retrieval_threshold"`
	Status             string   `json:"status"`
}

type UConfig struct {
	SystemPrompt    string                      `json:"system_prompt"`
	Asr             AsrConfig                   `json:"asr"`
	Tts             TtsConfig                   `json:"tts"`
	Llm             LlmConfig                   `json:"llm"`
	Vad             VadConfig                   `json:"vad"`
	Memory          MemoryConfig                `json:"memory"`
	VoiceIdentify   map[string]SpeakerGroupInfo `json:"voice_identify"`    // 声纹识别配置
	MemoryMode      string                      `json:"memory_mode"`       // 记忆模式: none/short/long
	AgentId         string                      `json:"agent_id"`          // 所属agent_id
	MCPServiceNames string                      `json:"mcp_service_names"` // 逗号分隔的MCP服务名，空=使用全部已启用全局MCP服务
	KnowledgeBases  []KnowledgeBaseRef          `json:"knowledge_bases"`
}

type TtsConfigItem struct {
	ConfigID  string                 `json:"config_id"`
	Name      string                 `json:"name"`
	Provider  string                 `json:"provider"`
	Config    map[string]interface{} `json:"config"`
	IsDefault bool                   `json:"is_default"`
}

type KnowledgeSearchHit struct {
	Content string  `json:"content"`
	Title   string  `json:"title,omitempty"`
	Score   float64 `json:"score,omitempty"`
}
