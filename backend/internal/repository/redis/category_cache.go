package redis

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type CategoryCache interface {
	GetParentCategories() ([]*entity.Category, error)
	SetParentCategories(categories []*entity.Category) error
	GetChildCategories(parentID uint) ([]*entity.Category, error)
	SetChildCategories(parentID uint, categories []*entity.Category) error
	ClearAllCategoryCache() error
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

func (c *categoryCache) GetParentCategories() ([]*entity.Category, error) {
	key := c.getParentKey()
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var categories []*entity.Category
	if err := json.Unmarshal([]byte(data), &categories); err != nil {
		return nil, errors.Wrap(err, "解析缓存数据失败!")
	}
	return categories, nil
}

func (c *categoryCache) SetParentCategories(categories []*entity.Category) error {
	key := c.getParentKey()
	data, err := json.Marshal(categories)
	if err != nil {
		return errors.Wrap(err, "序列化缓存数据失败!")
	}
	return c.rdb.Set(c.ctx, key, data, 1*time.Hour).Err()
}

func (c *categoryCache) GetChildCategories(parentID uint) ([]*entity.Category, error) {
	key := c.getChildKey(parentID)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var categories []*entity.Category
	if err := json.Unmarshal([]byte(data), &categories); err != nil {
		return nil, errors.Wrap(err, "解析缓存数据失败!")
	}
	return categories, nil
}

func (c *categoryCache) SetChildCategories(parentID uint, categories []*entity.Category) error {
	key := c.getChildKey(parentID)
	data, err := json.Marshal(categories)
	if err != nil {
		return errors.Wrap(err, "序列化缓存数据失败!")
	}
	return c.rdb.Set(c.ctx, key, data, 1*time.Hour).Err()
}

func (c *categoryCache) ClearAllCategoryCache() error {
	// 清除父分类缓存
	if err := c.rdb.Del(c.ctx, c.getParentKey()).Err(); err != nil {
		return errors.Wrap(err, "清除父分类缓存失败!")
	}

	// 清除所有子分类缓存
	keys, err := c.rdb.Keys(c.ctx, "category:child:*").Result()
	if err != nil {
		return errors.Wrap(err, "获取子分类缓存键失败!")
	}
	if len(keys) > 0 {
		if err := c.rdb.Del(c.ctx, keys...).Err(); err != nil {
			return errors.Wrap(err, "清除子分类缓存失败!")
		}
	}

	return nil
}
