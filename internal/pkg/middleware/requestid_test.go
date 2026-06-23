package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestID_请求头透传：客户端传 X-Request-ID 时，中间件应原样保留。
func TestRequestID_请求头透传(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID(RequestIDOptions{}))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRequestID(c))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "trace-abc-123" {
		t.Fatalf("request_id 透传失败: want %q, got %q", "trace-abc-123", got)
	}
	if got := w.Header().Get("X-Request-ID"); got != "trace-abc-123" {
		t.Fatalf("响应头缺失/错误: %q", got)
	}
}

// TestRequestID_自动生成：客户端未传 X-Request-ID 时，中间件应自动生成。
func TestRequestID_自动生成(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID(RequestIDOptions{}))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRequestID(c))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Body.String(); got == "" {
		t.Fatal("request_id 未自动生成")
	}
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("响应头缺失自动生成的 request_id")
	}
	if w.Body.String() != w.Header().Get("X-Request-ID") {
		t.Fatalf("ctx 与响应头 request_id 不一致: body=%s header=%s",
			w.Body.String(), w.Header().Get("X-Request-ID"))
	}
}

// TestRequestID_自定义生成器：自定义 Generator 应被调用。
func TestRequestID_自定义生成器(t *testing.T) {
	gin.SetMode(gin.TestMode)

	want := "custom-id-fixed"
	r := gin.New()
	r.Use(RequestID(RequestIDOptions{Generator: func() string { return want }}))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, GetRequestID(c))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}
