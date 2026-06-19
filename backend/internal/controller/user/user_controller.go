package user

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/user"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService user.UserService
}

// NewUserController 创建用户控制器实例
func NewUserController(userService user.UserService) *UserController {
	return &UserController{userService: userService}
}

// 用户注册
func (c *UserController) Register(ctx *gin.Context) {
	var req request.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.userService.Register(&req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// ForgotPassword 找回密码（通过手机号验证后重置）
func (c *UserController) ForgotPassword(ctx *gin.Context) {
	var req request.ForgotPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.userService.ResetPasswordByPhone(&req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{"message": "密码重置成功，请使用新密码登录"})
}

// 用户登录
func (c *UserController) Login(ctx *gin.Context) {
	var req request.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	resp, err := c.userService.Login(&req)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 更新用户信息
func (c *UserController) UpdateUserInfo(ctx *gin.Context) {
	var req request.UpdateUserInfoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.userService.UpdateUserInfo(userID, &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 获取用户信息
func (c *UserController) GetUserInfo(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")

	resp, err := c.userService.GetUserInfo(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// ListUsers 管理员获取用户列表
func (c *UserController) ListUsers(ctx *gin.Context) {
	pageNum, _ := strconv.Atoi(ctx.DefaultQuery("page_num", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "10"))
	keyword := ctx.Query("keyword")

	resp, err := c.userService.ListUsers(pageNum, pageSize, keyword)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// 更新用户状态（管理员）
func (c *UserController) UpdateUserStatus(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("用户ID无效"))
		return
	}

	var req request.UpdateUserStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.userService.UpdateUserStatus(uint(id), *req.Status); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 删除用户（管理员）
func (c *UserController) DeleteUser(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("用户ID无效"))
		return
	}

	if err := c.userService.DeleteUser(uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// VerifyAdminPin 验证管理员PIN码（二次验证）
func (c *UserController) VerifyAdminPin(ctx *gin.Context) {
	var req request.VerifyAdminPinRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.userService.VerifyAdminPin(userID, &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{"verified": true})
}

// SetAdminPin 设置管理员PIN码
func (c *UserController) SetAdminPin(ctx *gin.Context) {
	var req request.SetAdminPinRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	if err := c.userService.SetAdminPin(userID, &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{"message": "PIN码设置成功"})
}

// 管理员重置用户密码（bcrypt不可逆，只能重置不能查看）
func (c *UserController) ResetUserPassword(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("用户ID无效"))
		return
	}

	var req request.ResetUserPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.userService.ResetUserPassword(uint(id), &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}
