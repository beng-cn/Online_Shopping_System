package mysql

import (
	"backend/internal/config"
	"backend/internal/pkg/logger"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 初始化数据库连接（接收AppConfig参数）
func InitDB(cfg *config.AppConfig) (*gorm.DB, error) {
	dsn := cfg.MySQL.DSN()

	// 根据运行环境选择慢查询阈值
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "dev"
	}
	slowThreshold := logger.RecommendThreshold(env)

	// 创建 GORM Logger 插件：慢查询自动埋点
	gormLog := logger.NewSlowQueryLogger(slowThreshold)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLog, // 使用自定义慢查询日志记录器
	})
	if err != nil {
		return nil, err
	}

	// 配置连接池参数，防止高并发下连接耗尽
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)                 // 最大打开连接数
	sqlDB.SetMaxIdleConns(10)                 // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(1 * time.Hour)   // 连接最大存活时间
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大存活时间

	return db, nil
}
