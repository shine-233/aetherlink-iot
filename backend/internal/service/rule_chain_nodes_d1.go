// 文件用途：规则链 2.0 新节点执行分发器与共享注入点（PHASE-D-D1）。
// 核心逻辑：executeRuleChainNodeD1 是 2.0 新类型的统一入口，按类型分派到
//
//	enrichment/transform/flow/analytics/external 各 handler 文件；
//	外部依赖（属性/遥测/租户元数据读取、脚本加载、设备租户校验）全部走
//	包级注入点，默认实现直查 global.DB（租户维度硬过滤），测试替换为内存桩。
//
// 关键注意事项：所有默认实现必须 fail-closed——查不到、租户不符一律返回错误，
//	绝不跨租户读取；热路径注入点为 nil 时保持零开销旁路。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// D1 节点配置/运行时限额。
const (
	// ruleChainMaxDelayMs flow.delay 单节点最大延时（须小于执行总超时 10s）。
	ruleChainMaxDelayMs = 5000
	// ruleChainMaxDedupWindowMs transform.dedup 时间窗上限。
	ruleChainMaxDedupWindow = time.Hour
	// ruleChainMaxScriptBytes transform.script 内联脚本字节数上限。
	ruleChainMaxScriptBytes = 8 * 1024
)

// ---- 数据读取注入点（默认实现在本文件底部；测试可整体替换）----

// ruleChainAttributeFetcher 按租户+设备读取属性键值（enrichment.originator/related_attributes）。
var ruleChainAttributeFetcher = func(ctx context.Context, tenantID, deviceID string, keys []string) (map[string]any, error) {
	return fetchColumnValues(ctx, model.TableNameAttributeData, tenantID, deviceID, keys)
}

// ruleChainLatestTelemetryFetcher 按租户+设备读取最新遥测键值（enrichment.latest_telemetry/analytics.latest）。
var ruleChainLatestTelemetryFetcher = func(ctx context.Context, tenantID, deviceID string, keys []string) (map[string]any, error) {
	return fetchLatestColumnValues(ctx, tenantID, deviceID, keys)
}

// ruleChainTenantMetadataFetcher 读取租户元数据（管理员账号的组织机构/时区等）。
var ruleChainTenantMetadataFetcher = fetchTenantMetadata

