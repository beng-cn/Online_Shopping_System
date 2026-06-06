package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type CartRepository interface {
	Create(cart *entity.Cart) error
	Update(cart *entity.Cart) error
	Delete(id uint) error
	DeleteByIDs(ids []uint) error
	GetByID(id uint) (*entity.Cart, error)
	GetByUserAndProduct(userID uint, productID uint) (*entity.Cart, error)
	GetByUserID(userID uint) ([]*entity.Cart, error)
	GetByIDsAndUserID(ids []uint, userID uint) ([]*entity.Cart, error)
	WithTx(tx *gorm.DB) CartRepository
}

type cartRepository struct {
	db *gorm.DB
}

func NewCartRepository(db *gorm.DB) CartRepository {
	return &cartRepository{db: db}
}

func (r *cartRepository) Create(cart *entity.Cart) error {
	if err := r.db.Create(cart).Error; err != nil {
		return errors.Wrap(err, "添加购物车失败!")
	}
	return nil
}

func (r *cartRepository) Update(cart *entity.Cart) error {
	if err := r.db.Model(cart).Update("quantity", cart.Quantity).Error; err != nil {
		return errors.Wrap(err, "更新购物车失败!")
	}
	return nil
}

func (r *cartRepository) Delete(id uint) error {
	result := r.db.Delete(&entity.Cart{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除购物车失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeNotFound, "购物车记录不存在!")
	}
	return nil
}

func (r *cartRepository) DeleteByIDs(ids []uint) error {
	if err := r.db.Delete(&entity.Cart{}, ids).Error; err != nil {
		return errors.Wrap(err, "删除购物车失败!")
	}
	return nil
}

func (r *cartRepository) GetByID(id uint) (*entity.Cart, error) {
	var cart entity.Cart
	if err := r.db.First(&cart, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeNotFound, "购物车记录不存在!")
		}
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return &cart, nil
}

func (r *cartRepository) GetByUserAndProduct(userID uint, productID uint) (*entity.Cart, error) {
	var cart entity.Cart
	if err := r.db.Where("user_id = ? AND product_id = ?", userID, productID).First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return &cart, nil
}

func (r *cartRepository) GetByUserID(userID uint) ([]*entity.Cart, error) {
	var carts []*entity.Cart
	if err := r.db.Where("user_id = ?", userID).Find(&carts).Error; err != nil {
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return carts, nil
}

func (r *cartRepository) GetByIDsAndUserID(ids []uint, userID uint) ([]*entity.Cart, error) {
	var carts []*entity.Cart
	if err := r.db.Where("id IN ? AND user_id = ?", ids, userID).Find(&carts).Error; err != nil {
		return nil, errors.Wrap(err, "查询购物车失败!")
	}
	return carts, nil
}

func (r *cartRepository) WithTx(tx *gorm.DB) CartRepository {
	return &cartRepository{db: tx}
}
