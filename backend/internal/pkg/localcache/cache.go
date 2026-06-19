// Package localcache 本地内存两级缓存（L1 本地 + L2 Redis）
//
// 设计目标：
//   - 作为 L1 缓存层，在 Redis（L2）之前拦截热点请求
//   - 本地内存访问延迟 ~50ns，Redis 网络延迟 ~0.5ms，性能差距 10000x
//   - 适合缓存高频访问的热点数据（如首页商品列表、秒杀活动信息）
//
// 面试亮点：两级缓存架构在微服务中的典型应用，理解缓存层级间的性能差异和一致性权衡
package localcache

import (
	"sort"
	"sync"
	"time"
)

// entry 缓存条目
type entry struct {
	value      interface{}
	expireAt   time.Time
	lastAccess time.Time // 最近一次被访问的时间（LRU 淘汰用）
}

// Cache 线程安全的本地内存缓存（支持 TTL 过期）
type Cache struct {
	mu         sync.RWMutex
	items      map[string]*entry
	maxSize    int
	cleanupInt time.Duration
	stopCh     chan struct{}
}

// New 创建本地缓存实例
//
// 参数：
//
//	maxSize     — 最大缓存条目数（防止内存无限增长）
//	cleanupInt  — 后台清理间隔（定期删除过期条目）
func New(maxSize int, cleanupInt time.Duration) *Cache {
	c := &Cache{
		items:      make(map[string]*entry, maxSize),
		maxSize:    maxSize,
		cleanupInt: cleanupInt,
		stopCh:     make(chan struct{}),
	}
	// 启动后台清理协程
	go c.cleanupLoop()
	return c
}

// Get 获取缓存值
//
// 返回 nil 表示未命中或已过期
func (c *Cache) Get(key string) interface{} {
	c.mu.RLock()
	ent, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}
	if time.Now().After(ent.expireAt) {
		c.Delete(key) // 惰性删除
		return nil
	}
	ent.lastAccess = time.Now() // 更新访问时间（LRU）
	return ent.value
}

// Set 设置缓存值
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 容量保护：超过最大条目数时 LRU 淘汰 20%
	if len(c.items) >= c.maxSize {
		c.evict(20)
	}

	now := time.Now()
	c.items[key] = &entry{
		value:      value,
		expireAt:   now.Add(ttl),
		lastAccess: now,
	}
}

// Delete 删除缓存条目
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Size 返回当前缓存条目数
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// NewDefault 创建默认配置的本地缓存（供 Wire 依赖注入使用）
// 默认：10000 条目 + 5 分钟清理间隔
func NewDefault() *Cache {
	return New(10000, 5*time.Minute)
}

// Close 停止后台清理协程
func (c *Cache) Close() {
	close(c.stopCh)
}

// evict LRU 淘汰指定百分比的条目（内部方法，调用前需持有写锁）
// 优先清理过期条目（零成本回收），不足时按 lastAccess 升序淘汰最久未访问的
func (c *Cache) evict(percent int) {
	target := len(c.items) * percent / 100
	now := time.Now()

	// 第1步：优先清理已过期条目（白捡的空间）
	for key, ent := range c.items {
		if now.After(ent.expireAt) {
			delete(c.items, key)
		}
	}
	// 过期清理后已满足目标，无需进一步淘汰
	if len(c.items) <= c.maxSize-target {
		return
	}

	// 第2步：按 lastAccess 升序排列，淘汰最久未访问的
	type kv struct {
		key        string
		lastAccess time.Time
	}
	sorted := make([]kv, 0, len(c.items))
	for key, ent := range c.items {
		sorted = append(sorted, kv{key, ent.lastAccess})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lastAccess.Before(sorted[j].lastAccess)
	})

	for i := 0; i < target && i < len(sorted); i++ {
		delete(c.items, sorted[i].key)
	}
}

// cleanupLoop 后台定期清理过期条目
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInt)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cache) cleanup() {
	now := time.Now()
	c.mu.Lock()
	for key, ent := range c.items {
		if now.After(ent.expireAt) {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}
