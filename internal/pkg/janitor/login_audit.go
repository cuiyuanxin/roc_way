// Package janitor: login_audits 表定期清理。
//
// 说明：auth_login_logs（登录日志）由独立的「登录日志管理」模块处理
// （保留 / 删除策略由业务侧决定），不归本包 janitor 清理。
package janitor

import (
	"context"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
)

// NewLoginAuditJanitor 构造 login_audits 清理任务（锁定跟踪表）。
//
// interval 与 retention 由 config.LoginPolicyConfig 注入。
func NewLoginAuditJanitor(repo repository.LoginAuditRepository, interval, retention time.Duration) *Janitor {
	return &Janitor{
		Name:      "login_audit_cleanup",
		Interval:  interval,
		Retention: retention,
		Cleanup: func(ctx context.Context, cutoff time.Time) (int64, error) {
			return repo.CleanupExpired(ctx, cutoff)
		},
	}
}
