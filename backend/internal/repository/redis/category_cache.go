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

// 分类缓存策略常量
const (
	categoryBaseExpire    = 1 * time.Hour
	categoryMaxExpire     = 6 * time.Hour
	categoryNilExpire     = 5 * time.Minute
	categoryRenewDuration = 30 * time.Minute
)

type CategoryCache interface {
	GetParentCategories() ([]*entity.Category, error)
	SetParentCategories(categories []*entity.Category) error
	GetChildCategories(parentID uint) ([]*entity.Category, error)
	SetChildCategories(parentID uint, categories []*entity.Category) error
	ClearAllCategoryCache() error
	WarmUpAllCategories(categoryRepo mysql.CategoryRepository) error // 新增预热方法
}

type categoryCache struct {
	rdb *redis.Client
	ctx context.Context
}

func NewCategoryCache(rdb *redis.Client) CategoryCache {
	return &categoryCache{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (c *categoryCache) getParentKey() string {
	return "category:parent"
}

func (c *categoryCache) getChildKey(parentID uint) string {
	return fmt.Sprintf("category:child:%d", parentID)
}

// GetParentCategories 增强版：支持自动续期、缓存降级
func (c *categoryCache) GetParentCategories() ([]*entity.Category, error) {
	key := c.getParentKey()
	data, err := c.rdb.Get(c.ctx, key).Result()

	if err != nil {
		if err != redis.Nil {
			log.Printf("⚠️ Redis父分类缓存读取失败: %v", err)
		}
		return nil, nil
	}

	if data == "nil" {
		return nil, nil
	}

	var categories []*entity.Category
	if err := json.Unmarshal([]byte(data), &categories); err != nil {
		log.Printf("⚠️ 父分类缓存数据解析失败，已删除坏缓存: %v", err)
		c.rdb.Del(c.ctx, key)
		return nil, nil
	}

	// 自动续期
	ttl, err := c.rdb.TTL(c.ctx, key).Result()
	if err == nil && ttl < categoryMaxExpire {
		newTTL := ttl + categoryRenewDuration
		if newTTL > categoryMaxExpire {
			newTTL = categoryMaxExpire
		}
		c.rdb.Expire(c.ctx, key, newTTL)
	}

	return categories, nil
}

// SetParentCategories 增强版：支持空值缓存
func (c *categoryCache) SetParentCategories(categories []*entity.Category) error {
	key := c.getParentKey()

	if categories == nil {
		return c.rdb.Set(c.ctx, key, "nil", categoryNilExpire).Err()
	}

	data, err := json.Marshal(categories)
	if err != nil {
		return errors.Wrap(err, "序列化父分类缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, categoryBaseExpire).Err()
}

// GetChildCategories 增强版：支持自动续期、缓存降级
func (c *categoryCache) GetChildCategories(parentID uint) ([]*entity.Category, error) {
	key := c.getChildKey(parentID)
	data, err := c.rdb.Get(c.ctx, key).Result()

	if err != nil {
		if err != redis.Nil {
			log.Printf("⚠️ Redis子分类缓存读取失败: %v", err)
		}
		return nil, nil
	}

	if data == "nil" {
		return nil, nil
	}

	var categories []*entity.Category
	if err := json.Unmarshal([]byte(data), &categories); err != nil {
		log.Printf("⚠️ 子分类缓存数据解析失败，已删除坏缓存: %v", err)
		c.rdb.Del(c.ctx, key)
		return nil, nil
	}

	// 自动续期
	ttl, err := c.rdb.TTL(c.ctx, key).Result()
	if err == nil && ttl < categoryMaxExpire {
		newTTL := ttl + categoryRenewDuration
		if newTTL > categoryMaxExpire {
			newTTL = categoryMaxExpire
		}
		c.rdb.Expire(c.ctx, key, newTTL)
	}

	return categories, nil
}

// SetChildCategories 增强版：支持空值缓存
func (c *categoryCache) SetChildCategories(parentID uint, categories []*entity.Category) error {
	key := c.getChildKey(parentID)

	if categories == nil {
		return c.rdb.Set(c.ctx, key, "nil", categoryNilExpire).Err()
	}

	data, err := json.Marshal(categories)
	if err != nil {
		return errors.Wrap(err, "序列化子分类缓存数据失败")
	}
	return c.rdb.Set(c.ctx, key, data, categoryBaseExpire).Err()
}

// ClearAllCategoryCache 使用 SCAN 命令清除所有分类缓存（避免 KEYS 阻塞 Redis）
func (c *categoryCache) ClearAllCategoryCache() error {
	// 1. 先删除父分类缓存
	if err := c.rdb.Del(c.ctx, c.getParentKey()).Err(); err != nil {
		return errors.Wrap(err, "清除父分类缓存失败")
	}

	// 2. 使用 SCAN 删除所有子分类缓存
	pattern := "category:child:*"
	var cursor uint64
	for {
		keys, nextCursor, err := c.rdb.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return errors.Wrap(err, "扫描子分类缓存键失败")
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(c.ctx, keys...).Err(); err != nil {
				return errors.Wrap(err, "清除子分类缓存失败")
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// WarmUpAllCategories 全部分类缓存预热
func (c *categoryCache) WarmUpAllCategories(categoryRepo mysql.CategoryRepository) error {
	log.Println("🚀 开始预热分类缓存...")

	// 预热父分类
	parentCategories, err := categoryRepo.GetParentCategories()
	if err != nil {
		return errors.Wrap(err, "预热父分类缓存查询数据库失败")
	}
	if err := c.SetParentCategories(parentCategories); err != nil {
		log.Printf("⚠️ 预热父分类缓存失败: %v", err)
	}

	// 预热所有子分类
	successCount := 0
	for _, parent := range parentCategories {
		childCategories, err := categoryRepo.GetChildCategories(parent.ID)
		if err != nil {
			log.Printf("⚠️ 查询父分类ID %d 的子分类失败: %v", parent.ID, err)
			continue
		}
		if err := c.SetChildCategories(parent.ID, childCategories); err != nil {
			log.Printf("⚠️ 预热父分类ID %d 的子分类失败: %v", parent.ID, err)
		} else {
			successCount++
		}
	}

	log.Printf("✅ 分类缓存预热完成，父分类: %d个，子分类: %d个", len(parentCategories), successCount)
	return nil
}
