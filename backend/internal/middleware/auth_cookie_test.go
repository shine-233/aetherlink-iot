// 文件用途：覆盖认证 cookie 双模式行为的 Go 测试。
// 核心逻辑：通过 httptest 驱动真实 Gin 路由，断言 Set-Cookie 标志位（HttpOnly/SameSite/Path/Secure）、
// 开关行为，以及刷新端点 cookie 优先 / x-token 头回退的 token 选择契约。
// 关键注意事项：viper 全局状态需逐用例显式设置并在结束时 Reset，避免污染同包其他测试。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func setupAuthCookieTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)
	// viper.Reset 会清掉启动时注册的环境变量映射，这里按生产配置重建，
	// 保证 GOTP_AUTH_COOKIE_* 环境变量在测试中真实生效。
	viper.SetEnvPrefix("GOTP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault(authCookieEnabledKey, true)
	viper.SetDefault(authCookieSecureKey, false)
}

func newAuthCookieRouter(t *testing.T, handle gin.HandlerFunc) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.GET("/api/v1/user/refresh", handle)
	return router
}

// parseSetCookie 从响应头中取出认证 cookie 原始串，便于断言标志位。
func rawAuthSetCookie(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	for _, ck := range res.Header().Values("Set-Cookie") {
		if len(ck) >= len(AuthCookieName) && ck[:len(AuthCookieName)] == AuthCookieName {
			return ck
		}
	}
	return ""
}

func TestSetAuthCookieWritesExpectedFlags(t *testing.T) {
	setupAuthCookieTest(t)

	var capturedHeader http.Header
	router := newAuthCookieRouter(t, func(c *gin.Context) {
		SetAuthCookie(c, "token-value", 3600)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		capturedHeader = c.Writer.Header().Clone()
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/refresh", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	raw := rawAuthSetCookie(t, res)
	if raw == "" {
		t.Fatal("Set-Cookie header missing, want auth cookie set")
	}
	cookies := (&http.Response{Header: capturedHeader}).Cookies()
	var got *http.Cookie
	for _, ck := range cookies {
		if ck.Name == AuthCookieName {
			got = ck
			break
		}
	}
	if got == nil {
		t.Fatalf("auth cookie %q not parsed from response headers", AuthCookieName)
	}
	if got.Value != "token-value" {
		t.Fatalf("cookie value = %q, want token-value", got.Value)
	}
	if !got.HttpOnly {
		t.Fatal("cookie HttpOnly = false, want true")
	}
	if got.Path != AuthCookiePath {
		t.Fatalf("cookie Path = %q, want %q", got.Path, AuthCookiePath)
	}
	if got.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", got.SameSite)
	}
	if got.Secure {
		t.Fatal("cookie Secure = true, want false by default (本地 HTTP 可用)")
	}
}

func TestSetAuthCookieHonorsSecureEnv(t *testing.T) {
	setupAuthCookieTest(t)
	t.Setenv("GOTP_AUTH_COOKIE_SECURE", "true")

	router := newAuthCookieRouter(t, func(c *gin.Context) {
		SetAuthCookie(c, "token-value", 3600)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/refresh", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	raw := rawAuthSetCookie(t, res)
	if raw == "" {
		t.Fatal("Set-Cookie header missing, want auth cookie set")
	}
	found := false
	for _, ck := range res.Result().Cookies() {
		if ck.Name == AuthCookieName {
			found = true
			if !ck.Secure {
				t.Fatal("cookie Secure = false, want true when GOTP_AUTH_COOKIE_SECURE=true")
			}
		}
	}
	if !found {
		t.Fatalf("auth cookie %q not found in response cookies", AuthCookieName)
	}
}

func TestSetAuthCookieDisabledEmitsNoCookie(t *testing.T) {
	setupAuthCookieTest(t)
	viper.Set(authCookieEnabledKey, false)
	t.Setenv("GOTP_AUTH_COOKIE_ENABLED", "false")

	router := newAuthCookieRouter(t, func(c *gin.Context) {
		SetAuthCookie(c, "token-value", 3600)
		ReadAuthCookie(c)
		selectJWTAuthToken(c, "header-token")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/refresh", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "cookie-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if raw := rawAuthSetCookie(t, res); raw != "" {
		t.Fatalf("Set-Cookie header = %q, want empty when GOTP_AUTH_COOKIE_ENABLED=false", raw)
	}
}

// TestSelectJWTAuthTokenDualMode 覆盖刷新端点的双模式来源选择契约：
// cookie 优先；仅 x-token 头时保持兼容走 header；非刷新路由不受 cookie 影响。
func TestSelectJWTAuthTokenDualMode(t *testing.T) {
	setupAuthCookieTest(t)

	tests := []struct {
		name        string
		method      string
		path        string
		headerToken string
		cookieToken string
		want        string
	}{
		{
			name:        "refresh prefers cookie over header",
			method:      http.MethodGet,
			path:        "/api/v1/user/refresh",
			headerToken: "header-token",
			cookieToken: "cookie-token",
			want:        "cookie-token",
		},
		{
			name:        "refresh falls back to header when cookie missing (存量客户端兼容)",
			method:      http.MethodGet,
			path:        "/api/v1/user/refresh",
			headerToken: "header-token",
			want:        "header-token",
		},
		{
			name:        "non-refresh route ignores cookie",
			method:      http.MethodGet,
			path:        "/api/v1/devices",
			headerToken: "header-token",
			cookieToken: "cookie-token",
			want:        "header-token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, tc.path, nil)
			if tc.cookieToken != "" {
				c.Request.AddCookie(&http.Cookie{Name: AuthCookieName, Value: tc.cookieToken})
			}
			c.Request.Header.Set("x-token", tc.headerToken)

			if got := selectJWTAuthToken(c, tc.headerToken); got != tc.want {
				t.Fatalf("selectJWTAuthToken() = %q, want %q", got, tc.want)
			}
		})
	}
}
