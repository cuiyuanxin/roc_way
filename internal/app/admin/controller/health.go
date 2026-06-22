package controller

import "github.com/gin-gonic/gin"

// Health 简单健康检查。
type Health struct{}

func NewHealth() *Health { return &Health{} }

// Register 绑定路由。
func (h *Health) Register(r gin.IRouter) {
	r.GET("/healthz", h.handle)
}

func (h *Health) handle(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
