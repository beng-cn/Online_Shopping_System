package entity

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	Name      string         `gorm:"not null" json:"name"`
	ParentID  uint           `gorm:"default:0;index:idx_category_parent_status,priority:1" json:"parent_id"`
	Status    int            `gorm:"default:1;index:idx_category_parent_status,priority:2" json:"status"`
	Products  []Product      `gorm:"foreignKey:CategoryID" json:"-"`
}
