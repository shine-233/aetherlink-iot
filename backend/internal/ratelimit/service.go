package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// DefaultService 全局限流服务实例。
	DefaultService *RateLimitService
	initOnce       sync.Once
)

// RateLimitService 集群限流与多策略配额综合服务。
type RateLimitService struct {
	evaluator Evaluator
	backend   string
	logger    *logrus.Logger

	defaultTenantAPIRules []RateLimitRule
	defaultTenantTxRules  []RateLimitRule
	defaultDeviceTxRules  []RateLimitRule

	overrideMu sync.RWMutex
	overrides  map[string][]RateLimitRule // key: tenantID:targetType:targetID:limitType

	// Metrics
	totalChecked   atomic.Int64
	totalAllowed   atomic.Int64
	totalBlocked   atomic.Int64
	blockedAPICnt  atomic.Int64
	blockedTenTx   atomic.Int64
	blockedDevTx   atomic.Int64
}

// ConfigKeys
const (
	ConfigKeyBackend       = "api-rate-limit.backend"
	ConfigKeyDefaultAPI    = "api-rate-limit.default-tenant-limits"
	ConfigKeyDefaultLegacy = "api-rate-limit.requests-per-minute"
	ConfigKeyDefaultTenTx  = "transport-rate-limit.default-tenant-limits"
	ConfigKeyDefaultDevTx  = "transport-rate-limit.default-device-limits"
)

// GetDefaultService 获取全局单例。
func GetDefaultService() *RateLimitService {
	initOnce.Do(func() {
		DefaultService = NewRateLimitService(nil, nil)
	})
	return DefaultService
}

// NewRateLimitService 创建限流服务。
func NewRateLimitService(evaluator Evaluator, logger *logrus.Logger) *RateLimitService {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	backend := strings.ToLower(strings.TrimSpace(viper.GetString(ConfigKeyBackend)))
	if evaluator == nil {
		if backend == "redis" && global.REDIS != nil {
			evaluator = NewRedisClusterEvaluator(global.REDIS, logger)
		} else {
			if backend == "redis" {
				logger.Warn("api-rate-limit.backend=redis but Redis not initialized, falling back to memory")
			}
			backend = "memory"
			evaluator = NewMemoryEvaluator()
		}
	} else if backend == "" {
		backend = "memory"
	}

	// 默认 API 限流：优先从 default-tenant-limits 读取，次之 requests-per-minute，最后保底 600:60
	apiExpr := viper.GetString(ConfigKeyDefaultAPI)
	if apiExpr == "" {
		rpm := viper.GetInt64(ConfigKeyDefaultLegacy)
		if rpm > 0 {
			apiExpr = fmt.Sprintf("%d:60", rpm)
		} else {
			apiExpr = "600:60"
		}
	}
	apiRules, err := ParseRateLimitRules(apiExpr)
	if err != nil {
		logger.WithError(err).Warnf("Invalid default API rate limits %q, fallback to 600:60", apiExpr)
		apiRules, _ = ParseRateLimitRules("600:60")
	}

	// 默认租户上行消息限流
	tenTxExpr := viper.GetString(ConfigKeyDefaultTenTx)
	if tenTxExpr == "" {
		tenTxExpr = "1000:1,30000:60"
	}
	tenTxRules, err := ParseRateLimitRules(tenTxExpr)
	if err != nil {
		tenTxRules, _ = ParseRateLimitRules("1000:1,30000:60")
	}

	// 默认设备上行防护限流
	devTxExpr := viper.GetString(ConfigKeyDefaultDevTx)
	if devTxExpr == "" {
		devTxExpr = "50:1,1000:60"
	}
	devTxRules, err := ParseRateLimitRules(devTxExpr)
	if err != nil {
		devTxRules, _ = ParseRateLimitRules("50:1,1000:60")
	}

	s := &RateLimitService{
		evaluator:             evaluator,
		backend:               backend,
		logger:                logger,
		defaultTenantAPIRules: apiRules,
		defaultTenantTxRules:  tenTxRules,
		defaultDeviceTxRules:  devTxRules,
		overrides:             make(map[string][]RateLimitRule),
	}

	// 尝试预加载数据库覆盖配置
	_ = s.ReloadOverrides(context.Background())

	return s
}

