package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// 用户模型
type User struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Username  string    `json:"username" gorm:"type:varchar(50);uniqueIndex:idx_users_username;not null"`
	Password  string    `json:"-" gorm:"type:varchar(255);not null"`
	Email     string    `json:"email" gorm:"type:varchar(100);uniqueIndex:idx_users_email"`
	Role      string    `json:"role" gorm:"type:varchar(20);not null;default:'user'"` // admin, user
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 设备模型
type Device struct {
	ID           uint       `json:"id" gorm:"primarykey"`
	UserID       uint       `json:"user_id" gorm:"not null"`
	AgentID      uint       `json:"agent_id" gorm:"not null;default:0"`                                       // 智能体ID，一台设备只能属于一个智能体
	DeviceCode   string     `json:"device_code" gorm:"type:varchar(100);uniqueIndex:idx_devices_device_code"` // 6位激活码
	DeviceName   string     `json:"device_name" gorm:"type:varchar(100)"`
	Challenge    string     `json:"challenge" gorm:"type:varchar(128)"`      // 激活挑战码
	PreSecretKey string     `json:"pre_secret_key" gorm:"type:varchar(128)"` // 预激活密钥
	Activated    bool       `json:"activated" gorm:"default:false"`          // 设备是否已激活
	LastActiveAt *time.Time `json:"last_active_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// 智能体模型
type Agent struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	UserID       uint      `json:"user_id" gorm:"not null"`
	Name         string    `json:"name" gorm:"type:varchar(100);not null"`             // 昵称
	CustomPrompt string    `json:"custom_prompt" gorm:"type:text"`                     // 角色介绍(prompt)
	LLMConfigID  *string   `json:"llm_config_id" gorm:"type:varchar(100)"`             // 语言模型配置ID
	TTSConfigID  *string   `json:"tts_config_id" gorm:"type:varchar(100)"`             // 音色配置ID
	Voice        *string   `json:"voice" gorm:"type:varchar(200)"`                     // 音色值
	ASRSpeed     string    `json:"asr_speed" gorm:"type:varchar(20);default:'normal'"` // 语音识别速度: normal/patient/fast
	Status       string    `json:"status" gorm:"type:varchar(20);default:'active'"`    // active, inactive
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// 通用配置模型
type Config struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Type      string    `json:"type" gorm:"type:varchar(50);not null;uniqueIndex:type_config_id,priority:1"` // vad, asr, llm, tts, ota, mqtt, udp, mqtt_server, vision
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	ConfigID  string    `json:"config_id" gorm:"type:varchar(100);not null;uniqueIndex:type_config_id,priority:2"` // 配置ID，用于关联
	Provider  string    `json:"provider" gorm:"type:varchar(50)"`                                                  // 某些配置类型需要provider字段
	JsonData  string    `json:"json_data" gorm:"type:text"`                                                        // JSON配置数据
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	IsDefault bool      `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 全局角色模型
type GlobalRole struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Prompt      string    `json:"prompt" gorm:"type:text"`
	IsDefault   bool      `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChatMessage 聊天消息模型
type ChatMessage struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	MessageID string    `json:"message_id" gorm:"type:varchar(64);uniqueIndex:idx_chat_messages_message_id;not null"`

	// 关联信息（不使用外键）
	DeviceID  string    `json:"device_id" gorm:"type:varchar(100);index:idx_device_id;not null"`
	AgentID   string    `json:"agent_id" gorm:"type:varchar(64);index:idx_agent_id;not null"`
	UserID    uint      `json:"user_id" gorm:"index:idx_user_id;not null"`
	SessionID string    `json:"session_id" gorm:"type:varchar(64);index:idx_session_id"` // 仅作分组标记

	// 消息内容
	Role    string `json:"role" gorm:"type:varchar(20);index;not null;comment:user|assistant|system|tool"`
	Content string `json:"content" gorm:"type:text;not null"`

	// 音频文件信息 (文件系统存储，两级hash打散)
	AudioPath     string `json:"audio_path,omitempty" gorm:"type:varchar(512);comment:音频文件相对路径（两级hash打散）"`
	AudioDuration *int   `json:"audio_duration,omitempty" gorm:"comment:毫秒"`
	AudioSize     *int   `json:"audio_size,omitempty" gorm:"comment:字节"`
	AudioFormat   string `json:"audio_format,omitempty" gorm:"type:varchar(20);default:'wav';comment:音频格式（固定为wav）"`

	// 元数据
	MetadataJSON string                 `json:"-" gorm:"type:json;column:metadata"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" gorm:"-"`

	// 状态
	IsDeleted bool      `json:"is_deleted" gorm:"default:false;index"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_created_at"`
}

// TableName 指定表名
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// BeforeSave GORM hook - 序列化metadata
func (m *ChatMessage) BeforeSave(tx *gorm.DB) error {
	if m.Metadata != nil {
		data, err := json.Marshal(m.Metadata)
		if err != nil {
			return err
		}
		m.MetadataJSON = string(data)
	}
	return nil
}

// AfterFind GORM hook - 反序列化metadata
func (m *ChatMessage) AfterFind(tx *gorm.DB) error {
	if m.MetadataJSON != "" {
		return json.Unmarshal([]byte(m.MetadataJSON), &m.Metadata)
	}
	return nil
}
