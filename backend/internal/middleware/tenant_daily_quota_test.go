// 文件用途：TB-17 租户 API 日配额执法的 HTTP 契约测试（真实生产中间件 + 注入替身配额服务）。
// 核心逻辑：超限 → 429 + Retry-After 头 + code/message/retry_after 响应体（与 per-minute
// 限流 429 同构）；未超限放行；无租户上下文不执法；per-minute 关闭时日配额仍独立生效。
// 关键注意事项：dailyQuotaService 包级函数变量是唯一注入点，用例结束后必须恢复；
// viper 的 api-rate-limit 配置同样需要用例内自清理，避免污染其他用例。
// 重构建议：若执法语义扩展（如软限额），在本文件追加对应契约用例。
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/quota"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeQuotaMeter / fakeQuotaLimits 可编程替身：控制计数与限额，验证执法路径。
type fakeQuotaMeter struct {
	count int64
	err   error
}

func (f *fakeQuotaMeter) Incr(context.Context, string) (int64, error) {
	f.count++
	return f.count, f.err
}

func (f *fakeQuotaMeter) TodayUsage(context.Context, string) (int64, error) {
	return f.count, f.err
}

type fakeQuotaLimits struct {
	limit int64
	err   error
}

func (f *fakeQuotaLimits) DailyAPILimit(string) (int64, error) { return f.limit, f.err }

// withQuotaTestEnv 注入替身配额服务并关闭 per-minute 限流，返回恢复函数。
// rpm=0 且未配置 default-tenant-limits 时中间件跳过 per-minute 路径（既有语义），
// 用例得以单独钉死日配额契约。
func withQuotaTestEnv(t *testing.T, meter *fakeQuotaMeter, limits *fakeQuotaLimits) {
	t.Helper()
	rpmKey := "api-rate-limit.requests-per-minute"
	prevRPM := viper.Get(rpmKey)
	viper.Set(rpmKey, 0)
	t.Cleanup(func() { viper.Set(rpmKey, prevRPM) })
	original := dailyQuotaService
	svc := quota.NewService(meter, limits)
	dailyQuotaService = func() *quota.Service { return svc }
	t.Cleanup(func() { dailyQuotaService = original })
}

// newQuotaTestRouter 构建带 claims 注入的最小 gin 路由。
func newQuotaTestRouter(t *testing.T, tenantID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &utils.UserClaims{ID: "user-1", TenantID: tenantID, Authority: "TENANT_ADMIN"})
		c.Next()
	})
	router.GET("/ping", TenantRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	return router
}

func TestDailyQuotaBlocksOverLimitWith429Contract(t *testing.T) {
	meter := &fakeQuotaMeter{}
	withQuotaTestEnv(t, meter, &fakeQuotaLimits{limit: 2})
	router := newQuotaTestRouter(t, "tenant-a")

	// 前两次放行（used=1,2），第三次 used=3 > 2 被拒。
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		assert.Equal(t, http.StatusOK, w.Code, "request %d must pass", i+1)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code)

	retryAfterHeader := w.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfterHeader, "429 must carry Retry-After header")
	_, err := time.ParseDuration(retryAfterHeader + "s")
	assert.NoError(t, err, "Retry-After must be a second-count string")

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(errcode.CodeTooManyAttempts), body["code"])
	assert.Equal(t, "daily API quota exceeded for tenant", body["message"])
	assert.Contains(t, body, "retry_after", "429 body must expose retry_after like per-minute limiting")
}

func TestDailyQuotaAllowsUnderLimitAndUnlimited(t *testing.T) {
	meter := &fakeQuotaMeter{}
	withQuotaTestEnv(t, meter, &fakeQuotaLimits{limit: 5})
	router := newQuotaTestRouter(t, "tenant-a")

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		assert.Equal(t, http.StatusOK, w.Code, "request %d within limit must pass", i+1)
	}

	// 限额未配置（<=0）→ 不执法，超出次数也放行。
	withQuotaTestEnv(t, &fakeQuotaMeter{}, &fakeQuotaLimits{limit: 0})
	router = newQuotaTestRouter(t, "tenant-a")
	for i := 0; i < 7; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		assert.Equal(t, http.StatusOK, w.Code, "unenforced quota must pass, request %d", i+1)
	}
}

func TestDailyQuotaFailsOpenOnMeteringError(t *testing.T) {
	meter := &fakeQuotaMeter{err: assert.AnError}
	withQuotaTestEnv(t, meter, &fakeQuotaLimits{limit: 1})
	router := newQuotaTestRouter(t, "tenant-a")

	// 计量失败：请求必须放行（fail-open），不得以计量故障拒绝业务。
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDailyQuotaSkipsWhenNoTenantContext(t *testing.T) {
	meter := &fakeQuotaMeter{}
	withQuotaTestEnv(t, meter, &fakeQuotaLimits{limit: 1})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// 超管个人操作：无租户上下文。
		c.Set("claims", &utils.UserClaims{ID: "sys-user", TenantID: "", Authority: "SYS_ADMIN"})
		c.Next()
	})
	router.GET("/ping", TenantRateLimit(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"message": "pong"}) })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		assert.Equal(t, http.StatusOK, w.Code, "no-tenant request %d must not be quota-enforced", i+1)
	}
	assert.Zero(t, meter.count, "no-tenant requests must not be metered")
}

func TestDailyQuotaStillEnforcedWhenPerMinuteLimitDisabled(t *testing.T) {
	// per-minute 限流关闭（rpm<=0 且未配置 default-tenant-limits）不影响日配额执法。
	meter := &fakeQuotaMeter{}
	withQuotaTestEnv(t, meter, &fakeQuotaLimits{limit: 1})
	router := newQuotaTestRouter(t, "tenant-a")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
