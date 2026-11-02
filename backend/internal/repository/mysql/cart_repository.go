package mysql

import (
	"context"

	"backend/internal/model/entity"
	"backend/internal/pkg/ctxutil"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type CartRepository interface {
	Create(ctx context.Context, cart *entity.Cart) error
	Update(ctx context.Context, cart *entity.Cart) error
	Delete(ctx context.Context, id uint) error
	DeleteByIDs(ctx context.Context, ids []uint) error
	GetByID(ctx context.Context, id uint) (*entity.Cart, error)
	GetByUserAndProduct(ctx context.Context, userID uint, productID uint) (*entity.Cart, error)
	GetByUserID(ctx context.Context, userID uint) ([]*entity.Cart, error)
	GetByIDsAndUserID(ctx context.Context, ids []uint, userID uint) ([]*entity.Cart, error)
	WithTx(tx *gorm.DB) CartRepository
}

type cartRepository struct {
	db *gorm.DB
}

// NewCartRepository 创建购物车仓储实例
func NewCartRepository(db *gorm.DB) CartRepository {
	return &cartRepository{db: db}
}

// Create 新增购物车条目
func (r *cartRepository) Create(ctx context.Context, cart *entity.Cart) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Create(cart).Error; err != nil {
		return errors.Wrap(err, "添加购物车失败!")
	}
	return nil
}

// Update 更新购物车条目（通常修改数量）
func (r *cartRepository) Update(ctx context.Context, cart *entity.Cart) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Model(cart).Update("quantity", cart.Quantity).Error; err != nil {
		return errors.Wrap(err, "更新购物车失败!")
	}
	return nil
}

// Delete 软删除购物车条目
func (r *cartRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	result := db.Delete(&entity.Cart{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除购物车失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeNotFound, "购物车记录不存在!")
	}
	return nil
}

// DeleteByIDs 批量删除用户的指定购物车条目
func (r *cartRepository) DeleteByIDs(ctx context.Context, ids []uint) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Delete(&entity.Cart{}, ids).Error; err != nil {
		return errors.Wrap(err, "删除购物车失败!")
	}
	return nil
}

// GetByID 根据主键查询购物车条目
func (r *cartRepository) GetByID(ctx context.Context, id uint) (*entity.Cart, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var cart entity.Cart
	if err := db.First(&cart, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeNotFound, "购物车记录不存在!")
		}
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return &cart, nil
}

// GetByUserAndProduct 查询用户购物车中指定商品条目（用于自动合并）
func (r *cartRepository) GetByUserAndProduct(ctx context.Context, userID uint, productID uint) (*entity.Cart, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var cart entity.Cart
	if err := db.Where("user_id = ? AND product_id = ?", userID, productID).First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return &cart, nil
}

// GetByUserID 获取用户全部购物车条目
func (r *cartRepository) GetByUserID(ctx context.Context, userID uint) ([]*entity.Cart, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var carts []*entity.Cart
	if err := db.Where("user_id = ?", userID).Find(&carts).Error; err != nil {
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return carts, nil
}

// GetByIDsAndUserID 批量获取用户指定条目（用于前端同步）
func (r *cartRepository) GetByIDsAndUserID(ctx context.Context, ids []uint, userID uint) ([]*entity.Cart, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var carts []*entity.Cart
	if err := db.Where("id IN ? AND user_id = ?", ids, userID).Find(&carts).Error; err != nil {
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return carts, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *cartRepository) WithTx(tx *gorm.DB) CartRepository {
	return &cartRepository{db: tx}
}
