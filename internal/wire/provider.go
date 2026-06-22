// Package wire 集中所有 Provider，编译期生成 wire_gen.go。
//
// 严格遵循 DDD 原则：Provider 仅覆盖外部可变依赖（DB / Cache / Storage /
// MQ / Enforcer / Logger / Config / Auth）；领域模型、值对象、工具函数
// 不参与注入。
package wire

import (
	"github.com/google/wire"

	"github.com/cuiyuanxin/roc_way/internal/app/admin"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// ProviderSet 暴露所有 Provider，供 wire.Build 引用。
var ProviderSet = wire.NewSet(
	config.New,
	provideLogger,
	database.Open,
	cache.New,
	auth.New,
	provideEnforcer,
	realtime.NewHub,
	wire.Struct(new(admin.Deps), "Cfg", "Log", "DB", "Auth", "Enforcer", "Hub"),
	admin.NewApp,
)
