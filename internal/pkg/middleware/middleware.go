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

	"github.com/cuiyuanxin/roc_way/api/response"
	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
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

// RateLimitOptions 限流器选项。
type RateLimitOptions struct {
	Enabled   bool    // 是否启用限流
	Driver    string  // "memory" 或 "redis"
	RPS       float64 // 每秒请求数
	Burst     int     // 突发容量
	KeyPrefix string  // Redis key 前缀
	Cache     *cache.Client
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
func NewRateLimiter(opt RateLimitOptions) (gin.HandlerFunc, func(), error) {
	if !opt.Enabled {
		return func(c *gin.Context) { c.Next() }, func() {}, nil
	}

	if opt.RPS <= 0 {
		opt.RPS = 100
	}
	if opt.Burst <= 0 {
		opt.Burst = 200
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
		limiter = NewRedisLimiter(opt.Cache.RDB(), opt.KeyPrefix, opt.RPS)
	default: // "memory"
		limiter = newMemoryLimiter(opt.RPS, opt.Burst)
	}

	rps := opt.RPS

	return func(c *gin.Context) {
			key := fmt.Sprintf("%s:%s", opt.KeyPrefix, c.ClientIP())
			ctx := c.Request.Context()

			allowed, err := limiter.Allow(ctx, key)
			if err != nil {
				// Redis 出错时放行，避免影响可用性
				c.Next()
				return
			}

			c.Header("X-RateLimit-Limit", fmt.Sprintf("%.0f", rps))
			if !allowed {
				c.Header("X-RateLimit-Remaining", "0")
				c.AbortWithStatusJSON(errcode.ErrRateLimited.HTTPStatus, response.NewErrorResponse(
					errcode.ErrRateLimited.Code,
					errcode.ErrRateLimited.Message,
					GetRequestID(c),
					nil,
				))
				return
			}

			c.Next()
		}, func() {
			if ml, ok := limiter.(*memoryLimiter); ok {
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

// JWT 解析 Authorization Bearer 并注入 user_id 到 context。
func JWT(a *auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.Next()
			return
		}
		token := strings.TrimPrefix(h, "Bearer ")
		claims, err := a.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(errcode.ErrTokenInvalid.HTTPStatus,
				gin.H{
					"code":       errcode.ErrTokenInvalid.Code,
					"message":    errcode.ErrTokenInvalid.Message,
					"request_id": GetRequestID(c),
				})
			return
		}
		revoked, _ := a.IsRevoked(c.Request.Context(), claims.ID)
		if revoked {
			c.AbortWithStatusJSON(errcode.ErrTokenInvalid.HTTPStatus,
				gin.H{
					"code":       errcode.ErrTokenInvalid.Code,
					"message":    "token 已吊销",
					"request_id": GetRequestID(c),
				})
			return
		}
		c.Set("user_id", claims.Subject)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// CSRF 校验非幂等方法的 X-CSRF-Token 头（与 cookie 中的 token 一致）。
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := c.Request.Method
		if m == "GET" || m == "HEAD" || m == "OPTIONS" {
			c.Next()
			return
		}
		token := c.GetHeader("X-CSRF-Token")
		cookie, _ := c.Cookie("csrf_token")
		if token == "" || cookie == "" || token != cookie {
			c.AbortWithStatusJSON(errcode.ErrCSRFToken.HTTPStatus,
				gin.H{
					"code":       errcode.ErrCSRFToken.Code,
					"message":    errcode.ErrCSRFToken.Message,
					"request_id": GetRequestID(c),
				})
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
