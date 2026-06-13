package mysql

import (
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *entity.User) error
	Update(user *entity.User) error
	UpdateStatus(id uint, status int) error
	Delete(id uint) error
	GetByID(id uint) (*entity.User, error)
	GetByUsername(username string) (*entity.User, error)
	GetByPhone(phone string) (*entity.User, error) // 通过手机号查找用户
	List(pageNum, pageSize int, keyword string) ([]*entity.User, int64, error)
	WithTx(tx *gorm.DB) UserRepository
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *entity.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return errors.Wrap(err, "创建用户失败!")
	}
	return nil
}

func (r *userRepository) Update(user *entity.User) error {
	if err := r.db.Model(user).Updates(user).Error; err != nil {
		return errors.Wrap(err, "更新用户失败!")
	}
	return nil
}

func (r *userRepository) UpdateStatus(id uint, status int) error {
	result := r.db.Model(&entity.User{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return errors.Wrap(result.Error, "更新用户状态失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeUserNotFound, "用户不存在!")
	}
	return nil
}

func (r *userRepository) Delete(id uint) error {
	result := r.db.Delete(&entity.User{}, id)
	if result.Error != nil {
		return errors.Wrap(result.Error, "删除用户失败!")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.CodeUserNotFound, "用户不存在!")
	}
	return nil
}

func (r *userRepository) GetByID(id uint) (*entity.User, error) {
	var user entity.User
	if err := r.db.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在!")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(username string) (*entity.User, error) {
	var user entity.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "用户不存在!")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

// GetByPhone 通过手机号查找用户（用于找回密码）
func (r *userRepository) GetByPhone(phone string) (*entity.User, error) {
	var user entity.User
	if err := r.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeUserNotFound, "该手机号未注册")
		}
		return nil, errors.Wrap(err, "查询用户失败!")
	}
	return &user, nil
}

func (r *userRepository) List(pageNum, pageSize int, keyword string) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	tx := r.db.Model(&entity.User{})
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

func (r *userRepository) WithTx(tx *gorm.DB) UserRepository {
	return &userRepository{db: tx}
}
