package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/domain"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/pkg/utils"
)

// AuthService 认证应用服务。
//
// 不属于任何聚合，是「领域服务」（Domain Service）：
//   - 跨聚合操作：user + auth + lock
//   - 编排密码校验 + 锁定检查 + token 签发 + 登录日志入库
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
		return nil, errcode.New(errcode.ErrInvalidParam, errors.New("login: username and password required"))
	}

	// 1. 查锁定
	if lock := s.locks.GetLock(ctx, username); lock.Active() {
		event.Status = model.LoginStatusLockedAttempt
		event.Reason = "account_locked_short"
		if lock.Level == domain.LockLong {
			event.Reason = "account_locked_long"
		}
		s.loginLogs.RecordLogin(ctx, event)
		return nil, errcode.ErrAccountLocked
	}

	// 2. 查用户
	u, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		s.log.Errorw("auth.login_failed", "username", username, "error", err.Error())
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	if u == nil {
		// 用户不存在也计数（防枚举）
		s.locks.RecordFailure(ctx, username, in.IP)
		event.Status = model.LoginStatusFailure
		event.Reason = "user_not_found"
		s.loginLogs.RecordLogin(ctx, event)
		return nil, errcode.New(errcode.ErrUserNotFound, errors.New("login: user not found"))
	}

	// 3. 校验密码
	if !utils.Verify(u.Password, in.Password) {
		level := s.locks.RecordFailure(ctx, username, in.IP)
		event.Status = model.LoginStatusFailure
		event.Reason = "password_mismatch"
		s.loginLogs.RecordLogin(ctx, event)
		// 密码错时如果新触发了锁定，优先返回锁定错误
		if level == domain.LockShort || level == domain.LockLong {
			return nil, errcode.ErrAccountLocked
		}
		return nil, errcode.New(errcode.ErrPasswordMismatched, errors.New("login: password mismatched"))
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
		s.log.Errorw("auth.login_failed", "username", username, "error", err.Error())
		return nil, errcode.New(errcode.ErrInternal, err)
	}
	event.Status = model.LoginStatusSuccess
	event.UserID = u.ID
	s.loginLogs.RecordLogin(ctx, event)

	return pair, nil
}

// Logout 用例：将当前 token 加入黑名单。
func (s *AuthService) Logout(ctx context.Context, jti string) error {
	if jti == "" {
		return errcode.New(errcode.ErrUnauthorized, errors.New("auth: missing jti"))
	}
	if err := s.tokens.Revoke(ctx, jti, 24*time.Hour); err != nil {
		return errcode.New(errcode.ErrInternal, err)
	}
	s.log.Infow("auth.logout", "jti", jti)
	return nil
}

// Refresh 用例：用 refresh token 换新一对 token。
//
// 修复 [C4]：原 handler.refresh 仅回显入参；现通过 service 编排
// auth.Auth.Refresh 的真校验（验签 + Kind 断言 + 黑名单 + rotation）。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	if refreshToken == "" {
		return nil, errcode.New(errcode.ErrInvalidParam, errors.New("refresh: empty token"))
	}
	pair, err := s.tokens.Refresh(ctx, refreshToken)
	if err != nil {
		s.log.Warnw("auth.refresh_failed", "error", err.Error())
		return nil, err
	}
	return pair, nil
}
