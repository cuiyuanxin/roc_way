package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
)

// Health 简单健康检查。
type Health struct {
	limitMw gin.HandlerFunc
}

// NewHealth 健康检查控制器构造函数。
//
// limitMw 为路由级限流中间件，配额由 ratelimit.DefaultLimit 控制；
// app.go 通过 ratelimit.NewRoute 注入，nil 表示不限流。
func NewHealth(limitMw ...gin.HandlerFunc) *Health {
	var mw gin.HandlerFunc
	if len(limitMw) > 0 {
		mw = limitMw[0]
	}
	return &Health{limitMw: mw}
}

// Register 绑定路由。
func (h *Health) Register(r gin.IRouter) {
	handlers := []gin.HandlerFunc{}
	if h.limitMw != nil {
		handlers = append(handlers, h.limitMw)
	}
	handlers = append(handlers, h.handle)
	r.GET("/healthz", handlers...)
}

// @Summary 健康检查
// @Description 返回服务健康状态
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string "status":"ok"
// @Router /healthz [get]
func (h *Health) handle(c *gin.Context) {
	response.WriteOK(c, gin.H{"status": "ok"})
}