// ruleChainDeviceLookup 租户守卫的设备查询（related_device/change_originator）；
// 设备不存在或不属于该租户时返回 (nil, nil)，由调用方 fail-closed。
var ruleChainDeviceLookup = func(ctx context.Context, tenantID, deviceID string) (*model.Device, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	var device model.Device
	err := global.DB.WithContext(ctx).
		Table(model.TableNameDevice).
		Where("id = ? AND tenant_id = ?", deviceID, tenantID).
		Take(&device).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

// ruleChainScriptLoader 加载租户内数据脚本内容（transform.script）；找不到返回空串。
var ruleChainScriptLoader = func(ctx context.Context, tenantID, scriptID string) (string, error) {
	if global.DB == nil {
		return "", fmt.Errorf("db is not initialized")
	}
	var content string
	err := global.DB.WithContext(ctx).
		Table(model.TableNameDataScript+" AS ds").
		Joins("JOIN "+model.TableNameDeviceConfig+" AS dc ON dc.id = ds.device_config_id").
		Where("ds.id = ? AND dc.tenant_id = ?", scriptID, tenantID).
		Limit(1).
		Pluck("ds.content", &content).Error
	if err != nil {
		if isRecordNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return content, nil
}

// ruleChainCheckpointWriter checkpoint 落库注入点（flow.checkpoint）。
var ruleChainCheckpointWriter = func(ctx context.Context, cp *model.RuleChainCheckpoint) error {
	return createRuleChainCheckpointRow(ctx, cp)
}

// ruleChainMQTTPublisher MQTT 外发注入点（external.mqtt_forward）；
// 由 app 装配层（internal/app）以 PHASE-D-D1 标记块注入真实 adapter 客户端，未注入时 fail-fast。
var ruleChainMQTTPublisher = func(_ context.Context, _ string, _ byte, _ []byte) error {
	return fmt.Errorf("mqtt forward publisher is not wired (requires app assembly injection)")
}

// ruleChainKafkaProducer kafka 外发注入点（external.kafka 骨架）；未注入时 noop。
var ruleChainKafkaProducer func(ctx context.Context, topic, key string, payload []byte) error

// ruleChainDedupChecker 时间窗去重检查（transform.dedup）；scope 内 signature 窗口已见过返回 true。
var ruleChainDedupChecker = ruleChainDedupSeenOrMark

// ---- 分发器 ----

// executeRuleChainNodeD1 分派 PHASE-D-D1 新增节点类型。
func executeRuleChainNodeD1(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, error) {
	rcc := msg.Rcc
	if rcc == nil {
		rcc = &RuleChainContext{}
	}
	payload := msg.Payload
	metadata := msg.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	switch node.Type {
	// ---- Enrichment：结果合入 msg metadata ----
	case RuleChainEnrichmentOriginatorAttributes:
		return ruleChainEnrichmentAttributes(e.ctx, node, rcc, payload, metadata, rcc.DeviceID, ruleChainAttributeFetcher)
	case RuleChainEnrichmentLatestTelemetry:
		return ruleChainEnrichmentAttributes(e.ctx, node, rcc, payload, metadata, rcc.DeviceID, ruleChainLatestTelemetryFetcher)
	case RuleChainEnrichmentRelatedAttributes:
		return ruleChainEnrichmentRelatedAttributes(e.ctx, node, rcc, payload, metadata)
	case RuleChainEnrichmentTenantMetadata:
		return ruleChainEnrichmentTenantMetadata(e.ctx, node, rcc, payload, metadata)
	// ---- Transformation 扩充 ----
	case RuleChainTransformScript:
		return ruleChainTransformScript(e.ctx, node, msg, rcc)
	case RuleChainTransformRenameKeys:
		return ruleChainTransformRenameKeys(node, payload, metadata, rcc)
	case RuleChainTransformSplitArray:
		return ruleChainTransformSplitArray(node, payload, metadata, rcc)
	case RuleChainTransformDedup:
		return ruleChainTransformDedup(e, node, rcc, payload, metadata)
	case RuleChainTransformChangeOriginator:
		return ruleChainTransformChangeOriginator(e.ctx, node, rcc, payload, metadata)
	// ---- Filter 扩充 ----
	case RuleChainFilterExists:
		return ruleChainFilterExists(node, payload, metadata, rcc)
	case RuleChainFilterStringMatch:
		return ruleChainFilterStringMatch(node, payload, metadata, rcc)
	case RuleChainFilterInRange:
		return ruleChainFilterInRange(node, payload, metadata, rcc)
	// ---- Flow ----
	case RuleChainFlowSubchain:
		return ruleChainFlowSubchain(e, node, msg)
	case RuleChainFlowDelay:
		return ruleChainFlowDelay(e.ctx, node, payload, metadata, rcc)
	case RuleChainFlowCheckpoint:
		return ruleChainFlowCheckpoint(e, node, msg)
	// ---- Analytics ----
	case RuleChainAnalyticsGenerator:
		return ruleChainAnalyticsGenerator(node, payload, metadata, rcc)
	case RuleChainAnalyticsLatest:
		return ruleChainAnalyticsLatest(e.ctx, node, rcc, payload, metadata)
	case RuleChainAnalyticsMessageCount:
		return ruleChainAnalyticsMessageCount(e, node, rcc, payload, metadata)
	// ---- External ----
	case RuleChainExternalMQTTForward:
		return ruleChainExternalMQTTForward(e, node, rcc, payload, metadata)
	case RuleChainExternalKafka:
		return ruleChainExternalKafka(e.ctx, node, rcc, payload, metadata)
	default:
		return ruleChainNodeResult{}, fmt.Errorf("unknown node type %q", node.Type)
	}
}

// ---- 共享助手 ----

// mergePrefixValues 把 values 以 prefix 前缀合入 target（metadata 合入统一入口）。
func mergePrefixValues(target map[string]any, prefix string, values map[string]any) {
	if target == nil {
		return
	}
	for key, value := range values {
		target[prefix+key] = value
	}
}

// enrichmentPrefix 读取富化节点的 metadata 前缀配置。
func enrichmentPrefix(cfg map[string]any) string {
	prefix, _ := cfg["prefix"].(string)
	return prefix
}

// isRecordNotFound 判断 gorm 未命中错误。
func isRecordNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "record not found")
}

// ---- 默认数据读取实现（直查 global.DB，租户维度硬过滤）----

// fetchColumnValues attribute_datas 键值读取：按租户+设备+键集合，取每键最新一条。
func fetchColumnValues(ctx context.Context, table string, tenantID, deviceID string, keys []string) (map[string]any, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("tenant id and device id are required")
	}
	query := global.DB.WithContext(ctx).
		Table(table).
		Where("device_id = ?", deviceID).
		Where("(tenant_id = ? OR tenant_id IS NULL)", tenantID)
	if len(keys) > 0 {
		query = query.Where("`key` IN ?", keys)
	}
	var rows []model.AttributeData
	if err := query.Order("ts DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make(map[string]any, len(rows))
	for i := range rows {
		row := &rows[i]
		if _, exists := values[row.Key]; exists {
			continue // 已按 ts 倒序，首个即最新
		}
		if v := attributeRowValue(row); v != nil {
			values[row.Key] = v
		}
	}
	return values, nil
}

