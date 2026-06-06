package entity

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `gorm:"index:idx_order_user_created,priority:2,sort:desc" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	UserID    uint           `gorm:"index:idx_order_user_created,priority:1" json:"user_id"`
	OrderNo   string         `gorm:"unique;not null" json:"order_no"`
	Total     float64        `json:"total"`
	Status    int            `gorm:"default:1;index" json:"status"`
}

type OrderItem struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	OrderID   uint           `gorm:"uniqueIndex:idx_order_item_order_product,priority:1;index" json:"order_id"`
	ProductID uint           `gorm:"uniqueIndex:idx_order_item_order_product,priority:2;index" json:"product_id"`
	Quantity  int            `json:"quantity"`
	Price     float64        `json:"price"`
}
