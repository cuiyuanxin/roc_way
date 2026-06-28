// Package admin 是 roc_way 框架的 admin 应用入口（DDD 分层 + 企业级生产基线）。
//
// 分层结构：
//
//	repository/     ← 仓储（接口 + GORM 实现）
//	service/        ← 应用服务（业务编排）
//	handler/        ← HTTP 表现层（薄）
//	model/          ← 持久化对象（GORM 映射）
//	dto/            ← 跨层 POJO（不依赖 ORM）
//	app.go          ← 组装层（依赖注入 + 路由注册）
package admin

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/api"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/handler"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/repository"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/service"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/janitor"
	"github.com/cuiyuanxin/roc_way/internal/pkg/logger"
	"github.com/cuiyuanxin/roc_way/internal/pkg/middleware"
	"github.com/cuiyuanxin/roc_way/internal/pkg/notify"
	"github.com/cuiyuanxin/roc_way/internal/pkg/ratelimit"
	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
	rocvalidator "github.com/cuiyuanxin/roc_way/internal/pkg/validator"
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
	deps             Deps
	rateLimitCleanup func()
	janitors         *janitor.Runners
	hubStop          chan struct{} // 关闭 Hub.Run 主循环
	closeOnce        sync.Once     // 保证 Close 幂等
}

// NewApp 构造 gin 引擎：装配中间件链、装配 DDD 分层、注册 handler。
func NewApp(d Deps) *App {
	gin.SetMode(d.Cfg.Server.Mode)

	// ============ DDD 装配（依赖注入）============
	userRepo := repository.NewUserRepository(d.DB)
	// 锁定跟踪（DB 主源 + Redis 加速缓存：失败计数 + 锁状态）
	auditRepo := repository.NewLoginAuditRepository(d.DB, d.Cache, repository.LockTTL{
		Short: d.Cfg.LoginPolicy.ShortDuration,
		Long:  d.Cfg.LoginPolicy.LongDuration,
	}, d.Log.Security())
	loginLogRepo := repository.NewLoginLogRepository(d.DB) // 登录日志（auth_login_logs）

	// 安全事件通知（noop 默认实现 + zap 安全日志）
	notifier := notify.NewNoopNotifier(d.Log.Security())

	// 锁定服务（编排：调 repo.RecordFailure / RecordLock / LatestActiveLock；
	// Redis 加速 + DB 兜底的细节在 repo 内部封装，service 不直接依赖 cache）
	lockSvc := service.NewLockService(auditRepo, notifier, service.LockPolicy{
		Window:         d.Cfg.LoginPolicy.AuditRetention,
		ShortThreshold: d.Cfg.LoginPolicy.ShortThreshold,
		ShortDuration:  d.Cfg.LoginPolicy.ShortDuration,
		LongThreshold:  d.Cfg.LoginPolicy.LongThreshold,
		LongDuration:   d.Cfg.LoginPolicy.LongDuration,
	}, d.Log.API())

	// 登录日志服务（独立于锁定；写 login_logs）
	loginAuditSvc := service.NewLoginLogService(loginLogRepo, d.Log.API())

	userSvc := service.NewUserService(userRepo, d.Log.API())
	authSvc := service.NewAuthService(userRepo, d.Auth, lockSvc, loginAuditSvc, d.Log.API())

	// 修复 [C5]：把 CORS Origin 白名单注入 WebSocket Hub，
	// 避免 CSWSH（跨站 WebSocket 劫持）—— 原实现 CheckOrigin 永远 true。
	d.Hub.SetAllowedOrigins(d.Cfg.Server.CORS.Origins)

	// ============ Gin Engine + 中间件链 ============
	e := gin.New()
	// 启用 method-not-allowed 检测，让 PUT/DELETE/PATCH 等不匹配方法时走 NoMethod（405）而非 NoRoute（404）。
	e.HandleMethodNotAllowed = true
	if len(d.Cfg.Server.TrustedProxies) > 0 {
		e.SetTrustedProxies(d.Cfg.Server.TrustedProxies)
	}
	// RequestID 必须最先注册
	e.Use(middleware.RequestID(middleware.RequestIDOptions{}))
	if d.Cfg.Server.Mode == gin.ReleaseMode {
		e.Use(middleware.Recovery(d.Log.API()))
		e.Use(middleware.AccessLog(d.Log.API()))
	} else {
		e.Use(gin.Recovery())
		e.Use(gin.Logger())
	}

	e.Use(middleware.CORS(middleware.CORSOptions{
		Origins:          d.Cfg.Server.CORS.Origins,
		Methods:          d.Cfg.Server.CORS.Methods,
		Headers:          d.Cfg.Server.CORS.Headers,
		ExposeHeaders:    d.Cfg.Server.CORS.ExposeHeaders,
		MaxAge:           d.Cfg.Server.CORS.MaxAge,
		AllowCredentials: d.Cfg.Server.CORS.AllowCredentials,
	}))

	// HSTS：仅当请求是 TLS 时才设置头（HTTP 请求 c.Request.TLS == nil 自动跳过）。
	// 与 HTTP/HTTPS 双 server 共享同一 Engine 无冲突。
	e.Use(middleware.HSTS())

	// 全局限流（令牌桶，机器承载力兜底）
	if err := ratelimit.Validate(d.Cfg.Server.RateLimit, d.Cache, d.Cfg.Server.DeployMode); err != nil {
		panic("rate limiter: " + err.Error())
	}
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

	if d.Cfg.Server.Timeout > 0 {
		e.Use(middleware.Timeout(time.Duration(d.Cfg.Server.Timeout) * time.Second))
	}

	// ============ 路由注册 ============
	v := rocvalidator.New()
	dto.RegisterAll(v)

	api.RegisterRoutes(e)

	// 路由级限流：健康检查 + login 各 20次/分钟/IP（配额集中在 ratelimit 包维护；驱动与全局限流共用 d.Cfg.Server.RateLimit）
	handler.NewHealth(ratelimit.NewRoute(d.Cfg.Server.RateLimit, d.Cache, "healthz")).Register(e)
	authH := handler.NewAuth(authSvc, v, ratelimit.NewRoute(d.Cfg.Server.RateLimit, d.Cache, "login"))
	authH.Register(e) // 公开路径：login / refresh

	apiGroup := e.Group("/")
	apiGroup.Use(middleware.JWT(d.Auth))
	// CSRF 跳过纯 token 鉴权的 logout：它用 Authorization 头而非 cookie 鉴权，
	// 不存在「浏览器自动带 cookie」的攻击面，CSRF 防护对它无意义且会误拦 API 调用方。
	apiGroup.Use(middleware.CSRF(middleware.CSRFOptions{
		SkipPaths: []string{"/api/auth/logout"},
	}))
	handler.NewUser(userSvc, v).Register(apiGroup)
	handler.NewRealtime(d.Hub).Register(apiGroup)
	authH.RegisterLogout(apiGroup) // 受保护路径：logout（需 JWT，跳过 CSRF）

	// 兜底错误响应：路由不存在 / HTTP 方法不允许。
	//
	// **为什么必须显式注册 NoRoute / NoMethod 处理器？**
	// GIN 内置 NoRoute 默认走 c.writermem.status = 404 + 默认 404 body，
	// 但当 c.Writer 被 gin-contrib/timeout 等中间件替换为缓冲 Writer 时，
	// `c.writermem.status = 404` 不会通过 Writer.WriteHeader 传到底层 ResponseWriter，
	// 最终会以默认 200 发出。**显式 NoRoute 处理器通过 c.Writer.WriteHeader(404) 触发
	// timeout 中间件的 tw.code = 404**，case <-finish 路径会发 404 + body。
	e.NoRoute(func(c *gin.Context) {
		response.WriteErr(c, errcode.ErrNotFound)
	})
	e.NoMethod(func(c *gin.Context) {
		response.WriteErr(c, errcode.ErrMethodNotAllowed)
	})

	// ============ Janitor 启动 ============
	auditJanitor := janitor.NewLoginAuditJanitor(
		auditRepo,
		d.Cfg.LoginPolicy.JanitorInterval,
		d.Cfg.LoginPolicy.AuditRetention,
	)
	runners := janitor.StartAll(context.Background(), []*janitor.Janitor{auditJanitor},
		func(err error, name string) {
			d.Log.Security().Errorw("janitor_error", "name", name, "error", err.Error())
		},
	)

	// 修复 [C6]：启动 WebSocket Hub 主循环（之前全项目无调用方，
	// 所有 WS 消息静默丢失）。Close 时通过 hubStop channel 优雅停止。
	hubStop := make(chan struct{})
	go d.Hub.Run(hubStop)

	return &App{
		engine:           e,
		deps:             d,
		rateLimitCleanup: rateLimitCleanup,
		janitors:         runners,
		hubStop:          hubStop,
	}
}

