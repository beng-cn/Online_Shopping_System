package redis

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

// 缓存策略常量配置
const (
	// 商品缓存基础过期时间
	productBaseExpire = 10 * time.Minute
	// 商品缓存最大过期时间（防止热点数据永久不过期）
	productMaxExpire = 1 * time.Hour
	// 空值缓存过期时间（防止缓存穿透）
	productNilExpire = 1 * time.Minute
	// 每次访问续期时长
	productRenewDuration = 5 * time.Minute
	// 分布式互斥锁过期时间（防止锁死）
	productLockExpire = 5 * time.Second
	// 互斥锁等待重试间隔
	productLockRetryInterval = 50 * time.Millisecond
	// 互斥锁最大等待时间
	productLockMaxWait = 2 * time.Second
)

// cacheTTLWithJitter 在基础 TTL 上叠加随机抖动（±20%），防止缓存雪崩
//
// 原理：如果所有缓存 Key 使用相同的过期时间，同一时刻大量 Key 同时过期会导致
// 数据库瞬间承受巨大压力。通过随机抖动，将过期时间分散在 [80%*TTL, 120%*TTL] 区间。
func cacheTTLWithJitter(base time.Duration) time.Duration {
	jitter := time.Duration(float64(base) * 0.2 * (rand.Float64()*2 - 1)) // ±20%
	return base + jitter
}

// nilCacheTTL 空值缓存 TTL 也加抖动（防同时过期导致穿透风暴）
func nilCacheTTL() time.Duration {
	return cacheTTLWithJitter(productNilExpire)
}

type ProductCache interface {
	GetProduct(id uint) (*entity.Product, error)
	SetProduct(product *entity.Product) error
	DeleteProduct(id uint) error
	GetProductList(keyword string, categoryID string) ([]*entity.Product, error)
	SetProductList(keyword string, categoryID string, products []*entity.Product) error
	ClearAllProductList() error
	WarmUpHotProducts(productRepo mysql.ProductRepository, limit int) error // 预热方法
	// 分布式互斥锁（防缓存击穿）
	TryLockProduct(id uint) bool
	UnlockProduct(id uint)
	WaitAndRetry(id uint) (*entity.Product, bool)
}

type productCache struct {
	rdb *redis.Client
	ctx context.Context
}

