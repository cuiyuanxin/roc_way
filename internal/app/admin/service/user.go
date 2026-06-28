// Package service 是应用服务层（业务编排）。
//
// 职责：
//   - 编排 model 与 repository
//   - 处理跨聚合的业务用例（注册、登录、改昵称等）
//   - 不直接接触 HTTP / GORM，由 handler 传入的入参驱动
//   - 业务校验由 dto.Validate() 负责（rule 集中、handler 拦截、service 不重复）
//   - service 本身只做"业务逻辑"——如检查用户名是否被占用、密码哈希、保存
//
// 依赖方向：handler → service → model ← repository
package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/utils"
)

// UserService 用户应用服务：编排用户聚合的所有业务用例。
type UserService struct {
	repo repository.UserRepository
	log  *zap.SugaredLogger
}

// NewUserService 构造 UserService。
func NewUserService(repo repository.UserRepository, log *zap.SugaredLogger) *UserService {
	return &UserService{repo: repo, log: log}
}

// Register 用例：注册新用户。
//  1. dto.RegisterReq.Validate() 已在 handler 层做过完整校验（长度 / 格式 / 强度）
//  2. 检查用户名是否被占用
//  3. 密码 bcrypt 哈希
//  4. 通过仓储保存
//
// 返回 dto.UserInfo 而非 model.User：避免 handler 再次手写映射函数；
// 字段列在 dto，密码哈希字段在 dto 不存在 = 不可能泄漏到前端。
func (s *UserService) Register(ctx context.Context, in dto.RegisterInput) (*dto.UserInfo, error) {
	if existing, _ := s.repo.FindByUsername(ctx, strings.TrimSpace(in.Username)); existing != nil {
		return nil, errcode.ErrUserExists.WithMessage("用户名已被占用")
	}
	hash, err := utils.Hash(in.Password)
	if err != nil {
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	now := time.Now()
	u := &model.User{
		Username:  strings.TrimSpace(in.Username),
		Email:     strings.TrimSpace(in.Email),
		NickName:  strings.TrimSpace(in.Name),
		Password:  hash,
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}
	if err := s.repo.Save(ctx, u); err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.registered", "user_id", u.ID, "email", u.Email)
	info := toUserInfo(u)
	return &info, nil
}

// GetByID 用例：按 ID 查询用户（直接返 dto，handler 不再做映射）。
func (s *UserService) GetByID(ctx context.Context, id uint) (*dto.UserInfo, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	if u == nil {
		return nil, nil
	}
	info := toUserInfo(u)
	return &info, nil
}

// FindByUsername 按用户名查询（供 AuthService 登录使用）。
//
// 内部用——AuthService 拿到的是 model.User（要读 Password 字段做 bcrypt 比对），
// 所以这个方法**不**返 dto（dto 没有 Password 字段，登录逻辑拿不到密码哈希）。
func (s *UserService) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.repo.FindByUsername(ctx, username)
}

// List 用例：分页查询用户。
func (s *UserService) List(ctx context.Context, in dto.ListInput) ([]dto.UserInfo, int64, error) {
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
	out := make([]dto.UserInfo, 0, len(users))
	for _, u := range users {
		out = append(out, toUserInfo(u))
	}
	return out, total, nil
}

// UpdateName 用例：修改昵称。
func (s *UserService) UpdateName(ctx context.Context, in dto.UpdateNameInput) (*dto.UserInfo, error) {
	u, err := s.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	if u == nil {
		return nil, errcode.ErrUserNotFound.WithMessage("用户不存在")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 64 {
		return nil, errcode.ErrInvalidParam.WithMessage("昵称长度必须在 1-64 之间")
	}
	u.NickName = name
	u.UpdatedAt = time.Now().Unix()
	if err := s.repo.Save(ctx, u); err != nil {
		return nil, errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.nick_name_updated", "user_id", u.ID, "nick_name", u.NickName)
	info := toUserInfo(u)
	return &info, nil
}

// Delete 用例：删除用户。
func (s *UserService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return errcode.New(errcode.ErrDatabase, err)
	}
	s.log.Infow("user.deleted", "user_id", id)
	return nil
}

// toUserInfo model.User → dto.UserInfo 映射。
//
// 放 service 包内（不是 handler 包）：service 直接返 dto 给前端，
// handler 拿到 dto 后只 WriteOK，不用再调一次映射函数。
// dto 不含 Password 字段 → model.User 的 Password 哈希不可能泄漏到前端。
func toUserInfo(u *model.User) dto.UserInfo {
	if u == nil {
		return dto.UserInfo{}
	}
	return dto.UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		NickName:    u.NickName,
		Avatar:      u.Avatar,
		Roles:       []string{"*:*:*"},
		Permissions: []string{"*:*:*"},
	}
}
