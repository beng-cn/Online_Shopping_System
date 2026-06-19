package cart

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/cart"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CartController struct {
	cartService cart.CartService
}

// NewCartController 创建购物车控制器实例
func NewCartController(cartService cart.CartService) *CartController {
	return &CartController{cartService: cartService}
}

// 获取购物车列表
func (c *CartController) GetCartList(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")
	resp, err := c.cartService.GetCartList(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 添加到购物车
func (c *CartController) AddToCart(ctx *gin.Context) {
	var req request.AddToCartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.cartService.AddToCart(userID, &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 修改购物车数量
func (c *CartController) UpdateCartQuantity(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("购物车ID无效"))
		return
	}

	var req request.UpdateCartQuantityRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.cartService.UpdateCartQuantity(userID, uint(id), req.Quantity); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 删除购物车商品
func (c *CartController) DeleteCartItem(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("购物车ID无效"))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.cartService.DeleteCartItem(userID, uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}
