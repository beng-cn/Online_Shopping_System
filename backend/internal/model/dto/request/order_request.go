package request

// 创建订单请求
type CreateOrderRequest struct {
	CartIDs []uint `json:"cart_ids" binding:"required,min=1" msg:"请选择要下单的商品"`
}

// 支付宝统一下单请求
type AliPayUnifiedOrderRequest struct {
	OrderID uint `json:"order_id" binding:"required,min=1" msg:"订单ID不能为空"`
}
