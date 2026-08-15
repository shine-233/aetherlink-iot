// 文件用途：覆盖 HTTP 中间件 cors 行为的 Go 测试。
// 核心逻辑：通过请求上下文、响应状态和边界输入验证认证、跨域、日志或安全处理逻辑，主要围绕 func TestCorsAllowsConfiguredOriginOnly、func TestCorsOptionsReturnsNoContent 等声明展开。
// 关键注意事项：中间件测试需保持状态码、上下文键和错误响应格式与客户端契约一致。
// 重构建议：后续可统一测试路由和上下文构造，减少重复的请求搭建代码。

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestCorsAllowsConfiguredOriginOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("cors.allowed_origins", []string{"http://127.0.0.1:5002"})
	viper.Set("cors.allow_credentials", true)

	router := gin.New()
	router.Use(Cors())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	allowedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	allowedReq.Header.Set("Origin", "http://127.0.0.1:5002")
	allowedRes := httptest.NewRecorder()
	router.ServeHTTP(allowedRes, allowedReq)

	if got := allowedRes.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5002" {
		t.Fatalf("allowed origin header = %q, want configured origin", got)
	}
	if got := allowedRes.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials header = %q, want true for configured origin", got)
	}

	blockedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	blockedReq.Header.Set("Origin", "http://evil.example")
	blockedRes := httptest.NewRecorder()
	router.ServeHTTP(blockedRes, blockedReq)

	if got := blockedRes.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("blocked origin header = %q, want empty", got)
	}
	if got := blockedRes.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("blocked credentials header = %q, want empty", got)
	}
}

func TestCorsOptionsReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("cors.allowed_origins", []string{"http://localhost:9725"})

	router := gin.New()
	router.Use(Cors())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:9725")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:9725" {
		t.Fatalf("OPTIONS allow origin = %q, want configured origin", got)
	}
}
