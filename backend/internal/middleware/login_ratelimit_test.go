// 文件用途：覆盖登录入口按 IP 失败限流的窗口行为与中间件响应契约。
// 核心逻辑：验证失败计数累积触发 429、成功后清零、窗口过期自动重置。
// 关键注意事项：直接操作包内 sync.Map 状态，用例间必须清理，避免互相污染。

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func resetLoginRateLimitState(t *testing.T, ip string) {
	t.Helper()
	loginIPFailCounts.Delete(ip)
	t.Cleanup(func() { loginIPFailCounts.Delete(ip) })
}

func TestLoginRateLimitBlocksAfterFailureThreshold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const ip = "203.0.113.10"
	resetLoginRateLimitState(t, ip)

	if loginRateLimited(ip) {
		t.Fatalf("ip %s should not be limited before any failure", ip)
	}

	for i := 0; i < loginIPFailLimit; i++ {
		RecordLoginFailure(ip)
	}
	if !loginRateLimited(ip) {
		t.Fatalf("ip %s should be limited after %d failures", ip, loginIPFailLimit)
	}

	router := gin.New()
	router.POST("login", LoginRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = ip + ":12345"
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("limited login status = %d, want 429", recorder.Code)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if body.Code != ErrCodeLoginRateLimited {
		t.Fatalf("429 code = %d, want %d", body.Code, ErrCodeLoginRateLimited)
	}
}

func TestLoginRateLimitResetsOnSuccessAndWindowExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const ip = "203.0.113.11"
	resetLoginRateLimitState(t, ip)

	RecordLoginFailure(ip)
	RecordLoginFailure(ip)

	// 登录成功应清空该 IP 的失败计数。
	ResetLoginFailures(ip)
	if loginRateLimited(ip) {
		t.Fatalf("ip %s should not be limited after success reset", ip)
	}
	value, ok := loginIPFailCounts.Load(ip)
	if ok {
		if entry, ok := value.(*loginIPFailEntry); ok && entry.count != 0 {
			t.Fatalf("failure count = %d after reset, want deleted", entry.count)
		}
	}

	// 窗口过期后计数应重新开始。
	for i := 0; i < loginIPFailLimit; i++ {
		RecordLoginFailure(ip)
	}
	entryValue, _ := loginIPFailCounts.Load(ip)
	entry := entryValue.(*loginIPFailEntry)
	entry.mu.Lock()
	entry.windowEnd = time.Now().Add(-time.Second)
	entry.mu.Unlock()

	if loginRateLimited(ip) {
		t.Fatalf("ip %s should not be limited after window expiry", ip)
	}
	RecordLoginFailure(ip)
	if loginRateLimited(ip) {
		t.Fatalf("a single failure in a fresh window must not limit ip %s", ip)
	}
}

func TestLoginRateLimitAllowsOtherIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const blockedIP = "203.0.113.12"
	const otherIP = "203.0.113.13"
	resetLoginRateLimitState(t, blockedIP)
	resetLoginRateLimitState(t, otherIP)

	for i := 0; i < loginIPFailLimit+1; i++ {
		RecordLoginFailure(blockedIP)
	}

	router := gin.New()
	router.POST("login", LoginRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	blocked := httptest.NewRecorder()
	blockedReq := httptest.NewRequest(http.MethodPost, "/login", nil)
	blockedReq.RemoteAddr = blockedIP + ":1"
	router.ServeHTTP(blocked, blockedReq)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked ip status = %d, want 429", blocked.Code)
	}

	allowed := httptest.NewRecorder()
	allowedReq := httptest.NewRequest(http.MethodPost, "/login", nil)
	allowedReq.RemoteAddr = otherIP + ":1"
	router.ServeHTTP(allowed, allowedReq)
	if allowed.Code != http.StatusOK {
		t.Fatalf("other ip status = %d, want 200", allowed.Code)
	}
}
