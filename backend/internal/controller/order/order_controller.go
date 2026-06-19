package order

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/order"
	"backend/internal/service/payment"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	orderService  order.OrderService
	alipayService payment.AlipayService
}

// NewOrderController 创建订单控制器实例
func NewOrderController(
	orderService order.OrderService,
	alipayService payment.AlipayService, // 新增参数
) *OrderController {
	return &OrderController{
		orderService:  orderService,
		alipayService: alipayService, // 赋值
	}
}

// 创建订单
func (c *OrderController) CreateOrder(ctx *gin.Context) {
	var req request.CreateOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	resp, err := c.orderService.CreateOrder(userID, &req)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 获取订单列表
func (c *OrderController) GetOrderList(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")
	resp, err := c.orderService.GetOrderList(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 支付宝统一下单
func (c *OrderController) AliPayUnifiedOrder(ctx *gin.Context) {
	var req request.AliPayUnifiedOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	payURL, err := c.orderService.GetAliPayURL(req.OrderID, userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{"pay_url": payURL})
}

// 获取订单项
func (c *OrderController) GetOrderItems(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("订单ID无效"))
		return
	}

	userID := ctx.GetUint("user_id")
	resp, err := c.orderService.GetOrderItems(uint(id), userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 删除订单
func (c *OrderController) DeleteOrder(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("订单ID无效"))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.orderService.DeleteOrder(uint(id), userID); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 支付宝异步回调接口
func (c *OrderController) AliPayNotify(ctx *gin.Context) {
	// 解析并验证回调签名
	noti, err := c.alipayService.ParseNotify(ctx.Request)
	if err != nil {
		ctx.String(http.StatusBadRequest, "fail")
		return
	}

	// 只处理支付成功的状态
	if noti.TradeStatus != "TRADE_SUCCESS" && noti.TradeStatus != "TRADE_FINISHED" {
		ctx.String(http.StatusOK, "success")
		return
	}

	// 根据商户订单号查询订单
	order, err := c.orderService.GetOrderByOrderNo(noti.OutTradeNo)
	if err != nil {
		ctx.String(http.StatusBadRequest, "fail")
		return
	}

	// 幂等性处理：订单已支付直接返回成功
	if order.Status == 1 {
		ctx.String(http.StatusOK, "success")
		return
	}

	// 调用公共支付处理逻辑
	if err := c.orderService.ProcessOrderPayment(order.ID); err != nil {
		ctx.String(http.StatusInternalServerError, "fail")
		return
	}

	// 必须返回"success"，否则支付宝会持续回调
	ctx.String(http.StatusOK, "success")
}

// 支付宝同步跳转接口
func (c *OrderController) AliPayReturn(ctx *gin.Context) {
	// 1. 验证签名（关键步骤，防止伪造请求）
	if err := c.alipayService.VerifyReturnSign(ctx.Request); err != nil {
		response.Error(ctx, errors.New(errors.CodeParamError, "支付验证失败"))
		return
	}

	// 2. 获取订单号
	orderNo := ctx.Query("out_trade_no")

	// 3. 查询订单
	order, err := c.orderService.GetOrderByOrderNo(orderNo)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	// 4. 幂等性处理：订单已支付直接返回
	if order.Status == 1 {
		response.Success(ctx, gin.H{
			"message":  "订单已支付",
			"order_id": order.ID,
			"order_no": order.OrderNo,
		})
		return
	}

	// 5. 处理订单支付（本地开发核心，生产环境靠异步回调）
	if err := c.orderService.ProcessOrderPayment(order.ID); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{
		"message":  "支付成功",
		"order_id": order.ID,
		"order_no": order.OrderNo,
	})
}
