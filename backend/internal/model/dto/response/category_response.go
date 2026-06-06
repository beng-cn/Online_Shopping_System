package response

import "time"

// 分类信息响应
type CategoryResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	ParentID  uint      `json:"parent_id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
