package product

import (
	"context"
	"log"

	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/bloom"
	"backend/internal/pkg/breaker"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/keywords"
	"backend/internal/pkg/semaphore"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
)

type ProductService interface {
	CreateProduct(req *request.CreateProductRequest) (*response.ProductResponse, error)
	UpdateProduct(id uint, req *request.UpdateProductRequest) error
	DeleteProduct(id uint) error
	GetProductByID(id uint) (*response.ProductResponse, error)
	GetProductList(req *request.ProductListRequest) (*response.PageResponse, error)
	WarmUpHotProducts(limit int) error
	CalibrateAllSales() error
	BatchGenerateKeywords() (int, error) // 为所有关键词为空的商品批量生成
	InitBloomFilter() error               // 初始化布隆过滤器（启动时调用）
}

type productService struct {
	productRepo  mysql.ProductRepository
	productCache redis.ProductCache
	categoryRepo mysql.CategoryRepository // 用于自动生成关键词
	bloomFilter  *bloom.Filter            // 布隆过滤器：缓存穿透第一道防线
	dbBreaker    *breaker.CircuitBreaker  // 数据库熔断器
	dbLimiter    *semaphore.Limiter       // 数据库并发信号量
}

// NewProductService 创建商品服务实例，注入布隆过滤器+熔断器+信号量
func NewProductService(
	productRepo mysql.ProductRepository,
	productCache redis.ProductCache,
	categoryRepo mysql.CategoryRepository,
	bloomFilter *bloom.Filter,
	dbBreaker *breaker.CircuitBreaker,
	dbLimiter *semaphore.Limiter,
) ProductService {
	return &productService{
		productRepo:  productRepo,
		productCache: productCache,
		categoryRepo: categoryRepo,
		bloomFilter:  bloomFilter,
		dbBreaker:    dbBreaker,
		dbLimiter:    dbLimiter,
	}
}

// CreateProduct 创建商品，关键词留空则自动生成，清理旧缓存防空值遮蔽
func (s *productService) CreateProduct(req *request.CreateProductRequest) (*response.ProductResponse, error) {
	// 若未手动指定关键词，则自动从商品名和分类生成
	keywordsStr := req.Keywords
	if keywordsStr == "" {
		keywordsStr = s.autoGenerateKeywords(req.Name, req.CategoryID)
	}

	product := &entity.Product{
		CategoryID: req.CategoryID,
		Name:       req.Name,
		Keywords:   keywordsStr,
		Price:      req.Price,
		Stock:      req.Stock,
		Image:      req.Image,
		Status:     req.Status,
	}

	if err := s.productRepo.Create(product); err != nil {
		return nil, err
	}

	// 清理单品缓存（包括可能的空值缓存"nil"），防止新建商品被旧缓存挡住
	s.productCache.DeleteProduct(product.ID)
	// 清理商品列表缓存
	if err := s.productCache.ClearAllProductList(); err != nil {
		log.Printf("清理商品列表缓存失败: %v", err)
	}

	// 将新产品 ID 添加到布隆过滤器（实时更新，无需等待全量重建）
	if s.bloomFilter != nil {
		s.bloomFilter.Add(product.ID)
	}

	return &response.ProductResponse{
		ID:         product.ID,
		CategoryID: product.CategoryID,
		Name:       product.Name,
		Keywords:   product.Keywords,
		Price:      product.Price,
		Stock:      product.Stock,
		Image:      product.Image,
		Status:     product.Status,
		CreatedAt:  product.CreatedAt,
		Sales:      product.Sales,
	}, nil
}

// UpdateProduct 更新商品信息，同步清理单品和列表缓存
func (s *productService) UpdateProduct(id uint, req *request.UpdateProductRequest) error {
	product, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}

	product.CategoryID = req.CategoryID
	product.Name = req.Name
	// 关键词自动生成：若未手动填写则自动补充
	if req.Keywords == "" {
		product.Keywords = s.autoGenerateKeywords(req.Name, req.CategoryID)
	} else {
		product.Keywords = req.Keywords
	}
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

// DeleteProduct 软删除商品，清理所有相关缓存
func (s *productService) DeleteProduct(id uint) error {
	if err := s.productRepo.Delete(id); err != nil {
		return err
	}

	// 清理缓存
	s.productCache.DeleteProduct(id)
	s.productCache.ClearAllProductList()

	return nil
}

