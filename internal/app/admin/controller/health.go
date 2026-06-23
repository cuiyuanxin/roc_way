package controller

import "github.com/gin-gonic/gin"

// Health 简单健康检查。
type Health struct{}

// NewHealth 健康检查控制器构造函数。
func NewHealth() *Health { return &Health{} }

// Register 绑定路由。
func (h *Health) Register(r gin.IRouter) {
	r.GET("/healthz", h.handle)
}

// @Summary 健康检查
// @Description 返回服务健康状态
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string "status":"ok"
// @Router /healthz [get]
func (h *Health) handle(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
