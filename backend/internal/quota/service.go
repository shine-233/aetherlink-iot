// 文件用途：TB-17 日配额服务门面——把计量（meter）、限额（limit source）与执法判定串成一条链。
// 核心逻辑：ObserveAndDecide 在既有按租户限流路径上同日累加并判定是否超限；TodayUsage 供配额
// 查询端点读取当日已用量；Default 返回进程级单例并惰性启动定期落库协程。
// 关键注意事项：fail-open 是本服务的硬约束——计量失败、限额读取失败都不允许阻断请求；
// 无租户上下文（如超管个人操作）不计量也不执法（api_usage_daily 按租户键控）。
// 重构建议：若后续执法策略变化（如软限额告警），在 Decision 上加字段而非改变本门面签名。
package quota

import (
	"context"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// Meter 计量器抽象（单测注入假实现）。
type Meter interface {
	// Incr 同日累加一次并返回累加后的当日计数（含本次）；错误表示计数不可用（fail-open）。
	Incr(ctx context.Context, tenantID string) (int64, error)
	// TodayUsage 读取租户当日已用调用数。
	TodayUsage(ctx context.Context, tenantID string) (int64, error)
}

// Service 日配额服务：计量 + 限额 + 执法判定。
type Service struct {
	meter  Meter
	limits LimitSource
	now    func() time.Time
}

// NewService 组装服务（单测可注入替身；生产用 Default）。
func NewService(meter Meter, limits LimitSource) *Service {
	return &Service{meter: meter, limits: limits, now: time.Now}
}

var (
	defaultOnce sync.Once
	defaultSvc  *Service
)

// Default 返回进程级日配额服务单例：Redis 可用时多实例共享计数，并启动定期落库协程。
func Default() *Service {
	defaultOnce.Do(func() {
		meter := NewDailyMeter(global.REDIS, defaultFlushInterval)
		// 后台落库协程的生命周期与进程一致：用独立背景上下文，随 stopCh 退出前兜底落库一次。
		meter.StartFlusher(context.Background())
		defaultSvc = NewService(meter, NewBillingLimitSource(defaultLimitCacheTTL))
	})
	return defaultSvc
}

// ObserveAndDecide 既有按租户限流路径上的计量与执法入口：
// 先同日累加（被拒请求也计入当日用量），再按套餐限额判定。任何内部失败一律放行（fail-open）。
func (s *Service) ObserveAndDecide(ctx context.Context, tenantID string) Decision {
	if tenantID == "" {
		return Decision{Allowed: true, Status: "normal"}
	}

	used, err := s.meter.Incr(ctx, tenantID)
	if err != nil {
		// 计量失败不阻断请求：放行且不执法（执法依赖计数，计数不可用即无依据）。
		logrus.WithError(err).Debugf("quota: meter incr failed for tenant %s, fail-open", tenantID)
		return Decision{Allowed: true, Status: "normal"}
	}

	limit, err := s.limits.DailyAPILimit(tenantID)
	if err != nil {
		// 限额不可得同理放行：配额执法是商业约束，不得因读限额失败放大为服务故障。
		logrus.WithError(err).Debugf("quota: load daily limit failed for tenant %s, fail-open", tenantID)
		return Decision{Allowed: true, Used: used, Status: "normal"}
	}

	return DecideDailyQuota(limit, used, s.now())
}

// TodayUsage 读取租户当日已用调用数与计量日（UTC）；供配额查询端点使用。
func (s *Service) TodayUsage(ctx context.Context, tenantID string) (int64, string, error) {
	usage, err := s.meter.TodayUsage(ctx, tenantID)
	if err != nil {
		return 0, "", err
	}
	return usage, UsageDate(s.now()), nil
}
