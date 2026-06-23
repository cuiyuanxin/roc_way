// Package admin 演示 rocway 框架的使用方式。
package admin

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/api"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/controller"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
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
	Cache    *cache.Client
	Auth     *auth.Auth
	Enforcer *auth.Enforcer
	Hub      *realtime.Hub
}

// App 封装 gin.Engine。
type App struct {
	engine           *gin.Engine
	rateLimitClenaup func() // 限流器清理函数
}

// NewApp 构造 gin 引擎并注册中间件链、控制器。
func NewApp(d Deps) *App {
	gin.SetMode(d.Cfg.Server.Mode)
	e := gin.New()
	// 信任代理（支持 X-Forwarded-For、X-Real-IP 获取真实客户端 IP）
	if len(d.Cfg.Server.TrustedProxies) > 0 {
		e.SetTrustedProxies(d.Cfg.Server.TrustedProxies)
	} // 为空则使用 gin 默认行为（生产环境建议配置）
	// 注意顺序：RequestID 必须最先注册，
	// 后续所有中间件（AccessLog / Recovery / JWT / CSRF）才能从 context 读到 request_id。
	e.Use(middleware.RequestID(middleware.RequestIDOptions{}))
	e.Use(middleware.Recovery(d.Log.API()))
	e.Use(middleware.AccessLog(d.Log.API()))
	e.Use(middleware.CORS(middleware.CORSOptions{
		Origins:          d.Cfg.Server.CORS.Origins,
		Methods:          d.Cfg.Server.CORS.Methods,
		Headers:          d.Cfg.Server.CORS.Headers,
		ExposeHeaders:    d.Cfg.Server.CORS.ExposeHeaders,
		MaxAge:           d.Cfg.Server.CORS.MaxAge,
		AllowCredentials: d.Cfg.Server.CORS.AllowCredentials,
	}))

	// 限流（支持 memory 或 redis 后端）
	rateLimitMw, rateLimitCleanup, err := middleware.NewRateLimiter(middleware.RateLimitOptions{
		Enabled:   d.Cfg.Server.RateLimit.Enabled,
		Driver:    d.Cfg.Server.RateLimit.Driver,
		RPS:       d.Cfg.Server.RateLimit.RPS,
		Burst:     d.Cfg.Server.RateLimit.Burst,
		KeyPrefix: d.Cfg.Server.RateLimit.KeyPrefix,
		Cache:     d.Cache,
	})
	if err != nil {
		panic("rate limiter: " + err.Error())
	}
	e.Use(rateLimitMw)

	// 请求超时（可选，0 表示不启用）
	if d.Cfg.Server.Timeout > 0 {
		e.Use(timeoutMiddleware(d))
	}

	// Swagger UI
	api.RegisterRoutes(e)

	controller.NewHealth().Register(e)
	controller.NewAuth(d.Auth).Register(e)

	apiGroup := e.Group("/")
	apiGroup.Use(middleware.JWT(d.Auth))
	apiGroup.Use(middleware.CSRF())
	controller.NewUser(d.DB).Register(apiGroup)
	controller.NewRealtime(d.Hub).Register(apiGroup)

	// 受 RBAC 保护的示例
	apiGroup.GET("/api/v1/admin",
		d.Enforcer.RequirePermission("api/v1/admin", "GET"),
		func(c *gin.Context) { controller.WriteOK(c, gin.H{"role": "admin"}) },
	)

	return &App{
		engine:           e,
		rateLimitClenaup: rateLimitCleanup,
	}
}

// timeoutMiddleware 创建请求超时中间件。
func timeoutMiddleware(d Deps) gin.HandlerFunc {
	timeoutDur := time.Duration(d.Cfg.Server.Timeout) * time.Second
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutDur)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{}, 1)
		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// 请求完成
		case <-ctx.Done():
			c.AbortWithStatusJSON(504, gin.H{
				"code":       1504,
				"message":    "Gateway Timeout",
				"request_id": middleware.GetRequestID(c),
			})
		}
	}
}

// Engine 返回 gin 引擎。
func (a *App) Engine() *gin.Engine { return a.engine }

// Close 清理资源（停止时调用）。
func (a *App) Close() {
	if a.rateLimitClenaup != nil {
		a.rateLimitClenaup()
	}
}

// Migrate 执行 GORM AutoMigrate。
func (d Deps) Migrate() error {
	return d.DB.AutoMigrate(&model.User{})
}