// NewProductCache 创建商品缓存实例
func NewProductCache(rdb *redis.Client) ProductCache {
	return &productCache{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (c *productCache) getProductKey(id uint) string {
	return fmt.Sprintf("product:item:%d", id)
}

func (c *productCache) getProductListKey(keyword string, categoryID string) string {
	return fmt.Sprintf("product:list:%s:%s", keyword, categoryID)
}

// GetProduct 增强版：支持自动续期、缓存降级、空值处理
func (c *productCache) GetProduct(id uint) (*entity.Product, error) {
	key := c.getProductKey(id)
	data, err := c.rdb.Get(c.ctx, key).Result()

	// 缓存降级核心：Redis错误或key不存在时返回nil，让上层直接查数据库
	if err != nil {
		if err != redis.Nil {
			log.Printf("⚠️ Redis商品缓存读取失败: %v", err)
		}
		return nil, nil
	}

	// 空值缓存处理（防止缓存穿透）
	if data == "nil" {
		return nil, nil
	}

	var product entity.Product
	if err := json.Unmarshal([]byte(data), &product); err != nil {
		log.Printf("⚠️ 商品缓存数据解析失败，已删除坏缓存: %v", err)
		c.rdb.Del(c.ctx, key)
		return nil, nil
	}

	// 热点数据自动续期：延长过期时间但不超过最大值
	ttl, err := c.rdb.TTL(c.ctx, key).Result()
	if err == nil && ttl < productMaxExpire {
		newTTL := ttl + productRenewDuration
		if newTTL > productMaxExpire {
			newTTL = productMaxExpire
		}
		if err := c.rdb.Expire(c.ctx, key, newTTL).Err(); err != nil {
			log.Printf("⚠️ 商品缓存续期失败: %v", err)
		}
	}

	return &product, nil
}

// SetProduct 增强版：支持空值缓存 + TTL随机抖动防雪崩
func (c *productCache) SetProduct(product *entity.Product) error {
	key := c.getProductKey(product.ID)

	// 空值缓存（当数据库查询不到时，缓存"nil"标记）
	if product == nil {
		return c.rdb.Set(c.ctx, key, "nil", nilCacheTTL()).Err()
	}

	data, err := json.Marshal(product)
	if err != nil {
		return errors.Wrap(err, "序列化商品缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, cacheTTLWithJitter(productBaseExpire)).Err()
}

// DeleteProduct 删除单品缓存（更新商品后保证缓存一致性）
func (c *productCache) DeleteProduct(id uint) error {
	key := c.getProductKey(id)
	return c.rdb.Del(c.ctx, key).Err()
}

// GetProductList 增强版：支持缓存降级
func (c *productCache) GetProductList(keyword string, categoryID string) ([]*entity.Product, error) {
	key := c.getProductListKey(keyword, categoryID)
	data, err := c.rdb.Get(c.ctx, key).Result()

	if err != nil {
		if err != redis.Nil {
			log.Printf("⚠️ Redis商品列表缓存读取失败: %v", err)
		}
		return nil, nil
	}

	if data == "nil" {
		return nil, nil
	}

	var products []*entity.Product
	if err := json.Unmarshal([]byte(data), &products); err != nil {
		log.Printf("⚠️ 商品列表缓存数据解析失败，已删除坏缓存: %v", err)
		c.rdb.Del(c.ctx, key)
		return nil, nil
	}

	// 列表缓存自动续期
	ttl, err := c.rdb.TTL(c.ctx, key).Result()
	if err == nil && ttl < productMaxExpire {
		newTTL := ttl + productRenewDuration
		if newTTL > productMaxExpire {
			newTTL = productMaxExpire
		}
		c.rdb.Expire(c.ctx, key, newTTL)
	}

	return products, nil
}

// SetProductList 增强版：支持空值缓存 + TTL随机抖动
func (c *productCache) SetProductList(keyword string, categoryID string, products []*entity.Product) error {
	key := c.getProductListKey(keyword, categoryID)

	if products == nil {
		return c.rdb.Set(c.ctx, key, "nil", nilCacheTTL()).Err()
	}

	data, err := json.Marshal(products)
	if err != nil {
		return errors.Wrap(err, "序列化商品列表缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, cacheTTLWithJitter(productBaseExpire)).Err()
}

// getProductLockKey 获取商品缓存的分布式互斥锁 Key
func (c *productCache) getProductLockKey(id uint) string {
	return fmt.Sprintf("product:lock:%d", id)
}

// TryLockProduct 尝试获取分布式互斥锁（Redis SETNX）
//
// 用途：防缓存击穿 — 热点 Key 过期时，多个并发请求只有一个能拿到锁去查 DB，
//
//	其他请求等待锁释放后直接从缓存读取
//
// 返回 true 表示获取锁成功（当前 goroutine 负责查 DB 并回写缓存）
// 返回 false 表示锁被其他 goroutine 持有（应等待后重试读缓存）
func (c *productCache) TryLockProduct(id uint) bool {
	lockKey := c.getProductLockKey(id)
	ok, err := c.rdb.SetNX(c.ctx, lockKey, "1", productLockExpire).Result()
	if err != nil {
		// Redis 故障时降级：不阻塞，直接放行去查 DB
		log.Printf("⚠️ 分布式互斥锁获取失败，降级放行: %v", err)
		return true
	}
	return ok
}

// UnlockProduct 释放分布式互斥锁（查 DB 并回写缓存后调用）
func (c *productCache) UnlockProduct(id uint) {
	lockKey := c.getProductLockKey(id)
	if err := c.rdb.Del(c.ctx, lockKey).Err(); err != nil {
		log.Printf("⚠️ 分布式互斥锁释放失败: %v", err)
	}
}

// WaitAndRetry 等待其他 goroutine 完成 DB 查询后重试读缓存（带超时保护）
//
// 返回 true 表示重试期间命中了缓存
// 返回 false 表示超时，调用方应降级为自行查 DB
func (c *productCache) WaitAndRetry(id uint) (*entity.Product, bool) {
	deadline := time.Now().Add(productLockMaxWait)
	for time.Now().Before(deadline) {
		time.Sleep(productLockRetryInterval)
		product, _ := c.GetProduct(id)
		if product != nil {
			return product, true
		}
	}
	return nil, false
}

// ClearAllProductList 使用 SCAN 命令删除所有商品列表缓存（避免 KEYS 阻塞 Redis）
func (c *productCache) ClearAllProductList() error {
	pattern := "product:list:*"
	var cursor uint64
	var totalDeleted int

	for {
		// 每次 SCAN 一批 key，不会阻塞 Redis
		keys, nextCursor, err := c.rdb.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return errors.Wrap(err, "扫描商品列表缓存键失败")
		}

		if len(keys) > 0 {
			if err := c.rdb.Del(c.ctx, keys...).Err(); err != nil {
				return errors.Wrap(err, "删除商品列表缓存失败")
			}
			totalDeleted += len(keys)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if totalDeleted > 0 {
		log.Printf("🗑️  已清除 %d 个商品列表缓存键", totalDeleted)
	}
	return nil
}

// WarmUpHotProducts 按真实销量预热热门商品缓存
func (c *productCache) WarmUpHotProducts(productRepo mysql.ProductRepository, limit int) error {
	log.Println("🚀 开始预热热门商品缓存（按销量排序）...")

	// 第一步：优先按销量获取热门商品
	products, err := productRepo.ListHotProductsBySales(limit)
	if err != nil {
		log.Printf("⚠️ 按销量查询热门商品失败，降级为按创建时间排序: %v", err)
		// 降级策略：按创建时间获取最新商品
		products, err = productRepo.List("", "")
		if err != nil {
			return errors.Wrap(err, "预热商品缓存查询数据库失败")
		}
		// 截取前limit个
		if len(products) > limit {
			products = products[:limit]
		}
	}

	successCount := 0
	for _, product := range products {
		if err := c.SetProduct(product); err != nil {
			log.Printf("⚠️ 预热商品ID %d 失败: %v", product.ID, err)
		} else {
			successCount++
		}
	}

	log.Printf("✅ 热门商品缓存预热完成，成功: %d/%d", successCount, len(products))
	return nil
}
