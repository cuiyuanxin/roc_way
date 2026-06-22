// Package wire: wire.go 是 wire.Build 的入口。
//
// 由 `wire ./internal/wire` 生成 wire_gen.go；InitApp 是启动入口。
//go:build wireinject
// +build wireinject

package wire

import (
	"context"

	"github.com/google/wire"

	"github.com/cuiyuanxin/roc_way/internal/app/admin"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/logger"
	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// InitApp 由 wire 生成，调用者通过 wire_gen.go 实际执行注入。
func InitApp(ctx context.Context, cfg config.Config) (*admin.App, func(), error) {
	wire.Build(
		wire.FieldsOf(&cfg, "Database", "Cache", "Auth"),
		provideLogger,
		database.Open,
		cache.New,
		auth.New,
		provideEnforcer,
		realtime.NewHub,
		wire.Struct(new(admin.Deps), "Cfg", "Log", "DB", "Auth", "Enforcer", "Hub"),
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

// provideEnforcer 装配 casbin enforcer。
func provideEnforcer(cfg config.Config) (*auth.Enforcer, error) {
	return auth.NewEnforcer(cfg.Auth.ModelPath, cfg.Auth.PolicyPath)
}
