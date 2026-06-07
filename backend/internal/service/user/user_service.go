package user

import (
	"backend/internal/model/dto/request"
	"backend/internal/model/dto/response"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/jwt"
	"backend/internal/repository/mysql"

	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Register(req *request.RegisterRequest) error
	Login(req *request.LoginRequest) (*response.LoginResponse, error)
	UpdateUserInfo(userID uint, req *request.UpdateUserInfoRequest) error
	GetUserInfo(userID uint) (*response.UserResponse, error)
	UpdateUserStatus(id uint, status int) error
	DeleteUser(id uint) error
	ListUsers(pageNum int, pageSize int, keyword string) (*response.PageResponse, error)
	ResetUserPassword(id uint, req *request.ResetUserPasswordRequest) error // 管理员重置用户密码
}

type userService struct {
	userRepo mysql.UserRepository
	jwtUtil  *jwt.JWTUtil
}

func NewUserService(userRepo mysql.UserRepository, jwtUtil *jwt.JWTUtil) UserService {
	return &userService{
		userRepo: userRepo,
		jwtUtil:  jwtUtil,
	}
}

func (s *userService) Register(req *request.RegisterRequest) error {
	// 检查用户名是否已存在
	_, err := s.userRepo.GetByUsername(req.Username)
	if err == nil {
		return errors.New(errors.CodeUserAlreadyExists, "用户名已存在")
	}
	// 非"用户不存在"的错误才是真正的系统错误
	if !errors.IsCode(err, errors.CodeUserNotFound) {
		return errors.Wrap(err, "检查用户名失败")
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "密码加密失败")
	}

	// 创建用户
	user := &entity.User{
		Username: req.Username,
		Password: string(hash),
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   1, // 默认启用
		RoleID:   2, // 默认普通用户
	}

	return s.userRepo.Create(user)
}

func (s *userService) Login(req *request.LoginRequest) (*response.LoginResponse, error) {
	// 查询用户
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return nil, errors.Wrap(err, "查询用户失败")
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, errors.New(errors.CodeUserDisabled, "用户已被禁用")
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return nil, errors.New(errors.CodePasswordError, "密码错误")
	}

	// 生成JWT令牌
	token, err := s.jwtUtil.GenerateToken(user.ID, user.RoleID, user.Username)
	if err != nil {
		return nil, errors.Wrap(err, "生成令牌失败")
	}

	// 转换为响应DTO
	userResp := &response.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Phone:     user.Phone,
		Status:    user.Status,
		RoleID:    user.RoleID,
		CreatedAt: user.CreatedAt,
	}

	return &response.LoginResponse{
		Token: token,
		User:  *userResp,
	}, nil
}

func (s *userService) UpdateUserInfo(userID uint, req *request.UpdateUserInfoRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "查询用户失败")
	}

	// 只更新允许修改的字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}

	return s.userRepo.Update(user)
}

// GetUserInfo 根据用户ID获取用户信息
func (s *userService) GetUserInfo(userID uint) (*response.UserResponse, error) {
	// 防御性检查：防止无效ID查询
	if userID == 0 {
		return nil, errors.New(errors.CodeUserNotFound, "用户不存在")
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return nil, errors.Wrap(err, "查询用户信息失败")
	}

	if user == nil {
		return nil, errors.New(errors.CodeUserNotFound, "用户不存在")
	}

	// 构建响应DTO
	return &response.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Phone:     user.Phone,
		Status:    user.Status,
		RoleID:    user.RoleID,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *userService) ListUsers(pageNum int, pageSize int, keyword string) (*response.PageResponse, error) {
	// 参数校验和默认值处理
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	users, total, err := s.userRepo.List(pageNum, pageSize, keyword)
	if err != nil {
		return nil, err
	}

	// 转换为响应DTO
	var userList []response.UserResponse
	for _, user := range users {
		userList = append(userList, response.UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			Email:     user.Email,
			Phone:     user.Phone,
			Status:    user.Status,
			RoleID:    user.RoleID,
			CreatedAt: user.CreatedAt,
		})
	}

	// 返回统一分页响应
	return response.NewPageResponse(userList, total, pageNum, pageSize), nil
}

func (s *userService) UpdateUserStatus(id uint, status int) error {
	// 禁止禁用管理员
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "查询用户失败")
	}
	if user.RoleID == 1 && status == 0 {
		return errors.New(errors.CodeForbidden, "禁止禁用管理员用户")
	}

	return s.userRepo.UpdateStatus(id, status)
}

func (s *userService) DeleteUser(id uint) error {
	// 禁止删除管理员
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "查询用户失败")
	}
	if user.RoleID == 1 {
		return errors.New(errors.CodeForbidden, "禁止删除管理员用户")
	}

	return s.userRepo.Delete(id)
}

// ResetUserPassword 管理员重置用户密码
// bcrypt 是单向加密，无法解密原文，因此提供重置功能而非查看
func (s *userService) ResetUserPassword(id uint, req *request.ResetUserPasswordRequest) error {
	// 查询用户是否存在
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		if errors.IsCode(err, errors.CodeUserNotFound) {
			return errors.New(errors.CodeUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "查询用户失败")
	}

	// 加密新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "密码加密失败")
	}

	user.Password = string(hash)
	return s.userRepo.Update(user)
}
