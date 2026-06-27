// Package service 是应用服务层（业务编排）。
//
// 职责：
//   - 编排 domain 与 repository
//   - 处理跨聚合的业务用例（注册、登录、改昵称等）
//   - 不直接接触 HTTP / GORM，由 handler 传入的入参驱动
//
// 依赖方向：handler → service → domain ← repository
package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/domain"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/utils"
)

// Clock 时间抽象（便于测试注入）。
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// UserService 用户应用服务：编排用户聚合的所有业务用例。
type UserService struct {
	repo  repository.UserRepository
	clock Clock
	log   *zap.SugaredLogger
}

// NewUserService 构造 UserService。
func NewUserService(repo repository.UserRepository, log *zap.SugaredLogger) *UserService {
	return &UserService{repo: repo, clock: realClock{}, log: log}
}

// Register 用例：注册新用户。
//  1. 校验入参（用户名 / 邮箱 / 密码强度由 dto + domain.NewUser 校验）
//  2. 检查用户名是否被占用
//  3. 密码 bcrypt 哈希
//  4. 通过仓储保存
func (s *UserService) Register(ctx context.Context, in dto.RegisterInput) (*domain.User, error) {
	if existing, _ := s.repo.FindByUsername(ctx, strings.TrimSpace(in.Username)); existing != nil {
		return nil, errcode.ErrUserExists.WithMessage("用户名已被占用")
	}
	hash, err := utils.Hash(in.Password)
	if err != nil {
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	u, err := domain.NewUser(0, in.Username, in.Email, in.Name, hash, s.clock.Now())
	if err != nil {
		return nil, errcode.New(errcode.ErrInvalidParam, err)
	}
	if err := s.repo.Save(ctx, u); err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.registered", "user_id", u.ID, "email", u.Email)
	return u, nil
}

// GetByID 用例：按 ID 查询用户。
func (s *UserService) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errcode.New(errcode.ErrUserNotFound, domain.ErrNotFound)
	}
	return u, nil
}

// FindByEmail 按邮箱查询（供 AuthService 登录使用）。
func (s *UserService) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.FindByEmail(ctx, email)
}

// List 用例：分页查询用户。
func (s *UserService) List(ctx context.Context, in dto.ListInput) ([]*domain.User, int64, error) {
	if in.Page < 1 {
		in.Page = 1
	}
	if in.PageSize < 1 || in.PageSize > 200 {
		in.PageSize = 20
	}
	offset := (in.Page - 1) * in.PageSize
	users, total, err := s.repo.List(ctx, offset, in.PageSize)
	if err != nil {
		return nil, 0, errcode.New(errcode.ErrDatabase, err)
	}
	return users, total, nil
}

// UpdateName 用例：修改昵称。
func (s *UserService) UpdateName(ctx context.Context, in dto.UpdateNameInput) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, errcode.New(errcode.ErrUserNotFound, domain.ErrNotFound)
	}
	if err := u.SetName(in.Name, s.clock.Now()); err != nil {
		return nil, errcode.New(errcode.ErrInvalidParam, err)
	}
	if err := s.repo.Save(ctx, u); err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.name_updated", "user_id", u.ID, "name", u.Name)
	return u, nil
}

// Delete 用例：删除用户。
func (s *UserService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.deleted", "user_id", id)
	return nil
}
