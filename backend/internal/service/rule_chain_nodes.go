// 文件用途：规则链 2.0 节点类型注册表（PHASE-D-D1）。
// 核心逻辑：在 B2 的 7 种节点之上扩到 28 种，按 kind 分类（trigger/filter/transform/
//
//	enrichment/flow/action/analytics/external）；每个类型提供配置校验器，
//	由 ParseRuleChainGraph → Validate 统一兜底，保证不合法配置进不了库。
//
// 关键注意事项：节点类型注册表与前端画布 palette 是 D1 独占面——
//
//	新增类型必须同时满足：注册 kind + 配置校验器 + 执行 handler（rule_chain_nodes_*.go）
//	+ 画布 palette（frontend editor.vue）+ i18n 键块（rulechainD1）。
package service

import (
	"fmt"
	"strings"
)

// 规则链 2.0 新增节点类型常量（PHASE-D-D1）。
const (
	// Enrichment 富化
	RuleChainEnrichmentOriginatorAttributes = "enrichment.originator_attributes"
	RuleChainEnrichmentLatestTelemetry      = "enrichment.latest_telemetry"
	RuleChainEnrichmentRelatedAttributes    = "enrichment.related_device_attributes"
	RuleChainEnrichmentTenantMetadata       = "enrichment.tenant_metadata"
	// Transformation 扩充
	RuleChainTransformScript           = "transform.script"
	RuleChainTransformRenameKeys       = "transform.rename_keys"
	RuleChainTransformSplitArray       = "transform.split_array"
	RuleChainTransformDedup            = "transform.dedup"
	RuleChainTransformChangeOriginator = "transform.change_originator"
	// Flow 控制
	RuleChainFlowSubchain   = "flow.subchain"
	RuleChainFlowDelay      = "flow.delay"
	RuleChainFlowCheckpoint = "flow.checkpoint"
	// Filter 扩充
	RuleChainFilterExists      = "filter.exists"
	RuleChainFilterStringMatch = "filter.string_match"
	RuleChainFilterInRange     = "filter.in_range"
	// Analytics
	RuleChainAnalyticsGenerator    = "analytics.generator"
	RuleChainAnalyticsLatest       = "analytics.latest"
	RuleChainAnalyticsMessageCount = "analytics.message_count"
	// External
	RuleChainExternalMQTTForward = "external.mqtt_forward"
	RuleChainExternalKafka       = "external.kafka"
)

// 规则链 kind 常量（前端 palette 分组依据）。
const (
	RuleChainKindTrigger    = "trigger"
	RuleChainKindFilter     = "filter"
	RuleChainKindTransform  = "transform"
	RuleChainKindEnrichment = "enrichment"
	RuleChainKindFlow       = "flow"
	RuleChainKindAction     = "action"
	RuleChainKindAnalytics  = "analytics"
	RuleChainKindExternal   = "external"
)

// RuleChainNodeSpec 单个节点类型的注册描述。
type RuleChainNodeSpec struct {
	Type     string                         // 节点类型（graph node type）
	Kind     string                         // 分类（trigger/filter/transform/enrichment/flow/action/analytics/external）
	Validate func(cfg map[string]any) error // 配置校验器（nil 表示无约束）
}

