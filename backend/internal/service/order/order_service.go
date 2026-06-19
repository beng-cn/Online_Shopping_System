package order

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"backend/internal/service/payment"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type OrderService interface {
	CreateOrder(userID uint, req *request.CreateOrderRequest) (*response.OrderResponse, error)
	PayOrder(orderID uint, userID uint) error
	ProcessOrderPayment(orderID uint) error // 公共支付处理逻辑
	GetOrderList(userID uint) ([]*response.OrderResponse, error)
	GetOrderItems(orderID uint, userID uint) ([]*response.OrderItemResponse, error)
	DeleteOrder(orderID uint, userID uint) error
	GetAliPayURL(orderID uint, userID uint) (string, error)
	GetOrderByOrderNo(orderNo string) (*entity.Order, error)
}

type orderService struct {
	db            *gorm.DB
	orderRepo     mysql.OrderRepository
	orderItemRepo mysql.OrderItemRepository
	cartRepo      mysql.CartRepository
	productRepo   mysql.ProductRepository
	productCache  redis.ProductCache
	alipayService payment.AlipayService
}

// NewOrderService 创建订单服务实例
func NewOrderService(
	db *gorm.DB,
	orderRepo mysql.OrderRepository,
	orderItemRepo mysql.OrderItemRepository,
	cartRepo mysql.CartRepository,
	productRepo mysql.ProductRepository,
	productCache redis.ProductCache,
	alipayService payment.AlipayService,
) OrderService {
	return &orderService{
		db:            db,
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		cartRepo:      cartRepo,
		productRepo:   productRepo,
		productCache:  productCache,
		alipayService: alipayService,
	}
}

