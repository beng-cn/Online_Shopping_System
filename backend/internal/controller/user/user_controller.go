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

// Register 用户注册
// @Summary      用户注册
// @Description  注册新用户账号
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body body request.RegisterRequest true "注册信息"
// @Success      200 {object} response.Response
// @Router       /user/register [post]
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

// Login 用户登录
// @Summary      用户登录
// @Description  使用用户名和密码登录，返回JWT令牌
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body body request.LoginRequest true "登录信息"
// @Success      200 {object} response.Response
// @Router       /user/login [post]
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

// ForgotPassword 找回密码
// @Summary      找回密码
// @Description  通过手机号验证后重置密码
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body body request.ForgotPasswordRequest true "找回密码信息"
// @Success      200 {object} response.Response
// @Router       /user/forgot-password [post]
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

// UpdateUserInfo 更新用户信息
// @Summary      更新用户信息
// @Description  更新当前登录用户的个人信息
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        body body request.UpdateUserInfoRequest true "用户信息"
// @Success      200 {object} response.Response
// @Router       /auth/user/info [put]
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

// GetUserInfo 获取用户信息
// @Summary      获取用户信息
// @Description  获取当前登录用户的详细信息
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Success      200 {object} response.Response
// @Router       /auth/user/info [get]
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
// @Summary      获取用户列表
// @Description  管理员分页查询用户列表，支持关键词搜索
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        page_num  query     int    false "页码"        default(1)
// @Param        page_size query     int    false "每页数量"    default(10)
// @Param        keyword   query     string false "搜索关键词"
// @Success      200 {object} response.Response
// @Router       /admin/user/list [get]
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

// UpdateUserStatus 更新用户状态（管理员）
// @Summary      更新用户状态
// @Description  管理员启用或禁用指定用户
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        id   path     int                           true "用户ID"
// @Param        body body     request.UpdateUserStatusRequest true "状态信息"
// @Success      200 {object} response.Response
// @Router       /admin/user/{id}/status [put]
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

// DeleteUser 删除用户（管理员）
// @Summary      删除用户
// @Description  管理员删除指定用户
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        id   path     int    true "用户ID"
// @Success      200 {object} response.Response
// @Router       /admin/user/{id} [delete]
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

// ResetUserPassword 管理员重置用户密码
// @Summary      重置用户密码
// @Description  管理员重置指定用户的密码（bcrypt不可逆，只能重置不能查看）
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        id   path     int                              true "用户ID"
// @Param        body body     request.ResetUserPasswordRequest  true "新密码信息"
// @Success      200 {object} response.Response
// @Router       /admin/user/{id}/reset-password [put]
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

// VerifyAdminPin 验证管理员PIN码
// @Summary      验证管理员PIN码
// @Description  管理员二次验证PIN码
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        body body request.VerifyAdminPinRequest true "PIN码验证信息"
// @Success      200 {object} response.Response
// @Router       /admin/verify-pin [post]
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
// @Summary      设置管理员PIN码
// @Description  管理员设置或修改PIN码
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Param        body body request.SetAdminPinRequest true "PIN码设置信息"
// @Success      200 {object} response.Response
// @Router       /admin/set-pin [post]
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