// ruleChainNodeSpecs 规则链 2.0 全量节点注册表（顺序即前端 palette 展示顺序）。
var ruleChainNodeSpecs = []RuleChainNodeSpec{
	// ---- 触发 ----
	{Type: RuleChainTriggerTelemetry, Kind: RuleChainKindTrigger},
	{Type: RuleChainTriggerOnline, Kind: RuleChainKindTrigger},
	{Type: RuleChainAnalyticsGenerator, Kind: RuleChainKindTrigger, Validate: validateGeneratorConfig},
	// ---- 过滤 ----
	{Type: RuleChainFilterThreshold, Kind: RuleChainKindFilter, Validate: func(cfg map[string]any) error {
		_, _, _, err := parseThresholdConfig(cfg)
		return err
	}},
	{Type: RuleChainFilterExists, Kind: RuleChainKindFilter, Validate: validateStringKeyConfig("exists filter config requires key")},
	{Type: RuleChainFilterStringMatch, Kind: RuleChainKindFilter, Validate: validateStringMatchConfig},
	{Type: RuleChainFilterInRange, Kind: RuleChainKindFilter, Validate: validateInRangeConfig},
	// ---- 转换 ----
	{Type: RuleChainTransformMapping, Kind: RuleChainKindTransform},
	{Type: RuleChainTransformScript, Kind: RuleChainKindTransform, Validate: validateScriptConfig},
	{Type: RuleChainTransformRenameKeys, Kind: RuleChainKindTransform, Validate: validateRenameKeysConfig},
	{Type: RuleChainTransformSplitArray, Kind: RuleChainKindTransform, Validate: validateStringKeyConfig("split_array config requires key")},
	{Type: RuleChainTransformDedup, Kind: RuleChainKindTransform, Validate: validateDedupConfig},
	{Type: RuleChainTransformChangeOriginator, Kind: RuleChainKindTransform, Validate: validateChangeOriginatorConfig},
	// ---- 富化 ----
	{Type: RuleChainEnrichmentOriginatorAttributes, Kind: RuleChainKindEnrichment, Validate: validateEnrichmentFetchConfig},
	{Type: RuleChainEnrichmentLatestTelemetry, Kind: RuleChainKindEnrichment, Validate: validateEnrichmentFetchConfig},
	{Type: RuleChainEnrichmentRelatedAttributes, Kind: RuleChainKindEnrichment, Validate: validateEnrichmentFetchConfig},
	{Type: RuleChainEnrichmentTenantMetadata, Kind: RuleChainKindEnrichment},
	// ---- 流控 ----
	{Type: RuleChainFlowSubchain, Kind: RuleChainKindFlow, Validate: validateSubchainConfig},
	{Type: RuleChainFlowDelay, Kind: RuleChainKindFlow, Validate: validateDelayConfig},
	{Type: RuleChainFlowCheckpoint, Kind: RuleChainKindFlow},
	// ---- 动作 ----
	{Type: RuleChainActionWebhook, Kind: RuleChainKindAction},
	{Type: RuleChainActionCommand, Kind: RuleChainKindAction},
	{Type: RuleChainActionAlarm, Kind: RuleChainKindAction, Validate: validateAlarmConfig},
	// ---- 分析 ----
	{Type: RuleChainAnalyticsLatest, Kind: RuleChainKindAnalytics, Validate: validateEnrichmentFetchConfig},
	{Type: RuleChainAnalyticsMessageCount, Kind: RuleChainKindAnalytics},
	// ---- 外部 ----
	{Type: RuleChainExternalMQTTForward, Kind: RuleChainKindExternal, Validate: validateMQTTForwardConfig},
	{Type: RuleChainExternalKafka, Kind: RuleChainKindExternal, Validate: validateKafkaForwardConfig},
	// ---- AI（PHASE-D-D7）----
	{Type: RuleChainAiInference, Kind: RuleChainKindExternal, Validate: validateAiInferenceConfig},
}

// ruleChainKindByType 按 type 索引的 kind 表（校验热路径用）。
var ruleChainKindByType = func() map[string]string {
	m := make(map[string]string, len(ruleChainNodeSpecs))
	for _, spec := range ruleChainNodeSpecs {
		m[spec.Type] = spec.Kind
	}
	return m
}()

// ruleChainSpecByType 按 type 索引的完整 spec 表。
var ruleChainSpecByType = func() map[string]RuleChainNodeSpec {
	m := make(map[string]RuleChainNodeSpec, len(ruleChainNodeSpecs))
	for _, spec := range ruleChainNodeSpecs {
		m[spec.Type] = spec
	}
	return m
}()

// RuleChainNodeSpecList 返回全量节点 spec（顺序稳定，供前端 palette/API 拉取）。
func RuleChainNodeSpecList() []RuleChainNodeSpec {
	out := make([]RuleChainNodeSpec, len(ruleChainNodeSpecs))
	copy(out, ruleChainNodeSpecs)
	return out
}

// RuleChainNodeKind 返回节点类型分类；未知类型返回空串。
func RuleChainNodeKind(nodeType string) string {
	return ruleChainKindByType[nodeType]
}