// GetProductByID 适配缓存降级 + 布隆过滤器防护
func (s *productService) GetProductByID(id uint) (*response.ProductResponse, error) {
	// 🔴 第一道防线：布隆过滤器快速判否（跳过不存在ID的缓存和数据库查询）
	if s.bloomFilter != nil && !s.bloomFilter.MightContain(id) {
		return nil, errors.New(errors.CodeNotFound, "商品不存在")
	}

	// 忽略缓存错误，自动降级到数据库
	product, _ := s.productCache.GetProduct(id)
	if product != nil {
		return convertToProductResponse(product), nil
	}

	// 缓存未命中或Redis故障 → 查数据库（熔断器 + 信号量保护）
	if s.dbBreaker != nil && !s.dbBreaker.Allow() {
		return nil, errors.New(errors.CodeServerError, "服务繁忙，请稍后重试")
	}
	ctx := context.Background()
	if s.dbLimiter != nil && !s.dbLimiter.Acquire(ctx) {
		return nil, errors.New(errors.CodeServerError, "系统繁忙，请稍后重试")
	}
	product, err := s.productRepo.GetByID(id)
	if s.dbLimiter != nil {
		s.dbLimiter.Release()
	}
	if err != nil {
		if s.dbBreaker != nil {
			s.dbBreaker.RecordFailure()
		}
		// 缓存空值防止缓存穿透
		s.productCache.SetProduct(nil)
		return nil, err
	}
	if s.dbBreaker != nil {
		s.dbBreaker.RecordSuccess()
	}

	// 异步写入缓存（不阻塞主流程，已增加 panic 恢复保护）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 异步写入商品缓存发生panic: %v", r)
			}
		}()
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

// autoGenerateKeywords 根据商品名和分类 ID 自动生成搜索关键词
// 管理员无需手动填写，系统从品牌映射 + 属性映射 + 分类层级自动推导
func (s *productService) autoGenerateKeywords(name string, categoryID uint) string {
	var categoryPath []string

	// 查询分类层级（最多向上查3层）
	category, err := s.categoryRepo.GetByID(categoryID)
	if err == nil && category != nil {
		categoryPath = append(categoryPath, category.Name)
		// 如果有父分类，也加入
		if category.ParentID > 0 {
			parent, err := s.categoryRepo.GetByID(category.ParentID)
			if err == nil && parent != nil {
				categoryPath = append([]string{parent.Name}, categoryPath...)
			}
		}
	}

	generated := keywords.Generate(name, categoryPath)
	if generated != "" {
		log.Printf("🔑 自动生成关键词: 商品[%s] → %s", name, generated)
	}
	return generated
}

// 转换为响应DTO
func convertToProductResponse(product *entity.Product) *response.ProductResponse {
	return &response.ProductResponse{
		ID:         product.ID,
		CategoryID: product.CategoryID,
		Name:       product.Name,
		Keywords:   product.Keywords,
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

// InitBloomFilter 启动时从数据库全量加载所有产品 ID 构建布隆过滤器
//
// 调用时机：服务启动后异步执行（不阻塞 HTTP 服务）
// 重建策略：每次调用重新分配内存并全量重建，避免删除产品的 ID 残留
func (s *productService) InitBloomFilter() error {
	if s.bloomFilter == nil {
		return nil
	}

	log.Println("🔍 开始构建布隆过滤器（从数据库加载所有产品ID）...")

	// 从数据库获取所有产品 ID
	ids, err := s.productRepo.GetAllIDs()
	if err != nil {
		return errors.Wrap(err, "加载产品ID列表失败，布隆过滤器初始化中止")
	}

	// 全量添加到布隆过滤器
	for _, id := range ids {
		s.bloomFilter.Add(id)
	}

	log.Printf("✅ 布隆过滤器构建完成，已加载 %d 个产品ID，内存占用约 %d KB",
		len(ids), s.bloomFilter.Size()/1024)
	return nil
}

// GetBloomFilter 暴露布隆过滤器以供外部（如 router）定时重建
func (s *productService) GetBloomFilter() *bloom.Filter {
	return s.bloomFilter
}

// BatchGenerateKeywords 为所有关键词为空的存量商品批量自动生成关键词
// 返回成功生成的数量
func (s *productService) BatchGenerateKeywords() (int, error) {
	log.Println("🔑 开始为存量商品批量生成关键词...")

	db := s.productRepo.GetDB()
	var products []entity.Product
	if err := db.Where("keywords = '' OR keywords IS NULL").Find(&products).Error; err != nil {
		return 0, errors.Wrap(err, "查询无关键词商品失败")
	}

	count := 0
	for _, product := range products {
		keywords := s.autoGenerateKeywords(product.Name, product.CategoryID)
		if keywords != "" {
			if err := db.Model(&product).Update("keywords", keywords).Error; err != nil {
				log.Printf("⚠️ 商品ID %d 关键词生成失败: %v", product.ID, err)
				continue
			}
			count++
		}
	}

	log.Printf("✅ 批量关键词生成完成: %d/%d", count, len(products))
	return count, nil
}
