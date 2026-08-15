// 文件用途：验证基础安全响应头覆盖正常、错误和短路响应。
// 核心逻辑：通过独立 Gin engine 断言中间件在 handler 或后续中间件提前终止时仍写入固定策略。
// 关键注意事项：测试不包含 HSTS；TLS 终止层必须在确认全站 HTTPS 后单独配置。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersCoverSuccessAndErrorResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/error", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	for _, path := range []string{"/ok", "/error", "/missing"} {
		t.Run(path, func(t *testing.T) {
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))

			assertSecurityHeaders(t, res)
		})
	}
}

func TestSecurityHeadersSurviveLaterMiddlewareAbort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.Use(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	router.GET("/short-circuit", func(c *gin.Context) {
		t.Fatal("handler ran after abort")
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodOptions, "/short-circuit", nil))

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	assertSecurityHeaders(t, res)
}

func assertSecurityHeaders(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for name, value := range want {
		if got := res.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if got := res.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want unset without guaranteed HTTPS", got)
	}
}