// validateRuleChainNodeConfig 按注册表校验单个节点配置。
func validateRuleChainNodeConfig(nodeType string, cfg map[string]any) error {
	spec, ok := ruleChainSpecByType[nodeType]
	if !ok {
		return fmt.Errorf("node type %q is not registered", nodeType)
	}
	if spec.Validate != nil {
		return spec.Validate(cfg)
	}
	return nil
}

// ---- 配置校验器（表驱动语义：纯函数、只读 cfg）----

// validateStringKeyConfig 生成「要求 cfg.key 为非空字符串」的校验器。
func validateStringKeyConfig(message string) func(map[string]any) error {
	return func(cfg map[string]any) error {
		key, _ := cfg["key"].(string)
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s", message)
		}
		return nil
	}
}

// validateEnrichmentFetchConfig 富化/聚合节点通用校验：keys 必须是非空字符串数组。
func validateEnrichmentFetchConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("enrichment config is required")
	}
	if _, err := configStringList(cfg, "keys"); err != nil {
		return err
	}
	return nil
}

// validateStringMatchConfig filter.string_match：{key, op(contains|equals|prefix|suffix|regex 简化为前四类), value}
func validateStringMatchConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("string_match config is required")
	}
	key, _ := cfg["key"].(string)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("string_match config requires key")
	}
	op, _ := cfg["op"].(string)
	switch op {
	case "contains", "equals", "prefix", "suffix":
	default:
		return fmt.Errorf("string_match op must be one of contains/equals/prefix/suffix")
	}
	value, _ := cfg["value"].(string)
	if value == "" {
		return fmt.Errorf("string_match config requires value")
	}
	return nil
}

// validateInRangeConfig filter.in_range：{key, min, max}（min<=max）。
func validateInRangeConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("in_range config is required")
	}
	key, _ := cfg["key"].(string)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("in_range config requires key")
	}
	minV, minOK := toFloat(cfg["min"])
	maxV, maxOK := toFloat(cfg["max"])
	if !minOK || !maxOK {
		return fmt.Errorf("in_range config requires numeric min and max")
	}
	if minV > maxV {
		return fmt.Errorf("in_range min must not exceed max")
	}
	return nil
}

// validateRenameKeysConfig transform.rename_keys：{mappings:{old:new,...}} 非空。
func validateRenameKeysConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("rename_keys config is required")
	}
	mappings, ok := cfg["mappings"].(map[string]any)
	if !ok || len(mappings) == 0 {
		return fmt.Errorf("rename_keys config requires non-empty mappings")
	}
	for from, toAny := range mappings {
		if strings.TrimSpace(from) == "" {
			return fmt.Errorf("rename_keys mappings key must not be empty")
		}
		to, _ := toAny.(string)
		if strings.TrimSpace(to) == "" {
			return fmt.Errorf("rename_keys target for %q must be a non-empty string", from)
		}
	}
	return nil
}

// validateDedupConfig transform.dedup：{window_ms>0, keys?(可选签名键)}。
func validateDedupConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("dedup config is required")
	}
	windowMs, ok := toFloat(cfg["window_ms"])
	if !ok || windowMs <= 0 {
		return fmt.Errorf("dedup config requires positive window_ms")
	}
	if windowMs > float64(ruleChainMaxDedupWindow.Milliseconds()) {
		return fmt.Errorf("dedup window_ms exceeds limit %d", ruleChainMaxDedupWindow.Milliseconds())
	}
	if _, err := configStringList(cfg, "keys"); err != nil {
		return err
	}
	return nil
}

// validateChangeOriginatorConfig transform.change_originator：mode=device_id 时要求 device_id；
// mode=from_key 时要求 key。
func validateChangeOriginatorConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("change_originator config is required")
	}
	mode, _ := cfg["mode"].(string)
	switch mode {
	case "device_id":
		deviceID, _ := cfg["device_id"].(string)
		if strings.TrimSpace(deviceID) == "" {
			return fmt.Errorf("change_originator device_id mode requires device_id")
		}
	case "from_key":
		key, _ := cfg["key"].(string)
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("change_originator from_key mode requires key")
		}
	default:
		return fmt.Errorf("change_originator mode must be device_id or from_key")
	}
	return nil
}

