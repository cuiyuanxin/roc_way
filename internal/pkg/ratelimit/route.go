// Package ratelimit 提供路由级限流中间件的统一构造入口。
//
// 设计动机：直接调用 middleware.NewRateLimiter 时，每条路由都要重复
// 写 Window/Limit/KeyPrefix/Driver 等字段，配额调整或驱动切换需要
// 扫多个调用点维护；集中到本包后，路由级限流的「配额、窗口」集中
// 在这里声明，「驱动 / RPS / Burst」等共享配置由调用方通过 config 传入，
// 与全局限流保持同一份配置语义。
//
// 典型场景：
//   - 健康检查：防探测刷接口
//   - 登录接口：防撞库 / 密码喷洒
package ratelimit

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/middleware"
)

// DefaultWindow / DefaultLimit 是路由级限流的默认配额（每 IP 每窗口）。
//
// 调整配额时，**只改这里**即可同步到所有调用方。
const (
	DefaultWindow = time.Minute
	DefaultLimit  = 20
	keyPrefixBase = "rl:route:"
)

// NewRoute 构造一条路由级限流中间件，配额按当前默认（每分钟 20 次 / IP）。
//
// cfg 决定驱动（memory / redis）与 Redis key 前缀等共享参数，
// 与全局限流共用同一份配置，保证「全局走 memory 时路由级也走 memory」
// 的统一行为，避免出现驱动分裂。
//
// 启动期通过 Validate 校验 driver / cache / deploy_mode 全部维度，
// 配置错误直接 panic（防 typo 静默退化为 memory、防 cluster 误配 memory）。
//
// routeKey 用于拼 Redis key：`rl:route:<routeKey>`，
// 取值建议用稳定的英文短串，例如 "healthz"、"login"。
func NewRoute(cfg config.RateLimitConfig, cache *cache.Client, routeKey string) gin.HandlerFunc {
	if err := Validate(cfg, cache, ""); err != nil {
		panic("ratelimit: NewRoute(" + routeKey + "): " + err.Error())
	}
	mw, _, err := middleware.NewRateLimiter(middleware.RateLimitOptions{
		Enabled:   cfg.Enabled,
		Driver:    cfg.Driver,
		Window:    DefaultWindow,
		Limit:     DefaultLimit,
		KeyPrefix: keyPrefixBase + routeKey,
		Cache:     cache,
	})
	if err != nil {
		// 启动期配置错误（Redis 客户端未注入等），按框架惯例直接 panic，
		// 与 app.go 中全局限流的处理方式一致。
		panic("ratelimit: NewRoute(" + routeKey + "): " + err.Error())
	}
	return mw
}