// Package repository: 登录日志仓储。
//
// 与 LoginAuditRepository（锁定跟踪）严格区分：本表管审计留痕，
// 不参与锁定决策；保留/删除策略由独立的"登录日志管理"模块决定，
// janitor 不自动清理。详见 service.LoginAuditService 的边界说明。
package repository

import (
	"context"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	infra "github.com/cuiyuanxin/roc_way/internal/pkg/database"
)

// LoginLogRepository 登录日志仓储接口。
type LoginLogRepository interface {
	// Record 写入一条登录日志。
	Record(ctx context.Context, log *model.LoginLog) error

	// CleanupExpired 删除 occurredAt < cutoff 的所有记录（业务侧调用，如独立的日志管理模块）。
	// 返回删除的行数。
	CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error)
}

// loginLogRepo LoginLogRepository 接口的 GORM 实现。
type loginLogRepo struct {
	db *infra.DB
}

// NewLoginLogRepository 构造仓储。
func NewLoginLogRepository(db *infra.DB) LoginLogRepository {
	return &loginLogRepo{db: db}
}

// Record 写入一条登录日志。
func (r *loginLogRepo) Record(ctx context.Context, log *model.LoginLog) error {
	return r.db.Write.WithContext(ctx).Create(log).Error
}

// CleanupExpired 删除 occurredAt < cutoff 的所有记录。
//
// 修复 [P0-colname]：原代码误用 occurred_at，LoginLog 实际列名是 created_at，
// 触发 unknown column 'occurred_at' 错误，所有清理操作实际从未生效。
// 时间字段注意：created_at 是 int64 unix 时间戳，cutoff 必须转 .Unix() 才能正确比较。
func (r *loginLogRepo) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.Write.WithContext(ctx).
		Where("created_at < ?", cutoff.Unix()).
		Delete(&model.LoginLog{})
	return res.RowsAffected, res.Error
}
