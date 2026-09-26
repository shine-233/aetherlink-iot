// 文件用途：Integration 上行转换管线（TB-45）——采集回填路径上的绑定解析与转换执行。
// 核心逻辑：按 (tenant, connector_type) 查启用集成实例，config.device_ids 命中设备即取
//
//	其上行转换器执行转换；绑定与转换器带 TTL 缓存（与采集发现缓存同口径 60s），
//	转换失败丢弃本轮遥测并计数，绝不向上阻断采集循环。
//
// 关键注意事项：租户隔离 fail-closed——租户 ID 来自 DB 发现结果（devices.tenant_id），
// 查询一律带 tenant_id 过滤；tenantID 为空直接旁路（不转换）。
// 重构建议：SNMP/插件连接器同构接入时复用本 Resolver，只需换 connector_type 与挂载点。
package collector

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// UplinkTransformHook 采集回填后的上行转换钩子签名。
// 返回 (转换后遥测, true) 表示正常发布；(nil, false) 表示转换失败丢弃本轮遥测。
type UplinkTransformHook func(ctx context.Context, t deviceTarget, values map[string]interface{}) (map[string]interface{}, bool)

// IntegrationTransformStats 管线转换诊断计数。
type IntegrationTransformStats struct {
	Transformed uint64 // 成功转换并替换遥测的次数
	Dropped     uint64 // 转换失败丢弃的次数
}

// integrationCacheEntry 绑定缓存条目：converter=nil 表示"设备未绑定/绑定无效"的负缓存。
type integrationCacheEntry struct {
	converter *model.DataConverter
	expireAt  time.Time
}

// IntegrationResolver Integration 上行转换执行器（DB 驱动，按设备缓存绑定）。
type IntegrationResolver struct {
	db            *gorm.DB
	connectorType string
	log           *logrus.Logger
	ttl           time.Duration

	// loadEnabledUplinkConverter 查询设备绑定的启用集成上行转换器（默认 DB 实现，单测可注入）。
	loadEnabledUplinkConverter func(tenantID, deviceID string) (*model.DataConverter, error)
	// executeUplink 执行上行转换（默认 service.ExecuteUplinkDataConverter，单测可注入）。
	executeUplink func(conv *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error)

	mu          sync.Mutex
	cache       map[string]integrationCacheEntry
	transformed atomic.Uint64
	dropped     atomic.Uint64
}

// NewIntegrationResolver 构造指定连接器类型的上行转换执行器。
func NewIntegrationResolver(db *gorm.DB, connectorType string, log *logrus.Logger) *IntegrationResolver {
	if log == nil {
		log = logrus.New()
	}
	r := &IntegrationResolver{
		db:            db,
		connectorType: connectorType,
		log:           log,
		ttl:           targetsCacheTTL,
		cache:         map[string]integrationCacheEntry{},
	}
	r.loadEnabledUplinkConverter = r.loadEnabledUplinkConverterFromDB
	r.executeUplink = service.ExecuteUplinkDataConverter
	return r
}

// Stats 返回诊断计数快照。
func (r *IntegrationResolver) Stats() IntegrationTransformStats {
	return IntegrationTransformStats{
		Transformed: r.transformed.Load(),
		Dropped:     r.dropped.Load(),
	}
}

// Hook 供 Poller.Transform 挂载：未绑定/未启用直接旁路；转换失败丢弃并计数。
func (r *IntegrationResolver) Hook(ctx context.Context, t deviceTarget, values map[string]interface{}) (map[string]interface{}, bool) {
	if r == nil || t.TenantID == "" {
		return values, true
	}
	conv := r.cachedConverter(t.TenantID, t.DeviceID)
	if conv == nil {
		return values, true
	}
	out, err := r.executeUplink(conv, values, map[string]string{
		"device_id":     t.DeviceID,
		"device_number": t.DeviceNumber,
	})
	if err != nil {
		r.dropped.Add(1)
		r.log.WithFields(logrus.Fields{
			"device_id":      sanitizeLogValue(t.DeviceID),
			"connector_type": r.connectorType,
			"error":          sanitizeLogValue(err.Error()),
		}).Warn("collector: Integration 上行转换失败，丢弃本轮遥测")
		return nil, false
	}
	if out == nil {
		r.dropped.Add(1)
		return nil, false
	}
	r.transformed.Add(1)
	return out, true
}

// cachedConverter 取绑定转换器（TTL 缓存内直读，过期重查；负缓存同样生效）。
func (r *IntegrationResolver) cachedConverter(tenantID, deviceID string) *model.DataConverter {
	key := tenantID + "/" + deviceID
	r.mu.Lock()
	if e, ok := r.cache[key]; ok && time.Now().Before(e.expireAt) {
		r.mu.Unlock()
		return e.converter
	}
	r.mu.Unlock()

	conv, err := r.loadEnabledUplinkConverter(tenantID, deviceID)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"device_id":      sanitizeLogValue(deviceID),
			"connector_type": r.connectorType,
			"error":          sanitizeLogValue(err.Error()),
		}).Warn("collector: Integration 绑定解析失败，本轮不转换")
	}

	r.mu.Lock()
	r.cache[key] = integrationCacheEntry{converter: conv, expireAt: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	return conv
}

// loadEnabledUplinkConverterFromDB 默认绑定解析：启用集成 ∩ device_ids 命中 ∩ 挂上行转换器。
// 租户隔离：tenant_id 过滤 + 转换器同租户校验（fail-closed，跨租户绑定视同未绑定）。
func (r *IntegrationResolver) loadEnabledUplinkConverterFromDB(tenantID, deviceID string) (*model.DataConverter, error) {
	if r.db == nil {
		return nil, nil
	}
	var integrations []*model.Integration
	if err := r.db.Where("tenant_id = ? AND connector_type = ? AND enabled = ?",
		tenantID, r.connectorType, true).
		Find(&integrations).Error; err != nil {
		return nil, err
	}
	for _, integration := range integrations {
		if integration == nil || integration.ConverterUplinkID == nil || *integration.ConverterUplinkID == "" {
			continue
		}
		if !deviceBound(integration.Config, deviceID) {
			continue
		}
		// 转换器必须同租户：converter_uplink_id 外键不约束租户，读取时显式校验。
		var conv model.DataConverter
		if err := r.db.Where("id = ? AND tenant_id = ?", *integration.ConverterUplinkID, tenantID).
			First(&conv).Error; err != nil {
			return nil, err
		}
		return &conv, nil
	}
	return nil, nil
}

// deviceBound 判断 config JSONB 的 device_ids 数组是否包含设备 ID。
func deviceBound(configJSON, deviceID string) bool {
	for _, id := range model.ParseDeviceIDs(configJSON) {
		if id == deviceID {
			return true
		}
	}
	return false
}
