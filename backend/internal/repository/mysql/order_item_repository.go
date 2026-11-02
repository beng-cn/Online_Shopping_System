package mysql

import (
	"context"

	"backend/internal/model/entity"
	"backend/internal/pkg/ctxutil"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type OrderItemRepository interface {
	BatchCreate(ctx context.Context, items []*entity.OrderItem) error
	DeleteByOrderID(ctx context.Context, orderID uint) error
	GetByOrderID(ctx context.Context, orderID uint) ([]*entity.OrderItem, error)
	GetByOrderIDs(ctx context.Context, orderIDs []uint) ([]*entity.OrderItem, error) // 批量查询，消除 N+1
	WithTx(tx *gorm.DB) OrderItemRepository
}

type orderItemRepository struct {
	db *gorm.DB
}

// NewOrderItemRepository 创建订单项仓储实例
func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: db}
}

// BatchCreate 批量创建订单项（事务中用）
func (r *orderItemRepository) BatchCreate(ctx context.Context, items []*entity.OrderItem) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Create(items).Error; err != nil {
		return errors.Wrap(err, "创建订单项失败!")
	}
	return nil
}

// DeleteByOrderID 删除指定订单的所有订单项
func (r *orderItemRepository) DeleteByOrderID(ctx context.Context, orderID uint) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Where("order_id = ?", orderID).Delete(&entity.OrderItem{}).Error; err != nil {
		return errors.Wrap(err, "删除订单项失败!")
	}
	return nil
}

// GetByOrderID 根据订单ID查询订单项
func (r *orderItemRepository) GetByOrderID(ctx context.Context, orderID uint) ([]*entity.OrderItem, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var items []*entity.OrderItem
	if err := db.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "查询订单项失败!")
	}
	return items, nil
}

// GetByOrderIDs 批量查询多个订单的订单项（消除 N+1 问题）
func (r *orderItemRepository) GetByOrderIDs(ctx context.Context, orderIDs []uint) ([]*entity.OrderItem, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if len(orderIDs) == 0 {
		return []*entity.OrderItem{}, nil
	}
	var items []*entity.OrderItem
	if err := db.Where("order_id IN ?", orderIDs).Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "批量查询订单项失败!")
	}
	return items, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *orderItemRepository) WithTx(tx *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: tx}
}
