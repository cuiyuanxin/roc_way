package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/realtime"
)

// Realtime 演示 SSE 与 WebSocket。
type Realtime struct {
	Hub *realtime.Hub
}

func NewRealtime(hub *realtime.Hub) *Realtime { return &Realtime{Hub: hub} }

func (r *Realtime) Register(g gin.IRouter) {
	g.GET("/sse/notifications", r.sse)
	g.GET("/ws/chat", r.ws)
}

func (r *Realtime) sse(c *gin.Context) {
	ch := make(chan any, 8)
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		i := 0
		for now := range t.C {
			i++
			select {
			case ch <- gin.H{"seq": i, "ts": now.Unix()}:
			default:
			}
		}
	}()
	realtime.SSE(c, ch)
}

func (r *Realtime) ws(c *gin.Context) {
	_ = r.Hub.Serve(c)
}
