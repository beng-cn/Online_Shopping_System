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

// GetCartList 获取购物车列表
// @Summary 获取购物车列表
// @Description 获取当前登录用户的购物车商品列表，包含商品信息和数量
// @Tags 购物车
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} response.Response "查询成功"
// @Router /auth/cart/list [get]
func (c *CartController) GetCartList(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")
	resp, err := c.cartService.GetCartList(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// AddToCart 添加到购物车
// @Summary 添加商品到购物车
// @Description 将指定商品添加到当前用户的购物车中，已存在则累加数量
// @Tags 购物车
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body request.AddToCartRequest true "添加购物车请求体"
// @Success 200 {object} response.Response "添加成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /auth/cart/add [post]
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

// UpdateCartQuantity 修改购物车数量
// @Summary 修改购物车商品数量
// @Description 修改当前用户购物车中指定商品的数量
// @Tags 购物车
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "购物车项ID"
// @Param body body request.UpdateCartQuantityRequest true "更新数量请求体"
// @Success 200 {object} response.Response "修改成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /auth/cart/{id} [put]
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

// DeleteCartItem 删除购物车商品
// @Summary 删除购物车商品
// @Description 从当前用户购物车中删除指定商品
// @Tags 购物车
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "购物车项ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /auth/cart/{id} [delete]
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
