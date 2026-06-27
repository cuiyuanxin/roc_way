// Package wire: wire.go 是 wire.Build 的入口。
//
// 由 `wire ./internal/wire` 生成 wire_gen.go；InitApp 是启动入口。
//go:build wireinject
// +build wireinject

package wire

import (
	"context"
	"strings"

	"github.com/google/wire"
	gormlogger "gorm.io/gorm/logger"

	"github.com/cuiyuanxin/roc_way/internal/app/admin"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/logger"
	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// InitApp 由 wire 生成，调用者通过 wire_gen.go 实际执行注入。
//
// 修复 [C7]：返回的 cleanup 函数集中执行 app.Close + db.Close + cache.Close，
// 避免 janitor / Hub / 限流器 / DB / Redis 任一资源泄漏。
func InitApp(ctx context.Context, cfg config.Config) (*admin.App, func(), error) {
	wire.Build(
		wire.FieldsOf(&cfg, "Database", "Cache", "Auth"),
		provideLogger,
		provideGormLogger,
		database.Open,
		provideCache,
		auth.New,
		provideEnforcer,
		realtime.NewHub,
		wire.Struct(new(admin.Deps), "Cfg", "Log", "DB", "Cache", "Auth", "Enforcer", "Hub"),
		admin.NewApp,
	)
	return nil, nil, nil
}

// provideLogger 把 logger.Config 包装为 *logger.Loggers。
func provideLogger(cfg config.Config) (*logger.Loggers, error) {
	return logger.New(logger.Config{
		Level:  cfg.Logger.Level,
		Dir:    cfg.Logger.Dir,
		MaxMB:  cfg.Logger.MaxMB,
		Backup: cfg.Logger.Backup,
	})
}

// provideGormLogger 根据 cfg.Logger.DBEnabled 决定是否把 GORM 日志接入 db channel。
//
// DBEnabled=false（默认）→ 返回 nil → database.Open 走 gorm 默认 logger，
// 完全不写 db.log，不增加任何 IO 开销，符合「server features opt-in」约束。
//
// DBEnabled=true  → 按 cfg.Logger.DBLogLevel / DBSlowThreshold 装配 [logger.NewGormLogger]，
// 所有 GORM Info/Warn/Error/Trace 都会写入 logs/db.log，caller 指向业务 repository。
func provideGormLogger(cfg config.Config, l *logger.Loggers) gormlogger.Interface {
	if !cfg.Logger.DBEnabled {
		return nil
	}
	return logger.NewGormLogger(l, parseGormLevel(cfg.Logger.DBLogLevel), cfg.Logger.DBSlowThreshold)
}

// parseGormLevel 把 yaml 字符串映射为 gorm logger 级别，未识别值降级为 Warn。
//
// 字符串大小写不敏感，匹配：silent/error/warn/info。
func parseGormLevel(s string) gormlogger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	case "warn", "":
		return gormlogger.Warn
	default:
		return gormlogger.Warn
	}
}

// provideCache 注入 cache.New，把 *logger.Loggers 拆出 api logger 传入 cache.New。
//
// 强制：cache.New 显式接收 *zap.SugaredLogger，**禁止**用 zap.L() 兜底。
// cache 启动失败归到 api 通道（与「连接池/外部服务」语义一致）。
func provideCache(cfg config.CacheConfig, l *logger.Loggers) (*cache.Client, error) {
	return cache.New(cfg, l.API())
}

// provideEnforcer 装配 casbin enforcer。
func provideEnforcer(cfg config.Config) (*auth.Enforcer, error) {
	return auth.NewEnforcer(cfg.Auth.ModelPath, cfg.Auth.PolicyPath)
}
