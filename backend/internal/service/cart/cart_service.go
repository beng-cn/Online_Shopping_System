package cart

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/repository/mysql"
)

type CartService interface {
	AddToCart(userID uint, req *request.AddToCartRequest) error
	UpdateCartQuantity(userID uint, cartID uint, quantity int) error
	DeleteCartItem(userID uint, cartID uint) error
	GetCartList(userID uint) ([]*response.CartResponse, error)
}

type cartService struct {
	cartRepo    mysql.CartRepository
	productRepo mysql.ProductRepository
}

func NewCartService(
	cartRepo mysql.CartRepository,
	productRepo mysql.ProductRepository,
) CartService {
	return &cartService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
	}
}

func (s *cartService) AddToCart(userID uint, req *request.AddToCartRequest) error {
	// 校验商品是否存在且上架
	product, err := s.productRepo.GetByID(req.ProductID)
	if err != nil {
		return err
	}
	if product.Status != 1 {
		return errors.New(errors.CodeProductNotFound, "商品已下架")
	}
	if product.Stock <= 0 {
		return errors.New(errors.CodeStockInsufficient, "商品库存不足")
	}
	if req.Quantity > product.Stock {
		return errors.Errorf(errors.CodeStockInsufficient, "商品库存不足，剩余%d件", product.Stock)
	}

	// 检查购物车中是否已有该商品
	existingCart, err := s.cartRepo.GetByUserAndProduct(userID, req.ProductID)
	if err != nil {
		return err
	}

	if existingCart == nil {
		newCart := &entity.Cart{
			UserID:    userID,
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
		}
		return s.cartRepo.Create(newCart)
	} else {
		// 更新购物车数量
		newQuantity := existingCart.Quantity + req.Quantity
		if newQuantity > product.Stock {
			return errors.Errorf(errors.CodeStockInsufficient, "购物车中该商品已达库存上限，剩余%d件", product.Stock)
		}
		existingCart.Quantity = newQuantity
		return s.cartRepo.Update(existingCart)
	}
}

func (s *cartService) UpdateCartQuantity(userID uint, cartID uint, quantity int) error {
	// 校验购物车记录是否属于当前用户
	cart, err := s.cartRepo.GetByID(cartID)
	if err != nil {
		return err
	}
	if cart.UserID != userID {
		return errors.New(errors.CodeForbidden, "无权修改他人的购物车")
	}

	// 校验商品库存
	product, err := s.productRepo.GetByID(cart.ProductID)
	if err != nil {
		return err
	}
	if quantity > product.Stock {
		return errors.Errorf(errors.CodeStockInsufficient, "商品库存不足，剩余%d件", product.Stock)
	}

	cart.Quantity = quantity
	return s.cartRepo.Update(cart)
}

func (s *cartService) DeleteCartItem(userID uint, cartID uint) error {
	// 校验购物车记录是否属于当前用户
	cart, err := s.cartRepo.GetByID(cartID)
	if err != nil {
		return err
	}
	if cart.UserID != userID {
		return errors.New(errors.CodeForbidden, "无权删除他人的购物车")
	}

	return s.cartRepo.Delete(cartID)
}

func (s *cartService) GetCartList(userID uint) ([]*response.CartResponse, error) {
	carts, err := s.cartRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	var resp []*response.CartResponse
	for _, cart := range carts {
		resp = append(resp, &response.CartResponse{
			ID:        cart.ID,
			ProductID: cart.ProductID,
			Quantity:  cart.Quantity,
			CreatedAt: cart.CreatedAt,
		})
	}
	return resp, nil
}