// ReloadOverrides 从数据库重新加载覆盖规则。
func (s *RateLimitService) ReloadOverrides(ctx context.Context) error {
	if global.DB == nil {
		return nil
	}
	records, err := dal.ListTenantRateLimits("")
	if err != nil {
		return err
	}

	s.overrideMu.Lock()
	defer s.overrideMu.Unlock()
	s.overrides = make(map[string][]RateLimitRule, len(records))

	for _, rec := range records {
		if !rec.Enabled {
			continue
		}
		rules, err := ParseRateLimitRules(rec.RateLimits)
		if err != nil {
			s.logger.WithError(err).Warnf("Failed to parse rate limit override %s: %s", rec.ID, rec.RateLimits)
			continue
		}
		key := s.makeOverrideKey(rec.TenantID, rec.TargetType, rec.TargetID, rec.LimitType)
		s.overrides[key] = rules
	}
	return nil
}

func (s *RateLimitService) makeOverrideKey(tenantID, targetType, targetID, limitType string) string {
	return fmt.Sprintf("%s:%s:%s:%s", tenantID, targetType, targetID, limitType)
}

// getRules 查找优先覆盖规则，无覆盖则回退到默认规则。
func (s *RateLimitService) getRules(tenantID, targetType, targetID, limitType string) []RateLimitRule {
	s.overrideMu.RLock()
	key := s.makeOverrideKey(tenantID, targetType, targetID, limitType)
	rules, exists := s.overrides[key]
	s.overrideMu.RUnlock()

	if exists && len(rules) > 0 {
		return rules
	}

	switch limitType {
	case model.LimitTypeAPI:
		// 动态感知测试注入或运行时配置变更（保持对 legacy requests-per-minute 的 100% 兼容）
		if rpm := viper.GetInt64(ConfigKeyDefaultLegacy); rpm > 0 && !viper.IsSet(ConfigKeyDefaultAPI) {
			return []RateLimitRule{{Limit: rpm, WindowSeconds: 60, Window: 60 * time.Second}}
		}
		return s.defaultTenantAPIRules
	case model.LimitTypeTenantTransport:
		return s.defaultTenantTxRules
	case model.LimitTypeDeviceTransport:
		return s.defaultDeviceTxRules
	default:
		return nil
	}
}

