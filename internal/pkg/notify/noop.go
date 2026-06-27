// Package notify: noop 默认实现 + zap 日志。
package notify

import (
	"context"

	"go.uber.org/zap"
)

// NoopNotifier 默认实现：仅记录安全日志。
//
// 适用场景：
//   - 测试环境；
//   - 暂无外部推送通道的生产环境（运维通过 logs/security.log 监控）；
//   - Notifier 接口的兜底实现（即使没配置具体通道，业务也可注入）。
//
// 实现体内**禁止** panic / 返回 error。
type NoopNotifier struct {
	Log *zap.SugaredLogger
}

// NewNoopNotifier 构造 NoopNotifier。
func NewNoopNotifier(log *zap.SugaredLogger) *NoopNotifier {
	return &NoopNotifier{Log: log}
}

// Notify 记录安全事件到 zap 日志。
func (n *NoopNotifier) Notify(_ context.Context, e Event) {
	if n.Log == nil {
		return
	}
	n.Log.Warnw("security_event",
		"type", e.Type,
		"username", e.Username,
		"level", e.Level,
		"ip", e.IP,
		"failed_count", e.FailedCount,
		"occurred_at", e.OccurredAt,
	)
}