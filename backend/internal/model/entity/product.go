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
	CategoryID uint           `json:"category_id"`
	Name       string         `gorm:"not null" json:"name"`
	Price      float64        `gorm:"not null" json:"price"`
	Stock      int            `gorm:"not null" json:"stock"`
	Image      string         `json:"image"`
	Status     int            `gorm:"default:1" json:"status"` // 0下架 1上架
	Version    int            `gorm:"default:0" json:"-"`      // 乐观锁版本号
	Category   Category       `gorm:"foreignKey:CategoryID" json:"category"`
}
