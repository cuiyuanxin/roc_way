// Package repository: 登录审计（锁定跟踪）仓储。
//
// 与 auth_login_logs（登录日志）严格区分：本表管锁定决策与降级，
// 不参与审计留痕。详见 service.LoginAuditService / service.LockService 的边界说明。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	infra "github.com/cuiyuanxin/roc_way/internal/pkg/database"
)

// LoginAuditRepository 登录审计仓储接口。
//
// 双存储用途：
//   - Redis 主存（service 层先查 Redis）；
//   - MySQL 兜底（Redis 故障时 service 降级查此接口）；
//   - 在线清理（每次 RecordFailure 附 LIMIT 1000 清理过期记录）。
type LoginAuditRepository interface {
	// RecordFailure 记录一次登录失败。
	RecordFailure(ctx context.Context, username, ip string, occurredAt time.Time) error

	// RecordLock 记录一次账号锁定事件。
	RecordLock(ctx context.Context, username string, level string, failedCount int, occurredAt, expiresAt time.Time) error

	// LatestActiveLock 查询某用户最近一次尚未过期的锁定事件。
	// 返回 nil 表示无活跃锁定。
	LatestActiveLock(ctx context.Context, username string, now time.Time) (*model.LoginAudit, error)

	// RecentFailuresCount 统计某用户最近 N 小时内的失败次数。
	RecentFailuresCount(ctx context.Context, username string, since time.Time) (int, error)

	// ClearFailures 清除某用户的所有 failure 记录（登录成功后调用）。
	ClearFailures(ctx context.Context, username string) error

	// CleanupExpired 删除 occurredAt 早于 cutoff 的所有记录（janitor 调用）。
	// 返回删除的行数。
	CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error)
}

// loginAuditRepo LoginAuditRepository 接口的 GORM 实现。
type loginAuditRepo struct {
	db *infra.DB
}

// NewLoginAuditRepository 构造仓储。
func NewLoginAuditRepository(db *infra.DB) LoginAuditRepository {
	return &loginAuditRepo{db: db}
}

// RecordFailure 记录一次失败事件，并附带清理过期 failure。
//
// 在线清理（应用 project_rules.md 第 19 条「写入路径在线清理」）：
// 每次写入后 `DELETE WHERE username=? AND event_type='failure' AND created_at < ? LIMIT 1000`，
// 避免 janitor 单次 DELETE 过多行（DB 长事务 / 锁等待）。
//
// 时间字段注意：created_at 是 int64 unix 时间戳，查询条件必须传 int64，
// 不能传 time.Time（GORM 不会自动转换 time.Time → int64，比较会失败）。
func (r *loginAuditRepo) RecordFailure(ctx context.Context, username, ip string, occurredAt time.Time) error {
	return r.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		m := &model.LoginAudit{
			Username:  username,
			EventType: model.EventFailure,
			IP:        ip,
		}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		// 在线清理：删除 24h 前的同 username failure 记录，最多 1000 行
		return tx.Where("username = ? AND event_type = ? AND created_at < ?",
			username, model.EventFailure, occurredAt.Add(-24*time.Hour).Unix()).
			Limit(1000).
			Delete(&model.LoginAudit{}).Error
	})
}

// RecordLock 记录一次锁定事件。
func (r *loginAuditRepo) RecordLock(ctx context.Context, username string, level string, failedCount int, occurredAt, expiresAt time.Time) error {
	m := &model.LoginAudit{
		Username:    username,
		EventType:   level,
		FailedCount: failedCount,
		ExpiresAt:   expiresAt.Unix(),
	}
	return r.db.Write.WithContext(ctx).Create(m).Error
}

// LatestActiveLock 查询最近一次尚未过期的 lock 事件。
//
// 时间字段注意：expires_at 是 int64 unix 时间戳，now 必须转 .Unix() 才能正确比较。
func (r *loginAuditRepo) LatestActiveLock(ctx context.Context, username string, now time.Time) (*model.LoginAudit, error) {
	var m model.LoginAudit
	err := r.db.RO().WithContext(ctx).
		Where("username = ? AND event_type IN (?, ?) AND expires_at > ?",
			username, model.EventLockShort, model.EventLockLong, now.Unix()).
		Order("created_at DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// RecentFailuresCount 统计最近 since 以来某用户的 failure 次数。
//
// 时间字段注意：created_at 是 int64 unix 时间戳，since 必须转 .Unix() 才能正确比较。
func (r *loginAuditRepo) RecentFailuresCount(ctx context.Context, username string, since time.Time) (int, error) {
	var count int64
	err := r.db.RO().WithContext(ctx).
		Model(&model.LoginAudit{}).
		Where("username = ? AND event_type = ? AND created_at >= ?",
			username, model.EventFailure, since.Unix()).
		Count(&count).Error
	return int(count), err
}

// ClearFailures 删除某用户的所有 failure 记录。
//
// 登录成功时调用，重置失败计数器。
// 不删除 lock 记录（防攻击者试探到 4 次后故意输对 1 次再继续刷）。
func (r *loginAuditRepo) ClearFailures(ctx context.Context, username string) error {
	return r.db.Write.WithContext(ctx).
		Where("username = ? AND event_type = ?", username, model.EventFailure).
		Delete(&model.LoginAudit{}).Error
}

// CleanupExpired 删除 occurredAt < cutoff 的所有记录。
func (r *loginAuditRepo) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.Write.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&model.LoginAudit{})
	return res.RowsAffected, res.Error
}
