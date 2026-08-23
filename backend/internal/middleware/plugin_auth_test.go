package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func setupPluginAuthRouter(t *testing.T, key string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	viper.Set("plugin.service.key", key)
	t.Cleanup(func() {
		viper.Set("plugin.service.key", "")
	})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("X-Request-ID", "req-test")
		c.Next()
	})
	r.POST("/plugin/heartbeat", PluginAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 200})
	})
	return r
}

func TestPluginAuthAllowsLoopbackWhenKeyUnset(t *testing.T) {
	r := setupPluginAuthRouter(t, "")
	req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "127.0.0.1:51000"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("loopback source should pass when key unset, got %d", w.Code)
	}
}

func TestPluginAuthAllowsPrivateSourceWhenKeyUnset(t *testing.T) {
	r := setupPluginAuthRouter(t, "")
	for _, addr := range []string{"192.168.1.20:5000", "10.0.0.7:5000", "172.18.0.5:5000"} {
		req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("private source %s should pass when key unset, got %d", addr, w.Code)
		}
	}
}

func TestPluginAuthRejectsPublicSourceWhenKeyUnset(t *testing.T) {
	r := setupPluginAuthRouter(t, "")
	for _, addr := range []string{"203.0.113.9:4000", "8.8.8.8:4000"} {
		req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("public source %s should be rejected when key unset, got %d", addr, w.Code)
		}
	}
}

func TestPluginAuthRejectsInvalidSourceString(t *testing.T) {
	if isTrustedPluginSource("not-an-addr") {
		t.Fatal("unparseable remote address must not be trusted")
	}
}

func TestPluginAuthStrictModeRequiresMatchingKey(t *testing.T) {
	r := setupPluginAuthRouter(t, "secret-key-1")

	req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "127.0.0.1:51001"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing header must fail in strict mode, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "127.0.0.1:51002"
	req.Header.Set(pluginKeyHeader, "wrong")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key must fail in strict mode, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set(pluginKeyHeader, "secret-key-1")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid key must pass from any source in strict mode, got %d", w.Code)
	}
}
