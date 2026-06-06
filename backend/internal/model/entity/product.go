package entity

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	CategoryID uint           `gorm:"index:idx_product_category_status,priority:1" json:"category_id"`
	Name       string         `gorm:"not null;index:idx_product_name_like,length:20" json:"name"`
	Price      float64        `gorm:"not null" json:"price"`
	Stock      int            `gorm:"not null" json:"stock"`
	Image      string         `json:"image"`
	Status     int            `gorm:"default:1;index:idx_product_category_status,priority:2" json:"status"`
	Version    int            `gorm:"default:0" json:"-"`
	Category   Category       `gorm:"foreignKey:CategoryID" json:"category"`
	Sales      int            `gorm:"not null;default:0;index:idx_product_sales,sort:desc" json:"sales"`
}
