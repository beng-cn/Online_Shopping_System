package entity

import (
	"time"

	"gorm.io/gorm"
)

type Cart struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	UserID    uint           `json:"user_id"`
	ProductID uint           `json:"product_id"`
	Quantity  int            `gorm:"not null" json:"quantity"`
}
