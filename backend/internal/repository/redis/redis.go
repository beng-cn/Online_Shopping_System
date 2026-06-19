package redis

import (
	"backend/internal/config"
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.AppConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		// 连接池配置
		PoolSize:     20,               // 连接池大小
		MinIdleConns: 5,                // 最小空闲连接数
		IdleTimeout:  5 * time.Minute,  // 空闲连接超时
		DialTimeout:  5 * time.Second,  // 连接超时
		ReadTimeout:  3 * time.Second,  // 读超时
		WriteTimeout: 3 * time.Second,  // 写超时
	})

	// 验证 Redis 连接是否可用（fail-fast）
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	return rdb, nil
}
