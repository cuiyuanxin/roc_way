// Package janitor 后台定时清理任务。
//
// 设计目标：
//   - 周期性清理过期数据（如 login_audits）；
//   - 独立 goroutine，通过 context 控制生命周期；
//   - 与 project_rules.md 第 17 条「禁止 init()」一致，**显式启动**；
//   - 与第 19 条 janitor 约束一致：失败记 zap error，但**不 panic、不阻断其他 janitor**。
package janitor

import (
	"context"
	"time"
)

// CleanupFunc 清理函数。
//
// 参数：
//   - ctx: 上下文，goroutine 退出信号；
//   - cutoff: 早于该时间的记录视为过期。
//
// 返回：
//   - affected: 影响的行数（用于日志）；
//   - error: 错误（**禁止 panic**）。
type CleanupFunc func(ctx context.Context, cutoff time.Time) (affected int64, err error)

// Janitor 单个清理任务。
type Janitor struct {
	Name     string
	Interval time.Duration
	Retention time.Duration // 数据保留时长（早于 now-Retention 的视为过期）
	Cleanup  CleanupFunc
}

// Run 启动 janitor goroutine。
//
// 返回的 stop 函数会取消 goroutine；调用方**必须**在退出时调用 stop 避免泄漏。
func (j *Janitor) Run(ctx context.Context, onError func(err error, name string)) (stop func()) {
	if j.Interval <= 0 || j.Cleanup == nil {
		return func() {} // 禁用
	}
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(j.Interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-j.Retention)
				if _, err := j.Cleanup(ctx, cutoff); err != nil {
					if onError != nil {
						onError(err, j.Name)
					}
				}
			}
		}
	}()
	return cancel
}

// Runners 管理多个 janitor 任务。
type Runners struct {
	cancels []func()
}

// StartAll 启动所有 janitor，返回 Runners 句柄。
func StartAll(ctx context.Context, janitors []*Janitor, onError func(err error, name string)) *Runners {
	r := &Runners{}
	for _, j := range janitors {
		if j == nil {
			continue
		}
		r.cancels = append(r.cancels, j.Run(ctx, onError))
	}
	return r
}

// Stop 停止所有 janitor。
func (r *Runners) Stop() {
	for _, cancel := range r.cancels {
		cancel()
	}
	r.cancels = nil
}