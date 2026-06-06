package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type OrderItemRepository interface {
	BatchCreate(items []*entity.OrderItem) error
	DeleteByOrderID(orderID uint) error
	GetByOrderID(orderID uint) ([]*entity.OrderItem, error)
	WithTx(tx *gorm.DB) OrderItemRepository
}

type orderItemRepository struct {
	db *gorm.DB
}

func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: db}
}

func (r *orderItemRepository) BatchCreate(items []*entity.OrderItem) error {
	if err := r.db.Create(items).Error; err != nil {
		return errors.Wrap(err, "创建订单项失败!")
	}
	return nil
}

func (r *orderItemRepository) DeleteByOrderID(orderID uint) error {
	if err := r.db.Where("order_id = ?", orderID).Delete(&entity.OrderItem{}).Error; err != nil {
		return errors.Wrap(err, "删除订单项失败!")
	}
	return nil
}

func (r *orderItemRepository) GetByOrderID(orderID uint) ([]*entity.OrderItem, error) {
	var items []*entity.OrderItem
	if err := r.db.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "查询订单项失败!")
	}
	return items, nil
}

func (r *orderItemRepository) WithTx(tx *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: tx}
}
