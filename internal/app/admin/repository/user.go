// Package repository 是仓储层：用户聚合的 GORM 实现。
//
// 仅依赖 model 与基础设施（database），**禁止**依赖 service / handler / dto。
package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	infra "github.com/cuiyuanxin/roc_way/internal/pkg/database"
)

// UserRepository 用户聚合的持久化接口。
type UserRepository interface {
	// Save 保存（新增或更新）。
	Save(ctx context.Context, u *model.User) error
	// FindByID 按主键查询。
	FindByID(ctx context.Context, id uint) (*model.User, error)
	// FindByUsername 根据用户名查询（登录账号）。
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	// FindByEmail 按邮箱查询（保留兼容，不作登录凭据）。
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// List 列表查询（只读路由走 RO 节点）。
	List(ctx context.Context, offset, limit int) ([]*model.User, int64, error)
	// Delete 删除聚合。
	Delete(ctx context.Context, id uint) error
}

// userRepo UserRepository 接口的 GORM 实现。
type userRepo struct {
	db *infra.DB
}

// NewUserRepository 构造仓储。
func NewUserRepository(db *infra.DB) UserRepository {
	return &userRepo{db: db}
}

// Save 新增或更新。
//
// 更新分支显式列出所有可写字段（username / email / nick_name / password / updated_at），
// 避免早期「map 只列 3 字段」导致邮箱改名后存不进去的问题。
func (r *userRepo) Save(ctx context.Context, u *model.User) error {
	if u.ID == 0 {
		// 新增
		if err := r.db.Write.WithContext(ctx).Create(u).Error; err != nil {
			return err
		}
		return nil
	}
	return r.db.Write.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", u.ID).
		Updates(map[string]any{
			"username":   u.Username,
			"email":      u.Email,
			"nick_name":  u.NickName,
			"password":   u.Password,
			"updated_at": u.UpdatedAt,
		}).Error
}

// FindByID 按主键查询。
func (r *userRepo) FindByID(ctx context.Context, id uint) (*model.User, error) {
	var m model.User
	err := r.db.RO().WithContext(ctx).First(&m, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// FindByEmail 按邮箱查询。
func (r *userRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var m model.User
	err := r.db.RO().WithContext(ctx).Where("email = ?", email).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// FindByUsername 按用户名查询。
func (r *userRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	username = strings.TrimSpace(username)
	var m model.User
	err := r.db.RO().WithContext(ctx).Where("username = ?", username).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// List 分页查询。
func (r *userRepo) List(ctx context.Context, offset, limit int) ([]*model.User, int64, error) {
	var (
		rows  []model.User
		total int64
	)
	if err := r.db.RO().WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.RO().WithContext(ctx).
		Order("id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	users := make([]*model.User, 0, len(rows))
	for i := range rows {
		users = append(users, &rows[i])
	}
	return users, total, nil
}

// Delete 删除。
func (r *userRepo) Delete(ctx context.Context, id uint) error {
	return r.db.Write.WithContext(ctx).Delete(&model.User{}, id).Error
}
