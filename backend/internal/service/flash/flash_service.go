package flash

import (
	"backend/internal/config"
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/flashinventory"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
)

// FlashService 秒杀业务逻辑接口
type FlashService interface {
	// === 管理员操作 ===
	CreateFlashSale(req *request.CreateFlashSaleRequest) (*entity.FlashSale, error)
	UpdateFlashSale(id uint, req *request.UpdateFlashSaleRequest) error
	WarmUpFlashSale(id uint) error               // 预热库存到Redis + 初始化内存标记
	EndFlashSale(id uint) error                  // 强制结束秒杀
	ListAllFlashSales() ([]*response.AdminFlashSaleResponse, error)

	// === 用户端操作 ===
	ListActiveFlashSales() ([]*response.FlashSaleListResponse, error)
	GetFlashSaleDetail(id uint) (*response.FlashSaleDetailResponse, error)
	EnterFlashSale(userID uint, req *request.FlashEnterRequest) (*response.FlashEnterResponse, error)
	SnatchFlashSale(userID uint, req *request.FlashSnatchRequest) (*response.FlashSnatchResponse, error)
	GetUserFlashOrders(userID uint) ([]*response.FlashOrderResponse, error)

	// === 后台定时任务 ===
	ScanExpiredOrders()                          // 扫描超时未支付订单
	RecoverPendingOrders()                       // 崩溃恢复：处理未完成的防丢失记录

	// === 注入方法 ===
	SetUserLimiter(limiter interface{ Allow(userID uint) bool }) // 注入 per-user 限流器

	// === 库存管理器 ===
	GetInventory() *flashinventory.Inventory
}

type flashService struct {
	db              *gorm.DB
	cfg             *config.AppConfig
	flashRepo       mysql.FlashRepository
	flashCache      redis.FlashCache
	orderRepo       mysql.OrderRepository
	orderItemRepo   mysql.OrderItemRepository
	productRepo     mysql.ProductRepository
	productCache    redis.ProductCache
	inventory       *flashinventory.Inventory
	userLimiter     interface{ Allow(userID uint) bool } // per-user 内存限流器
}

// NewFlashService 创建秒杀服务实例（供 Wire 注入）
func NewFlashService(
	db *gorm.DB,
	cfg *config.AppConfig,
	flashRepo mysql.FlashRepository,
	flashCache redis.FlashCache,
	orderRepo mysql.OrderRepository,
	orderItemRepo mysql.OrderItemRepository,
	productRepo mysql.ProductRepository,
	productCache redis.ProductCache,
) FlashService {
	svc := &flashService{
		db:            db,
		cfg:           cfg,
		flashRepo:     flashRepo,
		flashCache:    flashCache,
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		productRepo:   productRepo,
		productCache:  productCache,
		inventory:     flashinventory.New(),
	}
	return svc
}

// SetUserLimiter 注入用户限流器（Wire不支持直接注入，通过此方法设置）
func (s *flashService) SetUserLimiter(limiter interface{ Allow(userID uint) bool }) {
	s.userLimiter = limiter
}

// GetInventory 返回内存库存管理器（供 Router 和 main.go 访问）
func (s *flashService) GetInventory() *flashinventory.Inventory {
	return s.inventory
}

// ==================== 管理员操作 ====================

