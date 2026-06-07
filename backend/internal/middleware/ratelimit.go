package middleware

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 基于令牌桶的 IP 限流中间件
// 每个 IP 独立计数，防止单 IP 恶意刷接口
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // 每秒允许的请求数
	burst    int           // 突发允许的最大请求数
	cleanup  time.Duration // 清理过期访客的间隔
}

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter 创建限流器实例
// rate: 每秒填充的令牌数, burst: 桶容量（允许的突发请求数）
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		cleanup:  5 * time.Minute,
	}
	// 后台定时清理过期访客记录，防止内存泄漏
	go rl.cleanupLoop()
	return rl
}

// Handler 返回 Gin 中间件处理函数
func (rl *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.allow(ip) {
			response.Error(c, errors.New(errors.CodeForbidden, "请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// allow 检查指定 IP 是否允许通过（令牌桶算法）
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists {
		// 新访客：初始令牌数 = burst - 1（消耗当前请求的1个令牌）
		rl.visitors[ip] = &visitor{
			tokens:   float64(rl.burst) - 1,
			lastSeen: now,
		}
		return true
	}

	// 计算经过的时间内新增的令牌数
	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * float64(rl.rate)
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}
	v.lastSeen = now

	// 判断是否有足够令牌
	if v.tokens < 1 {
		return false
	}

	v.tokens--
	return true
}

// cleanupLoop 定时清理长时间未访问的访客记录
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for ip, v := range rl.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}
