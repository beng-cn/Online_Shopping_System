package response

import "time"

// 商品信息响应
type ProductResponse struct {
	ID         uint      `json:"id"`
	CategoryID uint      `json:"category_id"`
	Name       string    `json:"name"`
	Keywords   string    `json:"keywords"`
	Price      float64   `json:"price"`
	Stock      int       `json:"stock"`
	Image      string    `json:"image"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Sales      int       `json:"sales"`
}
