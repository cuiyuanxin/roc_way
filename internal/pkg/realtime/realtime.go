// Package realtime 提供 SSE 与 WebSocket 工具。
package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// SSE 持续从 ch 读事件并写入响应。事件类型为任意可 JSON 编码的值。
//
// 修复 [M7]：使用 select 监听 c.Request.Context().Done()，
// 客户端断开时立即退出协程，避免 goroutine 永久泄漏。
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
	ctx := c.Request.Context()
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(v)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// ---------- WebSocket Hub ----------

// Hub 管理一组客户端连接。
type Hub struct {
	mu             sync.RWMutex
	clients        map[*WSClient]struct{}
	reg            chan *WSClient
	unreg          chan *WSClient
	bcast          chan []byte
	allowedOrigins []string // 允许的 Origin 白名单；包含 "*" 表示放行所有
	upgrader       websocket.Upgrader
}

// NewHub 创建 Hub。
func NewHub() *Hub {
	h := &Hub{
		clients: make(map[*WSClient]struct{}),
		reg:     make(chan *WSClient, 16),
		unreg:   make(chan *WSClient, 16),
		bcast:   make(chan []byte, 64),
	}
	// 默认：允许所有 Origin（与原行为兼容）；生产环境应在 NewApp 里
	// 调用 SetAllowedOrigins 注入 CORS 共享的白名单，避免 CSWSH 攻击。
	h.upgrader = websocket.Upgrader{
		CheckOrigin: h.checkOrigin,
	}
	return h
}

// SetAllowedOrigins 注入允许的 Origin 白名单。
//
// 与 d.Cfg.Server.CORS.Origins 共享同一份配置：
//   - 包含 "*"：放行所有 origin（仅在 AllowCredentials=false 时安全）；
//   - 否则按精确匹配校验请求 Origin 头。
//
// 必须在 Run() 启动前调用；运行期变更需要重启 Hub。
func (h *Hub) SetAllowedOrigins(origins []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = origins
}

// checkOrigin 校验请求 Origin 是否在白名单内。
func (h *Hub) checkOrigin(r *http.Request) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.allowedOrigins) == 0 {
		return true // 未配置时保持放行（开发友好）
	}
	for _, o := range h.allowedOrigins {
		if o == "*" || strings.EqualFold(o, r.Header.Get("Origin")) {
			return true
		}
	}
	return false
}

// Run 启动 Hub 主循环。ctx 结束由调用方通过 close(stop) 触发，触发后会主动
// 关闭所有已注册 client 的 send 通道与底层连接，确保 readPump / writePump 协程退出，
// 避免上层关闭时 Goroutine 永久泄漏。
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
			h.closeAll()
			return
		}
	}
}

// closeAll 主动关闭所有 client 的 send 通道与底层连接（stop 触发时调用）。
func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		// 幂等：unreg 路径会 close(c.send)，此处仅在 client 仍在线时关闭。
		select {
		case <-c.send:
			// 已被关闭的 channel 从 receive 会立即返回零值，跳过
		default:
			close(c.send)
		}
		_ = c.conn.Close()
		delete(h.clients, c)
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

// WSClient 单个 WebSocket 客户端。
type WSClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Serve 处理单个 HTTP 升级请求并把客户端接入 Hub。
func (h *Hub) Serve(c *gin.Context) error {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
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
