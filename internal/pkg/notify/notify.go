// Package notify 提供安全事件通知抽象（Notifier）与默认 noop 实现。
//
// 设计目标：
//   - 登录失败 / 账号锁定等安全事件可通过统一接口推送给安全管理员；
//   - 默认实现 NoopNotifier 仅记录 zap 安全日志（logs/security.log），不接外部通道；
//   - 未来接邮件 / 钉钉 / IM 时**新增实现体**，业务代码零改动。
//
// 接口签名强制约束（应用 project_rules.md 第 19 条）：
//   - Notify **不**返回 error；
//   - Notify **不**允许 panic；
//   - 实现体内部 swallow 错误并 zap 日志；
//   - 避免「推送系统故障拖垮登录」。
package notify

import (
	"context"
	"time"
)

// Event 安全事件负载。
//
// Type 取值：
//   - "account_locked_short"  短期锁定（5-9 次连败，15 分钟）
//   - "account_locked_long"   长期锁定（>= 10 次连败，24 小时）
type Event struct {
	Type        string    `json:"type"`
	Username    string    `json:"username"`
	Level       string    `json:"level"` // "short" | "long"
	IP          string    `json:"ip"`
	FailedCount int       `json:"failed_count"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// Notifier 通知发送器抽象。
//
// 实现体可以同步阻塞（短超时）发送，也可以异步丢队列。
// 但**禁止**：
//   - panic（任何 panic 都会被业务调用方透传，影响登录主流程）
//   - 返回 error（签名不带 error，实现体内部必须 swallow）
type Notifier interface {
	Notify(ctx context.Context, event Event)
}