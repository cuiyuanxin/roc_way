// Package scheduler 基于 robfig/cron/v3 提供进程内任务调度。
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 进程内 cron。
type Scheduler struct {
	c   *cron.Cron
	log Logger
}

// Logger 抽象 zap.SugaredLogger 的最小接口，便于注入与单测。
type Logger interface {
	Errorw(msg string, keysAndValues ...any)
}

// New 创建 Scheduler（秒级粒度）。
func New(log Logger) *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{c: c, log: log}
}

// Cron 注册 cron 表达式任务。
func (s *Scheduler) Cron(expr string, job func(ctx context.Context)) error {
	id, err := s.c.AddFunc(expr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				s.log.Errorw("scheduler: job panic", "expr", expr, "err", fmt.Sprintf("%v", r))
			}
		}()
		job(ctx)
	})
	if err != nil {
		return fmt.Errorf("scheduler: add %q: %w", expr, err)
	}
	_ = id
	return nil
}

// Start 启动调度循环。
func (s *Scheduler) Start() { s.c.Start() }

// Stop 优雅停止。
func (s *Scheduler) Stop(_ context.Context) error {
	<-s.c.Stop().Done()
	return nil
}
