package product

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"log"
)

type ProductService interface {
	CreateProduct(req *request.CreateProductRequest) (*response.ProductResponse, error)
	UpdateProduct(id uint, req *request.UpdateProductRequest) error
	DeleteProduct(id uint) error
	GetProductByID(id uint) (*response.ProductResponse, error)
	GetProductList(keyword string, categoryID string) ([]*response.ProductResponse, error)
}

type productService struct {
	productRepo  mysql.ProductRepository
	productCache redis.ProductCache
}

func NewProductService(
	productRepo mysql.ProductRepository,
	productCache redis.ProductCache,
) ProductService {
	return &productService{
		productRepo:  productRepo,
		productCache: productCache,
	}
}

func (s *productService) CreateProduct(req *request.CreateProductRequest) (*response.ProductResponse, error) {
	product := &entity.Product{
		CategoryID: req.CategoryID,
		Name:       req.Name,
		Price:      req.Price,
		Stock:      req.Stock,
		Image:      req.Image,
		Status:     req.Status,
	}

	if err := s.productRepo.Create(product); err != nil {
		return nil, err
	}

	// 清理商品列表缓存
	if err := s.productCache.ClearAllProductList(); err != nil {
		// 缓存清理失败不影响主流程，只记录日志
		log.Printf("清理商品列表缓存失败: %v", err)
	}

	return &response.ProductResponse{
		ID:         product.ID,
		CategoryID: product.CategoryID,
		Name:       product.Name,
		Price:      product.Price,
		Stock:      product.Stock,
		Image:      product.Image,
		Status:     product.Status,
		CreatedAt:  product.CreatedAt,
	}, nil
}

func (s *productService) UpdateProduct(id uint, req *request.UpdateProductRequest) error {
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}

	product.CategoryID = req.CategoryID
	product.Name = req.Name
	product.Price = req.Price
	product.Stock = req.Stock
	product.Image = req.Image
	product.Status = req.Status

	if err := s.productRepo.Update(product); err != nil {
		return err
	}

	// 清理缓存
	s.productCache.DeleteProduct(id)
	s.productCache.ClearAllProductList()

	return nil
}

func (s *productService) DeleteProduct(id uint) error {
	if err := s.productRepo.Delete(id); err != nil {
		return err
	}

	// 清理缓存
	s.productCache.DeleteProduct(id)
	s.productCache.ClearAllProductList()

	return nil
}

func (s *productService) GetProductByID(id uint) (*response.ProductResponse, error) {
	// 先查缓存
	product, err := s.productCache.GetProduct(id)
	if err == nil {
		return convertToProductResponse(product), nil
	}

	// 缓存没命中，查数据库
	product, err = s.productRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	s.productCache.SetProduct(product)

	return convertToProductResponse(product), nil
}

func (s *productService) GetProductList(keyword string, categoryID string) ([]*response.ProductResponse, error) {
	// 先查缓存
	products, err := s.productCache.GetProductList(keyword, categoryID)
	if err == nil {
		return convertToProductResponseList(products), nil
	}

	// 缓存没命中，查数据库
	products, err = s.productRepo.List(keyword, categoryID)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	s.productCache.SetProductList(keyword, categoryID, products)

	return convertToProductResponseList(products), nil
}

// 转换为响应DTO
func convertToProductResponse(product *entity.Product) *response.ProductResponse {
	return &response.ProductResponse{
		ID:         product.ID,
		CategoryID: product.CategoryID,
		Name:       product.Name,
		Price:      product.Price,
		Stock:      product.Stock,
		Image:      product.Image,
		Status:     product.Status,
		CreatedAt:  product.CreatedAt,
	}
}

func convertToProductResponseList(products []*entity.Product) []*response.ProductResponse {
	var resp []*response.ProductResponse
	for _, product := range products {
		resp = append(resp, convertToProductResponse(product))
	}
	return resp
}
