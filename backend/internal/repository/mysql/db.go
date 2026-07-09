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

	// 使用配置文件中的连接池参数，如果未配置则使用默认值
	maxOpenConns := 50
	maxIdleConns := 10
	connMaxLifetime := 1 * time.Hour
	connMaxIdleTime := 10 * time.Minute

	if cfg.DBPool.MaxOpenConns > 0 {
		maxOpenConns = cfg.DBPool.MaxOpenConns
	}
	if cfg.DBPool.MaxIdleConns > 0 {
		maxIdleConns = cfg.DBPool.MaxIdleConns
	}
	if cfg.DBPool.ConnMaxLifetime > 0 {
		connMaxLifetime = time.Duration(cfg.DBPool.ConnMaxLifetime) * time.Second
	}
	if cfg.DBPool.ConnMaxIdleTime > 0 {
		connMaxIdleTime = time.Duration(cfg.DBPool.ConnMaxIdleTime) * time.Second
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	return db, nil
}
