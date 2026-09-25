// 文件用途：TB-17 日配额限额来源——经现有 billing dal 读取租户订阅套餐的 max_api_calls_per_day。
// 核心逻辑：tenant_subscriptions →（无订阅归 free）→ subscription_plans.MaxApiCallsPerDay；
// 进程内 TTL 缓存避免每请求查库；DB 失败时返回 stale 缓存（无则 0=不执法），执法链路 fail-open。
// 关键注意事项：本类型是 internal/dal 的薄封装，不做任何写操作；套餐字段缺失（plan 为 nil）
// 与 117.sql 的 GetTenantUsage 容错口径一致——按"未配置阈值"处理，不封禁租户。
// 重构建议：若套餐支持按租户覆盖限额，在 DailyAPILimit 内加一层覆盖查找即可，调用方无感。
package quota

import (
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"

	"github.com/sirupsen/logrus"
)

// LimitSource 日配额限额来源抽象（单测注入固定限额）。
type LimitSource interface {
	// DailyAPILimit 返回租户的每日 API 调用限额；<=0 表示未配置执法阈值（不执法）。
	DailyAPILimit(tenantID string) (int64, error)
}

// defaultLimitCacheTTL 限额缓存 TTL：套餐变更有至多 1 分钟的执法延迟，可接受。
const defaultLimitCacheTTL = time.Minute

type limitCacheEntry struct {
	limit     int64
	expiresAt time.Time
}

// BillingLimitSource 经 billing dal 读取套餐限额并做 TTL 缓存。
type BillingLimitSource struct {
	mu    sync.RWMutex
	cache map[string]limitCacheEntry
	ttl   time.Duration
	now   func() time.Time
}

// NewBillingLimitSource 构建套餐限额来源（默认 TTL 1 分钟）。
func NewBillingLimitSource(ttl time.Duration) *BillingLimitSource {
	if ttl <= 0 {
		ttl = defaultLimitCacheTTL
	}
	return &BillingLimitSource{
		cache: make(map[string]limitCacheEntry),
		ttl:   ttl,
		now:   time.Now,
	}
}

// DailyAPILimit 读取租户当日 API 调用限额：订阅套餐优先，无订阅归 free。
// 错误仅在 DB 不可用且无可用缓存时返回——调用方应 fail-open（视为不执法）。
func (s *BillingLimitSource) DailyAPILimit(tenantID string) (int64, error) {
	if tenantID == "" {
		return 0, nil
	}

	s.mu.RLock()
	entry, ok := s.cache[tenantID]
	s.mu.RUnlock()
	if ok && s.now().Before(entry.expiresAt) {
		return entry.limit, nil
	}

	limit, err := s.load(tenantID)
	if err != nil {
		// DB 抖动时退回过期缓存：执法面保持"最后已知限额"，而非骤然放开或误伤。
		if ok {
			logrus.WithError(err).Debugf("quota: load billing limit failed, serve stale cache for tenant %s", tenantID)
			return entry.limit, nil
		}
		return 0, err
	}

	s.mu.Lock()
	s.cache[tenantID] = limitCacheEntry{limit: limit, expiresAt: s.now().Add(s.ttl)}
	s.mu.Unlock()
	return limit, nil
}

// load 经现有 billing dal 解析租户当前套餐的 max_api_calls_per_day。
func (*BillingLimitSource) load(tenantID string) (int64, error) {
	sub, err := dal.GetTenantSubscription(tenantID)
	if err != nil {
		return 0, err
	}
	planCode := "free"
	if sub != nil && sub.PlanCode != "" {
		planCode = sub.PlanCode
	}
	plan, err := dal.GetSubscriptionPlanByCode(planCode)
	if err != nil {
		return 0, err
	}
	if plan == nil {
		// 套餐目录缺失（如运维清库）：与 GetTenantUsage 的容错口径一致，按未配置阈值处理。
		return 0, nil
	}
	return int64(plan.MaxApiCallsPerDay), nil
}
