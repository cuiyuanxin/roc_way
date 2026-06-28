// Package repository: 登录审计（锁定跟踪）仓储。
//
// 与 auth_login_logs（登录日志）严格区分：本表管锁定决策与降级，
// 不参与审计留痕。详见 service.LoginAuditService / service.LockService 的边界说明。
//
// 存储策略（DB 主源 + Redis 加速缓存）：
//   - **DB 永远是真源**（MySQL login_audits 表）。
//   - Redis 仅作为「失败计数 + 锁状态」查询的加速缓存，**不**作为权威存储。
//   - 增删改：**先写 DB（事务）→ 成功后写/更新 Redis**；Redis 失败仅 zap warn，业务不阻断。
//   - 读：**先查 Redis → 失败 fallback DB → DB 结果回填 Redis**。
//   - Redis 不可用（Ping 失败）时 service 仍能跑（DB 兜底）；进程不应被启动期连通性绑死。
package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	infra "github.com/cuiyuanxin/roc_way/internal/pkg/database"
)

// Redis key 命名（带项目统一前缀，由 cache.Client 内部再加 cfg.Prefix）。
// 这里是「裸」key，cache.Set/Get 会自动加前缀。
const (
	redisKeyFail      = "auth:fail:"       // + username → 失败计数（INCR 24h TTL）
	redisKeyLockShort = "auth:lock:short:" // + username → 短锁存在标记（TTL = ShortDuration）
	redisKeyLockLong  = "auth:lock:long:"  // + username → 长锁存在标记（TTL = LongDuration）
)

// LockTTL 锁的 Redis TTL 范围（外部注入）。
type LockTTL struct {
	Short time.Duration
	Long  time.Duration
}

// LoginAuditRepository 登录审计仓储接口。
type LoginAuditRepository interface {
	// RecordFailure 记录一次登录失败（DB 写成功后，Redis 计数 +1 并刷新 TTL）。
	RecordFailure(ctx context.Context, username, ip string, occurredAt time.Time) error

	// RecordLock 记录一次账号锁定事件（DB 写成功后，写 Redis 短/长锁 key + TTL）。
	RecordLock(ctx context.Context, username, level string, failedCount int, occurredAt, expiresAt time.Time) error

	// LatestActiveLock 查询某用户最近一次尚未过期的锁定事件。
	// 优先 Redis（O(1)），fallback DB。
	// 返回 nil 表示无活跃锁定。
	LatestActiveLock(ctx context.Context, username string, now time.Time) (*model.LoginAudit, error)

	// RecentFailuresCount 统计某用户最近 N 小时内的失败次数。
	// 优先 Redis GET 计数，fallback DB Count（DB 结果回填 Redis）。
	RecentFailuresCount(ctx context.Context, username string, since time.Time) (int, error)

	// ClearFailures 清除某用户的失败记录（登录成功后调用）：
	// DB 删 failure 记录 → Redis Del auth:fail:{user}。
	ClearFailures(ctx context.Context, username string) error

	// CleanupExpired 删除 occurredAt 早于 cutoff 的所有记录（janitor 调用）。
	// 返回删除的行数。
	CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error)
}

// loginAuditRepo LoginAuditRepository 接口的 GORM 实现（DB 主源 + Redis 加速）。
type loginAuditRepo struct {
	db    *infra.DB
	cache *cache.Client
	ttl   LockTTL
	log   *zap.SugaredLogger
}

// NewLoginAuditRepository 构造仓储。
//
// cache / ttl / log 可为 nil（向后兼容测试用例 / 单元测试）。
//   - cache == nil → 纯 DB 模式（所有 Redis 调用被 skip）
//   - log == nil   → Redis 失败不打印 warn（避免 nil panic）
func NewLoginAuditRepository(db *infra.DB, c *cache.Client, ttl LockTTL, log *zap.SugaredLogger) LoginAuditRepository {
	return &loginAuditRepo{db: db, cache: c, ttl: ttl, log: log}
}

// RecordFailure 记录一次失败事件（先 DB → 后 Redis 计数）。
//
// 在线清理（应用 project_rules.md 第 19 条「写入路径在线清理」）：
// 每次写入后 `DELETE WHERE username=? AND event_type='failure' AND created_at < ? LIMIT 1000`，
// 避免 janitor 单次 DELETE 过多行（DB 长事务 / 锁等待）。
//
// 时间字段注意：created_at 是 int64 unix 时间戳，查询条件必须传 int64，
// 不能传 time.Time（GORM 不会自动转换 time.Time → int64，比较会失败）。
func (r *loginAuditRepo) RecordFailure(ctx context.Context, username, ip string, occurredAt time.Time) error {
	// 1. 先写 DB（事务）
	if err := r.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	}); err != nil {
		return err
	}

	// 2. DB 成功后 → Redis 计数 +1 + 重设 TTL
	//    Redis 失败仅 warn，**不**回滚 DB（DB 已是真源，攻击者刷不到也无所谓）。
	if r.cache == nil {
		return nil
	}
	if _, err := r.cache.IncrWithTTL(ctx, redisKeyFail+username, 24*time.Hour); err != nil {
		r.log.Warnw("audit.cache.incr_failed",
			"username", username, "key", redisKeyFail+username, "error", err)
	}
	return nil
}

