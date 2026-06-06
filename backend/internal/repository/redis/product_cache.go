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

type ProductCache interface {
	GetProduct(id uint) (*entity.Product, error)
	SetProduct(product *entity.Product) error
	DeleteProduct(id uint) error
	GetProductList(keyword string, categoryID string) ([]*entity.Product, error)
	SetProductList(keyword string, categoryID string, products []*entity.Product) error
	ClearAllProductList() error
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

func (c *productCache) GetProduct(id uint) (*entity.Product, error) {
	key := c.getProductKey(id)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var product entity.Product
	if err := json.Unmarshal([]byte(data), &product); err != nil {
		return nil, errors.Wrap(err, "解析缓存数据失败!")
	}
	return &product, nil
}

func (c *productCache) SetProduct(product *entity.Product) error {
	key := c.getProductKey(product.ID)
	data, err := json.Marshal(product)
	if err != nil {
		return errors.Wrap(err, "序列化缓存数据失败!")
	}
	return c.rdb.Set(c.ctx, key, data, 10*time.Minute).Err()
}

func (c *productCache) DeleteProduct(id uint) error {
	key := c.getProductKey(id)
	return c.rdb.Del(c.ctx, key).Err()
}

func (c *productCache) GetProductList(keyword string, categoryID string) ([]*entity.Product, error) {
	key := c.getProductListKey(keyword, categoryID)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var products []*entity.Product
	if err := json.Unmarshal([]byte(data), &products); err != nil {
		return nil, errors.Wrap(err, "解析缓存数据失败!")
	}
	return products, nil
}

func (c *productCache) SetProductList(keyword string, categoryID string, products []*entity.Product) error {
	key := c.getProductListKey(keyword, categoryID)
	data, err := json.Marshal(products)
	if err != nil {
		return errors.Wrap(err, "序列化缓存数据失败!")
	}
	return c.rdb.Set(c.ctx, key, data, 5*time.Minute).Err()
}

func (c *productCache) ClearAllProductList() error {
	keys, err := c.rdb.Keys(c.ctx, "product:list:*").Result()
	if err != nil {
		return errors.Wrap(err, "获取缓存键失败!")
	}
	if len(keys) > 0 {
		return c.rdb.Del(c.ctx, keys...).Err()
	}
	return nil
}
