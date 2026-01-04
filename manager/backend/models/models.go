package models

import (
	"time"
)

// 用户模型
type User struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Username  string    `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"type:varchar(255);not null"`
	Email     string    `json:"email" gorm:"type:varchar(100);uniqueIndex"`
	Role      string    `json:"role" gorm:"type:varchar(20);not null;default:'user'"` // admin, user
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 设备模型
type Device struct {
	ID           uint       `json:"id" gorm:"primarykey"`
	UserID       uint       `json:"user_id" gorm:"not null"`
	AgentID      uint       `json:"agent_id" gorm:"not null;default:0"`               // 智能体ID，一台设备只能属于一个智能体
	DeviceCode   string     `json:"device_code" gorm:"type:varchar(100);uniqueIndex"` // 6位激活码
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
