// 文件用途：按租户维度的 API 限流中间件（对标 ThingsBoard PE 的 per-tenant rate limits）。
// 核心逻辑：固定窗口计数（窗口 60 秒），超过阈值返回 HTTP 429 + Retry-After。
// 存储后端二选一（api-rate-limit.backend）：
//   - memory（默认）：进程内计数，单机部署；零外部依赖；
//   - redis：固定窗口 Lua（INCR+PEXPIRE+PTTL 原子化），集群多实例共享配额；
//     Redis 故障 fail-open（放行并告警）——限流是滥用防护而非鉴权，不可用时不放大故障。
//
// 关键注意事项：与 apikey.go 的 IP 失败限流互补——那个防爆破，本件防滥用/失控轮询。
package middleware

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	tenantRateLimitRPMKey = "api-rate-limit.requests-per-minute"
	// 存储后端选择：memory（默认，单机）| redis（集群共享配额）。
	tenantRateLimitBackendKey = "api-rate-limit.backend"
	// 默认 600 req/min/tenant：覆盖正常控制台轮询与自动化脚本，
	// 同时能拦住失控的设备网关循环。<=0 表示关闭限流。
	defaultTenantRateLimitRPM = int64(600)
	tenantRateWindow          = time.Minute
)

// redisRateLimitScript 固定窗口计数脚本：INCR 首次命中设置窗口 TTL，
// 返回 [当前计数, 剩余 TTL(ms)]——PTTL 用于计算 Retry-After，无需第二次往返。
//
//go:embed tenant_rate_limit.lua
var redisRateLimitScript string

// tenantRateStore 固定窗口限流存储抽象。
type tenantRateStore interface {
	// allow 消费 subject 一次配额；返回是否放行与被拒时的 Retry-After 秒数。
	// ctx 为请求上下文（c.Request.Context()）：Redis 调用随请求取消，memory 后端忽略。
	allow(ctx context.Context, subject string) (allowed bool, retryAfter int64)
}

type tenantRateWindowEntry struct {
	count     int64
	windowEnd time.Time
}

// tenantRateLimiter 是可注入时钟的进程内固定窗口计数器（memory 后端），
// 便于单测覆盖翻转语义。
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
// （ctx 为请求上下文；进程内实现无 IO，忽略。）
func (l *tenantRateLimiter) allow(_ context.Context, tenantKey string) (bool, int64) {
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

// redisRateStore Redis 共享固定窗口计数器（redis 后端，集群多实例同配额）。
type redisRateStore struct {
	client *redis.Client
	rpm    int64
	window time.Duration
	// 告警节流：Redis 故障 fail-open 期间每分钟至多一条告警，防日志风暴。
	mu          sync.Mutex
	lastWarnAt  time.Time
	failOpenCnt uint64
}

func newRedisRateStore(client *redis.Client, rpm int64) *redisRateStore {
	return &redisRateStore{client: client, rpm: rpm, window: tenantRateWindow}
}

// redisRateLimitKeyPrefix 计数键前缀；窗口键随窗口自然过期，无需额外清理。
const redisRateLimitKeyPrefix = "aetherlink:ratelimit:tenant:"

// allow 原子消费配额；Redis 故障 fail-open（放行 + 节流告警）。
func (s *redisRateStore) allow(ctx context.Context, tenantKey string) (bool, int64) {
	if s.rpm <= 0 || tenantKey == "" {
		return true, 0
	}
	// 请求上下文 + 2s 短超时：客户端断开/请求结束即取消，Redis 抖动至多拖慢 2s。
	evalCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res, err := s.client.Eval(evalCtx, redisRateLimitScript,
		[]string{redisRateLimitKeyPrefix + tenantKey},
		s.rpm, s.window.Milliseconds()).Slice()
	if err != nil {
		s.warnFailOpen(err)
		return true, 0
	}
	if len(res) != 2 {
		s.warnFailOpen(fmt.Errorf("限流脚本返回形状异常: %v", res))
		return true, 0
	}
	count, _ := res[0].(int64)
	ttlMs, _ := res[1].(int64)
	if count > s.rpm {
		retryAfter := (ttlMs + 999) / 1000
		if retryAfter <= 0 {
			retryAfter = 1
		}
		return false, retryAfter
	}
	return true, 0
}

// warnFailOpen 记录 fail-open（每分钟至多一条）。
func (s *redisRateStore) warnFailOpen(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOpenCnt++
	now := time.Now()
	if now.Sub(s.lastWarnAt) < time.Minute {
		return
	}
	s.lastWarnAt = now
	logrus.WithError(err).Warnf("api-rate-limit: Redis 不可用，限流 fail-open（累计 %d 次）", s.failOpenCnt)
}

// tenantRateRPMFromConfig 读取 rpm 配置；未设置回落默认 600。
func tenantRateRPMFromConfig() int64 {
	rpm := viper.GetInt64(tenantRateLimitRPMKey)
	if rpm == 0 && !viper.IsSet(tenantRateLimitRPMKey) {
		rpm = defaultTenantRateLimitRPM
	}
	return rpm
}

// tenantRateStoreFromConfig 依据 api-rate-limit.backend 构建存储后端；
// 显式选择 redis 但 Redis 未就绪时回退 memory（限流可降级，不可缺失）。
func tenantRateStoreFromConfig(rpm int64) tenantRateStore {
	backend := strings.ToLower(strings.TrimSpace(viper.GetString(tenantRateLimitBackendKey)))
	switch backend {
	case "redis":
		if global.REDIS == nil {
			logrus.Warn("api-rate-limit.backend=redis 但 Redis 未就绪，限流回退 memory 后端")
			return newTenantRateLimiter(rpm)
		}
		return newRedisRateStore(global.REDIS, rpm)
	default:
		return newTenantRateLimiter(rpm)
	}
}

// TenantRateLimit 返回按租户计数的 API 限流中间件。
// 键优先取 claims.TenantID；无租户上下文的调用（如超管个人操作）回退用户 ID，
// 保证每个调用主体都有独立配额且匿名请求不会污染租户桶。
func TenantRateLimit() gin.HandlerFunc {
	rpm := tenantRateRPMFromConfig()
	store := tenantRateStoreFromConfig(rpm)
	return func(c *gin.Context) {
		if rpm <= 0 {
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
		allowed, retryAfter := store.allow(c.Request.Context(), subject)
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
