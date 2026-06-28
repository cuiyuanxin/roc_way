package service

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/utils"
)

// AuthService 认证应用服务。
//
// 跨聚合操作：user + auth + lock
// 编排密码校验 + 锁定检查 + token 签发 + 登录日志入库
type AuthService struct {
	repo      repository.UserRepository // 直接引用以使用 FindByUsername
	tokens    *auth.Auth
	locks     *LockService
	loginLogs *LoginLogService
	log       *zap.SugaredLogger
}

// NewAuthService 构造 AuthService。
func NewAuthService(
	repo repository.UserRepository,
	tokens *auth.Auth,
	locks *LockService,
	loginLogs *LoginLogService,
	log *zap.SugaredLogger,
) *AuthService {
	return &AuthService{
		repo:      repo,
		tokens:    tokens,
		locks:     locks,
		loginLogs: loginLogs,
		log:       log,
	}
}

// Login 用例：username + 密码登录（应用 project_rules.md 第 19 条）。
//
// 流程：
//  1. 入参非空检查 → 写 invalid_param 日志
//  2. 查锁定 → 锁定中写 locked_attempt 日志 + 返回 ErrAccountLocked
//  3. 查用户 → 不存在写 failure(user_not_found) + 返回 ErrUnauthorized（仍记录失败计数，防枚举）
//  4. 校验密码 → 失败写 failure(password_mismatch) + 记录失败计数 + 可能触发锁定
//  5. 成功 → 清失败计数 + 写 success 日志 + 签发 token
//
// 登录日志（auth_login_logs 表）在每个分支出口写入；写日志失败不阻断业务。
func (s *AuthService) Login(ctx context.Context, in dto.LoginInput) (*auth.TokenPair, error) {
	username := in.Username
	event := LogEvent{Username: username, IP: in.IP, UserAgent: in.UserAgent}

	if username == "" || in.Password == "" {
		event.Status = model.LoginStatusInvalidParam
		event.Reason = "username_or_password_empty"
		s.loginLogs.RecordLogin(ctx, event)
		return nil, errcode.ErrInvalidParam.WithMessage("username 和 password 不能为空")
	}

	// 1. 查锁定
	lock := s.locks.GetLock(ctx, username)
	if lock.Active() {
		event.Status = model.LoginStatusLockedAttempt
		event.Reason = "account_locked_short"
		if lock.Level == LockLong {
			event.Reason = "account_locked_long"
		}
		s.loginLogs.RecordLogin(ctx, event)

		// 长锁到底（不再累加计数、不再升级），直接拦截。
		if lock.Level == LockLong {
			return nil, errcode.ErrAccountLocked
		}

		// 短锁期间继续失败：仍要累加 count，便于达到 long 阈值时升级到 24h 长锁。
		level := s.locks.RecordFailure(ctx, username, in.IP)
		if level == LockLong || level == LockShort {
			return nil, errcode.ErrAccountLocked
		}
		return nil, errcode.ErrAccountLocked
	}

	// 2. 查用户
	u, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	if u == nil {
		// 用户不存在也计数（防枚举）
		s.locks.RecordFailure(ctx, username, in.IP)
		event.Status = model.LoginStatusFailure
		event.Reason = "user_not_found"
		s.loginLogs.RecordLogin(ctx, event)
		return nil, errcode.ErrUserNotFound.WithMessage("用户不存在")
	}

	// 3. 校验密码
	if !utils.Verify(u.Password, in.Password) {
		level := s.locks.RecordFailure(ctx, username, in.IP)
		event.Status = model.LoginStatusFailure
		event.Reason = "password_mismatch"
		s.loginLogs.RecordLogin(ctx, event)
		// 密码错时如果新触发了锁定，优先返回锁定错误
		if level == LockShort || level == LockLong {
			return nil, errcode.ErrAccountLocked
		}
		return nil, errcode.ErrPasswordMismatched.WithMessage("密码错误")
	}

	// 4. 成功：清失败计数 + 签发 token + 写日志
	s.locks.ClearFailures(ctx, username)
	pair, err := s.tokens.Issue(strconv.FormatUint(uint64(u.ID), 10))
	if err != nil {
		// token 签发失败不写 success，但仍要记录失败
		event.Status = model.LoginStatusFailure
		event.Reason = "token_issue_failed"
		event.UserID = u.ID
		s.loginLogs.RecordLogin(ctx, event)
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	event.Status = model.LoginStatusSuccess
	event.UserID = u.ID
	s.loginLogs.RecordLogin(ctx, event)

	return pair, nil
}

// Logout 用例：将 access / refresh 一起加入黑名单。
//
// 典型用法：客户端在 access 即将过期时主动 logout，会把同 pair 的 refresh
// 一起传过来；后端把两个 jti 都吊销，下一次即便有人截获旧 refresh 也无法
// 用它换新 access。
//
// refreshToken 为空时只吊销 access jti（向后兼容老调用方 / 没拿到 refresh）。
func (s *AuthService) Logout(ctx context.Context, jti, refreshToken string) error {
	if jti == "" {
		return errcode.ErrUnauthorized.WithMessage("缺少 jti")
	}
	if err := s.tokens.Revoke(ctx, jti, 24*time.Hour); err != nil {
		return errcode.New(errcode.ErrInternal, err)
	}
	s.log.Infow("auth.logout", "jti", jti)

	// 可选：同时吊销 refresh token（解析出 jti，写入黑名单）
	if refreshToken == "" {
		return nil
	}
	claims, err := s.tokens.Parse(refreshToken)
	if err != nil {
		// refresh 解析失败：access 已吊销，不阻断业务，仅记录
		s.log.Warnw("auth.logout.refresh_parse_failed", "error", err.Error())
		return nil
	}
	if claims.Kind != "refresh" {
		s.log.Warnw("auth.logout.not_refresh_token", "kind", claims.Kind)
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	if err := s.tokens.Revoke(ctx, claims.ID, ttl); err != nil {
		s.log.Warnw("auth.logout.refresh_revoke_failed", "error", err.Error())
		return nil
	}
	s.log.Infow("auth.logout.refresh_revoked", "jti", claims.ID)
	return nil
}

// Refresh 用例：用 refresh token 换新一对 token。
//
// 修复 [C4]：原 handler.refresh 仅回显入参；现通过 service 编排
// auth.Auth.Refresh 的真校验（验签 + Kind 断言 + 黑名单 + rotation）。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	if refreshToken == "" {
		return nil, errcode.ErrInvalidParam.WithMessage("refresh token 不能为空")
	}
	pair, err := s.tokens.Refresh(ctx, refreshToken)
	if err != nil {
		s.log.Warnw("auth.refresh_failed", "error", err.Error())
		return nil, err
	}
	return pair, nil
}
