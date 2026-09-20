// File purpose: P2.2 analytics orchestration - fetch both periods, compare, export.
// Core logic: for each device, read the current window and (optionally) the baseline window,
// hand both series to the pure comparison core, then render rows and optionally write a file.
// Key notes:
//   - Device access is re-checked per device with the existing helper. Batching by tenant is
//     not the same as being allowed to read each device.
//   - The baseline window is derived from the SAME duration as the current window. Comparing
//     a 7-day window against a 30-day baseline produces a number that means nothing.
//   - A device that cannot be read is reported per-device; one denial must not silently drop
//     the whole multi-device comparison.

package service

import (
	"context"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// TelemetryAnalysisDeviceResult 单设备的分析结果。
type TelemetryAnalysisDeviceResult struct {
	DeviceID string                   `json:"device_id"`
	Error    string                   `json:"error,omitempty"`
	Current  TelemetryAggregateResult `json:"current"`
	Baseline TelemetryAggregateResult `json:"baseline,omitempty"`
	Delta    *float64                 `json:"delta,omitempty"`
	Percent  *float64                 `json:"percent_change,omitempty"`
	// PercentReason 百分比未定义时说明原因，绝不把未定义渲染成数字。
	PercentReason string `json:"percent_change_reason,omitempty"`
	// UnitReason 本次**未做单位换算**时说明原因（TB-9）。
	// 有值即表示该设备的数值仍是源单位，调用方不得当成已换算结果展示。
	UnitReason string `json:"unit_reason,omitempty"`
}

type TelemetryAnalysisResult struct {
	Key        string                          `json:"key"`
	Aggregate  string                          `json:"aggregate"`
	Compare    string                          `json:"compare"`
	Devices    []TelemetryAnalysisDeviceResult `json:"devices"`
	ExportPath string                          `json:"export_path,omitempty"`
	Format     string                          `json:"format,omitempty"`
	// UnitSystem 请求的目标单位制式（未请求时为空）。
	UnitSystem string `json:"unit_system,omitempty"`
	// SourceUnit 换算前单位；仅在换算真的发生时给出。
	SourceUnit string `json:"source_unit,omitempty"`
	// TargetUnit 换算后单位；仅在换算真的发生时给出。
	// 刻意与 SourceUnit 一起只在成功时出现——只给 TargetUnit 会让"没换算"看起来像"换算过"。
	TargetUnit string `json:"target_unit,omitempty"`
}

// telemetryAnalysisOperations 副作用集合，可注入以便无数据库验证编排。
type telemetryAnalysisOperations struct {
	fetch       func(deviceID, key string, start, end int64, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error)
	access      func(deviceID string, claims *utils.UserClaims) (*model.Device, error)
	export      func(q model.TelemetryAnalysisQuery, rows [][]string) (*TelemetryAnalysisExportResult, error)
	resolveUnit func(ctx context.Context, deviceID, key string) (string, error)
}

var telemetryAnalysisOps = telemetryAnalysisOperations{
	fetch:       dal.GetTelemetrStatisticaAgregationData,
	access:      ensureTelemetryDeviceReadAccess,
	export:      ExportTelemetryAnalysis,
	resolveUnit: dal.ResolveDeviceTelemetryUnit,
}

// telemetryAnalysisValues 从聚合行里提取数值序列。
// 非数值行直接跳过而不是报错：聚合结果里偶发非数值不应让整次分析失败，
// 但也不能把它当 0 计入——那会把噪声算进均值。
func telemetryAnalysisValues(rows []map[string]interface{}) []float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if value, ok := row["y"].(float64); ok {
			values = append(values, value)
		}
	}
	return values
}

