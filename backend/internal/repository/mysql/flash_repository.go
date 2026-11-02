package mysql

import (
	"context"

	"backend/internal/model/entity"
	"backend/internal/pkg/ctxutil"
	"backend/internal/pkg/errors"

	"gorm.io/gorm"
)

// FlashRepository 秒杀活动数据库操作接口
type FlashRepository interface {
	Create(ctx context.Context, flash *entity.FlashSale) error
	Update(ctx context.Context, flash *entity.FlashSale) error
	GetByID(ctx context.Context, id uint) (*entity.FlashSale, error)
	GetByProductID(ctx context.Context, productID uint) (*entity.FlashSale, error)
	ListAll(ctx context.Context) ([]*entity.FlashSale, error)                                    // 管理员查看全部
	ListActive(ctx context.Context) ([]*entity.FlashSale, error)                                 // 用户端查看进行中的
	UpdateStatus(ctx context.Context, id uint, status int) error                                 // 更新活动状态
	UpdateStatusWithVersion(ctx context.Context, id uint, status int, version int) (bool, error) // 乐观锁状态更新
	WithTx(tx *gorm.DB) FlashRepository                                                          // 事务支持
}

type flashRepository struct {
	db *gorm.DB
}

// NewFlashRepository 创建秒杀活动DB仓库实例（供 Wire 注入）
func NewFlashRepository(db *gorm.DB) FlashRepository {
	return &flashRepository{db: db}
}

// Create 创建秒杀活动
func (r *flashRepository) Create(ctx context.Context, flash *entity.FlashSale) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Create(flash).Error; err != nil {
		return errors.Wrap(err, "创建秒杀活动失败")
	}
	return nil
}

// Update 更新秒杀活动
//
// 用 Select 白名单：struct Updates 会跳过零值（如把 queue_cap 改成 0 表示"按库存自动计算"），
// 同时排除 product_id 和 version —— product_id 不允许改，version 归乐观锁维护。
func (r *flashRepository) Update(ctx context.Context, flash *entity.FlashSale) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Model(flash).
		Select("flash_price", "flash_stock", "queue_cap", "start_time", "end_time", "status").
		Updates(flash).Error; err != nil {
		return errors.Wrap(err, "更新秒杀活动失败")
	}
	return nil
}

// GetByID 根据ID查询秒杀活动
func (r *flashRepository) GetByID(ctx context.Context, id uint) (*entity.FlashSale, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var flash entity.FlashSale
	if err := db.Preload("Product").First(&flash, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeFlashSaleNotFound, "秒杀活动不存在")
		}
		return nil, errors.Wrap(err, "查询秒杀活动失败")
	}
	return &flash, nil
}

// GetByProductID 根据商品ID查询秒杀活动
func (r *flashRepository) GetByProductID(ctx context.Context, productID uint) (*entity.FlashSale, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var flash entity.FlashSale
	if err := db.Preload("Product").Where("product_id = ?", productID).First(&flash).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.CodeFlashSaleNotFound, "秒杀活动不存在")
		}
		return nil, errors.Wrap(err, "查询秒杀活动失败")
	}
	return &flash, nil
}

// ListAll 查询所有秒杀活动（管理员用，包含已取消和已结束的）
func (r *flashRepository) ListAll(ctx context.Context) ([]*entity.FlashSale, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var list []*entity.FlashSale
	if err := db.Preload("Product").Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "查询秒杀活动列表失败")
	}
	return list, nil
}

// ListActive 查询进行中的秒杀活动（用户端用）
func (r *flashRepository) ListActive(ctx context.Context) ([]*entity.FlashSale, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	var list []*entity.FlashSale
	if err := db.Preload("Product").
		Where("status = 1").
		Order("end_time ASC").
		Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "查询秒杀活动列表失败")
	}
	return list, nil
}

// UpdateStatus 更新秒杀活动状态
func (r *flashRepository) UpdateStatus(ctx context.Context, id uint, status int) error {
	db := r.db.WithContext(ctxutil.Or(ctx))
	if err := db.Model(&entity.FlashSale{}).Where("id = ?", id).
		Update("status", status).Error; err != nil {
		return errors.Wrap(err, "更新秒杀状态失败")
	}
	return nil
}

// UpdateStatusWithVersion 使用乐观锁更新状态（防止并发冲突）
// 返回 true 表示更新成功，false 表示版本冲突
func (r *flashRepository) UpdateStatusWithVersion(ctx context.Context, id uint, status int, version int) (bool, error) {
	db := r.db.WithContext(ctxutil.Or(ctx))
	result := db.Model(&entity.FlashSale{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]interface{}{
			"status":  status,
			"version": version + 1,
		})
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "更新秒杀状态失败")
	}
	return result.RowsAffected > 0, nil
}

// WithTx 返回绑定事务的 Repository 实例
func (r *flashRepository) WithTx(tx *gorm.DB) FlashRepository {
	return &flashRepository{db: tx}
}
