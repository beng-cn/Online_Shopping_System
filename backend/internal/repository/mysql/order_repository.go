package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type OrderRepository interface {
	Create(order *entity.Order) error
	UpdateStatus(id uint, status int) error
	Delete(id uint) error
	GetByID(id uint) (*entity.Order, error)
	GetByOrderNo(orderNo string) (*entity.Order, error)
	GetByUserID(userID uint) ([]*entity.Order, error)
	WithTx(tx *gorm.DB) OrderRepository
}

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建订单仓储实例
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// Create 创建订单（通常在事务中调用）
func (r *orderRepository) Create(order *entity.Order) error {
	if err := r.db.Create(order).Error; err != nil {
		return errors.Wrap(err, "创建订单失败!")
	}
	return nil
}

// UpdateStatus 更新订单状态（条件更新防竞态）
func (r *orderRepository) UpdateStatus(id uint, status int) error {
	result := r.db.Model(&entity.Order{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.Wrap(result.Error, "更新订单状态失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeOrderNotFound, "订单不存在!")
	}
	return nil
}

// Delete 软删除订单
func (r *orderRepository) Delete(id uint) error {
	result := r.db.Delete(&entity.Order{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除订单失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeOrderNotFound, "订单不存在!")
	}
	return nil
}

// GetByID 根据主键查询订单
func (r *orderRepository) GetByID(id uint) (*entity.Order, error) {
	var order entity.Order
	if err := r.db.First(&order, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeOrderNotFound, "订单不存在!")
		}
		return nil, errors.Wrap(err, "查询订单失败!")
	}
	return &order, nil
}

// GetByOrderNo 根据订单号查询订单
func (r *orderRepository) GetByOrderNo(orderNo string) (*entity.Order, error) {
	var order entity.Order
	if err := r.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeOrderNotFound, "订单不存在!")
		}
		return nil, errors.Wrap(err, "查询订单失败!")
	}
	return &order, nil
}

// GetByUserID 获取用户全部订单（按创建时间倒序）
func (r *orderRepository) GetByUserID(userID uint) ([]*entity.Order, error) {
	var orders []*entity.Order
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, errors.Wrap(err, "查询订单失败!")
	}
	return orders, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *orderRepository) WithTx(tx *gorm.DB) OrderRepository {
	return &orderRepository{db: tx}
}
