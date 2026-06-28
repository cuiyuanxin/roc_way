// Package service 内的「登录锁定」子模块（独立文件便于阅读，但同包内可见）。
package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/pkg/notify"
)

// LockPolicy 锁定策略（项目规则 19：5 次连败锁 15min / 10 次连败锁 24h）。
//
// app.go 直接构造本结构体（从 config.LoginPolicyConfig 字段平铺过来），
// 不提供「FromConfig」类的工厂方法——少一个间接层，调用方一眼看清传给
// NewLockService 的是什么。
type LockPolicy struct {
	Window         time.Duration // 计数窗口（与 LoginPolicyConfig.AuditRetention 保持一致）
	ShortThreshold int           // 短期锁定阈值
	ShortDuration  time.Duration // 短期锁定时长
	LongThreshold  int           // 长期锁定阈值
	LongDuration   time.Duration // 长期锁定时长
}

// DefaultLockPolicy 默认锁定策略。
func DefaultLockPolicy() LockPolicy {
	return LockPolicy{
		Window:         24 * time.Hour,
		ShortThreshold: 5,
		ShortDuration:  15 * time.Minute,
		LongThreshold:  10,
		LongDuration:   24 * time.Hour,
	}
}

// LockService 登录失败 / 锁定编排服务（双存储：Redis 主 + MySQL 兜底）。
//
// 流程（应用 project_rules.md 第 19 条「失败锁定」）：
//  1. Redis INCR 失败计数（24h 窗口）+ 写 DB failure 记录
//  2. 短锁（>=5）→ Redis SetNX auth:lock:short:* + DB 写 lock_short
//  3. 长锁（>=10）→ Redis SetNX auth:lock:long:* + DB 写 lock_long
//  4. Notifier 通知（异步，不返回 error）
//  5. 短锁期间继续失败：仍累加 count，达到 long 阈值时强制升级
//
// 成功登录（ClearFailures）只清失败计数 + 删 failure 记录，**不删 lock 记录**
// （防攻击者试探到 4 次后故意输对 1 次再继续刷）。
type LockService struct {
	repo     repository.LoginAuditRepository
	notifier notify.Notifier
	policy   LockPolicy
	now      func() time.Time
}

// NewLockService 构造锁定服务。
func NewLockService(
	repo repository.LoginAuditRepository,
	notifier notify.Notifier,
	policy LockPolicy,
	log *zap.SugaredLogger,
) *LockService {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	if policy.ShortThreshold <= 0 || policy.LongThreshold <= policy.ShortThreshold ||
		policy.ShortDuration <= 0 || policy.LongDuration <= policy.ShortDuration {
		panic("lock_policy: 阈值/时长非法（要求 ShortThreshold>0 且 LongThreshold>ShortThreshold 且 LongDuration>ShortDuration）")
	}
	return &LockService{
		repo:     repo,
		notifier: notifier,
		policy:   policy,
		now:      time.Now,
	}
}

// GetLock 查询当前账号的活跃锁定状态。
//
// **关键不变量**：本方法**不增加**任何失败计数、**不写** DB 记录、**不触发** 任何通知，
// 只读。可能返回 nil（未锁定）。
func (s *LockService) GetLock(ctx context.Context, username string) *AccountLock {
	cur, err := s.repo.LatestActiveLock(ctx, username, s.now())
	if err != nil {
		// 查询失败按未锁定处理（业务不阻断）。
		return &AccountLock{Level: LockNone}
	}
	if cur == nil {
		return &AccountLock{Level: LockNone}
	}
	// model.LoginAudit → service.AccountLock 映射
	level := LockShort
	if cur.EventType == model.EventLockLong {
		level = LockLong
	}
	return &AccountLock{
		Username:    cur.Username,
		Level:       level,
		FailedCount: cur.FailedCount,
		LockedAt:    time.Unix(cur.CreatedAt, 0),
		ExpiresAt:   time.Unix(cur.ExpiresAt, 0),
	}
}

