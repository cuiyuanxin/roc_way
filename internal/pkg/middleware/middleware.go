// Package middleware 集中提供 gin 中间件：CORS / 限流 / 访问日志 / JWT / CSRF / Recovery。
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
)

// CORSOptions CORS 配置。
type CORSOptions struct {
	Origins          []string // 允许的来源列表，为空默认 "*"
	Methods          []string // 允许的方法，为空默认 "GET,POST,PUT,PATCH,DELETE,OPTIONS"
	Headers          []string // 允许的请求头，为空默认 "Origin,Content-Type,Authorization"
	ExposeHeaders    []string // 允许客户端访问的响应头
	MaxAge           int      // 预检请求缓存时间（秒），0 不发送
	AllowCredentials bool     // 是否允许携带凭证
}

// CORS 简单 CORS 实现（不依赖外部包）。
func CORS(opt CORSOptions) gin.HandlerFunc {
	allowOrigin := strings.Join(opt.Origins, ",")
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	methods := strings.Join(opt.Methods, ",")
	if methods == "" {
		methods = "GET,POST,PUT,PATCH,DELETE,OPTIONS"
	}
	headers := strings.Join(opt.Headers, ",")
	if headers == "" {
		headers = "Origin,Content-Type,Authorization"
	}
	exposeHeaders := strings.Join(opt.ExposeHeaders, ",")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		// 若 Origins 为空或包含 "*"，直接使用配置值；否则验证请求 Origin
		if len(opt.Origins) > 0 && allowOrigin != "*" {
			found := false
			for _, o := range opt.Origins {
				if o == origin || o == "*" {
					found = true
					break
				}
			}
			if !found {
				origin = ""
			}
		}

		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
		}

		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)

		if exposeHeaders != "" {
			c.Header("Access-Control-Expose-Headers", exposeHeaders)
		}
		if opt.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if opt.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", opt.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RateLimitOptions 限流器配置。
//
// 两种模式二选一：
//   - 令牌桶模式：RPS + Burst（保留给全局限流）
//   - 固定窗口模式：Window + Limit（路由级限流：20次/分钟这种语义）
//
// 同时设置时优先使用固定窗口模式（语义更贴业务「N 次 / Window」需求）。
type RateLimitOptions struct {
	Enabled   bool   // 是否启用限流
	Driver    string // "memory" 或 "redis"
	KeyPrefix string // Redis key 前缀
	Cache     *cache.Client

	// 固定窗口模式（路由级限流推荐）
	Window time.Duration // 例 time.Minute
	Limit  int           // 例 20

	// 令牌桶模式（全局限流，兼容保留）
	RPS   float64
	Burst int
}

// fixedWindowRedisLimiter Redis 固定窗口限流器（INCR + EXPIRE）。
//
// 适用场景：「N 次 / Window」语义（如 20次/分钟）。
// 与令牌桶相比：实现简单，一次 pipeline 往返，原子。
type fixedWindowRedisLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	window    time.Duration
	limit     int64
}

// Allow 检查是否允许请求（Redis INCR + EXPIRE）。
func (r *fixedWindowRedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", r.keyPrefix, key)
	pipe := r.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, r.window)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, err
	}
	return incrCmd.Val() <= r.limit, nil
}

// limiterEntry 内存限流器条目。
type limiterEntry struct {
	l          *rate.Limiter
	lastAccess time.Time // 最后访问时间，用于清理过期条目
}

// redisLimiter 分布式限流器（基于 Redis + 滑动窗口）。
type redisLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	rps       rate.Limit
}

// NewRedisLimiter 创建 Redis 分布式限流器。
func NewRedisLimiter(rdb *redis.Client, keyPrefix string, rps float64) *redisLimiter {
	return &redisLimiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		rps:       rate.Limit(rps),
	}
}

// Allow 检查是否允许请求（使用令牌桶算法模拟）。
func (r *redisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("%s:%s", r.keyPrefix, key)
	now := time.Now().UnixNano()
	windowStart := now - int64(time.Second)

	pipe := r.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, redisKey, 2*time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, err
	}

	count := countCmd.Val()
	limit := int64(r.rps * 1.5) // 稍微放宽限制以适应突发
	return count < limit, nil
}