// CreateOrder 从购物车创建订单，含库存乐观锁扣减和事务保护
func (s *orderService) CreateOrder(userID uint, req *request.CreateOrderRequest) (*response.OrderResponse, error) {
	// 开启事务
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, errors.Wrap(tx.Error, "开启事务失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. 查询用户选择的购物车商品
	carts, err := s.cartRepo.WithTx(tx).GetByIDsAndUserID(req.CartIDs, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(carts) == 0 {
		tx.Rollback()
		return nil, errors.New(errors.CodeParamError, "未选择有效商品")
	}

	// 2. 预校验库存并计算总金额
	var total float64
	var orderItems []*entity.OrderItem
	for _, cart := range carts {
		product, err := s.productRepo.WithTx(tx).GetByID(cart.ProductID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if product.Status != 1 {
			tx.Rollback()
			return nil, errors.Errorf(errors.CodeProductNotFound, "商品《%s》已下架", product.Name)
		}
		if product.Stock < cart.Quantity {
			tx.Rollback()
			return nil, errors.Errorf(errors.CodeStockInsufficient, "商品《%s》库存不足，剩余%d件", product.Name, product.Stock)
		}
		total += product.Price * float64(cart.Quantity)
		orderItems = append(orderItems, &entity.OrderItem{
			ProductID: cart.ProductID,
			Quantity:  cart.Quantity,
			Price:     product.Price,
		})
	}

	// 3. 生成唯一订单号（时间戳 + 8位随机十六进制，防止高并发碰撞）
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	orderNo := fmt.Sprintf("ORD%s%s", time.Now().Format("20060102150405"), hex.EncodeToString(randomBytes))

	// 4. 创建订单
	order := &entity.Order{
		UserID:  userID,
		OrderNo: orderNo,
		Total:   total,
		Status:  0,
	}
	if err := s.orderRepo.WithTx(tx).Create(order); err != nil {
		tx.Rollback()
		return nil, err
	}

	// 5. 批量创建订单项
	for i := range orderItems {
		orderItems[i].OrderID = order.ID
	}
	if err := s.orderItemRepo.WithTx(tx).BatchCreate(orderItems); err != nil {
		tx.Rollback()
		return nil, err
	}

	// 6. 删除已下单的购物车商品
	if err := s.cartRepo.WithTx(tx).DeleteByIDs(req.CartIDs); err != nil {
		tx.Rollback()
		return nil, err
	}

	// 7. 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, errors.Wrap(err, "提交事务失败")
	}

	return &response.OrderResponse{
		ID:        order.ID,
		OrderNo:   order.OrderNo,
		Total:     order.Total,
		Status:    order.Status,
		CreatedAt: order.CreatedAt,
	}, nil
}

// PayOrder 订单支付处理，校验归属并更新状态为已支付
func (s *orderService) PayOrder(orderID uint, userID uint) error {
	// 校验订单是否属于当前用户
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return errors.New(errors.CodeForbidden, "无权支付他人的订单")
	}

	// 校验订单状态
	if order.Status == 1 {
		return errors.New(errors.CodeOrderAlreadyPaid, "订单已支付")
	}
	if order.Status == 2 {
		return errors.New(errors.CodeOrderCancelled, "订单已取消")
	}

	// 调用公共支付处理逻辑
	return s.ProcessOrderPayment(orderID)
}

// 根据订单号查询订单
func (s *orderService) GetOrderByOrderNo(orderNo string) (*entity.Order, error) {
	order, err := s.orderRepo.GetByOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// ProcessOrderPayment 处理订单支付（公共方法，被 PayOrder、AliPayNotify、AliPayReturn 调用）
// 使用条件UPDATE防止与秒杀超时扫描器竞态
func (s *orderService) ProcessOrderPayment(orderID uint) error {
	// 开启事务
	tx := s.db.Begin()
	if tx.Error != nil {
		return errors.Wrap(tx.Error, "开启事务失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. 查询订单并做状态幂等校验
	order, err := s.orderRepo.WithTx(tx).GetByID(orderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 幂等性处理：已支付直接返回成功
	if order.Status == 1 {
		tx.Rollback()
		return nil
	}
	// 已取消的订单不可支付（status=2为普通取消，status=3为秒杀冷却期可支付）
	if order.Status == 2 {
		tx.Rollback()
		return errors.New(errors.CodeOrderCancelled, "订单已取消")
	}
	// status=0(待支付)或status=3(秒杀冷却期)均可支付

	// 2. 查询订单项
	orderItems, err := s.orderItemRepo.WithTx(tx).GetByOrderID(orderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 3. 乐观锁扣减库存
	for _, item := range orderItems {
		for retry := 0; retry < 3; retry++ {
			product, err := s.productRepo.WithTx(tx).GetByID(item.ProductID)
			if err != nil {
				tx.Rollback()
				return err
			}
			if product.Stock < item.Quantity {
				tx.Rollback()
				return errors.Errorf(errors.CodeStockInsufficient, "商品《%s》库存不足，剩余%d件", product.Name, product.Stock)
			}
			// 乐观锁更新库存
			success, err := s.productRepo.WithTx(tx).DecreaseStockWithVersion(product.ID, item.Quantity, product.Version)
			if err != nil {
				tx.Rollback()
				return err
			}
			if success {
				if err := s.productRepo.WithTx(tx).IncreaseSales(item.ProductID, item.Quantity); err != nil {
					tx.Rollback()
					return err
				}
				// 清理缓存
				s.productCache.DeleteProduct(product.ID)
				break
			}
			if retry == 2 {
				tx.Rollback()
				return errors.New(errors.CodeServerError, "系统繁忙，请稍后重试")
			}
		}
	}

	// 4. 条件更新订单状态为已支付
	// WHERE status IN(0,3) 确保不会覆盖已被超时扫描器取消的订单
	result := tx.Model(&entity.Order{}).
		Where("id = ? AND status IN (0, 3)", orderID).
		Update("status", 1)

	if result.Error != nil {
		tx.Rollback()
		return errors.Wrap(result.Error, "更新订单状态失败")
	}

	if result.RowsAffected == 0 {
		// 状态已被并发修改（超时扫描器可能已将status从0改为3或2）
		tx.Rollback()
		// 重新查询确认最终状态
		current, _ := s.orderRepo.GetByID(orderID)
		if current != nil && current.Status == 1 {
			return nil // 已被其他支付流程处理，幂等返回
		}
		return errors.New(errors.CodeOrderCancelled, "订单已超时取消")
	}

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		return errors.Wrap(err, "提交事务失败")
	}

	// 6. 清理商品列表缓存
	s.productCache.ClearAllProductList()

	return nil
}

// GetOrderList 分页获取用户订单列表
func (s *orderService) GetOrderList(userID uint) ([]*response.OrderResponse, error) {
	orders, err := s.orderRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	var resp []*response.OrderResponse
	for _, order := range orders {
		resp = append(resp, &response.OrderResponse{
			ID:        order.ID,
			OrderNo:   order.OrderNo,
			Total:     order.Total,
			Status:    order.Status,
			CreatedAt: order.CreatedAt,
		})
	}
	return resp, nil
}

// GetOrderItems 获取订单项详情（优化：批量查询商品，消除 N+1 问题）
func (s *orderService) GetOrderItems(orderID uint, userID uint) ([]*response.OrderItemResponse, error) {
	// 校验订单是否属于当前用户
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, errors.New(errors.CodeForbidden, "无权查看他人的订单")
	}

	// 查询订单项
	items, err := s.orderItemRepo.GetByOrderID(orderID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return []*response.OrderItemResponse{}, nil
	}

	// 收集所有商品ID，批量查询（避免 N+1）
	productIDs := make([]uint, len(items))
	for i, item := range items {
		productIDs[i] = item.ProductID
	}
	products, err := s.productRepo.GetByIDs(productIDs)
	if err != nil {
		return nil, err
	}

	// 构建 ID→商品 映射
	productMap := make(map[uint]*entity.Product, len(products))
	for _, p := range products {
		productMap[p.ID] = p
	}

	// 转换为响应DTO
	var resp []*response.OrderItemResponse
	for _, item := range items {
		product, ok := productMap[item.ProductID]
		name := "未知商品"
		if ok {
			name = product.Name
		}
		resp = append(resp, &response.OrderItemResponse{
			Name:     name,
			Price:    item.Price,
			Quantity: item.Quantity,
		})
	}

	return resp, nil
}

// DeleteOrder 软删除订单（仅待支付状态可删）
func (s *orderService) DeleteOrder(orderID uint, userID uint) error {
	// 校验订单是否属于当前用户
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return errors.New(errors.CodeForbidden, "无权删除他人的订单")
	}

	// 开启事务
	tx := s.db.Begin()
	if tx.Error != nil {
		return errors.Wrap(tx.Error, "开启事务失败")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除订单项
	if err := s.orderItemRepo.WithTx(tx).DeleteByOrderID(orderID); err != nil {
		tx.Rollback()
		return err
	}

	// 删除订单
	if err := s.orderRepo.WithTx(tx).Delete(orderID); err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return errors.Wrap(err, "提交事务失败")
	}

	return nil
}

// GetAliPayURL 生成支付宝支付链接
func (s *orderService) GetAliPayURL(orderID uint, userID uint) (string, error) {
	// 校验订单是否属于当前用户
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return "", err
	}
	if order.UserID != userID {
		return "", errors.New(errors.CodeForbidden, "无权支付他人的订单")
	}

	// 校验订单状态
	if order.Status == 1 {
		return "", errors.New(errors.CodeOrderAlreadyPaid, "订单已支付")
	}
	if order.Status == 2 {
		return "", errors.New(errors.CodeOrderCancelled, "订单已取消")
	}

	// 生成支付宝支付链接
	return s.alipayService.GeneratePayURL(order)
}
