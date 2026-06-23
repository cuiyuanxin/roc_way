// Package middleware: requestid 包装 gin-contrib/requestid。
//
// 行为：
//   - 优先取请求头 `X-Request-ID`（按用户传入透传，便于跨服务链路追踪）。
//   - 缺失或非法时自动生成一个 UUID v4 字符串（由 requestid 库默认实现）。
//   - 始终写入响应头 `X-Request-ID`，并把 ID 放进 gin.Context，
//     业务层可通过 `middleware.GetRequestID(c)` 取。
//
// 直接复用 `github.com/gin-contrib/requestid` 事实标准库，**禁止**自实现 ID 生成。
package middleware

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// 默认配置常量。
const (
	// DefaultRequestIDContextKey gin.Context 中的 key。
	DefaultRequestIDContextKey = "request_id"
	// DefaultRequestIDHeader 请求/响应头名（沿用 requestid 库默认值）。
	DefaultRequestIDHeader = "X-Request-ID"
)

// RequestIDOptions 包装项。
type RequestIDOptions struct {
	// ContextKey 注入 gin.Context 的 key，默认 "request_id"。
	ContextKey string
	// Header 请求/响应头名，默认 "X-Request-ID"。需要自定义时传 requestid.HeaderStrKey 类型。
	Header requestid.HeaderStrKey
	// Generator 自定义 ID 生成器；传 nil 则使用 requestid 库默认实现（UUID v4）。
	Generator func() string
}

// RequestID 返回一个把 requestid 注入到 ctx / response header 的 gin 中间件。
//
// 库内部已实现「优先读请求头，没有则自动生成」语义（详见 gin-contrib/requestid/requestid.go L36-42），
// 本中间件只在其上包一层，把 ID 同步写入 gin.Context，方便业务 handler / 日志 / 错误响应读取。
func RequestID(opt RequestIDOptions) gin.HandlerFunc {
	if opt.ContextKey == "" {
		opt.ContextKey = DefaultRequestIDContextKey
	}
	if opt.Header == "" {
		opt.Header = requestid.HeaderStrKey(DefaultRequestIDHeader)
	}

	cfg := []requestid.Option{
		requestid.WithCustomHeaderStrKey(opt.Header),
		requestid.WithHandler(func(c *gin.Context, id string) {
			c.Set(opt.ContextKey, id)
		}),
	}
	if opt.Generator != nil {
		cfg = append(cfg, requestid.WithGenerator(opt.Generator))
	}
	return requestid.New(cfg...)
}

// GetRequestID 从 gin.Context 取出 request_id。
//
// 业务 handler / 日志中间件中通过 `middleware.GetRequestID(c)` 调用；
// 找不到时返回空字符串（永远不要让 request_id 缺失阻塞业务）。
func GetRequestID(c *gin.Context) string {
	v, _ := c.Get(DefaultRequestIDContextKey)
	s, _ := v.(string)
	return s
}
