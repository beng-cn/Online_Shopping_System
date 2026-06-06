package mysql

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type SalesStat struct {
	ProductID uint
	Total     int
}

type ProductRepository interface {
	Create(product *entity.Product) error
	Update(product *entity.Product) error
	Delete(id uint) error
	GetByID(id uint) (*entity.Product, error)
	List(keyword string, categoryID string) ([]*entity.Product, error)
	// 乐观锁扣减库存，返回是否成功
	DecreaseStockWithVersion(id uint, quantity int, version int) (bool, error)
	WithTx(tx *gorm.DB) ProductRepository
	ListHotProductsBySales(limit int) ([]*entity.Product, error)
	IncreaseSales(id uint, quantity int) error
	UpdateSales(id uint, sales int) error
	GetDB() *gorm.DB
	BatchUpdateSales(stats []SalesStat) error
	ListPage(req *request.ProductListRequest) ([]*entity.Product, int64, error)
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

func (r *productRepository) ListHotProductsBySales(limit int) ([]*entity.Product, error) {
	var products []*entity.Product

	err := r.db.Model(&entity.Product{}).
		Where("status = ?", 1).       // 只查询上架商品
		Order("sales DESC, id DESC"). // 销量降序，销量相同按ID降序
		Limit(limit).
		Find(&products).Error

	if err != nil {
		return nil, errors.Wrap(err, "查询热门商品失败")
	}

	return products, nil
}

// IncreaseSales 原子增加商品销量
func (r *productRepository) IncreaseSales(id uint, quantity int) error {
	result := r.db.Model(&entity.Product{}).
		Where("id = ?", id).
		Update("sales", gorm.Expr("sales + ?", quantity))

	if result.Error != nil {
		return errors.Wrap(result.Error, "更新商品销量失败")
	}

	if result.RowsAffected == 0 {
		return errors.New(errors.CodeProductNotFound, "商品不存在")
	}

	return nil
}

// UpdateSales 全量更新商品销量
func (r *productRepository) UpdateSales(id uint, sales int) error {
	result := r.db.Model(&entity.Product{}).
		Where("id = ?", id).
		Update("sales", sales)

	if result.Error != nil {
		return errors.Wrap(result.Error, "更新商品销量失败")
	}

	if result.RowsAffected == 0 {
		return errors.New(errors.CodeProductNotFound, "商品不存在")
	}

	return nil
}

// GetDB 获取底层数据库连接
func (r *productRepository) GetDB() *gorm.DB {
	return r.db
}

// BatchUpdateSales 批量更新商品销量
func (r *productRepository) BatchUpdateSales(stats []SalesStat) error {
	if len(stats) == 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, stat := range stats {
			err := tx.Model(&entity.Product{}).
				Where("id = ?", stat.ProductID).
				Update("sales", stat.Total).Error

			if err != nil {
				return errors.Wrapf(err, "更新商品ID %d 销量失败", stat.ProductID)
			}
		}
		return nil
	})
}

// ListPage 分页查询商品列表
func (r *productRepository) ListPage(req *request.ProductListRequest) ([]*entity.Product, int64, error) {
	var products []*entity.Product
	var total int64

	db := r.db.Model(&entity.Product{}).Where("status = ?", 1) // 默认只查询上架商品
	db = db.Debug()                                            // 开启SQL日志，打印完整SQL

	// 关键词筛选
	if req.Keyword != "" {
		db = db.Where("name LIKE ?", "%"+req.Keyword+"%")
	}

	// 分类筛选
	if req.CategoryID != "" {
		db = db.Where("category_id = ?", req.CategoryID)
	}

	// 价格区间筛选
	if req.MinPrice > 0 {
		db = db.Where("price >= ?", req.MinPrice)
	}
	if req.MaxPrice > 0 && req.MaxPrice >= req.MinPrice {
		db = db.Where("price <= ?", req.MaxPrice)
	}

	// 统计总条数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "统计商品总数失败")
	}

	// 排序处理
	switch req.Sort {
	case "sales":
		db = db.Order("sales DESC")
	case "price_asc":
		db = db.Order("price ASC")
	case "price_desc":
		db = db.Order("price DESC")
	default: // created_at
		db = db.Order("created_at DESC")
	}

	// 分页查询
	offset := (req.PageNum - 1) * req.PageSize
	if err := db.Offset(offset).Limit(req.PageSize).Find(&products).Error; err != nil {
		return nil, 0, errors.Wrap(err, "分页查询商品列表失败")
	}

	return products, total, nil
}
