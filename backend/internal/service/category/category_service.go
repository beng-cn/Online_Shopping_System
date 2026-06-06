package category

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
)

type CategoryService interface {
	CreateCategory(req *request.CreateCategoryRequest) (*response.CategoryResponse, error)
	UpdateCategory(id uint, req *request.UpdateCategoryRequest) error
	DeleteCategory(id uint) error
	GetParentCategories() ([]*response.CategoryResponse, error)
	GetChildCategories(parentID uint) ([]*response.CategoryResponse, error)
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

func (s *categoryService) GetParentCategories() ([]*response.CategoryResponse, error) {
	// 先查缓存
	categories, err := s.categoryCache.GetParentCategories()
	if err == nil {
		return convertToCategoryResponseList(categories), nil
	}

	// 缓存没命中，查数据库
	categories, err = s.categoryRepo.GetParentCategories()
	if err != nil {
		return nil, err
	}

	// 写入缓存
	s.categoryCache.SetParentCategories(categories)

	return convertToCategoryResponseList(categories), nil
}

func (s *categoryService) GetChildCategories(parentID uint) ([]*response.CategoryResponse, error) {
	// 先查缓存
	categories, err := s.categoryCache.GetChildCategories(parentID)
	if err == nil {
		return convertToCategoryResponseList(categories), nil
	}

	// 缓存没命中，查数据库
	categories, err = s.categoryRepo.GetChildCategories(parentID)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	s.categoryCache.SetChildCategories(parentID, categories)

	return convertToCategoryResponseList(categories), nil
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