// RunTelemetryAnalysis 执行一次轻量分析（含可选同比/环比与导出）。
func RunTelemetryAnalysis(ctx context.Context, q model.TelemetryAnalysisQuery, claims *utils.UserClaims) (*TelemetryAnalysisResult, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if len(q.DeviceIDs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_ids is required")
	}
	if q.Key == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "key is required")
	}
	if q.EndTime <= q.StartTime {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "end_time must be after start_time")
	}
	aggregate, err := validateTelemetryAnalysisAggregate(q.Aggregate)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	compare := q.Compare
	if compare == "" {
		compare = model.TelemetryAnalysisCompareNone
	}

	windowMs := q.EndTime - q.StartTime

	// TB-9：单位换算方案。若未显式传 unit 但指定了 unit_system，尝试从首台设备的物模型自动两跳解析源单位。
	sourceUnit := strings.TrimSpace(q.Unit)
	if sourceUnit == "" && strings.TrimSpace(q.UnitSystem) != "" && len(q.DeviceIDs) > 0 && telemetryAnalysisOps.resolveUnit != nil {
		if resolvedUnit, err := telemetryAnalysisOps.resolveUnit(ctx, q.DeviceIDs[0], q.Key); err == nil && resolvedUnit != "" {
			sourceUnit = resolvedUnit
		}
	}

	// 未请求（UnitSystem 为空）时 TargetUnit 与 Reason 都为空，
	// 后续全部跳过，行为与本次改动前逐位一致。
	unitPlan := resolveTelemetryUnitPlan(sourceUnit, aggregate, q.UnitSystem)

	result := &TelemetryAnalysisResult{Key: q.Key, Aggregate: aggregate, Compare: compare}
	if requested := strings.TrimSpace(q.UnitSystem); requested != "" {
		result.UnitSystem = requested
	}
	if unitPlan.TargetUnit != "" {
		result.SourceUnit = unitPlan.SourceUnit
		result.TargetUnit = unitPlan.TargetUnit
	}

	// P2.3：取数路径解析——分析缓存（可选）包裹常规取数；
	// 整窗冷数据（早于降采样边界）回落 telemetry_rollups 冷层。
	fetch := telemetryAnalysisOps.fetch
	if cache := newTelemetryFetchCache(); cache != nil {
		fetch = cache.wrap(fetch)
	}
	coldCutoff := telemetryColdWindowCutoffMs()

	for _, deviceID := range q.DeviceIDs {
		deviceResult := TelemetryAnalysisDeviceResult{DeviceID: deviceID}

		if _, accessErr := telemetryAnalysisOps.access(deviceID, claims); accessErr != nil {
			// 单设备无权限如实记录，不中断整次多设备对比。
			deviceResult.Error = "device not readable"
			result.Devices = append(result.Devices, deviceResult)
			continue
		}

		currentRows, err := fetchTelemetryAnalysisSeries(fetch, deviceID, q.Key, q.StartTime, q.EndTime, windowMs, aggregate, coldCutoff)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		currentValues := telemetryAnalysisValues(currentRows)

		baselineValues := []float64(nil)
		if compare != model.TelemetryAnalysisCompareNone {
			baseStart, baseEnd, err := telemetryAnalysisBaselineWindow(q.StartTime, q.EndTime, compare, q.CompareOffsets)
			if err != nil {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
			}
			baselineRows, err := fetchTelemetryAnalysisSeries(fetch, deviceID, q.Key, baseStart, baseEnd, baseEnd-baseStart, aggregate, coldCutoff)
			if err != nil {
				return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
			}
			baselineValues = telemetryAnalysisValues(baselineRows)
		}

		// TB-9：单位换算必须在聚合与对比**之前**完成。
		// 若放在聚合之后，delta 与百分比会停留在源单位，与已换算的当前/基线值不是同一把尺子
		// ——数值换了单位、变化量没换，这种错误在界面上完全看不出来。
		if unitPlan.TargetUnit != "" {
			convertedCurrent, convertedBaseline, convErr := convertTelemetryAnalysisWindows(
				currentValues, baselineValues, unitPlan.SourceUnit, unitPlan.TargetUnit)
			if convErr != nil {
				// 换算失败即不换算：如实写明原因，绝不返回"看着像换算过"的原值。
				deviceResult.UnitReason = convErr.Error()
			} else {
				currentValues, baselineValues = convertedCurrent, convertedBaseline
			}
		} else if unitPlan.Reason != "" {
			deviceResult.UnitReason = unitPlan.Reason
		}

		comparison := CompareTelemetryPeriods(aggregate, currentValues, baselineValues)
		deviceResult.Current = comparison.Current
		deviceResult.Baseline = comparison.Baseline
		if comparison.DeltaOK {
			delta := comparison.Delta
			deviceResult.Delta = &delta
		}
		deviceResult.Percent = comparison.PercentChange
		if comparison.PercentChange == nil && compare != model.TelemetryAnalysisCompareNone {
			deviceResult.PercentReason = comparison.PercentUndefinedReason
		}
		result.Devices = append(result.Devices, deviceResult)
	}

	if q.Format != "" {
		exported, err := telemetryAnalysisOps.export(q, telemetryAnalysisRowsFromResult(result))
		if err != nil {
			return nil, err
		}
		result.ExportPath = exported.FilePath
		result.Format = exported.Format
	}
	return result, nil
}

// telemetryAnalysisRowsFromResult 把结果转成导出行。
// 每台设备使用**自己的**对比结果：共用一台的数值会让整张表看着齐全却全是假数。
func telemetryAnalysisRowsFromResult(result *TelemetryAnalysisResult) [][]string {
	period := "current"
	if result.Compare != model.TelemetryAnalysisCompareNone {
		period = result.Compare
	}
	inputs := make([]TelemetryAnalysisRowInput, 0, len(result.Devices))
	for _, device := range result.Devices {
		deltaOK := device.Delta != nil
		var delta float64
		if deltaOK {
			delta = *device.Delta
		}
		inputs = append(inputs, TelemetryAnalysisRowInput{
			DeviceID: device.DeviceID,
			Period:   period,
			Comparison: TelemetryPeriodComparison{
				Current:                device.Current,
				Baseline:               device.Baseline,
				Delta:                  delta,
				DeltaOK:                deltaOK,
				PercentChange:          device.Percent,
				PercentUndefinedReason: device.PercentReason,
			},
		})
	}
	return buildTelemetryAnalysisRows(result.Key, result.Aggregate, inputs)
}
