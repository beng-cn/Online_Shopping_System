package response

import "time"

// 购物车响应
type CartResponse struct {
	ID        uint      `json:"id"`
	ProductID uint      `json:"product_id"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
}
