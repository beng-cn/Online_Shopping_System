package redis

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
)

type ProductCache interface {
	GetProduct(id uint) (*entity.Product, error)
	SetProduct(product *entity.Product) error
	DeleteProduct(id uint) error
	GetProductList(keyword string, categoryID string) ([]*entity.Product, error)
	SetProductList(keyword string, categoryID string, products []*entity.Product) error
	ClearAllProductList() error
	WarmUpHotProducts(productRepo mysql.ProductRepository, limit int) error // 新增预热方法
}

type productCache struct {
	rdb *redis.Client
	ctx context.Context
}

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

// SetProduct 增强版：支持空值缓存
func (c *productCache) SetProduct(product *entity.Product) error {
	key := c.getProductKey(product.ID)

	// 空值缓存（当数据库查询不到时，缓存"nil"标记）
	if product == nil {
		return c.rdb.Set(c.ctx, key, "nil", productNilExpire).Err()
	}

	data, err := json.Marshal(product)
	if err != nil {
		return errors.Wrap(err, "序列化商品缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, productBaseExpire).Err()
}

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

// SetProductList 增强版：支持空值缓存
func (c *productCache) SetProductList(keyword string, categoryID string, products []*entity.Product) error {
	key := c.getProductListKey(keyword, categoryID)

	if products == nil {
		return c.rdb.Set(c.ctx, key, "nil", productNilExpire).Err()
	}

	data, err := json.Marshal(products)
	if err != nil {
		return errors.Wrap(err, "序列化商品列表缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, productBaseExpire).Err()
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
