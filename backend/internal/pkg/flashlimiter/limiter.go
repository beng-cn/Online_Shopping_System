package flashlimiter

import (
	"sync"
	"time"
)

// UserRateLimiter 秒杀专用 per-user 内存限流器
// 基于令牌桶算法，每个用户独立计数，纯内存操作（零Redis开销）
type UserRateLimiter struct {
	visitors sync.Map  // map[uint]*userBucket
	rate     float64   // 每秒允许的请求数
	burst    int       // 桶容量（突发上限）
}

// userBucket 单个用户的令牌桶
type userBucket struct {
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewUserRateLimiter 创建用户级别限流器
// rate: 每秒允许请求数（建议 1，即每人每秒最多1次抢购请求）
// burst: 突发容量（建议 1，即无突发）
func NewUserRateLimiter(rate, burst int) *UserRateLimiter {
	return &UserRateLimiter{
		rate:  float64(rate),
		burst: burst,
	}
}

// Allow 检查指定用户是否允许通过
// 返回 true 表示可以请求，false 表示需要等待
func (l *UserRateLimiter) Allow(userID uint) bool {
	bucket := l.getBucket(userID)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastTime).Seconds()

	// 补充令牌
	bucket.tokens += elapsed * l.rate
	if bucket.tokens > float64(l.burst) {
		bucket.tokens = float64(l.burst)
	}
	bucket.lastTime = now

	// 判断是否有可用令牌
	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}

// getBucket 获取或创建用户的令牌桶
func (l *UserRateLimiter) getBucket(userID uint) *userBucket {
	val, _ := l.visitors.LoadOrStore(userID, &userBucket{
		tokens:   float64(l.burst),
		lastTime: time.Now(),
	})
	return val.(*userBucket)
}

// CleanupExpired 清理长时间未活动的用户记录（防止内存泄漏）
// 建议每5分钟调用一次
func (l *UserRateLimiter) CleanupExpired(maxAge time.Duration) {
	now := time.Now()
	l.visitors.Range(func(key, value interface{}) bool {
		bucket := value.(*userBucket)
		bucket.mu.Lock()
		if now.Sub(bucket.lastTime) > maxAge {
			bucket.mu.Unlock()
			l.visitors.Delete(key)
			return true
		}
		bucket.mu.Unlock()
		return true
	})
}
