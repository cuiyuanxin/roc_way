package middleware

import "github.com/gin-gonic/gin"

// HSTS 设置 Strict-Transport-Security 响应头，强制浏览器后续走 HTTPS。
//
// 行为：
//   - **仅当请求是 TLS（c.Request.TLS != nil）时才设置 HSTS 头**。
//   - HTTP 请求不设置，避免误导浏览器把 HTTP 标记为 HTTPS only。
//   - 一个 gin.Engine 同时服务 HTTP + HTTPS 双端口时，HSTS 中间件可安全共享。
//
// 头值说明（符合 OWASP HSTS Cheat Sheet）：
//   - max-age=63072000：2 年，符合 OWASP 推荐值（>= 1 年）
//   - includeSubDomains：强制所有子域也走 HTTPS
//   - preload：允许加入浏览器内置 HSTS 列表（提交到 https://hstspreload.org/）
//
// 注意：HSTS 头只对**浏览器**生效（防降级攻击），不影响 API 客户端。
// curl / Postman / Go SDK 等 HTTP 客户端不会因为 HSTS 强制升级。
func HSTS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		c.Next()
	}
}
