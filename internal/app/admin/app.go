// Package admin 演示 rocway 框架的使用方式。
package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/controller"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/logger"
	"github.com/cuiyuanxin/roc_way/internal/pkg/middleware"
	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// Deps 聚合 admin 应用所需外部可变依赖（DDD：仅外部依赖）。
type Deps struct {
	Cfg      config.Config
	Log      *logger.Loggers
	DB       *database.DB
	Auth     *auth.Auth
	Enforcer *auth.Enforcer
	Hub      *realtime.Hub
}

// App 封装 gin.Engine。
type App struct {
	engine *gin.Engine
}

// NewApp 构造 gin 引擎并注册中间件链、控制器。
func NewApp(d Deps) *App {
	gin.SetMode(d.Cfg.Server.Mode)
	e := gin.New()
	e.Use(middleware.Recovery(d.Log.API()))
	e.Use(middleware.AccessLog(d.Log.API()))
	e.Use(middleware.CORS(middleware.CORSOptions{}))
	e.Use(middleware.RateLimit(100, 200))

	controller.NewHealth().Register(e)
	controller.NewAuth(d.Auth).Register(e)

	api := e.Group("/")
	api.Use(middleware.JWT(d.Auth))
	api.Use(middleware.CSRF())
	controller.NewUser(d.DB).Register(api)
	controller.NewRealtime(d.Hub).Register(api)

	// 受 RBAC 保护的示例
	api.GET("/api/v1/admin",
		d.Enforcer.RequirePermission("api/v1/admin", "GET"),
		func(c *gin.Context) { controller.WriteOK(c, gin.H{"role": "admin"}) },
	)

	return &App{engine: e}
}

// Engine 返回 gin 引擎。
func (a *App) Engine() *gin.Engine { return a.engine }

// Migrate 执行 GORM AutoMigrate。
func (d Deps) Migrate() error {
	return d.DB.AutoMigrate(&model.User{})
}