// NewRateLimiter 创建限流器（支持 memory 或 redis 后端）。
// 返回 gin.HandlerFunc 和一个清理函数（应用停止时调用）。
// 自动设置 X-RateLimit-* 响应头，触发限流返回 429。
//
// 模式选择：
//   - Window > 0 && Limit > 0 → 固定窗口模式（路由级限流）
//   - 否则 → 令牌桶模式（全局限流）
func NewRateLimiter(opt RateLimitOptions) (gin.HandlerFunc, func(), error) {
	if !opt.Enabled {
		return func(c *gin.Context) { c.Next() }, func() {}, nil
	}

	useFixedWindow := opt.Window > 0 && opt.Limit > 0

	if !useFixedWindow {
		if opt.RPS <= 0 {
			opt.RPS = 100
		}
		if opt.Burst <= 0 {
			opt.Burst = 200
		}
	}
	if opt.KeyPrefix == "" {
		opt.KeyPrefix = "limiter"
	}

	var limiter interface {
		Allow(ctx context.Context, key string) (bool, error)
	}

	switch opt.Driver {
	case "redis":
		if opt.Cache == nil {
			return nil, nil, fmt.Errorf("redis limiter requires cache client")
		}
		if useFixedWindow {
			limiter = &fixedWindowRedisLimiter{
				rdb:       opt.Cache.RDB(),
				keyPrefix: opt.KeyPrefix,
				window:    opt.Window,
				limit:     int64(opt.Limit),
			}
		} else {
			limiter = NewRedisLimiter(opt.Cache.RDB(), opt.KeyPrefix, opt.RPS)
		}
	default: // "memory"
		if useFixedWindow {
			// 内存固定窗口：map[ip]count + TTL
			limiter = newFixedWindowMemoryLimiter(opt.Window, opt.Limit)
		} else {
			limiter = newMemoryLimiter(opt.RPS, opt.Burst)
		}
	}

	return func(c *gin.Context) {
			key := c.ClientIP()
			ctx := c.Request.Context()

			allowed, err := limiter.Allow(ctx, key)
			if err != nil {
				// Redis 出错时放行，避免影响可用性
				c.Next()
				return
			}

			if useFixedWindow {
				c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", opt.Limit))
				if !allowed {
					c.Header("X-RateLimit-Remaining", "0")
					response.WriteErr(c, errcode.ErrRateLimited)
					c.Abort()
					return
				}
			} else {
				c.Header("X-RateLimit-Limit", fmt.Sprintf("%.0f", opt.RPS))
				if !allowed {
					c.Header("X-RateLimit-Remaining", "0")
					response.WriteErr(c, errcode.ErrRateLimited)
					c.Abort()
					return
				}
			}

			c.Next()
		}, func() {
			if ml, ok := limiter.(*memoryLimiter); ok {
				ml.Clear()
			}
			if ml, ok := limiter.(*fixedWindowMemoryLimiter); ok {
				ml.Clear()
			}
		}, nil
}

// newMemoryLimiter 创建内存限流器。
func newMemoryLimiter(rps float64, burst int) *memoryLimiter {
	return &memoryLimiter{
		rps:    rate.Limit(rps),
		burst:  burst,
		mu:     &sync.Mutex{},
		bucket: make(map[string]*limiterEntry),
	}
}

// Clear 清除所有限流器条目。
func (m *memoryLimiter) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bucket = make(map[string]*limiterEntry)
}

// memoryLimiter 内存限流器。
type memoryLimiter struct {
	rps    rate.Limit
	burst  int
	mu     *sync.Mutex
	bucket map[string]*limiterEntry
}

// Allow 检查是否允许请求。
func (m *memoryLimiter) Allow(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	e, ok := m.bucket[key]
	if !ok {
		e = &limiterEntry{
			l:          rate.NewLimiter(m.rps, m.burst),
			lastAccess: now,
		}
		m.bucket[key] = e
	}

	e.l.SetLimit(m.rps)
	e.l.SetBurst(m.burst)
	e.lastAccess = now // 更新最后访问时间
	allowed := e.l.Allow()

	// 定期清理过期条目（超过 10 分钟未访问）
	if len(m.bucket) > 1024 {
		for k, v := range m.bucket {
			if now.Sub(v.lastAccess) > 10*time.Minute {
				delete(m.bucket, k)
			}
		}
	}

	return allowed, nil
}

// fixedWindowMemoryLimiter 内存固定窗口限流器（单机版）。
//
// map[ip] = { count, windowStart }；
// 每次请求若窗口已过则重置计数；否则递增。
type fixedWindowMemoryLimiter struct {
	window time.Duration
	limit  int64
	mu     sync.Mutex
	bucket map[string]*fixedWindowEntry
}

type fixedWindowEntry struct {
	count       int64
	windowStart time.Time
}

// newFixedWindowMemoryLimiter 构造内存固定窗口限流器。
func newFixedWindowMemoryLimiter(window time.Duration, limit int) *fixedWindowMemoryLimiter {
	return &fixedWindowMemoryLimiter{
		window: window,
		limit:  int64(limit),
		bucket: make(map[string]*fixedWindowEntry),
	}
}

// Allow 检查是否允许请求（内存版，固定窗口）。
func (m *fixedWindowMemoryLimiter) Allow(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	e, ok := m.bucket[key]
	if !ok || now.Sub(e.windowStart) >= m.window {
		m.bucket[key] = &fixedWindowEntry{count: 1, windowStart: now}
		return true, nil
	}
	e.count++
	return e.count <= m.limit, nil
}

// Clear 清空所有条目（应用停止时调用）。
func (m *fixedWindowMemoryLimiter) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bucket = make(map[string]*fixedWindowEntry)
}

