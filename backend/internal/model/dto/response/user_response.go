package response

import "time"

// 用户信息响应
type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Status    int       `json:"status"`
	RoleID    uint      `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

// 登录响应
type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// 用户列表响应（管理员）
type UserListResponse struct {
	List  []UserResponse `json:"list"`
	Total int64          `json:"total"`
}
