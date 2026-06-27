package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// Realtime 演示 SSE 与 WebSocket。
type Realtime struct {
	Hub *realtime.Hub
}

// NewRealtime 实时通信控制器构造函数。
func NewRealtime(hub *realtime.Hub) *Realtime { return &Realtime{Hub: hub} }

// Register 绑定路由。
func (r *Realtime) Register(g gin.IRouter) {
	g.GET("/sse/notifications", r.sse)
	g.GET("/ws/chat", r.ws)
}

// @Summary SSE 实时通知
// @Description 通过 Server-Sent Events 推送实时通知
// @Tags Realtime
// @Produce text/event-stream
// @Success 200 {string} string "event-stream"
// @Router /sse/notifications [get]
func (r *Realtime) sse(c *gin.Context) {
	ch := make(chan any, 8)
	// 修复 [M7]：ticker 协程监听 c.Request.Context().Done()，
	// 客户端断开后退出，避免 goroutine 永久泄漏。
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		i := 0
		for {
			select {
			case now := <-t.C:
				i++
				select {
				case ch <- gin.H{"seq": i, "ts": now.Unix()}:
				default:
				}
			case <-c.Request.Context().Done():
				close(ch)
				return
			}
		}
	}()
	realtime.SSE(c, ch)
}

// @Summary WebSocket 聊天
// @Description 通过 WebSocket 进行实时双向通信
// @Tags Realtime
// @Success 101 {string} string " Switching Protocols"
// @Router /ws/chat [get]
func (r *Realtime) ws(c *gin.Context) {
	_ = r.Hub.Serve(c)
}
