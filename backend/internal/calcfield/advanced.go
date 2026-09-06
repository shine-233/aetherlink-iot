// 文件用途:计算字段高级类型(PHASE-D-D4)——时序聚合/关联实体聚合/地理围栏/传播。
// 核心逻辑:FieldRule 扩展 Type+Config;simple 走既有 govaluate 路径完全不变;
// 高级类型统一经 evaluateAdvanced 求值(输入扁平遥测+时间戳)→ 输出值(可带派生目标)。
// 关键注意事项:
// 1. 窗口状态(时序聚合)在引擎内按 field+device 维护,惰性清理防泄漏;
// 2. 类型常量/配置解析/几何与聚合纯函数收敛至 leaf 包 internal/calcfield/types
//    (service 保存校验共用,规避 service→calcfield→uplink→service 导入环);
// 3. related_agg/propagation 的目标解析走 RelatedTargetsSource seam(显式 device_ids 即时生效,
//    资产树自动发现留集成阶段接线),解析失败 fail-closed 跳过,不阻塞主链路。
package calcfield

import (
	"encoding/json"
	"fmt"
	"sync"

	types "aetherlink-iot/backend/internal/calcfield/types"
)

// 高级类型常量(70.sql type 列取值;实现收敛至 types 包)。
const (
	FieldTypeSimple      = types.TypeSimple
	FieldTypeTimeseries  = types.TypeTimeseries
	FieldTypeRelatedAgg  = types.TypeRelatedAgg
	FieldTypeGeofence    = types.TypeGeofence
	FieldTypePropagation = types.TypePropagation
)

// advancedConfig 别名(引擎内引用沿用短名)。
type advancedConfig = types.AdvancedConfig

func parseAdvancedConfig(fieldType string, raw json.RawMessage) (*advancedConfig, error) {
	return types.ParseAdvancedConfig(fieldType, raw)
}

func toFloat(raw interface{}) (float64, bool) {
	return types.ToFloat(raw)
}

func aggregateFloats(values []float64, fn string, count int) float64 {
	return types.AggregateFloats(values, fn, count)
}

// ---- 时序聚合:滚动窗口状态 ----

type windowSample struct {
	ts    int64
	value float64
}

// windowStore 按 field+device 维护滑动窗口样本;惰性清理,防长驻泄漏。
type windowStore struct {
	mu      sync.Mutex
	windows map[string][]windowSample
}

var fieldWindows = &windowStore{windows: map[string][]windowSample{}}

const windowMaxSamples = 4096

func (w *windowStore) recordAndAggregate(key string, ts int64, value float64, windowSeconds int, fn string) float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	samples := append(w.windows[key], windowSample{ts: ts, value: value})
	cutoff := ts - int64(windowSeconds)*1000
	kept := samples[:0]
	for _, sample := range samples {
		if sample.ts >= cutoff {
			kept = append(kept, sample)
		}
	}
	if len(kept) > windowMaxSamples {
		kept = kept[len(kept)-windowMaxSamples:]
	}
	w.windows[key] = kept
	return types.AggregateFloats(keptToValues(kept), fn, len(kept))
}

func keptToValues(samples []windowSample) []float64 {
	values := make([]float64, len(samples))
	for i, sample := range samples {
		values[i] = sample.value
	}
	return values
}

// ---- 引擎挂点 ----

// RelatedTargetsSource 关联/传播目标解析 seam(显式 device_ids 时无需调用;
// 资产树自动发现在集成阶段注入实现,解析失败 fail-closed 跳过)。
var RelatedTargetsSource = func(tenantID, deviceID, direction string) ([]string, error) {
	return nil, fmt.Errorf("asset-tree target resolver is not wired (use explicit device_ids)")
}

// resolveTargets 关联/传播目标:显式列表优先,否则走 seam。
func resolveTargets(cfg *advancedConfig, tenantID, deviceID string) []string {
	if len(cfg.DeviceIDs) > 0 {
		return cfg.DeviceIDs
	}
	targets, err := RelatedTargetsSource(tenantID, deviceID, cfg.Direction)
	if err != nil {
		return nil
	}
	return targets
}

// evaluateAdvanced 高级类型统一求值入口。
// 返回:(本机输出值, 派生目标(仅 propagation), 错误)。
func evaluateAdvanced(rule compiledRule, payload map[string]interface{}, ts int64, deviceID, tenantID string) (interface{}, []string, error) {
	cfg := rule.advanced
	switch rule.fieldType {
	case FieldTypeTimeseries:
		raw, exists := payload[cfg.SourceKey]
		if !exists {
			return nil, nil, nil
		}
		value, ok := toFloat(raw)
		if !ok {
			return nil, nil, nil
		}
		key := rule.id + "|" + deviceID
		return fieldWindows.recordAndAggregate(key, ts, value, cfg.WindowSeconds, cfg.Func), nil, nil
	case FieldTypeRelatedAgg:
		targets := resolveTargets(cfg, tenantID, deviceID)
		values := make([]float64, 0, len(targets))
		for _, target := range targets {
			// 关联设备的最新值从历史源读取(seam 注入;测试桩注入)。
			if v, ok := readRelatedLatest(rule, target, cfg.SourceKey); ok {
				values = append(values, v)
			}
		}
		return aggregateFloats(values, cfg.Func, len(values)), nil, nil
	case FieldTypeGeofence:
		inside, ok := types.EvaluateGeofence(cfg, payload)
		if !ok {
			return nil, nil, nil
		}
		return inside, nil, nil
	case FieldTypePropagation:
		raw, exists := payload[cfg.SourceKey]
		if !exists {
			return nil, nil, nil
		}
		value, ok := toFloat(raw)
		if !ok {
			return nil, nil, nil
		}
		targets := resolveTargets(cfg, tenantID, deviceID)
		if len(targets) == 0 {
			return nil, nil, nil
		}
		return value, targets, nil
	default:
		return nil, nil, fmt.Errorf("unknown advanced type %q", rule.fieldType)
	}
}

// readRelatedLatest 关联设备最新值读取缝(dal 实现查 telemetry_current_datas;测试桩)。
var readRelatedLatest = func(rule compiledRule, deviceID, sourceKey string) (float64, bool) {
	return 0, false
}

// ValidateFieldConfig 服务层保存校验入口(委托 types)。
func ValidateFieldConfig(fieldType string, config json.RawMessage) error {
	return types.ValidateFieldConfig(fieldType, config)
}