// CheckAPI 检验 REST API 请求限流。
func (s *RateLimitService) CheckAPI(ctx context.Context, tenantID, userID string) (bool, int64, string) {
	s.totalChecked.Add(1)
	subject := tenantID
	targetType := model.TargetTypeTenant
	if subject == "" {
		subject = "user:" + userID
		targetType = "user"
	}

	rules := s.getRules(tenantID, targetType, subject, model.LimitTypeAPI)
	if len(rules) == 0 {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	allowed, retryAfter, violatedRule, _ := s.evaluator.Allow(ctx, model.LimitTypeAPI, subject, rules)
	if allowed {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	s.totalBlocked.Add(1)
	s.blockedAPICnt.Add(1)
	return false, retryAfter, violatedRule
}

// CheckTenantTransport 检验租户上行消息聚合吞吐。
func (s *RateLimitService) CheckTenantTransport(ctx context.Context, tenantID string) (bool, int64, string) {
	if tenantID == "" {
		return true, 0, ""
	}
	s.totalChecked.Add(1)

	rules := s.getRules(tenantID, model.TargetTypeTenant, tenantID, model.LimitTypeTenantTransport)
	if len(rules) == 0 {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	allowed, retryAfter, violatedRule, _ := s.evaluator.Allow(ctx, model.LimitTypeTenantTransport, tenantID, rules)
	if allowed {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	s.totalBlocked.Add(1)
	s.blockedTenTx.Add(1)
	return false, retryAfter, violatedRule
}

// CheckDeviceTransport 检验单设备上行消息频率（防止坏设备轰炸）。
func (s *RateLimitService) CheckDeviceTransport(ctx context.Context, tenantID, deviceID string) (bool, int64, string) {
	if deviceID == "" {
		return true, 0, ""
	}
	s.totalChecked.Add(1)

	rules := s.getRules(tenantID, model.TargetTypeDevice, deviceID, model.LimitTypeDeviceTransport)
	if len(rules) == 0 {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	subject := fmt.Sprintf("%s:%s", tenantID, deviceID)
	allowed, retryAfter, violatedRule, _ := s.evaluator.Allow(ctx, model.LimitTypeDeviceTransport, subject, rules)
	if allowed {
		s.totalAllowed.Add(1)
		return true, 0, ""
	}

	s.totalBlocked.Add(1)
	s.blockedDevTx.Add(1)
	return false, retryAfter, violatedRule
}

// SetOverride 设置或更新特定租户/设备的自定义限流配额。
func (s *RateLimitService) SetOverride(ctx context.Context, req *model.SetRateLimitRequest) error {
	rules, err := ParseRateLimitRules(req.RateLimits)
	if err != nil {
		return err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	record := &model.TenantRateLimit{
		TenantID:    req.TenantID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		LimitType:   req.LimitType,
		RateLimits:  FormatRateLimitRules(rules),
		Enabled:     enabled,
		Description: req.Description,
	}

	if err := dal.SaveTenantRateLimit(record); err != nil {
		return err
	}

	// 立即刷新内存快照
	s.overrideMu.Lock()
	key := s.makeOverrideKey(req.TenantID, req.TargetType, req.TargetID, req.LimitType)
	if enabled {
		s.overrides[key] = rules
	} else {
		delete(s.overrides, key)
	}
	s.overrideMu.Unlock()

	return nil
}

// DeleteOverride 删除自定义限流配额。
func (s *RateLimitService) DeleteOverride(ctx context.Context, tenantID, targetType, targetID, limitType string) error {
	if err := dal.DeleteTenantRateLimit(tenantID, targetType, targetID, limitType); err != nil {
		return err
	}

	s.overrideMu.Lock()
	key := s.makeOverrideKey(tenantID, targetType, targetID, limitType)
	delete(s.overrides, key)
	s.overrideMu.Unlock()

	return nil
}

// ListOverrides 获取限流自定义列表。
func (s *RateLimitService) ListOverrides(ctx context.Context, tenantID string) ([]model.TenantRateLimit, error) {
	return dal.ListTenantRateLimits(tenantID)
}

// GetConfig 返回当前系统生效的限流配置。
func (s *RateLimitService) GetConfig() model.RateLimitConfigResponse {
	return model.RateLimitConfigResponse{
		Backend:               s.backend,
		DefaultTenantAPILimit: FormatRateLimitRules(s.defaultTenantAPIRules),
		DefaultTenantTxLimit:  FormatRateLimitRules(s.defaultTenantTxRules),
		DefaultDeviceTxLimit:  FormatRateLimitRules(s.defaultDeviceTxRules),
	}
}

// GetMetrics 返回限流度量指标。
func (s *RateLimitService) GetMetrics() model.RateLimitMetricsResponse {
	return model.RateLimitMetricsResponse{
		TotalChecked: s.totalChecked.Load(),
		TotalAllowed: s.totalAllowed.Load(),
		TotalBlocked: s.totalBlocked.Load(),
		BlockedByScope: map[string]int64{
			model.LimitTypeAPI:             s.blockedAPICnt.Load(),
			model.LimitTypeTenantTransport: s.blockedTenTx.Load(),
			model.LimitTypeDeviceTransport: s.blockedDevTx.Load(),
		},
	}
}
