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

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// TelemetryAnalysisDeviceResult 单设备的分析结果。
type TelemetryAnalysisDeviceResult struct {
	DeviceID string                     `json:"device_id"`
	Error    string                     `json:"error,omitempty"`
	Current  TelemetryAggregateResult   `json:"current"`
	Baseline TelemetryAggregateResult   `json:"baseline,omitempty"`
	Delta    *float64                   `json:"delta,omitempty"`
	Percent  *float64                   `json:"percent_change,omitempty"`
	// PercentReason 百分比未定义时说明原因，绝不把未定义渲染成数字。
	PercentReason string `json:"percent_change_reason,omitempty"`
}

type TelemetryAnalysisResult struct {
	Key        string                          `json:"key"`
	Aggregate  string                          `json:"aggregate"`
	Compare    string                          `json:"compare"`
	Devices    []TelemetryAnalysisDeviceResult `json:"devices"`
	ExportPath string                          `json:"export_path,omitempty"`
	Format     string                          `json:"format,omitempty"`
}

// telemetryAnalysisOperations 副作用集合，可注入以便无数据库验证编排。
type telemetryAnalysisOperations struct {
	fetch  func(deviceID, key string, start, end int64, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error)
	access func(deviceID string, claims *utils.UserClaims) (*model.Device, error)
	export func(q model.TelemetryAnalysisQuery, rows [][]string) (*TelemetryAnalysisExportResult, error)
}

var telemetryAnalysisOps = telemetryAnalysisOperations{
	fetch:  dal.GetTelemetrStatisticaAgregationData,
	access: ensureTelemetryDeviceReadAccess,
	export: ExportTelemetryAnalysis,
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
	result := &TelemetryAnalysisResult{Key: q.Key, Aggregate: aggregate, Compare: compare}

	for _, deviceID := range q.DeviceIDs {
		deviceResult := TelemetryAnalysisDeviceResult{DeviceID: deviceID}

		if _, accessErr := telemetryAnalysisOps.access(deviceID, claims); accessErr != nil {
			// 单设备无权限如实记录，不中断整次多设备对比。
			deviceResult.Error = "device not readable"
			result.Devices = append(result.Devices, deviceResult)
			continue
		}

		currentRows, err := telemetryAnalysisOps.fetch(deviceID, q.Key, q.StartTime, q.EndTime, windowMs, aggregate)
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
			baselineRows, err := telemetryAnalysisOps.fetch(deviceID, q.Key, baseStart, baseEnd, baseEnd-baseStart, aggregate)
			if err != nil {
				return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
			}
			baselineValues = telemetryAnalysisValues(baselineRows)
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
				Current:                 device.Current,
				Baseline:                device.Baseline,
				Delta:                   delta,
				DeltaOK:                 deltaOK,
				PercentChange:           device.Percent,
				PercentUndefinedReason:  device.PercentReason,
			},
		})
	}
	return buildTelemetryAnalysisRows(result.Key, result.Aggregate, inputs)
}
