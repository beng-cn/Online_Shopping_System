package redis

import (
	"backend/internal/config"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// 初始化Redis连接（接收AppConfig参数）
func InitRedis(cfg *config.AppConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	return rdb, nil
}
