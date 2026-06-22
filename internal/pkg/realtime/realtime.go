// Package realtime 提供 SSE 与 WebSocket 工具。
package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// SSE 持续从 ch 读事件并写入响应。事件类型为任意可 JSON 编码的值。
func SSE(c *gin.Context, ch <-chan any) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	for v := range ch {
		data, err := json.Marshal(v)
		if err != nil {
			continue
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// ---------- WebSocket Hub ----------

// Hub 管理一组客户端连接。
type Hub struct {
	mu      sync.RWMutex
	clients map[*WSClient]struct{}
	reg     chan *WSClient
	unreg   chan *WSClient
	bcast   chan []byte
}

// NewHub 创建 Hub。
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*WSClient]struct{}),
		reg:     make(chan *WSClient, 16),
		unreg:   make(chan *WSClient, 16),
		bcast:   make(chan []byte, 64),
	}
}

// Run 启动 Hub 主循环。ctx 结束由调用方通过 close(unreg/bcast) 触发，或在调用方用 select<-ctx.Done()。
func (h *Hub) Run(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case c := <-h.reg:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unreg:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case msg := <-h.bcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
				}
			}
			h.mu.RUnlock()
		case <-ticker.C:
			h.pingAll()
		case <-stop:
			return
		}
	}
}

func (h *Hub) pingAll() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		_ = c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
	}
}

// Broadcast 发送文本消息给所有客户端。
func (h *Hub) Broadcast(msg []byte) { h.bcast <- msg }

// Upgrader 默认 upgrader，允许跨域。
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// WSClient 单个 WebSocket 客户端。
type WSClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Serve 处理单个 HTTP 升级请求并把客户端接入 Hub。
func (h *Hub) Serve(c *gin.Context) error {
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return err
	}
	client := &WSClient{hub: h, conn: conn, send: make(chan []byte, 32)}
	h.reg <- client
	go client.writePump()
	go client.readPump()
	return nil
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unreg <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.hub.bcast <- msg
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
