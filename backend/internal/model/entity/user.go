package entity

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	Username  string         `gorm:"unique;not null;index:idx_user_username_like,length:10" json:"username"`
	Password  string         `gorm:"not null" json:"-"`
	AdminPin  string         `gorm:"type:varchar(255);comment:'管理员PIN码，bcrypt加密，非管理员为NULL'" json:"-"`
	Nickname  string         `json:"nickname"`
	Email     string         `json:"email"`
	Phone     string         `json:"phone"`
	Status    int            `gorm:"default:1;index" json:"status"`
	RoleID    uint           `gorm:"default:2;comment:'角色ID:1=管理员,2=普通用户'" json:"role_id"`
}
