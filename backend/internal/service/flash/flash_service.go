package flash

import (
	"backend/internal/config"
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/captcha"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/flashinventory"
	"backend/internal/pkg/flashlimiter"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
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
	GenerateCaptcha() (*captcha.Captcha, error)    // 生成验证码（人机验证）

	// === 后台定时任务 ===
	ScanExpiredOrders()                          // 扫描超时未支付订单
	GuardConsistency()                           // 一致性守护：崩溃恢复 + 库存对账，每5秒执行
	AutoWarmUp()                                 // 自动预热即将开始的秒杀活动
	RecoverPendingOrders()                       // 崩溃恢复：启动时单独调用一次

	// === 注入方法 ===
	SetUserLimiter(limiter flashlimiter.UserLimiter) // 注入 per-user 限流器

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
	userLimiter     flashlimiter.UserLimiter // per-user 内存限流器
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
func (s *flashService) SetUserLimiter(limiter flashlimiter.UserLimiter) {
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

	// 解析时间（使用本地时区，保证 time.Now() 比较时一致性）
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, time.Local)
	if err != nil {
		return nil, errors.New(errors.CodeParamError, "开始时间格式错误（应为 yyyy-MM-dd HH:mm:ss）")
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, time.Local)
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
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, time.Local)
		if err != nil {
			return errors.New(errors.CodeParamError, "开始时间格式错误")
		}
		flash.StartTime = t
	}
	if req.EndTime != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, time.Local)
		if err != nil {
			return errors.New(errors.CodeParamError, "结束时间格式错误")
		}
		flash.EndTime = t
	}
	if req.Status != nil {
		flash.Status = *req.Status
	}

	// 修改成功后主动失效活动缓存，防止前端读到过期数据
	s.flashCache.DeleteFlashInfo(id)
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

	// 4. 更新活动状态为进行中（仅在开始时间已到达时切换）
	if flash.Status == 0 && !time.Now().Before(flash.StartTime) {
		if err := s.flashRepo.UpdateStatus(id, 1); err != nil {
			return err
		}
	} else if flash.Status == 0 {
		log.Printf("📋 秒杀活动 %d 已预热但尚未到开始时间（%s），保持未开始状态",
			id, flash.StartTime.Format("2006-01-02 15:04:05"))
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
			ServerTime:  time.Now().Format("2006-01-02 15:04:05"),
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

	// ★ 随机延迟（公平调度）
	// 将同一瞬间涌入的请求打散到 RandomDelayMaxMs 窗口内，
	// 使排队序号不再严格取决于请求到达时序，防止脚本垄断前排位置
	if delayMax := s.cfg.FlashSale.RandomDelayMaxMs; delayMax > 0 {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(delayMax)))
		time.Sleep(time.Duration(n.Int64()) * time.Millisecond)
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
		// 回滚排队计数，释放被浪费的名额，防止"幽灵用户"占满队列
		if _, decrErr := s.flashCache.DecrQueueCount(req.FlashSaleID); decrErr != nil {
			log.Printf("⚠️ 排队计数回滚失败（可能导致名额浪费）: flashSaleID=%d err=%v", req.FlashSaleID, decrErr)
		}
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
// 流程：验证码 → 限流 → 校验资格 → 内存售罄检查 → Redis Lua原子扣减 → 同步DB写入 → 返回
func (s *flashService) SnatchFlashSale(userID uint, req *request.FlashSnatchRequest) (*response.FlashSnatchResponse, error) {
	// 第1层：per-user 限流检查（放在验证码消费之前，避免触发限流时白白浪费验证码）
	if s.userLimiter != nil && !s.userLimiter.Allow(userID) {
		return &response.FlashSnatchResponse{Success: false, Message: "操作过于频繁，请稍后再试"}, nil
	}

	// 第2层：验证码校验（人机验证，一次性使用）
	expectedAnswer, err := s.flashCache.GetAndDeleteCaptcha(req.CaptchaID)
	if err != nil || expectedAnswer == "" {
		return nil, errors.New(errors.CodeParamError, "验证码已过期，请刷新后重试")
	}
	if strings.ToUpper(strings.TrimSpace(req.CaptchaAnswer)) != strings.ToUpper(expectedAnswer) {
		return nil, errors.New(errors.CodeParamError, "验证码错误")
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

	// 第3层：Redis 全局售罄标记（多实例共享，1次 GET）
	if soldOut, err := s.flashCache.IsFlashSoldOut(req.FlashSaleID); err == nil && soldOut {
		return &response.FlashSnatchResponse{Success: false, Message: "秒杀商品已售罄"}, nil
	}

	// 本地内存售罄检查（纯内存优化，~30ns，减少无效 Redis 调用）
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
		// 库存不足 → 设置全局 + 本地售罄标记
		s.flashCache.SetFlashSoldOut(req.FlashSaleID) // Redis 全局（多实例即时可见）
		s.inventory.Init(req.FlashSaleID, 0)           // 本地内存优化
		return &response.FlashSnatchResponse{Success: false, Message: "秒杀商品已售罄"}, nil
	case 2:
		return &response.FlashSnatchResponse{Success: false, Message: "您已参与过该秒杀"}, nil
	case 3:
		return nil, errors.New(errors.CodeFlashSaleNotStarted, "秒杀活动未预热，请稍后再试")
	}

	// code==0：扣减成功！立即生成订单号并写防丢失记录（提到最前，缩崩溃窗口）
	orderNo := s.generateOrderNo()
	if err := s.flashCache.SetPendingOrder(req.FlashSaleID, userID, orderNo); err != nil {
		log.Printf("⚠️ 设置防丢失记录失败: %v", err)
		// 不阻断流程，继续
	}

	// 更新本地内存计数和价格
	s.inventory.Decrement(req.FlashSaleID)
	productPrice := flash.FlashPrice

	// 同步写DB
	err = s.createFlashOrderInDB(flash, userID, orderNo, productPrice)
	if err != nil {
		// 判断是否为重复下单（UNIQUE约束冲突）—— 此时不应回滚Redis
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
			log.Printf("⚠️ 用户重复下单（已由DB唯一约束拦截）: userID=%d flashSaleID=%d", userID, req.FlashSaleID)
			return &response.FlashSnatchResponse{Success: false, Message: "您已参与过该秒杀"}, nil
		}
		// 其他DB写入失败 → 回滚Redis
			log.Printf("❌ 秒杀订单DB写入失败，回滚Redis: userID=%d flashSaleID=%d err=%v", userID, req.FlashSaleID, err)
			rollbackErr := s.flashCache.RollbackDeduct(req.FlashSaleID, userID)
			if rollbackErr != nil {
				// 回滚失败 → 保留 pending，由 RecoverPendingOrders 兜底补偿
				log.Printf("🚨 回滚Redis失败，保留pending等待补偿: userID=%d flashSaleID=%d rollbackErr=%v", userID, req.FlashSaleID, rollbackErr)
				return nil, errors.New(errors.CodeServerError, "订单创建失败，请稍后重试")
			}
			// 回滚成功 → 正常清理
			s.flashCache.MarkPendingRolledBack(req.FlashSaleID, userID)
			s.inventory.Increment(req.FlashSaleID)
			s.flashCache.DeleteFlashSoldOut(req.FlashSaleID) // 释放库存后清除售罄标记
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
// 使用命名返回值 err，确保 panic 恢复后调用方能感知到错误
func (s *flashService) createFlashOrderInDB(flash *entity.FlashSale, userID uint, orderNo string, price float64) (err error) {
	tx := s.db.Begin()
	if tx.Error != nil {
		return errors.Wrap(tx.Error, "开启事务失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = fmt.Errorf("创建订单事务发生panic: %v", r)
			log.Printf("❌ 创建秒杀订单panic（已回滚事务）: %v", r)
		}
	}()

	// DB 层防重复兜底：orders 表有 UNIQUE(flash_sale_id, user_id) 约束
	// INSERT 冲突 → 自动拒绝重复（MySQL 内核保证原子性），无需应用层 COUNT 判断
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
		// 唯一约束冲突 = 用户已抢过或重复提交，静默回滚 Redis
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

// GetUserFlashOrders 获取用户秒杀订单列表（批量查询优化，消除 N+1）
func (s *flashService) GetUserFlashOrders(userID uint) ([]*response.FlashOrderResponse, error) {
	orders, err := s.orderRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	// 过滤秒杀订单
	var flashOrders []*entity.Order
	var orderIDs []uint
	for _, o := range orders {
		if o.FlashSaleID != nil {
			flashOrders = append(flashOrders, o)
			orderIDs = append(orderIDs, o.ID)
		}
	}

	if len(flashOrders) == 0 {
		return []*response.FlashOrderResponse{}, nil
	}

	// 批量查询订单项（1次 SQL）
	allItems, err := s.orderItemRepo.GetByOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}

	// 构建 orderID → orderItem 映射 + 收集 productID
	orderItemMap := make(map[uint]*entity.OrderItem, len(allItems))
	var productIDs []uint
	for _, item := range allItems {
		productIDs = append(productIDs, item.ProductID)
		if _, ok := orderItemMap[item.OrderID]; !ok {
			orderItemMap[item.OrderID] = item
		}
	}

	// 批量查询商品（1次 SQL）
	productMap := make(map[uint]string, len(productIDs))
	if len(productIDs) > 0 {
		products, err := s.productRepo.GetByIDs(productIDs)
		if err != nil {
			return nil, err
		}
		for _, p := range products {
			productMap[p.ID] = p.Name
		}
	}

	// 组装响应
	var resp []*response.FlashOrderResponse
	for _, o := range flashOrders {
		productName := "秒杀商品"
		if item, ok := orderItemMap[o.ID]; ok {
			if name, ok := productMap[item.ProductID]; ok {
				productName = name
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

// GenerateCaptcha 生成验证码图片并存储答案到 Redis（2分钟有效）
func (s *flashService) GenerateCaptcha() (*captcha.Captcha, error) {
	c, err := captcha.Generate()
	if err != nil {
		return nil, errors.Wrap(err, "验证码生成失败")
	}
	if err := s.flashCache.SetCaptcha(c.ID, c.Answer, 2*time.Minute); err != nil {
		return nil, errors.Wrap(err, "验证码存储失败")
	}
	return c, nil
}

// ==================== 定时任务 ====================

// AutoWarmUp 自动预热即将开始的秒杀活动（未来30秒内开始的未预热活动）
func (s *flashService) AutoWarmUp() {
	var upcoming []entity.FlashSale
	now := time.Now()
	window := now.Add(30 * time.Second)

	s.db.Where("status = 0 AND start_time <= ? AND start_time >= ?", window, now.Add(-10*time.Second)).
		Find(&upcoming)

	if len(upcoming) > 0 {
		log.Printf("🔥 自动预热扫描：发现 %d 个即将开始的秒杀活动", len(upcoming))
	}

	for _, f := range upcoming {
		log.Printf("🔥 自动预热秒杀活动: id=%d product_id=%d stock=%d", f.ID, f.ProductID, f.FlashStock)
		if err := s.WarmUpFlashSale(f.ID); err != nil {
			log.Printf("⚠️ 自动预热失败: id=%d err=%v", f.ID, err)
		}
	}
}

// scanMutex 防止 ScanExpiredOrders 并发执行
var scanMutex sync.Mutex

// ScanExpiredOrders 扫描超时未支付的秒杀订单
// 每30秒执行一次（由 main.go 中的 goroutine 驱动）
func (s *flashService) ScanExpiredOrders() {
	if !scanMutex.TryLock() {
		return // 上一次扫描还未完成，跳过
	}
	defer scanMutex.Unlock()

	timeoutThreshold := time.Now().Add(-s.getPaymentTimeout())
	cooldownThreshold := time.Now().Add(-s.getCooldown())

	// === 第1步：将超时订单标记为"待释放"（status: 0→3），分批处理防止OOM ===
	var expiredOrders []entity.Order
	s.db.Model(&entity.Order{}).
		Where("flash_sale_id IS NOT NULL AND status = 0 AND created_at < ?", timeoutThreshold).
		Limit(1000).
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

	// === 第2步：处理冷却期满的订单（status: 3→2，释放库存），分批处理 ===
	var coolingOrders []entity.Order
	s.db.Model(&entity.Order{}).
		Where("flash_sale_id IS NOT NULL AND status = 3 AND updated_at < ?", cooldownThreshold).
		Limit(1000).
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
		if err := s.flashCache.RemoveAdmittedUser(fid, o.UserID); err != nil {
			log.Printf("⚠️ 释放入场资格失败: orderID=%d err=%v", o.ID, err)
		}

		// 重置本地内存售罄标记
		s.inventory.ResetSoldOut(fid)
		// 清除全局售罄标记（失败重试3次，兜底由 healSoldOutFlag 保证）
		for retry := 0; retry < 3; retry++ {
			if err := s.flashCache.DeleteFlashSoldOut(fid); err == nil {
				break
			}
			if retry == 2 {
				log.Printf("⚠️ 清除售罄标记失败（已重试3次）: flashSaleID=%d", fid)
			}
		}

		log.Printf("🔄 秒杀库存已释放: orderID=%d flashSaleID=%d userID=%d", o.ID, fid, o.UserID)
	}
}

// RecoverPendingOrders 崩溃恢复：扫描Redis中的防丢失记录，补建丢失的DB订单
// GuardConsistency 一致性守护：崩溃恢复 + 库存对账 + 幽灵用户检测 + 售罄标记校验
// 每5秒由 main.go 定时触发
func (s *flashService) GuardConsistency() {
	s.RecoverPendingOrders()
	s.ReconcileStock()
	s.purgeGhostPurchasers()   // 抽样清理已购集合中的幽灵用户
	s.healSoldOutFlag()        // 校验售罄标记与真实库存的一致性
}

// purgeGhostPurchasers 从已购集合随机抽取用户，比对DB订单，移除无订单的幽灵用户
func (s *flashService) purgeGhostPurchasers() {
	flashes, err := s.flashRepo.ListActive()
	if err != nil {
		return
	}
	for _, f := range flashes {
		// 每轮每活动最多抽样20个，避免影响性能
		sample, err := s.flashCache.GetRandomPurchasedUsers(f.ID, 20)
		if err != nil || len(sample) == 0 {
			continue
		}
		for _, uidStr := range sample {
			uid, err := strconv.ParseUint(uidStr, 10, 64)
			if err != nil {
				continue
			}
			// 检查 DB 中是否有该用户的已支付或待支付秒杀订单
			var count int64
			s.db.Model(&entity.Order{}).
				Where("user_id = ? AND flash_sale_id = ? AND status IN (0, 1, 3)", uid, f.ID).
				Count(&count)
			if count == 0 {
				// 幽灵用户：在已购集合中但无有效订单 → 回滚Redis已购资格
				s.flashCache.RollbackDeduct(f.ID, uint(uid))
				log.Printf("👻 幽灵用户已清理: flashSaleID=%d userID=%d", f.ID, uid)
			}
		}
	}
}

// healSoldOutFlag 校验售罄标记：Redis 有库存但标记仍为售罄 → 删除标记
func (s *flashService) healSoldOutFlag() {
	flashes, err := s.flashRepo.ListActive()
	if err != nil {
		return
	}
	for _, f := range flashes {
		remaining, err := s.flashCache.GetRemainingStock(f.ID)
		if err != nil {
			continue
		}
		if remaining > 0 {
			if soldOut, _ := s.flashCache.IsFlashSoldOut(f.ID); soldOut {
				s.flashCache.DeleteFlashSoldOut(f.ID)
				s.inventory.ResetSoldOut(f.ID)
				log.Printf("🩹 售罄标记自愈: flashSaleID=%d 实际库存=%d 已清除错误标记", f.ID, remaining)
			}
		}
	}
}

// RecoverPendingOrders 崩溃恢复：扫描防丢失记录和已购追踪列表，补建丢失订单
// 在 main.go 启动时调用
func (s *flashService) RecoverPendingOrders() {
	log.Println("🔍 开始扫描秒杀防丢失记录...")

	keys, err := s.flashCache.ScanAllPendingKeys()
	if err != nil {
		log.Printf("⚠️ 扫描防丢失记录失败: %v", err)
	} else if len(keys) > 0 {
		log.Printf("📋 发现 %d 个待恢复的活动", len(keys))
		recovered := 0
		rolledBack := 0

		for _, key := range keys {
			var flashSaleID uint
			if n, _ := fmt.Sscanf(key, "flash:pending:%d", &flashSaleID); n != 1 {
				log.Printf("⚠️ 跳过格式异常的防丢失记录 key: %s", key)
				continue
			}

			pendingOrders, err := s.flashCache.GetPendingOrders(flashSaleID)
			if err != nil {
				log.Printf("⚠️ 读取防丢失记录失败: key=%s err=%v", key, err)
				continue
			}

			for userIDStr, rawValue := range pendingOrders {
				userID, _ := strconv.ParseUint(userIDStr, 10, 64)
				// 解析 pending 记录格式: "orderNo|status"（兼容旧格式）
				orderNo, status := parsePendingValue(rawValue)

				// 已回滚的记录 → 直接清理，不重建订单（防止场景C/E误重建）
				if status == "rolled_back" {
					s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
					log.Printf("  ✓ 已回滚记录已清理: orderNo=%s userID=%d", orderNo, userID)
					continue
				}

				// 检查DB中是否已存在该订单
				existing, err := s.orderRepo.GetByOrderNo(orderNo)
				if err == nil && existing != nil {
					s.flashCache.RemovePendingOrder(flashSaleID, uint(userID))
					log.Printf("  ✓ 订单已存在于DB，清理Redis: orderNo=%s", orderNo)
					continue
				}

				// DB中不存在 → 补建订单（仅 confirmed 状态）
				flash, err := s.flashRepo.GetByID(flashSaleID)
				if err != nil {
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

	// === 第二阶段：扫描 deduct track 列表，修复场景A ===
	// 在 Lua 扣减成功后、SetPendingOrder 之前崩溃的用户，
	// 他们在 Redis 已购集合中但没有 pending 记录也没有 DB 订单
	activeFlashes, err := s.flashRepo.ListActive()
	if err != nil {
		log.Printf("⚠️ 获取活动列表失败，跳过 deduct track 扫描: %v", err)
		return
	}
	for _, f := range activeFlashes {
		userIDs, err := s.flashCache.GetDeductTrackUsers(f.ID)
		if err != nil || len(userIDs) == 0 {
			continue
		}
		for _, uidStr := range userIDs {
			uid, err := strconv.ParseUint(uidStr, 10, 64)
			if err != nil {
				continue
			}
			// 检查是否有对应的 pending 记录（有 pending 则第一阶段的 pending 扫描已处理）
			pendingOrders, _ := s.flashCache.GetPendingOrders(f.ID)
			if _, hasPending := pendingOrders[uidStr]; hasPending {
				continue // 已被 pending 扫描处理
			}
			// 检查 DB 中是否已有该用户的秒杀订单
			var count int64
			s.db.Model(&entity.Order{}).
				Where("user_id = ? AND flash_sale_id = ? AND status IN (0, 1, 3)", uid, f.ID).
				Count(&count)
			if count > 0 {
				continue // DB 中已有订单
			}
			// 用户已在 Redis 已购集合中但无订单 → 补建
			orderNo := s.generateOrderNo()
			err = s.createFlashOrderInDB(f, uint(uid), orderNo, f.FlashPrice)
			if err != nil {
				log.Printf("  ⚠ deduct track 补建失败（回滚Redis）: flashSaleID=%d userID=%d err=%v", f.ID, uid, err)
				s.flashCache.RollbackDeduct(f.ID, uint(uid))
			} else {
				log.Printf("  ✅ deduct track 补建订单成功（场景A恢复）: orderNo=%s userID=%d flashSaleID=%d", orderNo, uid, f.ID)
			}
		}
	}
}

// parsePendingValue 解析 pending 记录的存储格式，返回 (orderNo, status)
// 兼容旧格式（无状态字段，默认视为 confirmed）
func parsePendingValue(raw string) (orderNo, status string) {
	parts := strings.SplitN(raw, "|", 2)
	orderNo = parts[0]
	if len(parts) == 2 {
		status = parts[1]
	} else {
		status = "confirmed" // 旧格式兼容
	}
	return
}

// ReconcileStock 对账任务：比较 Redis 扣减数与 DB 订单数，发现不一致则告警
// 每小时执行一次（由 main.go 中的 goroutine 驱动）
func (s *flashService) ReconcileStock() {
	// 获取所有进行中的秒杀活动
	flashes, err := s.flashRepo.ListActive()
	if err != nil {
		log.Printf("⚠️ 对账-查询活动列表失败: %v", err)
		return
	}

	for _, f := range flashes {
		// Redis 已扣减数 = 原始库存 - Redis 剩余
		redisRemaining, err := s.flashCache.GetRemainingStock(f.ID)
		if err != nil {
			log.Printf("⚠️ 对账-活动%d Redis查询失败: %v", f.ID, err)
			continue
		}
		redisSold := int64(f.FlashStock) - redisRemaining

		// DB 成功订单数
		var dbSold int64
		s.db.Model(&entity.Order{}).
			Where("flash_sale_id = ? AND status IN (0, 1, 3)", f.ID).
			Count(&dbSold)

	// 自动修复：Redis 还有货但本地内存误标售罄 → 重置
	if redisRemaining > 0 && s.inventory.IsSoldOut(f.ID) {
		s.inventory.ResetSoldOut(f.ID)
		log.Printf("🔧 自愈-活动%d: 本地售罄标记已重置（Redis剩余=%d）", f.ID, redisRemaining)
	}

	if redisSold != dbSold {
			deviation := redisSold - dbSold
			log.Printf("🚨 对账异常！活动%d: Redis已售=%d DB已售=%d 偏差=%d", f.ID, redisSold, dbSold, deviation)

			if deviation > 0 {
				// Redis 多扣了 → 检查pending，无pending则修正Redis
				pendingOrders, _ := s.flashCache.GetPendingOrders(f.ID)
				if len(pendingOrders) > 0 {
					log.Printf("🔧 对账-活动%d: 存在%d条pending，等待RecoverPendingOrders补偿", f.ID, len(pendingOrders))
				} else {
					correctedStock := int64(f.FlashStock) - dbSold
					if correctedStock >= 0 {
						// 使用 CAS 原子设置，防止与并发抢购的 DECR 冲突（修复场景G）
						code, actualVal, err := s.flashCache.AtomicSetStock(f.ID, redisRemaining, correctedStock)
						if err != nil {
							log.Printf("⚠️ 对账-活动%d: CAS修正失败: %v", f.ID, err)
						} else if code == 0 {
							log.Printf("⚠️ 对账-活动%d: CAS冲突（期间有并发扣减），放弃本轮修正，当前值=%d", f.ID, actualVal)
						} else {
							s.inventory.Init(f.ID, int(correctedStock))
							s.flashCache.DeleteFlashSoldOut(f.ID)
							log.Printf("🔧 自愈-活动%d: Redis库存CAS修正为%d，已清除售罄标记", f.ID, correctedStock)
						}
					}
				}
			} else if deviation < 0 {
				// DB 多记录了 → 修正Redis计数器为DB值
				correctedStock := int64(f.FlashStock) - dbSold
				if correctedStock >= 0 {
					code, actualVal, err := s.flashCache.AtomicSetStock(f.ID, redisRemaining, correctedStock)
					if err != nil {
						log.Printf("⚠️ 对账-活动%d: CAS修正失败: %v", f.ID, err)
					} else if code == 0 {
						log.Printf("⚠️ 对账-活动%d: CAS冲突（期间有并发扣减），放弃本轮修正，当前值=%d", f.ID, actualVal)
					} else {
						s.inventory.Init(f.ID, int(correctedStock))
						log.Printf("🔧 自愈-活动%d: DB已售>Redis已售，Redis库存CAS修正为%d", f.ID, correctedStock)
					}
				}
			}
		} else {
			log.Printf("✅ 对账通过 活动%d: 已售=%d", f.ID, redisSold)
		}
	}
}
