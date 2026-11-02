package mysql

import (
	"context"

	"backend/internal/model/entity"
	"backend/internal/pkg/ctxutil"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *entity.Category) error
	Update(ctx context.Context, category *entity.Category) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*entity.Category, error)
	GetParentCategories(ctx context.Context) ([]*entity.Category, error)
	GetChildCategories(ctx context.Context, parentID uint) ([]*entity.Category, error)
	CountProductsByCategoryID(ctx context.Context, categoryID uint) (int64, error)
	CountChildCategories(ctx context.Context, parentID uint) (int64, error)
	WithTx(tx *gorm.DB) CategoryRepository
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 创建分类仓储实例
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

// Create 新增分类
func (r *categoryRepository) Create(ctx context.Context, category *entity.Category) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Create(category).Error; err != nil {
		return errors.Wrap(err, "创建分类失败!")
	}
	return nil
}

// Update 更新分类信息
//
// 同样用 Select 白名单：GORM 的 struct Updates 会跳过零值，
// 否则"把分类禁用(status=0)"、"把子分类改成顶级分类(parent_id=0)"都不会落库。
func (r *categoryRepository) Update(ctx context.Context, category *entity.Category) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Model(category).
		Select("name", "parent_id", "status").
		Updates(category).Error; err != nil {
		return errors.Wrap(err, "更新分类失败!")
	}
	return nil
}

// Delete 软删除分类
func (r *categoryRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	result := db.Delete(&entity.Category{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除分类失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeNotFound, "分类不存在!")
	}
	return nil
}

// GetByID 根据主键查询分类
func (r *categoryRepository) GetByID(ctx context.Context, id uint) (*entity.Category, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var category entity.Category
	if err := db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeNotFound, "分类不存在!")
		}
		return nil, errors.Wrap(err, "查询分类失败!")
	}
	return &category, nil
}

// GetParentCategories 获取所有顶级分类（parent_id=0 且未删除）
func (r *categoryRepository) GetParentCategories(ctx context.Context) ([]*entity.Category, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var categories []*entity.Category
	if err := db.Where("parent_id = 0 AND status = 1").Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询父分类失败!")
	}
	return categories, nil
}

// GetChildCategories 获取指定父分类下的子分类
func (r *categoryRepository) GetChildCategories(ctx context.Context, parentID uint) ([]*entity.Category, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var categories []*entity.Category
	if err := db.Where("parent_id = ? AND status = 1", parentID).Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询子分类失败!")
	}
	return categories, nil
}

// CountProductsByCategoryID 统计分类下的商品数量
func (r *categoryRepository) CountProductsByCategoryID(ctx context.Context, categoryID uint) (int64, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var count int64
	if err := db.Model(&entity.Product{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计商品数量失败!")
	}
	return count, nil
}

// CountChildCategories 统计子分类数量
func (r *categoryRepository) CountChildCategories(ctx context.Context, parentID uint) (int64, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var count int64
	if err := db.Model(&entity.Category{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计子分类数量失败!")
	}
	return count, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *categoryRepository) WithTx(tx *gorm.DB) CategoryRepository {
	return &categoryRepository{db: tx}
}