// CreateFlashSale 创建秒杀活动
func (s *flashService) CreateFlashSale(req *request.CreateFlashSaleRequest) (*entity.FlashSale, error) {
	// 校验商品是否存在且上架
	product, err := s.productRepo.GetByID(req.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Status != 1 {
		return nil, errors.New(errors.CodeParamError, "商品已下架，无法创建秒杀")
	}

	// 解析时间
	startTime, err := time.Parse("2006-01-02 15:04:05", req.StartTime)
	if err != nil {
		return nil, errors.New(errors.CodeParamError, "开始时间格式错误（应为 yyyy-MM-dd HH:mm:ss）")
	}
	endTime, err := time.Parse("2006-01-02 15:04:05", req.EndTime)
	if err != nil {
		return nil, errors.New(errors.CodeParamError, "结束时间格式错误（应为 yyyy-MM-dd HH:mm:ss）")
	}
	if !endTime.After(startTime) {
		return nil, errors.New(errors.CodeParamError, "结束时间必须晚于开始时间")
	}

	// 自动计算排队上限（0 = stock × 10，最小值500）
	queueCap := req.QueueCap
	if queueCap == 0 {
		queueCap = req.FlashStock * 10
		if queueCap < 500 {
			queueCap = 500
		}
	}

	flash := &entity.FlashSale{
		ProductID:  req.ProductID,
		FlashPrice: req.FlashPrice,
		FlashStock: req.FlashStock,
		QueueCap:   queueCap,
		StartTime:  startTime,
		EndTime:    endTime,
		Status:     0, // 默认未开始
	}

	if err := s.flashRepo.Create(flash); err != nil {
		return nil, err
	}
	return flash, nil
}

// UpdateFlashSale 修改秒杀活动（仅允许修改未开始或进行中的活动）
func (s *flashService) UpdateFlashSale(id uint, req *request.UpdateFlashSaleRequest) error {
	flash, err := s.flashRepo.GetByID(id)
	if err != nil {
		return err
	}

	if flash.Status == 2 || flash.Status == 3 {
		return errors.New(errors.CodeFlashSaleCancelled, "已结束或已取消的活动无法修改")
	}

	// 仅更新非零字段
	if req.FlashPrice > 0 {
		flash.FlashPrice = req.FlashPrice
	}
	if req.FlashStock > 0 {
		flash.FlashStock = req.FlashStock
	}
	if req.QueueCap >= 0 {
		flash.QueueCap = req.QueueCap
	}
	if req.StartTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", req.StartTime)
		if err != nil {
			return errors.New(errors.CodeParamError, "开始时间格式错误")
		}
		flash.StartTime = t
	}
	if req.EndTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", req.EndTime)
		if err != nil {
			return errors.New(errors.CodeParamError, "结束时间格式错误")
		}
		flash.EndTime = t
	}
	if req.Status != nil {
		flash.Status = *req.Status
	}

	return s.flashRepo.Update(flash)
}

// WarmUpFlashSale 预热秒杀库存到Redis
func (s *flashService) WarmUpFlashSale(id uint) error {
	flash, err := s.flashRepo.GetByID(id)
	if err != nil {
		return err
	}

	// 仅未开始和进行中的活动可以预热
	if flash.Status != 0 && flash.Status != 1 {
		return errors.New(errors.CodeParamError, "仅未开始或进行中的活动可以预热")
	}

	// 计算排队上限
	queueCap := flash.QueueCap
	if queueCap == 0 {
		queueCap = flash.FlashStock * 10
		if queueCap < 500 {
			queueCap = 500
		}
	}

	// 1. 预热库存到Redis
	if err := s.flashCache.WarmUpStock(id, flash.FlashStock); err != nil {
		return err
	}

	// 2. 初始化本地内存库存计数
	s.inventory.Init(id, flash.FlashStock)

	// 3. 缓存活动信息
	info := redis.MarshalFlashInfo(flash)
	if err := s.flashCache.SetFlashInfo(id, info); err != nil {
		return err
	}

	// 4. 更新活动状态为进行中
	if flash.Status == 0 {
		if err := s.flashRepo.UpdateStatus(id, 1); err != nil {
			return err
		}
	}

	log.Printf("✅ 秒杀活动 %d 预热完成：库存=%d，排队上限=%d", id, flash.FlashStock, queueCap)
	return nil
}

// EndFlashSale 强制结束秒杀活动
func (s *flashService) EndFlashSale(id uint) error {
	flash, err := s.flashRepo.GetByID(id)
	if err != nil {
		return err
	}

	if flash.Status != 1 {
		return errors.New(errors.CodeParamError, "仅进行中的活动可以强制结束")
	}

	// 更新状态为已结束
	if err := s.flashRepo.UpdateStatus(id, 2); err != nil {
		return err
	}

	// 清理Redis和内存
	s.flashCache.ClearFlashCache(id)
	s.inventory.Cleanup(id)

	log.Printf("🛑 秒杀活动 %d 已强制结束", id)
	return nil
}

// ListAllFlashSales 管理员查看所有秒杀活动
func (s *flashService) ListAllFlashSales() ([]*response.AdminFlashSaleResponse, error) {
	list, err := s.flashRepo.ListAll()
	if err != nil {
		return nil, err
	}

	var resp []*response.AdminFlashSaleResponse
	for _, f := range list {
		resp = append(resp, &response.AdminFlashSaleResponse{
			ID:         f.ID,
			ProductID:  f.ProductID,
			FlashPrice: f.FlashPrice,
			FlashStock: f.FlashStock,
			QueueCap:   f.QueueCap,
			StartTime:  f.StartTime,
			EndTime:    f.EndTime,
			Status:     f.Status,
			CreatedAt:  f.CreatedAt,
		})
	}
	return resp, nil
}

