// Package service: 登录日志服务（独立于账号锁定）。
//
// 职责：
//   - 接收 auth.go 在 Login 流程各分支传入的事件（成功 / 失败 / 被锁拦截 / 参数错误），
//     统一写入 auth_login_logs 表。
//   - 写日志**不阻断业务**：DB 故障仅 zap warn，登录本身仍按既定逻辑成功 / 失败。
//
// 与 LockService 的边界（**严格隔离**）：
//   - LoginLogService：本文件，写「登录日志」，不参与锁定决策。
//   - LockService：写「锁定跟踪」(login_audits)，决定是否触发账号锁定。
//   - 两者职责正交；任何一方故障不影响另一方。
package service

import (
	"context"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"go.uber.org/zap"
)

// LoginLogService 登录日志服务。
type LoginLogService struct {
	repo repository.LoginLogRepository
	log  *zap.SugaredLogger
}

// NewLoginLogService 构造登录日志服务。
func NewLoginLogService(repo repository.LoginLogRepository, log *zap.SugaredLogger) *LoginLogService {
	return &LoginLogService{repo: repo, log: log}
}

// RecordLogin 写入一条登录日志。
//
// 写库失败仅 zap warn，不返回 error（业务流不应被审计失败阻断）。
func (s *LoginLogService) RecordLogin(ctx context.Context, e LogEvent) {
	if s.repo == nil {
		return
	}
	m := &model.LoginLog{
		Username:  e.Username,
		UserID:    e.UserID,
		Status:    e.Status,
		Reason:    e.Reason,
		IP:        e.IP,
		UserAgent: e.UserAgent,
	}
	if err := s.repo.Record(ctx, m); err != nil {
		s.log.Warnw("login_log.record_failed",
			"username", e.Username,
			"status", e.Status,
			"error", err.Error())
	}
}

// CleanupExpired 删除过期日志（业务侧调用，如独立的日志管理模块）。
func (s *LoginLogService) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	if s.repo == nil {
		return 0, nil
	}
	return s.repo.CleanupExpired(ctx, cutoff)
}

// LogEvent 一次登录事件（参数扁平化，避免调用方构造 model）。
type LogEvent struct {
	Username  string // 登录账号
	UserID    uint   // 成功登录时记录 user.id；其它为 0
	Status    string // model.LoginStatusSuccess / Failure / LockedAttempt / InvalidParam
	Reason    string // 失败原因（password_mismatch / user_not_found / ...）
	IP        string // handler 注入 c.ClientIP()
	UserAgent string // handler 注入 c.GetHeader("User-Agent")
}
