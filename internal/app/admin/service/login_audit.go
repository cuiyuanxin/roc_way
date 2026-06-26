// Package service: 账号锁定服务（应用层）。
//
// 双存储策略（应用 project_rules.md 第 19 条）：
//   - Redis 主存（auth:fail: / auth:lock:short: / auth:lock:long:）
//   - MySQL 兜底（login_audits 表）
//   - Redis 故障 → 降级查 DB；Redis 写失败 → 同步写 DB；DB 也失败 → zap error 但业务不阻断。
//
// 业务不变量：
//   - 成功登录重置失败计数（Redis Del + DB ClearFailures）
//   - 锁定到期自动解锁（Redis TTL / DB expires_at）
//   - 不主动 Del lock 记录（防攻击者试探到 4 次后故意输对 1 次再继续）
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/domain"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/notify"
)

// redisLockMeta Redis 中存储的锁定元数据（JSON）。
type redisLockMeta struct {
	Level       string    `json:"level"`
	FailedCount int       `json:"failed_count"`
	LockedAt    time.Time `json:"locked_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// LockService 账号锁定服务。
type LockService struct {
	auditRepo repository.LoginAuditRepository
	cache     *cache.Client
	notifier  notify.Notifier
	policy    config.LoginPolicyConfig
	log       *zap.SugaredLogger
}

// NewLockService 构造锁定服务。
func NewLockService(
	auditRepo repository.LoginAuditRepository,
	c *cache.Client,
	n notify.Notifier,
	policy config.LoginPolicyConfig,
	log *zap.SugaredLogger,
) *LockService {
	return &LockService{
		auditRepo: auditRepo,
		cache:     c,
		notifier:  n,
		policy:    policy,
		log:       log,
	}
}

// failKey Redis 失败计数 key。
func failKey(username string) string { return fmt.Sprintf("auth:fail:%s", username) }

// shortLockKey Redis 短期锁定 key。
func shortLockKey(username string) string { return fmt.Sprintf("auth:lock:short:%s", username) }

// longLockKey Redis 长期锁定 key。
func longLockKey(username string) string { return fmt.Sprintf("auth:lock:long:%s", username) }

// GetLock 查询某用户的活跃锁定。
//
// 读取顺序：Redis → MySQL 兜底；任何失败都继续向下走（业务不阻断）。
// 返回 nil 表示无活跃锁定。
func (s *LockService) GetLock(ctx context.Context, username string) *domain.AccountLock {
	now := time.Now()

	// 1. 优先 Redis（短期 / 长期谁后写谁生效）
	if s.cache != nil {
		// 检查长期锁定（优先）
		if lock, ok := s.getRedisLock(ctx, longLockKey(username)); ok && lock.ExpiresAt.After(now) {
			return &domain.AccountLock{
				Username:    username,
				Level:       domain.LockLong,
				FailedCount: lock.FailedCount,
				LockedAt:    lock.LockedAt,
				ExpiresAt:   lock.ExpiresAt,
			}
		}
		// 检查短期锁定
		if lock, ok := s.getRedisLock(ctx, shortLockKey(username)); ok && lock.ExpiresAt.After(now) {
			return &domain.AccountLock{
				Username:    username,
				Level:       domain.LockShort,
				FailedCount: lock.FailedCount,
				LockedAt:    lock.LockedAt,
				ExpiresAt:   lock.ExpiresAt,
			}
		}
	}

	// 2. Redis 失败 / 无数据 → 降级 DB
	if s.auditRepo != nil {
		m, err := s.auditRepo.LatestActiveLock(ctx, username, now)
		if err != nil {
			s.log.Errorw("lock.get_db_failed", "username", username, "error", err.Error())
			return nil
		}
		if m == nil {
			return nil
		}
		level := domain.LockShort
		if m.EventType == model.EventLockLong {
			level = domain.LockLong
		}
		// 修复 [C2]：DB 兜底路径必须把 ExpiresAt / LockedAt 转回 time.Time，
		// 否则 AccountLock.Expired() 中 time.Now().After(time.Time{}) 永远 true，
		// Active() 永远 false，DB 中的 lock 记录形同虚设（安全漏洞）。
		return &domain.AccountLock{
			Username:    username,
			Level:       level,
			FailedCount: m.FailedCount,
			LockedAt:    time.Unix(m.CreatedAt, 0),
			ExpiresAt:   time.Unix(m.ExpiresAt, 0),
		}
	}

	return nil
}

// getRedisLock 从 Redis 读取锁定元数据。
func (s *LockService) getRedisLock(ctx context.Context, key string) (redisLockMeta, bool) {
	raw, err := s.cache.Get(ctx, key)
	if err != nil {
		return redisLockMeta{}, false
	}
	var meta redisLockMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return redisLockMeta{}, false
	}
	return meta, true
}

// RecordFailure 记录一次登录失败 + 判断是否触发锁定。
//
// 返回最新的锁定级别（LockNone / LockShort / LockLong）。
//
// 流程：
//  1. Redis INCR 失败计数（24h 滚动窗口）
//  2. 写 DB failure 记录（带在线清理）
//  3. 若新计数 >= long_threshold → 写长期锁定
//  4. 否则若新计数 >= short_threshold → 写短期锁定
//  5. Notify 推送
func (s *LockService) RecordFailure(ctx context.Context, username, ip string) domain.LockLevel {
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	// 1. 失败计数（Redis 优先，DB 兜底）
	count := s.recordFailureCount(ctx, username, ip, now, cutoff)

	level := domain.LockNone
	var duration time.Duration
	switch {
	case count >= s.policy.LongThreshold:
		level = domain.LockLong
		duration = s.policy.LongDuration
	case count >= s.policy.ShortThreshold:
		level = domain.LockShort
		duration = s.policy.ShortDuration
	default:
		_ = s.cache // 兼容未来扩展：仅记录，不触发锁定
		return domain.LockNone
	}

	// 2. 写锁定（Redis 主存 + DB 兜底）
	expiresAt := now.Add(duration)
	s.writeLock(ctx, username, level, count, now, expiresAt)

	// 3. 通知安全管理员（异步，Notifier 不返回 error / 不 panic）
	if s.notifier != nil {
		eventType := "account_locked_short"
		if level == domain.LockLong {
			eventType = "account_locked_long"
		}
		s.notifier.Notify(ctx, notify.Event{
			Type:        eventType,
			Username:    username,
			Level:       level.String(),
			IP:          ip,
			FailedCount: count,
			OccurredAt:  now,
		})
	}

	return level
}

// recordFailureCount 累加失败计数（Redis INCR，失败降级 DB Count）。
func (s *LockService) recordFailureCount(ctx context.Context, username, ip string, now, cutoff time.Time) int {
	// 1. Redis INCR + 24h 滚动
	if s.cache != nil {
		count, err := s.cache.IncrWithTTL(ctx, failKey(username), 24*time.Hour)
		if err == nil {
			// 写 DB failure 记录（含在线清理）
			s.recordFailureDB(ctx, username, ip, now)
			return int(count)
		}
		s.log.Errorw("lock.incr_failed", "username", username, "error", err.Error())
	}

	// 2. Redis 失败 → DB 兜底：用 COUNT 估算 + +1
	if s.auditRepo != nil {
		count, err := s.auditRepo.RecentFailuresCount(ctx, username, cutoff)
		if err != nil {
			s.log.Errorw("lock.db_count_failed", "username", username, "error", err.Error())
			return 0
		}
		s.recordFailureDB(ctx, username, ip, now)
		return count + 1
	}
	return 0
}

// recordFailureDB 写 DB failure 记录。
func (s *LockService) recordFailureDB(ctx context.Context, username, ip string, now time.Time) {
	if s.auditRepo == nil {
		return
	}
	if err := s.auditRepo.RecordFailure(ctx, username, ip, now); err != nil {
		s.log.Errorw("lock.record_failure_failed", "username", username, "error", err.Error())
	}
}

// writeLock 写锁定（Redis 主存 + DB 兜底）。
func (s *LockService) writeLock(ctx context.Context, username string, level domain.LockLevel, failedCount int, lockedAt, expiresAt time.Time) {
	eventType := model.EventLockShort
	key := shortLockKey(username)
	if level == domain.LockLong {
		eventType = model.EventLockLong
		key = longLockKey(username)
	}
	duration := time.Until(expiresAt)
	if duration <= 0 {
		duration = time.Minute // 兜底
	}

	// 1. Redis SetNX + TTL（避免覆盖更晚的锁定）
	if s.cache != nil {
		meta := redisLockMeta{
			Level:       level.String(),
			FailedCount: failedCount,
			LockedAt:    lockedAt,
			ExpiresAt:   expiresAt,
		}
		data, _ := json.Marshal(meta)
		if _, err := s.cache.SetNX(ctx, key, string(data), duration); err != nil {
			s.log.Errorw("lock.setnx_failed", "username", username, "error", err.Error())
		}
	}

	// 2. DB 兜底写
	if s.auditRepo != nil {
		if err := s.auditRepo.RecordLock(ctx, username, eventType, failedCount, lockedAt, expiresAt); err != nil {
			s.log.Errorw("lock.record_db_failed", "username", username, "error", err.Error())
		}
	}
}

// ClearFailures 登录成功后清除失败计数。
//
// 不删除 lock 记录（防攻击者试探到 4 次后故意输对 1 次）。
func (s *LockService) ClearFailures(ctx context.Context, username string) {
	if s.cache != nil {
		if err := s.cache.Del(ctx, failKey(username)); err != nil {
			s.log.Errorw("lock.clear_redis_failed", "username", username, "error", err.Error())
		}
	}
	if s.auditRepo != nil {
		if err := s.auditRepo.ClearFailures(ctx, username); err != nil {
			s.log.Errorw("lock.clear_db_failed", "username", username, "error", err.Error())
		}
	}
}
