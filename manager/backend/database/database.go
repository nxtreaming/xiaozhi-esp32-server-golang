package database

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) *gorm.DB {
	var db *gorm.DB
	var err error

	if cfg.Database == "sqlite" {
		// SQLite 数据库连接
		log.Println("使用SQLite数据库:", cfg.Host)
		db, err = gorm.Open(sqlite.Open(cfg.Host), &gorm.Config{})
	} else {
		// MySQL 数据库连接
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Println("数据库连接失败:", err)
		log.Println("将使用fallback模式运行（硬编码用户验证）")
		return nil
	}

	log.Println("数据库连接成功")

	// 自动迁移数据库表结构
	log.Println("开始自动迁移数据库表结构...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.GlobalRole{},
		&models.ChatMessage{},
	)
	if err != nil {
		log.Printf("数据库表结构迁移失败: %v", err)
		log.Println("将使用fallback模式运行（硬编码用户验证）")
		return nil
	}
	log.Println("数据库表结构迁移成功")

	return db
}

func Close(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("获取数据库连接失败:", err)
		return
	}
	sqlDB.Close()
}
