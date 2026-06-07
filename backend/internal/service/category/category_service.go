package category

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"log"
)

type CategoryService interface {
	CreateCategory(req *request.CreateCategoryRequest) (*response.CategoryResponse, error)
	UpdateCategory(id uint, req *request.UpdateCategoryRequest) error
	DeleteCategory(id uint) error
	GetParentCategories() ([]*response.CategoryResponse, error)
	GetChildCategories(parentID uint) ([]*response.CategoryResponse, error)
	WarmUpAllCategories() error
}

type categoryService struct {
	categoryRepo  mysql.CategoryRepository
	categoryCache redis.CategoryCache
}

func NewCategoryService(
	categoryRepo mysql.CategoryRepository,
	categoryCache redis.CategoryCache,
) CategoryService {
	return &categoryService{
		categoryRepo:  categoryRepo,
		categoryCache: categoryCache,
	}
}

func (s *categoryService) CreateCategory(req *request.CreateCategoryRequest) (*response.CategoryResponse, error) {
	category := &entity.Category{
		Name:     req.Name,
		ParentID: req.ParentID,
		Status:   req.Status,
	}

	if err := s.categoryRepo.Create(category); err != nil {
		return nil, err
	}

	// 清理分类缓存
	if err := s.categoryCache.ClearAllCategoryCache(); err != nil {
	}

	return &response.CategoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		ParentID:  category.ParentID,
		Status:    category.Status,
		CreatedAt: category.CreatedAt,
	}, nil
}

func (s *categoryService) UpdateCategory(id uint, req *request.UpdateCategoryRequest) error {
	category, err := s.categoryRepo.GetByID(id)
	if err != nil {
		return err
	}

	category.Name = req.Name
	category.ParentID = req.ParentID
	category.Status = req.Status

	if err := s.categoryRepo.Update(category); err != nil {
		return err
	}

	// 清理分类缓存
	s.categoryCache.ClearAllCategoryCache()

	return nil
}

func (s *categoryService) DeleteCategory(id uint) error {
	// 检查分类下是否有商品
	productCount, err := s.categoryRepo.CountProductsByCategoryID(id)
	if err != nil {
		return err
	}
	if productCount > 0 {
		return errors.New(errors.CodeCategoryHasProduct, "该分类下存在商品，无法删除")
	}

	// 检查分类下是否有子分类
	childCount, err := s.categoryRepo.CountChildCategories(id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errors.New(errors.CodeCategoryHasChild, "该分类下存在子分类，无法删除")
	}

	if err := s.categoryRepo.Delete(id); err != nil {
		return err
	}

	// 清理分类缓存
	s.categoryCache.ClearAllCategoryCache()

	return nil
}

// GetParentCategories 适配缓存降级
func (s *categoryService) GetParentCategories() ([]*response.CategoryResponse, error) {
	categories, _ := s.categoryCache.GetParentCategories()
	if categories != nil {
		return convertToCategoryResponseList(categories), nil
	}

	categories, err := s.categoryRepo.GetParentCategories()
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 异步写入父分类缓存发生panic: %v", r)
			}
		}()
		if err := s.categoryCache.SetParentCategories(categories); err != nil {
			log.Printf("⚠️ 写入父分类缓存失败: %v", err)
		}
	}()

	return convertToCategoryResponseList(categories), nil
}

// GetChildCategories 适配缓存降级
func (s *categoryService) GetChildCategories(parentID uint) ([]*response.CategoryResponse, error) {
	categories, _ := s.categoryCache.GetChildCategories(parentID)
	if categories != nil {
		return convertToCategoryResponseList(categories), nil
	}

	categories, err := s.categoryRepo.GetChildCategories(parentID)
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 异步写入子分类缓存发生panic: %v", r)
			}
		}()
		if err := s.categoryCache.SetChildCategories(parentID, categories); err != nil {
			log.Printf("⚠️ 写入子分类缓存失败: %v", err)
		}
	}()

	return convertToCategoryResponseList(categories), nil
}

// WarmUpAllCategories 暴露预热方法给上层
func (s *categoryService) WarmUpAllCategories() error {
	return s.categoryCache.WarmUpAllCategories(s.categoryRepo)
}

// 转换为响应DTO
func convertToCategoryResponse(category *entity.Category) *response.CategoryResponse {
	return &response.CategoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		ParentID:  category.ParentID,
		Status:    category.Status,
		CreatedAt: category.CreatedAt,
	}
}

func convertToCategoryResponseList(categories []*entity.Category) []*response.CategoryResponse {
	var resp []*response.CategoryResponse
	for _, category := range categories {
		resp = append(resp, convertToCategoryResponse(category))
	}
	return resp
}
