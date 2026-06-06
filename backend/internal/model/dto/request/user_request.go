package request

// 用户注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20" msg:"用户名长度需在3-20位之间"`
	Password string `json:"password" binding:"required,min=6" msg:"密码长度不能少于6位"`
	Nickname string `json:"nickname"`
	Email    string `json:"email" binding:"omitempty,email" msg:"邮箱格式不正确"`
	Phone    string `json:"phone" binding:"omitempty,len=11" msg:"手机号格式不正确"`
}

// 用户登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required" msg:"用户名不能为空"`
	Password string `json:"password" binding:"required" msg:"密码不能为空"`
}

// 更新用户信息请求
type UpdateUserInfoRequest struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email" binding:"omitempty,email" msg:"邮箱格式不正确"`
	Phone    string `json:"phone" binding:"omitempty,len=11" msg:"手机号格式不正确"`
}

// 更新用户状态请求（管理员）
type UpdateUserStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=0 1" msg:"状态只能是0或1"`
}
