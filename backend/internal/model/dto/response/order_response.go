package response

import "time"

// 订单响应
type OrderResponse struct {
	ID        uint      `json:"id"`
	OrderNo   string    `json:"order_no"`
	Total     float64   `json:"total"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// 订单项响应
type OrderItemResponse struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// 支付宝支付响应
type AliPayResponse struct {
	PayURL string `json:"pay_url"`
}