// RecordFailure 记录一次登录失败 + 判断是否触发锁定。
//
// 返回最新的锁定级别（LockNone / LockShort / LockLong）。
//
// 升级逻辑：
//  1. 失败计数 < short_threshold：返回 LockNone，仅写 failure 记录
//  2. 失败计数 >= short_threshold & 当前无锁：写 short lock
//  3. 失败计数 >= long_threshold & 当前无 short lock：写 long lock
//  4. 失败计数 >= long_threshold & 已有 short lock：写 long lock（覆盖升级）
//  5. 已有 long lock：不动（长锁到底）
func (s *LockService) RecordFailure(ctx context.Context, username, ip string) LockLevel {
	now := s.now()
	cutoff := now.Add(-s.policy.Window)

	// 1. 写 failure + 累计 count
	if err := s.repo.RecordFailure(ctx, username, ip, now); err != nil {
		// 记录失败不阻断业务（仅 zap）
	}
	count, err := s.repo.RecentFailuresCount(ctx, username, cutoff)
	if err != nil {
		count = 0
	}

	// 2. 判断是否要写/升级锁定
	newLevel := LockNone
	var duration time.Duration
	switch {
	case count >= s.policy.LongThreshold:
		newLevel = LockLong
		duration = s.policy.LongDuration
	case count >= s.policy.ShortThreshold:
		newLevel = LockShort
		duration = s.policy.ShortDuration
	default:
		return LockNone
	}

	// 3. 当前是否有活跃锁？
	cur := s.GetLock(ctx, username)
	// 同级别不重复写
	if cur != nil && cur.Active() && cur.Level == newLevel {
		return newLevel
	}

	// 4. 写新锁
	expiresAt := now.Add(duration)
	eventType := model.EventLockShort
	if newLevel == LockLong {
		eventType = model.EventLockLong
	}
	if err := s.repo.RecordLock(ctx, username, eventType, count, now, expiresAt); err != nil {
		// 写锁失败不阻断业务
	}

	// 5. 通知
	if s.notifier != nil {
		notifyEventType := "account_locked_short"
		if newLevel == LockLong {
			notifyEventType = "account_locked_long"
		}
		s.notifier.Notify(ctx, notify.Event{
			Type:        notifyEventType,
			Username:    username,
			Level:       newLevel.String(),
			IP:          ip,
			FailedCount: count,
			OccurredAt:  now,
		})
	}
	return newLevel
}

// ClearFailures 成功登录时清失败计数 + 删 failure 记录，**保留** lock 记录。
func (s *LockService) ClearFailures(ctx context.Context, username string) {
	_ = s.repo.ClearFailures(ctx, username)
}

// ===== LockLevel / AccountLock =====
//
// 这两个类型只在本文件（service/LoginLockService）内部消费，
// 用来表达「账号当前是否锁定 + 锁定到什么级别」——运行期状态，
// 不是持久化映射，因此放 service 包内（不放 model/）。
// model/ 只装 GORM 表结构（LoginAudit、User 等）。

// LockLevel 账号锁定级别。
type LockLevel int

const (
	LockNone  LockLevel = iota // 未锁定
	LockShort                  // 短期：5-9 次连败，15 分钟
	LockLong                   // 长期：>= 10 次连败，24 小时
)

// String 返回级别字符串。
func (l LockLevel) String() string {
	switch l {
	case LockShort:
		return "short"
	case LockLong:
		return "long"
	default:
		return "none"
	}
}

// AccountLock 账号锁定状态（运行期聚合体，非 GORM 映射）。
type AccountLock struct {
	Username    string
	Level       LockLevel
	FailedCount int
	LockedAt    time.Time
	ExpiresAt   time.Time
}

// Expired 锁定是否已过期。
func (a *AccountLock) Expired() bool { return time.Now().After(a.ExpiresAt) }

// Active 锁定是否生效（未过期）。
func (a *AccountLock) Active() bool { return a != nil && a.Level != LockNone && !a.Expired() }
