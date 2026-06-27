// Package domain 账号锁定领域模型。
package domain

import "time"

// LockLevel 锁定级别。
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

// AccountLock 账号锁定聚合。
type AccountLock struct {
	Username    string
	Level       LockLevel
	FailedCount int
	LockedAt    time.Time
	ExpiresAt   time.Time
}

// Expired 锁定是否已过期。
func (a *AccountLock) Expired() bool {
	return time.Now().After(a.ExpiresAt)
}

// Active 锁定是否生效（未过期）。
func (a *AccountLock) Active() bool {
	return a != nil && a.Level != LockNone && !a.Expired()
}