package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// TestTimeout_正常返回：handler 200ms 完成，1s 超时下应正常返回 200。
func TestTimeout_正常返回(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID(RequestIDOptions{}))
	r.Use(Timeout(1 * time.Second))
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(200 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/slow", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "ok" {
		t.Fatalf("want body=ok, got %q", w.Body.String())
	}
}

// TestTimeout_超时返回504：handler 故意 sleep 500ms，200ms 超时下应返回 504 且 body 含 request_id。
func TestTimeout_超时返回504(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID(RequestIDOptions{}))
	r.Use(Timeout(200 * time.Millisecond))
	r.GET("/slow", func(c *gin.Context) {
		// 故意阻塞超过 timeout 阈值
		time.Sleep(500 * time.Millisecond)
		c.String(http.StatusOK, "should-not-reach")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/slow", nil)
	req.Header.Set("X-Request-ID", "trace-timeout-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("want 504, got %d, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !contains(body, `"code":1504`) {
		t.Fatalf("响应缺少 code:1504, body=%s", body)
	}
	if !contains(body, `"request_id":"trace-timeout-1"`) {
		t.Fatalf("响应缺少透传的 request_id, body=%s", body)
	}
	if contains(body, "should-not-reach") {
		t.Fatalf("超时应丢弃 handler 输出，但仍泄漏: %s", body)
	}
}

// TestTimeout_零值禁用：传入 <=0 时返回 nil，调用方应能 nil-safe Use。
func TestTimeout_零值禁用(t *testing.T) {
	if got := Timeout(0); got != nil {
		t.Fatalf("Timeout(0) 应返回 nil, got %T", got)
	}
	if got := Timeout(-1 * time.Second); got != nil {
		t.Fatalf("Timeout(-1s) 应返回 nil, got %T", got)
	}
}

// TestErrTimeout_已注册：确保 errcode 中确实注册了 ErrTimeout。
func TestErrTimeout_已注册(t *testing.T) {
	if errcode.ErrTimeout.Code != 1504 {
		t.Fatalf("ErrTimeout.Code want 1504, got %d", errcode.ErrTimeout.Code)
	}
	if errcode.ErrTimeout.HTTPStatus != 504 {
		t.Fatalf("ErrTimeout.HTTPStatus want 504, got %d", errcode.ErrTimeout.HTTPStatus)
	}
	if errcode.ErrTimeout.Message == "" {
		t.Fatalf("ErrTimeout.Message 不应为空")
	}
}