// ==================== 用户端操作 ====================

// ListActiveFlashSales 获取进行中的秒杀活动列表
func (s *flashService) ListActiveFlashSales() ([]*response.FlashSaleListResponse, error) {
	list, err := s.flashRepo.ListActive()
	if err != nil {
		return nil, err
	}

	var resp []*response.FlashSaleListResponse
	for _, f := range list {
		remaining, _ := s.flashCache.GetRemainingStock(f.ID)
		queueCount, _ := s.flashCache.GetQueueCount(f.ID)

		resp = append(resp, &response.FlashSaleListResponse{
			ID:          f.ID,
			ProductID:   f.ProductID,
			ProductName: f.Product.Name,
			FlashPrice:  f.FlashPrice,
			OriginPrice: f.Product.Price,
			FlashStock:  f.FlashStock,
			Remaining:   int(remaining),
			QueueCount:  int(queueCount),
			StartTime:   f.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:     f.EndTime.Format("2006-01-02 15:04:05"),
			Status:      f.Status,
			Image:       f.Product.Image,
		})
	}
	return resp, nil
}

// GetFlashSaleDetail 获取秒杀活动详情
func (s *flashService) GetFlashSaleDetail(id uint) (*response.FlashSaleDetailResponse, error) {
	flash, err := s.flashRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	remaining, _ := s.flashCache.GetRemainingStock(id)
	queueCount, _ := s.flashCache.GetQueueCount(id)

	queueCap := flash.QueueCap
	if queueCap == 0 {
		queueCap = flash.FlashStock * 10
		if queueCap < 500 {
			queueCap = 500
		}
	}

	return &response.FlashSaleDetailResponse{
		ID:          flash.ID,
		ProductID:   flash.ProductID,
		ProductName: flash.Product.Name,
		FlashPrice:  flash.FlashPrice,
		OriginPrice: flash.Product.Price,
		FlashStock:  flash.FlashStock,
		Remaining:   int(remaining),
		QueueCount:  int(queueCount),
		QueueCap:    queueCap,
		StartTime:   flash.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:     flash.EndTime.Format("2006-01-02 15:04:05"),
		Status:      flash.Status,
		Image:       flash.Product.Image,
		ServerTime:  time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// EnterFlashSale 排队入场
// 核心逻辑：Redis INCR 计数，≤ cap → 允许入场，> cap → 拒绝
func (s *flashService) EnterFlashSale(userID uint, req *request.FlashEnterRequest) (*response.FlashEnterResponse, error) {
	// 校验活动有效性
	flash, err := s.flashRepo.GetByID(req.FlashSaleID)
	if err != nil {
		return nil, err
	}

	if flash.Status != 1 {
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动未在进行中")
	}

	now := time.Now()
	if now.Before(flash.StartTime) {
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动尚未开始")
	}
	if now.After(flash.EndTime) {
		return nil, errors.New(errors.CodeFlashSaleEnded, "秒杀活动已结束")
	}

	// 计算排队上限
	queueCap := flash.QueueCap
	if queueCap == 0 {
		queueCap = flash.FlashStock * 10
		if queueCap < 500 {
			queueCap = 500
		}
	}

	// ★ 原子递增排队计数
	count, err := s.flashCache.IncrQueueCount(req.FlashSaleID)
	if err != nil {
		// Redis故障时降级放行
		log.Printf("⚠️ 秒杀排队Redis故障，降级放行: %v", err)
		return &response.FlashEnterResponse{
			Admitted:    true,
			QueueNumber: 0,
			Message:     "已入场（降级模式）",
		}, nil
	}

	if count > int64(queueCap) {
		return &response.FlashEnterResponse{
			Admitted:    false,
			QueueNumber: count,
			Message:     "当前人数已满，请稍后访问",
		}, nil
	}

	// 记录入场资格
	if err := s.flashCache.AddAdmittedUser(req.FlashSaleID, userID); err != nil {
		log.Printf("⚠️ 记录入场资格失败: %v", err)
	}

	return &response.FlashEnterResponse{
		Admitted:    true,
		QueueNumber: count,
		Message:     "已入场",
	}, nil
}

// getPaymentTimeout 获取支付超时时间（从配置读取，默认2小时）
func (s *flashService) getPaymentTimeout() time.Duration {
	hours := s.cfg.FlashSale.PaymentTimeoutHours
	if hours <= 0 {
		hours = 2 // 默认2小时
	}
	return time.Duration(hours) * time.Hour
}

// getCooldown 获取冷却时间（从配置读取，默认2分钟）
func (s *flashService) getCooldown() time.Duration {
	minutes := s.cfg.FlashSale.CoolDownMinutes
	if minutes <= 0 {
		minutes = 2 // 默认2分钟
	}
	return time.Duration(minutes) * time.Minute
}

// SnatchFlashSale 秒杀抢购核心逻辑
// 流程：校验资格 → 内存售罄检查 → Redis Lua原子扣减 → 同步DB写入 → 返回
func (s *flashService) SnatchFlashSale(userID uint, req *request.FlashSnatchRequest) (*response.FlashSnatchResponse, error) {
	// 第2层：per-user 限流检查
	if s.userLimiter != nil && !s.userLimiter.Allow(userID) {
		return &response.FlashSnatchResponse{Success: false, Message: "操作过于频繁，请稍后再试"}, nil
	}

	// 校验活动有效性和时间窗口
	flash, err := s.flashRepo.GetByID(req.FlashSaleID)
	if err != nil {
		return nil, err
	}

	if flash.Status != 1 {
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动未在进行中")
	}
	now := time.Now()
	if now.Before(flash.StartTime) {
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动尚未开始")
	}
	if now.After(flash.EndTime) {
		return nil, errors.New(errors.CodeFlashSaleEnded, "秒杀活动已结束")
	}

	// 校验入场资格
	admitted, err := s.flashCache.IsUserAdmitted(req.FlashSaleID, userID)
	if err != nil || !admitted {
		return nil, errors.New(errors.CodeFlashNotEntered, "请先排队入场")
	}

	// 第3层：本地内存售罄检查（5ns）
	if s.inventory.IsSoldOut(req.FlashSaleID) {
		return &response.FlashSnatchResponse{Success: false, Message: "秒杀商品已售罄"}, nil
	}

	// 第4层：Redis Lua 原子扣减
	code, remaining, err := s.flashCache.AtomicDeductStock(req.FlashSaleID, userID)
	if err != nil {
		return nil, errors.Wrap(err, "系统繁忙，请重试")
	}

	switch code {
	case 1:
		// 库存不足 → 设置售罄标记
		s.inventory.Init(req.FlashSaleID, 0) // 确保售罄标记生效
		return &response.FlashSnatchResponse{Success: false, Message: "秒杀商品已售罄"}, nil
	case 2:
		return &response.FlashSnatchResponse{Success: false, Message: "您已参与过该秒杀"}, nil
	case 3:
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动未预热，请稍后再试")
	}

	// code==0：扣减成功！更新本地内存计数
	s.inventory.Decrement(req.FlashSaleID)

	// 生成订单号
	orderNo := s.generateOrderNo()
	productPrice := flash.FlashPrice

	// ★ 第5层：防丢失记录 → 同步DB写入 ★
	// 先写Redis防丢失记录（数据库崩溃恢复用）
	if err := s.flashCache.SetPendingOrder(req.FlashSaleID, userID, orderNo); err != nil {
		log.Printf("⚠️ 设置防丢失记录失败: %v", err)
		// 不阻断流程，继续写DB
	}

	// 同步写DB
	err = s.createFlashOrderInDB(flash, userID, orderNo, productPrice)
	if err != nil {
		// DB写入失败 → 回滚Redis
		log.Printf("❌ 秒杀订单DB写入失败，回滚Redis: userID=%d flashSaleID=%d err=%v", userID, req.FlashSaleID, err)
		s.flashCache.RollbackDeduct(req.FlashSaleID, userID)
		s.flashCache.RemovePendingOrder(req.FlashSaleID, userID)
		s.inventory.Increment(req.FlashSaleID)
		return nil, errors.New(errors.CodeServerError, "订单创建失败，秒杀资格已保留，请重新尝试")
	}

	// DB写入成功 → 清理防丢失记录
	s.flashCache.RemovePendingOrder(req.FlashSaleID, userID)

	log.Printf("🎉 秒杀成功: userID=%d flashSaleID=%d orderNo=%s remaining=%d", userID, req.FlashSaleID, orderNo, remaining)
	return &response.FlashSnatchResponse{
		Success: true,
		OrderNo: orderNo,
		Message: "抢购成功！请在2小时内完成支付",
	}, nil
}

// createFlashOrderInDB 在DB事务中创建秒杀订单
func (s *flashService) createFlashOrderInDB(flash *entity.FlashSale, userID uint, orderNo string, price float64) error {
	tx := s.db.Begin()
	if tx.Error != nil {
		return errors.Wrap(tx.Error, "开启事务失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 第6层：乐观锁校验（防止Redis极端异常导致的超卖）
	// 统计该活动已成功抢购的订单数
	var count int64
	if err := tx.Model(&entity.Order{}).
		Where("flash_sale_id = ? AND status IN (0, 1, 3)", flash.ID).
		Count(&count).Error; err != nil {
		tx.Rollback()
		return errors.Wrap(err, "统计已售订单失败")
	}
	if int(count) >= flash.FlashStock {
		tx.Rollback()
		return errors.New(errors.CodeFlashSaleSoldOut, "秒杀库存已售罄（DB校验）")
	}

	// 创建订单
	fid := flash.ID
	order := &entity.Order{
		UserID:      userID,
		FlashSaleID: &fid,
		OrderNo:     orderNo,
		Total:       price,
		Status:      0, // 待支付
	}
	if err := s.orderRepo.WithTx(tx).Create(order); err != nil {
		tx.Rollback()
		return err
	}

	// 创建订单项
	orderItem := &entity.OrderItem{
		OrderID:   order.ID,
		ProductID: flash.ProductID,
		Quantity:  1,
		Price:     price,
	}
	if err := s.orderItemRepo.WithTx(tx).BatchCreate([]*entity.OrderItem{orderItem}); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return errors.Wrap(err, "提交事务失败")
	}

	// 清理商品缓存
	s.productCache.DeleteProduct(flash.ProductID)

	return nil
}

// generateOrderNo 生成秒杀订单号（FS + 时间戳 + 随机十六进制）
func (s *flashService) generateOrderNo() string {
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	return fmt.Sprintf("FS%s%s", time.Now().Format("20060102150405"), hex.EncodeToString(randomBytes))
}

// GetUserFlashOrders 获取用户秒杀订单列表
func (s *flashService) GetUserFlashOrders(userID uint) ([]*response.FlashOrderResponse, error) {
	// 复用现有 OrderRepository 查订单
	orders, err := s.orderRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	var resp []*response.FlashOrderResponse
	for _, o := range orders {
		if o.FlashSaleID == nil {
			continue // 跳过普通订单
		}

		// 获取订单项中的商品名称
		productName := "秒杀商品"
		items, _ := s.orderItemRepo.GetByOrderID(o.ID)
		if len(items) > 0 {
			product, _ := s.productRepo.GetByID(items[0].ProductID)
			if product != nil {
				productName = product.Name
			}
		}

		resp = append(resp, &response.FlashOrderResponse{
			ID:          o.ID,
			OrderNo:     o.OrderNo,
			FlashPrice:  o.Total,
			ProductName: productName,
			Status:      o.Status,
			CreatedAt:   o.CreatedAt,
		})
	}
	return resp, nil
}

// ==================== 定时任务 ====================

// cooldownMutex 防止 ScanExpiredOrders 并发执行
var cooldownMutex sync.Mutex

// ScanExpiredOrders 扫描超时未支付的秒杀订单
// 每30秒执行一次（由 main.go 中的 goroutine 驱动）
func (s *flashService) ScanExpiredOrders() {
	if !cooldownMutex.TryLock() {
		return // 上一次扫描还未完成，跳过
	}
	defer cooldownMutex.Unlock()

	timeoutThreshold := time.Now().Add(-s.getPaymentTimeout())
	cooldownThreshold := time.Now().Add(-s.getCooldown())

	// === 第1步：将超时订单标记为"待释放"（status: 0→3）===
	var expiredOrders []entity.Order
	s.db.Model(&entity.Order{}).
		Where("flash_sale_id IS NOT NULL AND status = 0 AND created_at < ?", timeoutThreshold).
		Find(&expiredOrders)

	for _, o := range expiredOrders {
		// 条件UPDATE：仅在 status=0 时更新为3（防支付回调竞态）
		result := s.db.Model(&entity.Order{}).
			Where("id = ? AND status = 0", o.ID).
			Update("status", 3)

		if result.RowsAffected == 0 {
			continue // 用户恰好支付了，跳过
		}

		log.Printf("⏰ 秒杀订单超时，进入冷却期: orderID=%d flashSaleID=%d userID=%d", o.ID, *o.FlashSaleID, o.UserID)
	}

	// === 第2步：处理冷却期满的订单（status: 3→2，释放库存）===
	var coolingOrders []entity.Order
	s.db.Model(&entity.Order{}).
		Where("flash_sale_id IS NOT NULL AND status = 3 AND updated_at < ?", cooldownThreshold).
		Find(&coolingOrders)

	for _, o := range coolingOrders {
		// 条件UPDATE：仅在 status=3 时更新为2（冷却期内支付了则不释放）
		result := s.db.Model(&entity.Order{}).
			Where("id = ? AND status = 3", o.ID).
			Update("status", 2)

		if result.RowsAffected == 0 {
			continue // 冷却期内支付成功，跳过
		}

		fid := *o.FlashSaleID

		// 释放Redis库存
		if err := s.flashCache.RollbackDeduct(fid, o.UserID); err != nil {
			log.Printf("⚠️ 释放Redis库存失败: orderID=%d err=%v", o.ID, err)
		}

		// 释放入场资格（允许用户重新排队）
		// ...

		// 重置本地内存售罄标记
		s.inventory.ResetSoldOut(fid)

		log.Printf("🔄 秒杀库存已释放: orderID=%d flashSaleID=%d userID=%d", o.ID, fid, o.UserID)
	}
}

// RecoverPendingOrders 崩溃恢复：扫描Redis中的防丢失记录，补建丢失的DB订单
// 在 main.go 启动时调用
func (s *flashService) RecoverPendingOrders() {
	log.Println("🔍 开始扫描秒杀防丢失记录...")

	keys, err := s.flashCache.ScanAllPendingKeys()
	if err != nil {
		log.Printf("⚠️ 扫描防丢失记录失败: %v", err)
		return
	}

	if len(keys) == 0 {
		log.Println("✅ 无待恢复的秒杀订单")
		return
	}

	log.Printf("📋 发现 %d 个待恢复的活动", len(keys))
	recovered := 0
	rolledBack := 0

	for _, key := range keys {
		// 从 key 中提取 flashSaleID: "flash:pending:123"
		var flashSaleID uint
		fmt.Sscanf(key, "flash:pending:%d", &flashSaleID)

		pendingOrders, err := s.flashCache.GetPendingOrders(flashSaleID)
		if err != nil {
			log.Printf("⚠️ 读取防丢失记录失败: key=%s err=%v", key, err)
			continue
		}

		for userIDStr, orderNo := range pendingOrders {
			userID, _ := strconv.ParseUint(userIDStr, 10, 64)

			// 检查DB中是否已存在该订单
			existing, err := s.orderRepo.GetByOrderNo(orderNo)
			if err == nil && existing != nil {
				// DB中已存在，只清理Redis记录
				s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
				log.Printf("  ✓ 订单已存在于DB，清理Redis: orderNo=%s", orderNo)
				continue
			}

			// DB中不存在 → 补建订单
			flash, err := s.flashRepo.GetByID(flashSaleID)
			if err != nil {
				// 活动都不存在了，回滚Redis库存
				s.flashCache.RollbackDeduct(flashSaleID, uint(userID))
				s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
				rolledBack++
				log.Printf("  ✗ 活动不存在，回滚库存: flashSaleID=%d userID=%d", flashSaleID, userID)
				continue
			}

			err = s.createFlashOrderInDB(flash, uint(userID), orderNo, flash.FlashPrice)
			if err != nil {
				log.Printf("  ⚠ 补建订单失败（回滚Redis）: orderNo=%s err=%v", orderNo, err)
				s.flashCache.RollbackDeduct(flashSaleID, uint(userID))
				s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
				rolledBack++
			} else {
				s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
				recovered++
				log.Printf("  ✅ 补建订单成功: orderNo=%s userID=%d", orderNo, userID)
			}
		}
	}

	log.Printf("✅ 崩溃恢复完成：补建=%d 回滚=%d", recovered, rolledBack)
}
