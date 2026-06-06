package product

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"log"
)

type ProductService interface {
	CreateProduct(req *request.CreateProductRequest) (*response.ProductResponse, error)
	UpdateProduct(id uint, req *request.UpdateProductRequest) error
	DeleteProduct(id uint) error
	GetProductByID(id uint) (*response.ProductResponse, error)
	GetProductList(req *request.ProductListRequest) (*response.PageResponse, error)
	WarmUpHotProducts(limit int) error
	CalibrateAllSales() error
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
		Sales:      product.Sales,
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

// GetProductByID 适配缓存降级
func (s *productService) GetProductByID(id uint) (*response.ProductResponse, error) {
	// 忽略缓存错误，自动降级到数据库
	product, _ := s.productCache.GetProduct(id)
	if product != nil {
		return convertToProductResponse(product), nil
	}

	// 缓存未命中或Redis故障，直接查数据库
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		// 缓存空值防止缓存穿透
		s.productCache.SetProduct(nil)
		return nil, err
	}

	// 异步写入缓存（不阻塞主流程）
	go func() {
		if err := s.productCache.SetProduct(product); err != nil {
			log.Printf("⚠️ 写入商品缓存失败: %v", err)
		}
	}()

	return convertToProductResponse(product), nil
}

// GetProductList 分页查询商品列表（支持复杂筛选和排序）
func (s *productService) GetProductList(req *request.ProductListRequest) (*response.PageResponse, error) {
	// 参数默认值处理
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}
	if req.Sort == "" {
		req.Sort = "created_at" // 默认按创建时间倒序
	}

	// 调用Repository层的分页查询方法
	products, total, err := s.productRepo.ListPage(req)
	if err != nil {
		return nil, err
	}

	// 转换为响应DTO
	productRespList := convertToProductResponseList(products)

	// 返回统一分页响应
	return response.NewPageResponse(productRespList, total, req.PageNum, req.PageSize), nil
}

// WarmUpHotProducts 暴露预热方法给上层
func (s *productService) WarmUpHotProducts(limit int) error {
	return s.productCache.WarmUpHotProducts(s.productRepo, limit)
}

// CalibrateAllSales 全量校准所有商品销量（每日凌晨执行）
func (s *productService) CalibrateAllSales() error {
	log.Println("🔧 开始全量校准商品销量数据...")

	// 1. 从订单表统计真实销量
	var stats []mysql.SalesStat

	db := s.productRepo.GetDB()

	err := db.Model(&entity.OrderItem{}).
		Select("product_id, SUM(quantity) as total").
		Joins("LEFT JOIN `orders` ON order_items.order_id = `orders`.id").
		Where("`orders`.status = ?", 1). // 只统计已支付订单
		Group("product_id").
		Scan(&stats).Error

	if err != nil {
		return errors.Wrap(err, "统计真实销量失败")
	}

	// 2. 批量更新product表的sales字段
	err = s.productRepo.BatchUpdateSales(stats)
	if err != nil {
		return errors.Wrap(err, "批量更新商品销量失败")
	}

	// 3. 清理所有商品缓存
	for _, stat := range stats {
		s.productCache.DeleteProduct(stat.ProductID)
	}
	s.productCache.ClearAllProductList()

	log.Printf("✅ 商品销量校准完成，共更新: %d 个商品", len(stats))
	return nil
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
		Sales:      product.Sales,
	}
}

func convertToProductResponseList(products []*entity.Product) []*response.ProductResponse {
	var resp []*response.ProductResponse
	for _, product := range products {
		resp = append(resp, convertToProductResponse(product))
	}
	return resp
}
