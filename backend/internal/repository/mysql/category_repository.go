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

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(category *entity.Category) error {
	if err := r.db.Create(category).Error; err != nil {
		return errors.Wrap(err, "创建分类失败!")
	}
	return nil
}

func (r *categoryRepository) Update(category *entity.Category) error {
	if err := r.db.Model(category).Updates(category).Error; err != nil {
		return errors.Wrap(err, "更新分类失败!")
	}
	return nil
}

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

func (r *categoryRepository) GetParentCategories() ([]*entity.Category, error) {
	var categories []*entity.Category
	if err := r.db.Where("parent_id = 0 AND status = 1").Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询父分类失败!")
	}
	return categories, nil
}

func (r *categoryRepository) GetChildCategories(parentID uint) ([]*entity.Category, error) {
	var categories []*entity.Category
	if err := r.db.Where("parent_id = ? AND status = 1", parentID).Find(&categories).Error; err != nil {
		return nil, errors.Wrap(err, "查询子分类失败!")
	}
	return categories, nil
}

func (r *categoryRepository) CountProductsByCategoryID(categoryID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&entity.Product{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计商品数量失败!")
	}
	return count, nil
}

func (r *categoryRepository) CountChildCategories(parentID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&entity.Category{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "统计子分类数量失败!")
	}
	return count, nil
}

func (r *categoryRepository) WithTx(tx *gorm.DB) CategoryRepository {
	return &categoryRepository{db: tx}
}
