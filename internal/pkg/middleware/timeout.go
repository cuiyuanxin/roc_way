// Package middleware: timeout 包装 gin-contrib/timeout。
//
// 行为：
//   - 给整个请求链路设置硬超时；超过设定时长后中断 handler goroutine 并返回 504。
//   - 超时响应统一带 `code` / `message` / `request_id`，与项目其它错误响应格式一致。
//
// 直接复用 `github.com/gin-contrib/timeout` 事实标准库，**禁止**自实现超时逻辑。
package middleware

import (
	"time"

	gintimeout "github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
)

// Timeout 创建一个请求级超时中间件。
//
// d <= 0 时返回 nil（等价于不启用），方便上层按配置开关。
// 推荐挂在 RequestID / Recovery 之后、Auth / Controller 之前。
func Timeout(d time.Duration) gin.HandlerFunc {
	if d <= 0 {
		return nil
	}
	return gintimeout.New(
		gintimeout.WithTimeout(d),
		gintimeout.WithResponse(func(c *gin.Context) {
			// gin-contrib/timeout 在触发时会把 Writer 切回原始 ResponseWriter（见上游 timeout.go L133-134），
			// 所以这里可以直接 AbortWithStatusJSON，与项目其它错误响应路径行为一致。
			response.WriteErr(c, errcode.ErrTimeout)
		}),
	)
}
