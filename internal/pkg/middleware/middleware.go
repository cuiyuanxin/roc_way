// Package middleware 集中提供 gin 中间件：CORS / 限流 / 访问日志 / JWT / CSRF / Recovery。
package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// CORSOptions CORS 配置。
type CORSOptions struct {
	Origins []string
	Methods []string
	Headers []string
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
		headers = "Origin,Content-Type,Authorization,X-CSRF-Token"
	}
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// RateLimit 按 IP 限流（令牌桶）。
func RateLimit(rps, burst int) gin.HandlerFunc {
	type entry struct {
		l *rate.Limiter
		t time.Time
	}
	mu := sync.Mutex{}
	bucket := map[string]*entry{}
	now := time.Now
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		e, ok := bucket[ip]
		if !ok {
			e = &entry{l: rate.NewLimiter(rate.Limit(rps), burst)}
			bucket[ip] = e
		}
		e.t = now()
		ok = e.l.Allow()
		// 简单清理
		if len(bucket) > 1024 {
			for k, v := range bucket {
				if now().Sub(v.t) > 10*time.Minute {
					delete(bucket, k)
				}
			}
		}
		mu.Unlock()
		if !ok {
			c.AbortWithStatusJSON(errcode.ErrRateLimited.HTTPStatus,
				gin.H{"code": errcode.ErrRateLimited.Code, "message": errcode.ErrRateLimited.Message})
			return
		}
		c.Next()
	}
}

// AccessLog 输出 JSON 行到 api logger。
func AccessLog(log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infow("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
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
				gin.H{"code": errcode.ErrTokenInvalid.Code, "message": errcode.ErrTokenInvalid.Message})
			return
		}
		revoked, _ := a.IsRevoked(c.Request.Context(), claims.ID)
		if revoked {
			c.AbortWithStatusJSON(errcode.ErrTokenInvalid.HTTPStatus,
				gin.H{"code": errcode.ErrTokenInvalid.Code, "message": "token 已吊销"})
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
				gin.H{"code": errcode.ErrCSRFToken.Code, "message": errcode.ErrCSRFToken.Message})
			return
		}
		c.Next()
	}
}

// Recovery panic 转 errcode.ErrInternal JSON。
func Recovery(log *zap.SugaredLogger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		log.Errorw("panic", "err", err, "path", c.Request.URL.Path)
		c.AbortWithStatusJSON(errcode.ErrInternal.HTTPStatus,
			gin.H{"code": errcode.ErrInternal.Code, "message": errcode.ErrInternal.Message})
	})
}
