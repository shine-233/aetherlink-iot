package oidc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 未注入 HTTP 客户端时不得回落到无超时的 http.DefaultClient。
func TestClientDefaultHTTPHasTimeout(t *testing.T) {
	c := &Client{}
	got := c.http()
	if got == http.DefaultClient {
		t.Fatal("default client must not be http.DefaultClient (no timeout)")
	}
	if got.Timeout <= 0 {
		t.Fatalf("default client timeout = %v, want > 0", got.Timeout)
	}
	injected := &http.Client{}
	c.HTTP = injected
	if c.http() != injected {
		t.Fatal("injected client must be used")
	}
}

// 回调作废 state cookie 时 Secure 须与写入一致：HTTP 源上带 Secure 的删除指令会被浏览器丢弃。
func TestCallbackClearsStateCookieWithoutSecureOnPlainHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 空 ProviderConfig：discovery 会失败（502），但 cookie 作废发生在 discovery 之前。
	m := NewMiddleware(ProviderConfig{DiscoveryURL: "http://127.0.0.1:1/unreachable"}, nil)
	r := gin.New()
	r.GET(m.CallbackPath, m.handleCallback)

	req := httptest.NewRequest(http.MethodGet, m.CallbackPath+"?code=abc&state=STATE1", nil)
	req.AddCookie(&http.Cookie{Name: m.StateCookie, Value: "STATE1.NONCE1", Path: "/"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var cleared *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == m.StateCookie {
			cleared = ck
		}
	}
	if cleared == nil {
		t.Fatal("callback must clear the state cookie")
	}
	if cleared.MaxAge >= 0 {
		t.Fatalf("cleared cookie MaxAge = %d, want < 0", cleared.MaxAge)
	}
	if cleared.Secure {
		t.Fatal("state cookie deletion over plain HTTP must not carry Secure (browser would ignore it)")
	}
}