// Engine 返回 gin 引擎。
func (a *App) Engine() *gin.Engine { return a.engine }

// Close 清理资源（停止时调用）。幂等：可重复调用，仅生效一次。
//
// **职责边界**：App 持有 Deps（Cfg/Log/DB/Cache/Auth/Enforcer/Hub），
// 因此 Close 负责释放这些「应用持有的全部外部资源」——
// main.go / wire cleanup 不需要再单独关闭 db / cache，避免资源泄漏。
//
// 关闭顺序（先业务后基建）：
//  1. Hub 主循环（不再接收新 reg/unreg）
//  2. janitor 后台 ticker
//  3. 限流器内存条目清理
//  4. Hub 连接断开（在 1 之后才能安全关）
//  5. DB / Redis 连接（最后关，让业务有排干窗口）
func (a *App) Close() {
	a.closeOnce.Do(func() {
		// 1. 关 Hub 主循环
		if a.hubStop != nil {
			close(a.hubStop)
		}
		// 2. janitor 后台 ticker
		if a.janitors != nil {
			a.janitors.Stop()
		}
		// 3. 限流器内存条目清理
		if a.rateLimitCleanup != nil {
			a.rateLimitCleanup()
		}
		// 4. DB 关闭（sqlDB 内部会排干 in-flight 查询）
		if a.deps.DB != nil {
			_ = a.deps.DB.Close()
		}
		// 5. Redis 关闭
		if a.deps.Cache != nil {
			_ = a.deps.Cache.Close()
		}
		// 6. Logger 刷盘（zap）
		if a.deps.Log != nil {
			_ = a.deps.Log.Sync()
		}
	})
}

// Migrate 执行 GORM AutoMigrate。
//
// 迁移列表由 model.All() 集中维护，新增实体只需在 model 包加一行。
func (d Deps) Migrate() error {
	return model.Migrate(d.DB.Write)
}
