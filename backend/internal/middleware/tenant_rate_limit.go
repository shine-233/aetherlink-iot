// 文件用途：按租户维度的 API 限流中间件（对标 ThingsBoard PE 的 per-tenant rate limits）。
// 核心逻辑：进程内固定窗口计数，窗口 60 秒；超过阈值返回 HTTP 429 + Retry-After。
// 关键注意事项：与 apikey.go 的 IP 失败限流互补——那个防爆破，本件防滥用/失控轮询；
// 当前为单机进程内实现，集群部署需替换共享存储（Redis INCR+EXPIRE），接口形状已按可替换设计。
package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const (
	tenantRateLimitRPMKey = "api-rate-limit.requests-per-minute"
	// 默认 600 req/min/tenant：覆盖正常控制台轮询与自动化脚本，
	// 同时能拦住失控的设备网关循环。<=0 表示关闭限流。
	defaultTenantRateLimitRPM = int64(600)
	tenantRateWindow          = time.Minute
)

type tenantRateWindowEntry struct {
	count    int64
	windowEnd time.Time
}

// tenantRateLimiter 是可注入时钟的固定窗口计数器，便于单测覆盖翻转语义。
type tenantRateLimiter struct {
	mu     sync.Mutex
	rpm    int64
	now    func() time.Time
	counts map[string]*tenantRateWindowEntry
}

func newTenantRateLimiter(rpm int64) *tenantRateLimiter {
	if rpm < 0 {
		rpm = 0
	}
	return &tenantRateLimiter{
		rpm:    rpm,
		now:    time.Now,
		counts: make(map[string]*tenantRateWindowEntry),
	}
}

// allow 返回是否放行；被拒时附带建议的 Retry-After 秒数。
func (l *tenantRateLimiter) allow(tenantKey string) (bool, int64) {
	if l.rpm <= 0 || tenantKey == "" {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, ok := l.counts[tenantKey]
	if !ok || now.After(entry.windowEnd) {
		l.counts[tenantKey] = &tenantRateWindowEntry{count: 1, windowEnd: now.Add(tenantRateWindow)}
		return true, 0
	}
	if entry.count >= l.rpm {
		retryAfter := int64(entry.windowEnd.Sub(now).Seconds()) + 1
		return false, retryAfter
	}
	entry.count++
	return true, 0
}

func tenantRateLimiterFromConfig() *tenantRateLimiter {
	rpm := viper.GetInt64(tenantRateLimitRPMKey)
	if rpm == 0 && !viper.IsSet(tenantRateLimitRPMKey) {
		rpm = defaultTenantRateLimitRPM
	}
	return newTenantRateLimiter(rpm)
}

// TenantRateLimit 返回按租户计数的 API 限流中间件。
// 键优先取 claims.TenantID；无租户上下文的调用（如超管个人操作）回退用户 ID，
// 保证每个调用主体都有独立配额且匿名请求不会污染租户桶。
func TenantRateLimit() gin.HandlerFunc {
	limiter := tenantRateLimiterFromConfig()
	return func(c *gin.Context) {
		if limiter.rpm <= 0 {
			c.Next()
			return
		}
		claimsValue, exists := c.Get("claims")
		if !exists {
			c.Next()
			return
		}
		claims, ok := claimsValue.(*utils.UserClaims)
		if !ok || claims == nil {
			c.Next()
			return
		}
		subject := claims.TenantID
		if subject == "" {
			subject = "user:" + claims.ID
		}
		allowed, retryAfter := limiter.allow(subject)
		if allowed {
			c.Next()
			return
		}
		c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":        errcode.CodeTooManyAttempts,
			"message":     "API rate limit exceeded for tenant",
			"retry_after": retryAfter,
		})
	}
}
