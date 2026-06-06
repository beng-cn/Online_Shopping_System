package mysql

import (
	"backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 初始化数据库连接（接收AppConfig参数）
func InitDB(cfg *config.AppConfig) (*gorm.DB, error) {
	dsn := cfg.MySQL.DSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
