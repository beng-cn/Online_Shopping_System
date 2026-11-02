package mysql

import (
	"context"

	"backend/internal/model/entity"
	"backend/internal/pkg/ctxutil"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	UpdateStatus(ctx context.Context, id uint, status int) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	GetByPhone(ctx context.Context, phone string) (*entity.User, error) // 通过手机号查找用户
	List(ctx context.Context, pageNum, pageSize int, keyword string) ([]*entity.User, int64, error)
	WithTx(tx *gorm.DB) UserRepository
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储实例
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 新增用户
func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Create(user).Error; err != nil {
		return errors.Wrap(err, "创建用户失败!")
	}
	return nil
}

// Update 更新用户信息
//
// 用 Select 白名单：struct Updates 会跳过零值，而"把昵称/邮箱/手机号清空"是合法操作。
// 这里覆盖 UpdateUserInfo、重置密码、设置管理员 PIN 三类调用的全部字段；
// username / status / role_id 不允许通过本方法修改。
func (r *userRepository) Update(ctx context.Context, user *entity.User) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Model(user).
		Select("nickname", "email", "phone", "password", "admin_pin").
		Updates(user).Error; err != nil {
		return errors.Wrap(err, "更新用户失败!")
	}
	return nil
}

// UpdateStatus 更新用户状态（启用/禁用）
func (r *userRepository) UpdateStatus(ctx context.Context, id uint, status int) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	result := db.Model(&entity.User{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.Wrap(result.Error, "更新用户状态失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeUserNotFound, "用户不存在!")
	}
	return nil
}

// Delete 软删除用户
func (r *userRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	result := db.Delete(&entity.User{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除用户失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeUserNotFound, "用户不存在!")
	}
	return nil
}

// GetByID 根据主键查询用户
func (r *userRepository) GetByID(ctx context.Context, id uint) (*entity.User, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var user entity.User
	if err := db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在!")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

// GetByUsername 根据用户名查询用户（含软删除的用于注册校验）
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var user entity.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在!")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

// GetByPhone 通过手机号查找用户（用于找回密码）
func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var user entity.User
	if err := db.Where("phone = ?", phone).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "该手机号未注册")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

// List 分页查询用户列表
func (r *userRepository) List(ctx context.Context, pageNum, pageSize int, keyword string) ([]*entity.User, int64, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var users []*entity.User
	var total int64

	tx := db.Model(&entity.User{})
	if keyword != "" {
		tx = tx.Where("username LIKE ?", "%"+keyword+"%")
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "统计用户总数失败!")
	}

	offset := (pageNum - 1) * pageSize
	if err := tx.Order("id DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, errors.Wrap(err, "查询用户列表失败!")
	}

	return users, total, nil
}

// WithTx 事务传播：返回绑定事务的仓储实例
func (r *userRepository) WithTx(tx *gorm.DB) UserRepository {
	return &userRepository{db: tx}
}