// validateScriptConfig transform.script：script_id 与 code 二选一，均有长度约束。
func validateScriptConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("script config is required")
	}
	scriptID, _ := cfg["script_id"].(string)
	code, _ := cfg["code"].(string)
	if strings.TrimSpace(scriptID) != "" && strings.TrimSpace(code) != "" {
		return fmt.Errorf("script config accepts either script_id or code, not both")
	}
	if strings.TrimSpace(scriptID) == "" && strings.TrimSpace(code) == "" {
		return fmt.Errorf("script config requires script_id or code")
	}
	if len(code) > ruleChainMaxScriptBytes {
		return fmt.Errorf("script code exceeds limit %d bytes", ruleChainMaxScriptBytes)
	}
	return nil
}

// validateSubchainConfig flow.subchain：{chain_id 非空}。
func validateSubchainConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("subchain config is required")
	}
	chainID, _ := cfg["chain_id"].(string)
	if strings.TrimSpace(chainID) == "" {
		return fmt.Errorf("subchain config requires chain_id")
	}
	return nil
}

// validateDelayConfig flow.delay：{duration_ms ∈ (0, ruleChainMaxDelayMs]}。
func validateDelayConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("delay config is required")
	}
	durationMs, ok := toFloat(cfg["duration_ms"])
	if !ok || durationMs <= 0 {
		return fmt.Errorf("delay config requires positive duration_ms")
	}
	if durationMs > float64(ruleChainMaxDelayMs) {
		return fmt.Errorf("delay duration_ms exceeds limit %d", ruleChainMaxDelayMs)
	}
	return nil
}

// validateGeneratorConfig analytics.generator：{values:{key: value|{min,max}}} 非空。
func validateGeneratorConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("generator config is required")
	}
	values, ok := cfg["values"].(map[string]any)
	if !ok || len(values) == 0 {
		return fmt.Errorf("generator config requires non-empty values")
	}
	for key, raw := range values {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("generator values key must not be empty")
		}
		switch v := raw.(type) {
		case map[string]any:
			if _, okMin := toFloat(v["min"]); !okMin {
				return fmt.Errorf("generator range for %q requires numeric min", key)
			}
			if _, okMax := toFloat(v["max"]); !okMax {
				return fmt.Errorf("generator range for %q requires numeric max", key)
			}
		case string, float64, bool, nil:
		default:
			return fmt.Errorf("generator value for %q must be scalar or {min,max}", key)
		}
	}
	return nil
}

// validateAlarmConfig action.alarm：保留 L/M/H 语义并叠加 re-trigger 去重窗口校验。
func validateAlarmConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("alarm config is required")
	}
	name, _ := cfg["name"].(string)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("alarm action requires name")
	}
	severity, _ := cfg["severity"].(string)
	switch severity {
	case "", "L", "M", "H":
	default:
		return fmt.Errorf("alarm action severity must be one of L/M/H")
	}
	if windowMs, ok := toFloat(cfg["retrigger_dedup_ms"]); ok && windowMs < 0 {
		return fmt.Errorf("alarm retrigger_dedup_ms must not be negative")
	}
	return nil
}

// validateMQTTForwardConfig external.mqtt_forward：{topic 非空, qos ∈ {0,1}}。
func validateMQTTForwardConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("mqtt_forward config is required")
	}
	topic, _ := cfg["topic"].(string)
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("mqtt_forward config requires topic")
	}
	if strings.ContainsAny(topic, "#+") {
		return fmt.Errorf("mqtt_forward topic must not contain wildcards")
	}
	qos, ok := toFloat(cfg["qos"])
	if ok && qos != 0 && qos != 1 {
		return fmt.Errorf("mqtt_forward qos must be 0 or 1")
	}
	return nil
}

// validateKafkaForwardConfig external.kafka：{topic 非空}；实际生产由配置门控。
func validateKafkaForwardConfig(cfg map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("kafka config is required")
	}
	topic, _ := cfg["topic"].(string)
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("kafka config requires topic")
	}
	return nil
}

// configStringList 从 cfg 提取字符串数组（cfg[name] 为 []any 或 []string）。
func configStringList(cfg map[string]any, name string) ([]string, error) {
	if cfg == nil {
		return nil, nil
	}
	raw, ok := cfg[name]
	if !ok || raw == nil {
		return nil, nil
	}
	switch list := raw.(type) {
	case []string:
		return list, nil
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("%s must be an array of non-empty strings", name)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", name)
	}
}
