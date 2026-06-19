package mysql

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"strings"

	"gorm.io/gorm"
)

// escapeFulltextKeyword 转义 MySQL 布尔全文搜索的特殊字符
// 避免用户输入 + - > < ( ) ~ * " @ 等字符导致搜索语法错误
func escapeFulltextKeyword(kw string) string {
	replacer := strings.NewReplacer(
		"+", "\\+",
		"-", "\\-",
		">", "\\>",
		"<", "\\<",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"*", "\\*",
		"\"", "\\\"",
		"@", "\\@",
	)
	return replacer.Replace(kw)
}

type SalesStat struct {
	ProductID uint
	Total     int
}

type ProductRepository interface {
	Create(product *entity.Product) error
	Update(product *entity.Product) error
	Delete(id uint) error
	GetByID(id uint) (*entity.Product, error)
	GetByIDs(ids []uint) ([]*entity.Product, error) // 批量查询商品，避免 N+1 问题
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
	GetAllIDs() ([]uint, error) // 获取所有产品ID（用于布隆过滤器初始化）
}

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository 创建商品仓储实例
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

// Create 新增商品
func (r *productRepository) Create(product *entity.Product) error {
	if err := r.db.Create(product).Error; err != nil {
		return errors.Wrap(err, "创建商品失败!")
	}
	return nil
}

// Update 更新商品信息
func (r *productRepository) Update(product *entity.Product) error {
	if err := r.db.Model(product).Updates(product).Error; err != nil {
		return errors.Wrap(err, "更新商品失败!")
	}
	return nil
}

// Delete 软删除商品
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

// GetByID 根据主键查询商品
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

// GetByIDs 批量查询商品，使用 IN 查询一次获取所有商品，避免 N+1 问题
func (r *productRepository) GetByIDs(ids []uint) ([]*entity.Product, error) {
	if len(ids) == 0 {
		return []*entity.Product{}, nil
	}
	var products []*entity.Product
	if err := r.db.Where("id IN ?", ids).Find(&products).Error; err != nil {
		return nil, errors.Wrap(err, "批量查询商品失败!")
	}
	return products, nil
}

// GetAllIDs 获取所有产品主键 ID（用于布隆过滤器全量初始化）
//
// 使用 SELECT id 而非 SELECT *，减少数据传输量（每行仅 4-8 字节）
// 百万级产品仅需 ~8MB 内存
func (r *productRepository) GetAllIDs() ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&entity.Product{}).Pluck("id", &ids).Error; err != nil {
		return nil, errors.Wrap(err, "获取所有产品ID失败")
	}
	return ids, nil
}

// List 关键词搜索商品列表
func (r *productRepository) List(keyword string, categoryID string) ([]*entity.Product, error) {
	var products []*entity.Product
	db := r.db.Model(&entity.Product{})

	if keyword != "" {
		db = db.Where("MATCH(name, keywords) AGAINST(? IN BOOLEAN MODE)", escapeFulltextKeyword(keyword))
	}
	if categoryID != "" {
		db = db.Where("category_id = ?", categoryID)
	}

	if err := db.Find(&products).Error; err != nil {
		return nil, errors.Wrap(err, "查询商品列表失败!")
	}
	return products, nil
}

// DecreaseStockWithVersion 乐观锁扣减库存，version 不匹配返回 false
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

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *productRepository) WithTx(tx *gorm.DB) ProductRepository {
	return &productRepository{db: tx}
}

// ListHotProductsBySales 按销量降序获取热门商品
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

	// 关键词筛选：同时搜索商品名称和 keywords 别名字段
	if req.Keyword != "" {
		db = db.Where("MATCH(name, keywords) AGAINST(? IN BOOLEAN MODE)", escapeFulltextKeyword(req.Keyword))
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