// fetchLatestColumnValues telemetry_current_datas 键值读取（当前值表本身即最新）。
func fetchLatestColumnValues(ctx context.Context, tenantID, deviceID string, keys []string) (map[string]any, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("tenant id and device id are required")
	}
	query := global.DB.WithContext(ctx).
		Table(model.TableNameTelemetryCurrentData).
		Where("device_id = ?", deviceID).
		Where("(tenant_id = ? OR tenant_id IS NULL)", tenantID)
	if len(keys) > 0 {
		query = query.Where("`key` IN ?", keys)
	}
	var rows []model.TelemetryCurrentData
	if err := query.Order("ts DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make(map[string]any, len(rows))
	for i := range rows {
		row := &rows[i]
		if _, exists := values[row.Key]; exists {
			continue
		}
		values[row.Key] = columnValue(row.BoolV, row.NumberV, row.StringV)
	}
	return values, nil
}

// attributeRowValue AttributeData 三值列取非空者。
func attributeRowValue(row *model.AttributeData) any {
	return columnValue(row.BoolV, row.NumberV, row.StringV)
}

// columnValue 泛化三值列转换。
func columnValue(boolV *bool, numberV *float64, stringV *string) any {
	switch {
	case boolV != nil:
		return *boolV
	case numberV != nil:
		return *numberV
	case stringV != nil:
		return *stringV
	default:
		return nil
	}
}

// fetchTenantMetadata 租户元数据：取租户管理员账号的组织机构/时区/创建时间。
func fetchTenantMetadata(ctx context.Context, tenantID string) (map[string]any, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	var rows []model.User
	if err := global.DB.WithContext(ctx).
		Table(model.TableNameUser).
		Where("tenant_id = ? AND authority = ?", tenantID, "TENANT_ADMIN").
		Order("created_at ASC").
		Limit(1).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	meta := map[string]any{"tenant_id": tenantID}
	if len(rows) == 0 {
		return meta, nil
	}
	admin := rows[0]
	if admin.Organization != nil && *admin.Organization != "" {
		meta["organization"] = *admin.Organization
	}
	if admin.Timezone != nil && *admin.Timezone != "" {
		meta["timezone"] = *admin.Timezone
	}
	return meta, nil
}

// ---- 去重注册表（transform.dedup / action.alarm re-trigger 共用语义）----

type ruleChainDedupEntry struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

var (
	ruleChainDedupMu       sync.Mutex
	ruleChainDedupRegistry = map[string]*ruleChainDedupEntry{}
)

const ruleChainDedupSweepLimit = 512

// dedupEntry 取（或创建）scope 对应的去重状态。
func dedupEntry(scopeKey string) *ruleChainDedupEntry {
	ruleChainDedupMu.Lock()
	defer ruleChainDedupMu.Unlock()
	entry, ok := ruleChainDedupRegistry[scopeKey]
	if !ok {
		entry = &ruleChainDedupEntry{seen: map[string]time.Time{}}
		ruleChainDedupRegistry[scopeKey] = entry
	}
	return entry
}

// ruleChainDedupSeenOrMark 时间窗去重：窗口内 signature 已见过返回 true，否则登记并返回 false。
// 惰性清理过期项，防长驻进程泄漏。
func ruleChainDedupSeenOrMark(scopeKey, signature string, window time.Duration) bool {
	entry := dedupEntry(scopeKey)
	now := time.Now()
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if len(entry.seen) > ruleChainDedupSweepLimit {
		for sig, ts := range entry.seen {
			if now.Sub(ts) > window {
				delete(entry.seen, sig)
			}
		}
	}
	if ts, ok := entry.seen[signature]; ok && now.Sub(ts) <= window {
		return true
	}
	entry.seen[signature] = now
	return false
}

// ruleChainInMemoryAlarmDedup action.alarm re-trigger 去重的进程内实现。
type ruleChainInMemoryAlarmDedup struct{}

// SeenWithin 判断 key 在窗口内是否已出现；是则直接返回 true（不登记）。
func (ruleChainInMemoryAlarmDedup) SeenWithin(key string, window time.Duration) bool {
	alarmDedupMu.Lock()
	defer alarmDedupMu.Unlock()
	ts, ok := alarmDedupRegistry[key]
	return ok && time.Since(ts) <= window
}

// MarkSeen 登记 key 的本次出现时间。
func (ruleChainInMemoryAlarmDedup) MarkSeen(key string) {
	alarmDedupMu.Lock()
	defer alarmDedupMu.Unlock()
	alarmDedupRegistry[key] = time.Now()
}

var (
	alarmDedupMu       sync.Mutex
	alarmDedupRegistry = map[string]time.Time{}
)

// dedupSignature 计算去重签名：keys 为空时对整个 payload 规范化 JSON 签名。
func dedupSignature(payload map[string]any, keys []string) (string, error) {
	if len(keys) == 0 {
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	sub := make(map[string]any, len(keys))
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			sub[key] = v
		}
	}
	raw, err := json.Marshal(sub)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// logNodeDebug 节点级调试日志统一出口（避免各 handler 重复初始化 logrus 字段）。
func logNodeDebug(chainID, nodeID, format string, args ...any) {
	logrus.WithFields(logrus.Fields{
		"chain_id": chainID,
		"node_id":  nodeID,
	}).Debugf(format, args...)
}
