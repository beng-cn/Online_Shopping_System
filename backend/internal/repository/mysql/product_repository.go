package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type ProductRepository interface {
	Create(product *entity.Product) error
	Update(product *entity.Product) error
	Delete(id uint) error
	GetByID(id uint) (*entity.Product, error)
	List(keyword string, categoryID string) ([]*entity.Product, error)
	// 乐观锁扣减库存，返回是否成功
	DecreaseStockWithVersion(id uint, quantity int, version int) (bool, error)
	WithTx(tx *gorm.DB) ProductRepository
}

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(product *entity.Product) error {
	if err := r.db.Create(product).Error; err != nil {
		return errors.Wrap(err, "创建商品失败!")
	}
	return nil
}

func (r *productRepository) Update(product *entity.Product) error {
	if err := r.db.Model(product).Updates(product).Error; err != nil {
		return errors.Wrap(err, "更新商品失败!")
	}
	return nil
}

func (r *productRepository) Delete(id uint) error {
	result := r.db.Delete(&entity.Product{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除商品失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeProductNotFound, "商品不存在!")
	}
	return nil
}

func (r *productRepository) GetByID(id uint) (*entity.Product, error) {
	var product entity.Product
	if err := r.db.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeProductNotFound, "商品不存在!")
		}
		return nil, errors.Wrap(err, "查询商品失败!")
	}
	return &product, nil
}

func (r *productRepository) List(keyword string, categoryID string) ([]*entity.Product, error) {
	var products []*entity.Product
	db := r.db.Model(&entity.Product{})

	if keyword != "" {
		db = db.Where("name LIKE ?", "%"+keyword+"%")
	}
	if categoryID != "" {
		db = db.Where("category_id = ?", categoryID)
	}

	if err := db.Find(&products).Error; err != nil {
		return nil, errors.Wrap(err, "查询商品列表失败!")
	}
	return products, nil
}

func (r *productRepository) DecreaseStockWithVersion(id uint, quantity int, version int) (bool, error) {
	result := r.db.Model(&entity.Product{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]interface{}{
			"stock":   gorm.Expr("stock - ?", quantity),
			"version": version + 1,
		})
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "扣减库存失败!")
	}
	return result.RowsAffected > 0, nil
}

func (r *productRepository) WithTx(tx *gorm.DB) ProductRepository {
	return &productRepository{db: tx}
}
