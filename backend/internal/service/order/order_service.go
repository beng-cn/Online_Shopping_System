package order

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"backend/internal/service/payment"
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

	// 3. 生成唯一订单号
	orderNo := fmt.Sprintf("ORD%s%d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)

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

// 在ProcessOrderPayment方法中，库存扣减成功后立即更新销量
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

	// 1. 查询订单
	_, err := s.orderRepo.WithTx(tx).GetByID(orderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 幂等性校验（省略原有代码）

	// 2. 查询订单项
	orderItems, err := s.orderItemRepo.WithTx(tx).GetByOrderID(orderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 3. 乐观锁扣减库存（省略原有代码）
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
				// ====================== 新增：原子更新商品销量 ======================
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

	// 4. 更新订单状态为已支付（省略原有代码）

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		return errors.Wrap(err, "提交事务失败")
	}

	// 6. 清理商品列表缓存（省略原有代码）
	s.productCache.ClearAllProductList()

	return nil
}

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

	// 查询商品信息并转换为响应DTO
	var resp []*response.OrderItemResponse
	for _, item := range items {
		product, err := s.productRepo.GetByID(item.ProductID)
		if err != nil {
			return nil, err
		}
		resp = append(resp, &response.OrderItemResponse{
			Name:     product.Name,
			Price:    item.Price,
			Quantity: item.Quantity,
		})
	}

	return resp, nil
}

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
