package logger

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// GORM 日志级别（与 gorm.io/gorm/logger.LogLevel 对齐，类型别名避免业务代码直接 import gorm/logger）。
//
// 与 [gormlogger.LogLevel] 的对应关系：
//
//	Silent → 不写任何日志
//	Error  → 只记录失败查询
//	Warn   → Error + 慢查询
//	Info   → Error + 慢查询 + 所有成功查询
const (
	GormLogSilent gormlogger.LogLevel = gormlogger.Silent
	GormLogError  gormlogger.LogLevel = gormlogger.Error
	GormLogWarn   gormlogger.LogLevel = gormlogger.Warn
	GormLogInfo   gormlogger.LogLevel = gormlogger.Info
)

// gormLogger 把 GORM 日志接入 [Loggers] 的 db channel。
//
// 实现要点：
//   - Info / Warn / Error 三个简单方法：透传到 db logger 对应级别。
//   - Trace 是 GORM 真正调用的入口：拿到 SQL / rows / 耗时 / err，一次性写一条结构化日志。
//   - SlowThreshold 触发 warn；err != nil 触发 error；Info 级别时记录所有查询。
//   - caller 通过 channelSpec.CallerSkip 已跳过 GORM 栈层，日志里 caller 字段
//     指向触发该查询的业务 repository 代码（如 user_gorm.go:35），便于运维定位。
type gormLogger struct {
	log           *zap.SugaredLogger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger 构造 GORM logger.Interface。
//
// 入参：
//   - l: 总 logger；nil 时返回 nil（调用方应跳过 GORM.Logger 注入，沿用 gorm 默认）。
//   - level: GORM 日志级别（GormLogSilent/Error/Warn/Info）。
//   - slowThreshold: 慢查询阈值；0 表示不记录慢查询。
//
// 返回值可直接传给 gorm.Config{Logger: ...}；返回 nil 时调用方需自行兜底。
func NewGormLogger(l *Loggers, level gormlogger.LogLevel, slowThreshold time.Duration) gormlogger.Interface {
	if l == nil {
		return nil
	}
	dbLog := l.DB()
	if dbLog == nil {
		return nil
	}
	return &gormLogger{log: dbLog, level: level, slowThreshold: slowThreshold}
}

// LogMode 切换日志级别并返回新实例（GORM 推荐做法：返回新对象避免并发问题）。
func (g *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return &gormLogger{log: g.log, level: level, slowThreshold: g.slowThreshold}
}

// Info 实现 gormlogger.Interface。
func (g *gormLogger) Info(_ context.Context, msg string, args ...any) {
	if g.level < gormlogger.Info {
		return
	}
	g.log.Infow(msg, args...)
}

// Warn 实现 gormlogger.Interface。
func (g *gormLogger) Warn(_ context.Context, msg string, args ...any) {
	if g.level < gormlogger.Warn {
		return
	}
	g.log.Warnw(msg, args...)
}

// Error 实现 gormlogger.Interface。
func (g *gormLogger) Error(_ context.Context, msg string, args ...any) {
	if g.level < gormlogger.Error {
		return
	}
	g.log.Errorw(msg, args...)
}

// Trace 是 GORM 在每次 query / exec / create / update / delete 结束时的统一入口。
//
// 决策树：
//   - err != nil 且非「记录不存在」 → 写 error 级别，附 SQL / rows / 耗时
//   - 超过 slowThreshold          → 写 warn 级别
//   - level >= Info               → 写 info 级别（成功查询也记）
//   - 其它                        → 跳过
func (g *gormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()
	fields := []any{
		"elapsed_ms", elapsed.Milliseconds(),
		"rows", rows,
		"sql", sql,
	}

	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound):
		// 业务真错误（非「记录不存在」这种预期结果）。
		fields = append(fields, "err", err.Error())
		g.log.Errorw("gorm_error", fields...)
	case g.slowThreshold > 0 && elapsed >= g.slowThreshold:
		g.log.Warnw("gorm_slow", fields...)
	case g.level >= gormlogger.Info:
		g.log.Infow("gorm_query", fields...)
	}
}

// Compile-time assertion: gormLogger 实现 gormlogger.Interface。
var _ gormlogger.Interface = (*gormLogger)(nil)
