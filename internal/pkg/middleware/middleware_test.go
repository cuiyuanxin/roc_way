package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestRecovery_Release_不泄漏panic 详情：release 模式下响应里不应出现 panic 字符串。
func TestRecovery_Release_不泄漏panic(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(gin.TestMode)

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sweet := logger.Sugar()

	r := gin.New()
	r.Use(Recovery(sweet))
	r.GET("/boom", func(_ *gin.Context) { panic("super-secret-token-abc123") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("want 500, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] == nil || body["message"] == nil {
		t.Fatalf("missing code/message: %+v", body)
	}
	if body["error"] != nil {
		t.Fatalf("release 模式不应回显 panic 详情, got: %v", body["error"])
	}
	if contains(w.Body.String(), "super-secret-token-abc123") {
		t.Fatalf("响应体泄漏敏感信息: %s", w.Body.String())
	}
}

// TestRecovery_Debug_显示panic 详情：debug 模式响应里应包含 panic 字符串。
func TestRecovery_Debug_显示panic(t *testing.T) {
	gin.SetMode(gin.DebugMode)
	defer gin.SetMode(gin.TestMode)

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sweet := logger.Sugar()

	r := gin.New()
	r.Use(Recovery(sweet))
	r.GET("/boom", func(_ *gin.Context) { panic("debug-panic-detail") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/boom", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("want 500, got %d", w.Code)
	}
	if !contains(w.Body.String(), "debug-panic-detail") {
		t.Fatalf("debug 模式应回显 panic, got: %s", w.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
