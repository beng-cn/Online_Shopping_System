package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type CategoryRepository interface {
	Create(category *entity.Category) error
	Update(category *entity.Category) error
	Delete(id uint) error
	GetByID(id uint) (*entity.Category, error)
	GetParentCategories() ([]*entity.Category, error)
	GetChildCategories(parentID uint) ([]*entity.Category, error)
	CountProductsByCategoryID(categoryID uint) (int64, error)
	CountChildCategories(parentID uint) (int64, error)
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
func (r *categoryRepository) Create(category *entity.Category) error {
	if err := r.db.Create(category).Error; err != nil {
		return errors.Wrap(err, "创建分类失败!")
	}
	return nil
}

// Update 更新分类信息
func (r *categoryRepository) Update(category *entity.Category) error {
	if err := r.db.Model(category).Updates(category).Error; err != nil {
		return errors.Wrap(err, "更新分类失败!")
	}
	return nil
}

// Delete 软删除分类
func (r *categoryRepository) Delete(id uint) error {
	result := r.db.Delete(&entity.Category{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除分类失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeNotFound, "分类不存在!")
	}
	return nil
}

// GetByID 根据主键查询分类
func (r *categoryRepository) GetByID(id uint) (*entity.Category, error) {
	var category entity.Category
	if err := r.db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeNotFound, "分类不存在!")
		}
		return nil, errors.Wrap(err, "查询分类失败!")
	}
	return &category, nil
}

// GetParentCategories 获取所有顶级分类（parent_id=0 且未删除）
func (r *categoryRepository) GetParentCategories() ([]*entity.Category, error) {
	var categories []*entity.Category
	if err := r.db.Where("parent_id = 0 AND status = 1").Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询父分类失败!")
	}
	return categories, nil
}

// GetChildCategories 获取指定父分类下的子分类
func (r *categoryRepository) GetChildCategories(parentID uint) ([]*entity.Category, error) {
	var categories []*entity.Category
	if err := r.db.Where("parent_id = ? AND status = 1", parentID).Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询子分类失败!")
	}
	return categories, nil
}

// CountProductsByCategoryID 统计分类下的商品数量
func (r *categoryRepository) CountProductsByCategoryID(categoryID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&entity.Product{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计商品数量失败!")
	}
	return count, nil
}

// CountChildCategories 统计子分类数量
func (r *categoryRepository) CountChildCategories(parentID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&entity.Category{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计子分类数量失败!")
	}
	return count, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *categoryRepository) WithTx(tx *gorm.DB) CategoryRepository {
	return &categoryRepository{db: tx}
}
