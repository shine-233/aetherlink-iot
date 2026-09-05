// 文件用途：per-tenant API 限流中间件的窗口语义、关闭开关与 HTTP 契约测试。
// 核心逻辑：钉死固定窗口翻转、Retry-After 计算、无租户回退用户键、无 claims 透传与 rpm<=0 直通的契约。
// 关键注意事项：限流是可用性安全边界；时钟注入保证翻转用例不依赖真实时间；HTTP 用例走真实生产中间件。
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantRateLimiterAllowsUpToRpmWithinWindow(t *testing.T) {
	now := time.Now()
	limiter := newTenantRateLimiter(3)
	limiter.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		allowed, retry := limiter.allow(context.Background(), "tenant-a")
		require.True(t, allowed, "request %d should pass", i+1)
		assert.Zero(t, retry)
	}
	allowed, retryAfter := limiter.allow(context.Background(), "tenant-a")
	assert.False(t, allowed)
	assert.Positive(t, retryAfter)
	assert.LessOrEqual(t, retryAfter, int64(61))
}

func TestTenantRateLimiterResetsAfterWindowRolls(t *testing.T) {
	now := time.Now()
	limiter := newTenantRateLimiter(1)
	limiter.now = func() time.Time { return now }

	firstAllowed, _ := limiter.allow(context.Background(), "tenant-a")
	require.True(t, firstAllowed)
	allowed, _ := limiter.allow(context.Background(), "tenant-a")
	assert.False(t, allowed, "second request inside window must be limited")

	now = now.Add(2 * time.Minute)
	allowed, _ = limiter.allow(context.Background(), "tenant-a")
	assert.True(t, allowed, "new window must reset the counter")
}

func TestTenantRateLimiterIsolatesTenantsAndFallsBackToUserKey(t *testing.T) {
	now := time.Now()
	limiter := newTenantRateLimiter(1)
	limiter.now = func() time.Time { return now }

	firstOfA, _ := limiter.allow(context.Background(), "tenant-a")
	require.True(t, firstOfA)
	firstOfB, _ := limiter.allow(context.Background(), "tenant-b")
	assert.True(t, firstOfB, "other tenant must have its own bucket")

	firstOfUser, _ := limiter.allow(context.Background(), "user:u-1")
	require.True(t, firstOfUser)
	allowedUser, _ := limiter.allow(context.Background(), "user:u-1")
	assert.False(t, allowedUser, "empty-tenant subjects fall back to per-user keys")

	allowedEmpty, _ := limiter.allow(context.Background(), "")
	assert.True(t, allowedEmpty, "empty key is never limited")
}

func TestTenantRateLimiterDisabledWhenRpmNonPositive(t *testing.T) {
	now := time.Now()
	limiter := newTenantRateLimiter(0)
	limiter.now = func() time.Time { return now }
	negative := newTenantRateLimiter(-5)
	negative.now = func() time.Time { return now }

	for i := 0; i < 50; i++ {
		allowedA, _ := limiter.allow(context.Background(), "tenant-a")
		allowedB, _ := negative.allow(context.Background(), "tenant-b")
		require.True(t, allowedA)
		require.True(t, allowedB)
	}
}

// mountRateLimitedRoute 以生产中间件挂载测试路由；rpm 经 viper 注入（与 env 映射同键）。
func mountRateLimitedRoute(t *testing.T, rpm int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key := "api-rate-limit.requests-per-minute"
	prev := viper.Get(key)
	viper.Set(key, rpm)
	t.Cleanup(func() { viper.Set(key, prev) })

	engine := gin.New()
	engine.GET(
		"/t",
		func(c *gin.Context) {
			c.Set("claims", &utils.UserClaims{ID: "user-1", TenantID: "tenant-x"})
			c.Next()
		},
		TenantRateLimit(),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)
	return engine
}

func TestTenantRateLimitHTTPContractReturns429WithRetryAfter(t *testing.T) {
	engine := mountRateLimitedRoute(t, 1)

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/t", nil))
	require.Equal(t, http.StatusOK, first.Code, "first request passes")

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/t", nil))

	assert.Equal(t, http.StatusTooManyRequests, second.Code)
	retryHeader := second.Header().Get("Retry-After")
	require.NotEmpty(t, retryHeader)
	parsed, err := time.ParseDuration(retryHeader + "s")
	assert.NoError(t, err, "Retry-After must be an integer seconds value")
	assert.GreaterOrEqual(t, parsed, time.Second)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	assert.Equal(t, float64(errcode.CodeTooManyAttempts), body["code"])
	assert.NotEmpty(t, body["retry_after"])
}

func TestTenantRateLimitPassesThroughWithoutClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := "api-rate-limit.requests-per-minute"
	prev := viper.Get(key)
	viper.Set(key, 1)
	t.Cleanup(func() { viper.Set(key, prev) })

	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.GET("/t", TenantRateLimit(), func(c *gin.Context) { c.Status(http.StatusOK) })
	for i := 0; i < 3; i++ {
		recorder = httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/t", nil))
		require.Equal(t, http.StatusOK, recorder.Code, "requests without claims must not consume quota")
	}
}
