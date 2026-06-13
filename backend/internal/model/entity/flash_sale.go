package entity

import (
	"time"

	"gorm.io/gorm"
)

// FlashSale 秒杀活动实体（对应 flash_sales 表）
type FlashSale struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	ProductID  uint           `gorm:"not null;index:idx_flash_sale_product" json:"product_id"`
	FlashPrice float64        `gorm:"not null" json:"flash_price"`
	FlashStock int            `gorm:"not null" json:"flash_stock"`
	QueueCap   int            `gorm:"not null;default:0;comment:'排队入场上限，0表示按库存×10自动计算'" json:"queue_cap"`
	StartTime  time.Time      `gorm:"not null;index:idx_flash_sale_time_status,priority:1" json:"start_time"`
	EndTime    time.Time      `gorm:"not null;index:idx_flash_sale_time_status,priority:2" json:"end_time"`
	Status     int            `gorm:"not null;default:0;index:idx_flash_sale_time_status,priority:3;comment:'0=未开始 1=进行中 2=已结束 3=已取消'" json:"status"`
	Version    int            `gorm:"not null;default:0" json:"-"`
	Product    Product        `gorm:"foreignKey:ProductID" json:"product"`
}
