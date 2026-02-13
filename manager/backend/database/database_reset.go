package database

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitWithReset 初始化数据库并重置所有表（仅用于开发环境）
func InitWithReset(cfg config.DatabaseConfig) *gorm.DB {
	storageType := cfg.GetStorageType()
	var db *gorm.DB
	var err error

	if storageType == "sqlite" {
		if cfg.SQLite == nil {
			log.Fatal("SQLite配置为空")
		}
		db, err = gorm.Open(sqlite.Open(cfg.SQLite.FilePath), &gorm.Config{})
	} else {
		if cfg.MySQL == nil {
			log.Fatal("MySQL配置为空")
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	log.Println("警告：正在重置数据库表，所有数据将被删除！")

	// 删除所有表
	err = db.Migrator().DropTable(
		&models.User{},
		&models.Device{},
		&models.Agent{},
		&models.Config{},
		&models.GlobalRole{},
		&models.Role{},
		&models.SpeakerGroup{},
		&models.SpeakerSample{},
	)
	if err != nil {
		log.Printf("删除表时出现错误（可能表不存在）: %v", err)
	}

	log.Println("数据库表删除完成！")
	return db
}