// AccessLog 输出 JSON 行到 api logger。
func AccessLog(log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infow("http",
			"request_id", GetRequestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.Query(),
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

// JWT 解析 Authorization Bearer 并注入 user_id / jti / device_id 到 context。
//
// 校验链（按顺序，任一失败立即 abort）：
//  1. Authorization header 存在且以 "Bearer " 开头；
//  2. 验签通过（用 RS256 公钥，详见 [auth.Auth.Parse]）；
//  3. claims.Kind == "access"（拒绝 refresh 错用）；
//  4. jti 不在黑名单（已吊销的 token 拒绝）；
//  5. **DeviceID 绑定校验**：如果 token claim 里有 deviceID，请求头 X-Device-ID 必须一致，
//     防 token 泄露被异设备使用。
func JWT(a *auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			// 没带 token：放行，让后续 handler 自己决定是否 401
			c.Next()
			return
		}
		token := strings.TrimPrefix(h, "Bearer ")
		claims, err := a.Parse(token)
		if err != nil {
			response.WriteErr(c, errcode.ErrTokenInvalid)
			c.Abort()
			return
		}
		if claims.Kind != "access" {
			response.WriteErr(c, errcode.ErrTokenInvalid.WithMessage("not an access token"))
			c.Abort()
			return
		}
		revoked, _ := a.IsRevoked(c.Request.Context(), claims.ID)
		if revoked {
			response.WriteErr(c, errcode.ErrTokenInvalid.WithMessage("token 已吊销"))
			c.Abort()
			return
		}
		// 设备指纹绑定校验：token claim 有 deviceID 则必须与请求头一致
		if claims.DeviceID != "" {
			reqDevice := c.GetHeader("X-Device-ID")
			if reqDevice == "" || reqDevice != claims.DeviceID {
				response.WriteErr(c, errcode.ErrTokenInvalid.WithMessage("device mismatch"))
				c.Abort()
				return
			}
		}
		c.Set("user_id", claims.Subject)
		c.Set("jti", claims.ID)
		c.Set("device_id", claims.DeviceID)
		c.Next()
	}
}

// CSRFOptions CSRF 中间件配置。
type CSRFOptions struct {
	// SkipPaths 是跳过 CSRF 校验的路径前缀列表（精确匹配或前缀匹配）。
	// 典型场景：纯 token 鉴权的接口（Authorization 头而非 cookie 鉴权），
	// 不存在「浏览器自动带 cookie 发请求」的攻击面，CSRF 防护无意义。
	// 例：CSRFOptions{SkipPaths: []string{"/api/auth/logout"}}
	SkipPaths []string
}

// CSRF 校验非幂等方法的 X-CSRF-Token 头（与 cookie 中的 token 一致）。
//
// 设计要点：
//   - 仅校验「可能造成副作用」的方法（POST/PUT/PATCH/DELETE），GET/HEAD/OPTIONS 直接放行
//   - 通过 Options.SkipPaths 跳过纯 token 鉴权路径，避免误拦 API 调用方
//     （curl / 后端服务调用没有 csrf_token cookie）
func CSRF(opt ...CSRFOptions) gin.HandlerFunc {
	var skip []string
	if len(opt) > 0 {
		skip = opt[0].SkipPaths
	}
	return func(c *gin.Context) {
		m := c.Request.Method
		if m == "GET" || m == "HEAD" || m == "OPTIONS" {
			c.Next()
			return
		}
		// 跳过白名单路径
		path := c.Request.URL.Path
		for _, p := range skip {
			if p == path || strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		token := c.GetHeader("X-CSRF-Token")
		cookie, _ := c.Cookie("csrf_token")
		if token == "" || cookie == "" || token != cookie {
			response.WriteErr(c, errcode.ErrCSRFToken)
			c.Abort()
			return
		}
		c.Next()
	}
}

// Recovery 捕获 panic 并返回统一错误响应。
//
// 环境自适应：
//   - debug / test 模式：响应包含详细 panic 信息（err、trace），便于本地定位。
//   - release 模式：响应仅含 code/message（脱敏），原始 panic 详情写日志，
//     避免在生产环境泄漏堆栈、SQL、密钥等敏感信息。
func Recovery(log *zap.SugaredLogger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		// 始终把完整 panic 详情写日志（含堆栈），便于线上排障。
		stack := string(debug.Stack())
		rid := GetRequestID(c)
		log.Errorw("panic",
			"request_id", rid,
			"err", err,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"client_ip", c.ClientIP(),
			"stack", stack,
		)

		mode := gin.Mode()
		verbose := mode == gin.DebugMode || mode == gin.TestMode

		var details interface{}
		if verbose {
			details = fmt.Sprintf("%v", err)
		}

		resp := response.NewErrorResponse(
			errcode.ErrInternal.Code,
			errcode.ErrInternal.Message,
			rid,
			details,
		)
		c.AbortWithStatusJSON(errcode.ErrInternal.HTTPStatus, resp)
	})
}
