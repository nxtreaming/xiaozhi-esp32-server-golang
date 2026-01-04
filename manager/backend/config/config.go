package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
	History  HistoryConfig  `json:"history"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Mode string `json:"mode"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	ExpireHour int    `json:"expire_hour"`
}

type HistoryConfig struct {
	Enabled       bool   `json:"enabled"`
	AudioBasePath string `json:"audio_base_path"` // 音频存储基础路径
	MaxFileSize   int64  `json:"max_file_size"`   // 最大文件大小(字节)，默认10MB
}

func Load() *Config {
	return LoadWithPath("config/config.json")
}

func LoadWithPath(configPath string) *Config {
	config := LoadFromFile(configPath)

	// 优先使用环境变量覆盖数据库配置
	if host := os.Getenv("DB_HOST"); host != "" {
		config.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		config.Database.Port = port
	}
	if username := os.Getenv("DB_USER"); username != "" {
		config.Database.Username = username
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		config.Database.Password = password
	}
	if database := os.Getenv("DB_NAME"); database != "" {
		config.Database.Database = database
	}

	// 优先使用环境变量覆盖音频存储路径
	if audioBasePath := os.Getenv("AUDIO_BASE_PATH"); audioBasePath != "" {
		config.History.AudioBasePath = audioBasePath
	}

	fmt.Println("config", config)

	return config
}

func LoadFromFile(configPath string) *Config {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("无法打开配置文件 %s: %v", configPath, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("解析配置文件失败 %s: %v", configPath, err)
	}

	return &config
}
