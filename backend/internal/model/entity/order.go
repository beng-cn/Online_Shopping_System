package entity

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	UserID    uint           `json:"user_id"`
	OrderNo   string         `gorm:"unique;not null" json:"order_no"`
	Total     float64        `json:"total"`
	Status    int            `gorm:"default:0" json:"status"` // 0待支付 1已支付 2已取消
}

type OrderItem struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	OrderID   uint           `json:"order_id"`
	ProductID uint           `json:"product_id"`
	Quantity  int            `json:"quantity"`
	Price     float64        `json:"price"`
}
