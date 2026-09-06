// 文件用途:计算字段历史重算(PHASE-D-D4)。
// 核心逻辑:对指定字段+设备的时间范围,从历史源(telemetry_datas)取源键样本,
// 逐条重放求值并经 StorageEnqueuer 写回派生遥测(时间戳保持原值)。
// 关键注意事项:
// 1. 重算为确定性函数(同输入同输出)→ 重复执行幂等;
// 2. 时序聚合重放使用【本地隔离窗口】,不污染实时路径的 fieldWindows;
// 3. supported: simple / timeseries_agg / geofence / propagation;related_agg 依赖关联设备
//    全量历史对齐,重算暂不支持(显式报错)。
package calcfield

import (
	"context"
	"encoding/json"
	"fmt"

	types "aetherlink-iot/backend/internal/calcfield/types"
	"aetherlink-iot/backend/internal/uplink"
)

// HistorySample 历史样本。
type HistorySample struct {
	TS    int64
	Value float64
}

// HistorySource 历史读取 seam(dal 实现查 telemetry_datas;测试用内存桩)。
type HistorySource interface {
	// ListRange 按设备+键+时间范围(毫秒,左闭右开)读取数值样本,按 ts 升序。
	ListRange(ctx context.Context, tenantID, deviceID, key string, fromTS, toTS int64) ([]HistorySample, error)
}

// ProgressReporter 进度回写 seam(processed=已处理样本数,emitted=已产出派生值数)。
type ProgressReporter func(processed, emitted int64)

// RecomputeRange 对单字段单设备回放重算。返回 (processed, emitted)。
func RecomputeRange(ctx context.Context, rule FieldRule, deviceID, tenantID string, fromTS, toTS int64, source HistorySource, sink StorageEnqueuer, report ProgressReporter) (int64, int64, error) {
	if rule.Type == "" {
		rule.Type = FieldTypeSimple
	}
	switch rule.Type {
	case FieldTypeSimple, FieldTypeTimeseries, FieldTypeGeofence, FieldTypePropagation:
	default:
		return 0, 0, fmt.Errorf("recompute unsupported for type %q", rule.Type)
	}
	rules := compileFieldRules([]FieldRule{rule}, nil)
	if len(rules) == 0 {
		return 0, 0, fmt.Errorf("field rule failed to compile")
	}
	compiled := rules[0]

	sourceKeys := recomputeSourceKeys(compiled)
	if len(sourceKeys) == 0 {
		return 0, 0, fmt.Errorf("no source keys resolved for recompute")
	}

	// 加载各源键历史样本。
	history := map[string][]HistorySample{}
	for _, key := range sourceKeys {
		samples, err := source.ListRange(ctx, tenantID, deviceID, key, fromTS, toTS)
		if err != nil {
			return 0, 0, fmt.Errorf("list history for key %q: %w", key, err)
		}
		history[key] = samples
	}
	timestamps := unionTimestamps(history)

	var processed, emitted int64
	localWindows := &windowStore{windows: map[string][]windowSample{}}
	windowKey := rule.ID + "|" + deviceID

	for _, ts := range timestamps {
		payload := map[string]interface{}{}
		for _, key := range sourceKeys {
			if v, ok := valueAt(history[key], ts); ok {
				payload[key] = v
			}
		}
		processed++

		switch compiled.fieldType {
		case "", FieldTypeSimple:
			if value, ok := evaluateRule(compiled, payload); ok {
				emitted++
				emitRecompute(ctx, sink, tenantID, deviceID, compiled.outputKey, value, ts)
			}
		case FieldTypeTimeseries:
			raw, exists := payload[compiled.advanced.SourceKey]
			if !exists {
				break
			}
			value, ok := toFloat(raw)
			if !ok {
				break
			}
			agg := localWindows.recordAndAggregate(windowKey, ts, value, compiled.advanced.WindowSeconds, compiled.advanced.Func)
			emitted++
			emitRecompute(ctx, sink, tenantID, deviceID, compiled.outputKey, agg, ts)
		case FieldTypeGeofence:
			inside, ok := types.EvaluateGeofence(compiled.advanced, payload)
			if !ok {
				break
			}
			emitted++
			emitRecompute(ctx, sink, tenantID, deviceID, compiled.outputKey, inside, ts)
		case FieldTypePropagation:
			raw, exists := payload[compiled.advanced.SourceKey]
			if !exists {
				break
			}
			value, ok := toFloat(raw)
			if !ok || len(compiled.advanced.DeviceIDs) == 0 {
				break
			}
			for _, target := range compiled.advanced.DeviceIDs {
				emitted++
				emitRecompute(ctx, sink, tenantID, target, compiled.outputKey, value, ts)
			}
		}
		if report != nil {
			report(processed, emitted)
		}
	}
	return processed, emitted, nil
}

// emitRecompute 构造重算派生消息并写回(独立于 Engine 实例,metadata 标记来源)。
func emitRecompute(ctx context.Context, sink StorageEnqueuer, tenantID, deviceID, outputKey string, value interface{}, ts int64) {
	if sink == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{outputKey: value})
	if err != nil {
		return
	}
	metadata := map[string]interface{}{
		MetadataGeneratedFlag: true,
		"calcfield_recompute": true,
	}
	if tenantID != "" {
		metadata["tenant_id"] = tenantID
	}
	_ = sink.EnqueueDerivedTelemetry(ctx, &uplink.DeviceMessage{
		Type:      uplink.MessageTypeTelemetry,
		DeviceID:  deviceID,
		TenantID:  tenantID,
		Timestamp: ts,
		Payload:   payload,
		Metadata:  metadata,
	})
}

// unionTimestamps 全键时间戳并集升序。
func unionTimestamps(history map[string][]HistorySample) []int64 {
	seen := map[int64]struct{}{}
	for _, samples := range history {
		for _, sample := range samples {
			seen[sample.TS] = struct{}{}
		}
	}
	out := make([]int64, 0, len(seen))
	for ts := range seen {
		out = append(out, ts)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// valueAt 键在指定时间戳的值。
func valueAt(samples []HistorySample, ts int64) (float64, bool) {
	for _, sample := range samples {
		if sample.TS == ts {
			return sample.Value, true
		}
	}
	return 0, false
}

// recomputeSourceKeys 解析重算所需源键。
func recomputeSourceKeys(rule compiledRule) []string {
	if rule.fieldType == "" || rule.fieldType == FieldTypeSimple {
		return rule.variables
	}
	switch rule.fieldType {
	case FieldTypeTimeseries, FieldTypeRelatedAgg, FieldTypePropagation:
		return []string{rule.advanced.SourceKey}
	case FieldTypeGeofence:
		return []string{rule.advanced.LatKey, rule.advanced.LngKey}
	default:
		return nil
	}
}