// RecordLock 记录一次锁定事件（先 DB → 后 Redis 锁 key）。
func (r *loginAuditRepo) RecordLock(ctx context.Context, username, level string, failedCount int, occurredAt, expiresAt time.Time) error {
	// 1. DB 写
	m := &model.LoginAudit{
		Username:    username,
		EventType:   level,
		FailedCount: failedCount,
		ExpiresAt:   expiresAt.Unix(),
	}
	if err := r.db.Write.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}

	// 2. DB 成功 → Redis 锁 key（带 TTL = DB 记录的 expires_at - now）
	if r.cache == nil {
		return nil
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		// 已过期，不写 Redis
		return nil
	}
	key := redisKeyLockShort + username
	if level == model.EventLockLong {
		key = redisKeyLockLong + username
	}
	if _, err := r.cache.SetNX(ctx, key, 1, ttl); err != nil {
		r.log.Warnw("audit.cache.set_lock_failed",
			"username", username, "level", level, "key", key, "error", err)
	}
	return nil
}

// LatestActiveLock 查询最近一次尚未过期的 lock 事件（Redis 优先 → DB 兜底）。
//
// 读路径策略：
//  1. 先 Redis GET auth:lock:long:{user}（长锁优先），存在则返 LockLong
//  2. 再 Redis GET auth:lock:short:{user}，存在则返 LockShort
//  3. Redis 都没有 → 查 DB（LatestActiveLock 走 GORM）
//  4. DB 查到 → 写回 Redis（恢复缓存一致性）
//
// 时间字段注意：expires_at 是 int64 unix 时间戳，now 必须转 .Unix() 才能正确比较。
func (r *loginAuditRepo) LatestActiveLock(ctx context.Context, username string, now time.Time) (*model.LoginAudit, error) {
	// 1. Redis 优先
	if r.cache != nil {
		// 长锁优先（覆盖关系：长锁期间短锁 key 可能存在但被忽略）
		_, ttl, err := r.cache.GetWithTTL(ctx, redisKeyLockLong+username)
		if err == nil && ttl > 0 {
			return &model.LoginAudit{
				Username:  username,
				EventType: model.EventLockLong,
				CreatedAt: now.Add(-ttl).Unix(),
				ExpiresAt: now.Add(ttl).Unix(),
			}, nil
		}
		if err != nil && !errors.Is(err, cache.ErrNil) {
			r.log.Warnw("audit.cache.get_lock_long_failed", "username", username, "error", err)
		}
		_, ttl, err = r.cache.GetWithTTL(ctx, redisKeyLockShort+username)
		if err == nil && ttl > 0 {
			return &model.LoginAudit{
				Username:  username,
				EventType: model.EventLockShort,
				CreatedAt: now.Add(-ttl).Unix(),
				ExpiresAt: now.Add(ttl).Unix(),
			}, nil
		}
		if err != nil && !errors.Is(err, cache.ErrNil) {
			r.log.Warnw("audit.cache.get_lock_short_failed", "username", username, "error", err)
		}
	}

	// 2. Redis miss → DB 查
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

	// 3. DB 查到 → 回填 Redis（恢复缓存一致性）
	if r.cache != nil {
		ttl := time.Until(time.Unix(m.ExpiresAt, 0))
		if ttl > 0 {
			key := redisKeyLockShort + username
			if m.EventType == model.EventLockLong {
				key = redisKeyLockLong + username
			}
			if _, err := r.cache.SetNX(ctx, key, 1, ttl); err != nil {
				r.log.Warnw("audit.cache.refill_lock_failed",
					"username", username, "level", m.EventType, "error", err)
			}
		}
	}
	return &m, nil
}

// RecentFailuresCount 统计最近 since 以来某用户的 failure 次数（Redis 优先 → DB 兜底）。
//
// 时间字段注意：created_at 是 int64 unix 时间戳，since 必须转 .Unix() 才能正确比较。
func (r *loginAuditRepo) RecentFailuresCount(ctx context.Context, username string, since time.Time) (int, error) {
	// 1. Redis GET（count 直接存为 key 的 value）
	if r.cache != nil {
		val, err := r.cache.Get(ctx, redisKeyFail+username)
		if err == nil && val != "" {
			// 解析成功直接返回（节省一次 DB Count）
			if n, atoiErr := strconv.Atoi(val); atoiErr == nil {
				return n, nil
			}
		}
		if err != nil && !errors.Is(err, cache.ErrNil) {
			r.log.Warnw("audit.cache.get_fail_count_failed", "username", username, "error", err)
		}
	}

	// 2. Redis miss / 失败 → DB Count
	var count int64
	err := r.db.RO().WithContext(ctx).
		Model(&model.LoginAudit{}).
		Where("username = ? AND event_type = ? AND created_at >= ?",
			username, model.EventFailure, since.Unix()).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// 3. DB 查到 → 回填 Redis（24h TTL，与计数窗口一致）
	if r.cache != nil && count > 0 {
		if err := r.cache.Set(ctx, redisKeyFail+username, count, 24*time.Hour); err != nil {
			r.log.Warnw("audit.cache.refill_fail_count_failed", "username", username, "error", err)
		}
	}
	return int(count), nil
}

// ClearFailures 清除某用户的失败记录（DB 删 → Redis Del）。
//
// 登录成功时调用，重置失败计数器。
// 不删除 lock 记录（防攻击者试探到 4 次后故意输对 1 次再继续刷）。
func (r *loginAuditRepo) ClearFailures(ctx context.Context, username string) error {
	// 1. DB 删
	if err := r.db.Write.WithContext(ctx).
		Where("username = ? AND event_type = ?", username, model.EventFailure).
		Delete(&model.LoginAudit{}).Error; err != nil {
		return err
	}

	// 2. Redis Del
	if r.cache == nil {
		return nil
	}
	if err := r.cache.Del(ctx, redisKeyFail+username); err != nil {
		r.log.Warnw("audit.cache.clear_fail_failed", "username", username, "error", err)
	}
	return nil
}

// CleanupExpired 删除 occurredAt < cutoff 的所有记录。
func (r *loginAuditRepo) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.Write.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&model.LoginAudit{})
	return res.RowsAffected, res.Error
}
