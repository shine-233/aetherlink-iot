// File purpose: P2.2 pure analytics core - aggregation, period comparison, export rows.
// Core logic: aggregate a numeric series, compare two periods, and render export rows.
// Key notes: this file is DB-free on purpose so the numeric semantics can be proven offline.
// The dangerous part of "period over period" is not the subtraction, it is silently turning
// *undefined* into a number:
//   - no baseline data is NOT a baseline of 0 (that would fabricate a +100% surge)
//   - a baseline of 0 makes percent change mathematically undefined (not +Inf, not 0)
// Both cases MUST come back as "undefined with a reason", never as a number.

package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
)

// TelemetryAggregateResult 一组样本的聚合结果。
// OK 为 false 表示"没有可计算的值"（空序列），与"值为 0"是两种事实。
type TelemetryAggregateResult struct {
	Value     float64
	OK        bool
	Aggregate string
}

// aggregateTelemetrySeries 对数值序列做聚合。
func aggregateTelemetrySeries(kind string, values []float64) TelemetryAggregateResult {
	if len(values) == 0 {
		return TelemetryAggregateResult{Aggregate: kind}
	}
	switch kind {
	case model.TelemetryAnalysisAggSum:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return TelemetryAggregateResult{Value: sum, OK: true, Aggregate: kind}
	case model.TelemetryAnalysisAggMin:
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return TelemetryAggregateResult{Value: min, OK: true, Aggregate: kind}
	case model.TelemetryAnalysisAggMax:
		maxValue := values[0]
		for _, v := range values[1:] {
			if v > maxValue {
				maxValue = v
			}
		}
		return TelemetryAggregateResult{Value: maxValue, OK: true, Aggregate: kind}
	case model.TelemetryAnalysisAggCount:
		return TelemetryAggregateResult{Value: float64(len(values)), OK: true, Aggregate: kind}
	case model.TelemetryAnalysisAggLast:
		return TelemetryAggregateResult{Value: values[len(values)-1], OK: true, Aggregate: kind}
	default:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return TelemetryAggregateResult{Value: sum / float64(len(values)), OK: true, Aggregate: model.TelemetryAnalysisAggAvg}
	}
}

// TelemetryPeriodComparison 两个周期的对比结果。
type TelemetryPeriodComparison struct {
	Current   TelemetryAggregateResult
	Baseline  TelemetryAggregateResult
	// Delta 绝对变化量。仅在两端都有值时有效。
	Delta     float64
	DeltaOK   bool
	// PercentChange 相对变化百分比。**nil 表示未定义**，绝不回填 0 或 Inf。
	PercentChange *float64
	PercentUndefinedReason string
}

// CompareTelemetryPeriods 计算同比/环比。
// 基线缺失或基线为 0 时，百分比一律为 nil 并给出原因。
func CompareTelemetryPeriods(kind string, current, baseline []float64) TelemetryPeriodComparison {
	result := TelemetryPeriodComparison{
		Current:  aggregateTelemetrySeries(kind, current),
		Baseline: aggregateTelemetrySeries(kind, baseline),
	}
	if !result.Current.OK || !result.Baseline.OK {
		result.PercentUndefinedReason = "insufficient data in one or both periods"
		return result
	}
	result.Delta = result.Current.Value - result.Baseline.Value
	result.DeltaOK = true
	if result.Baseline.Value == 0 {
		// 基数为 0 时相对变化在数学上无定义。给 0 会假装"没变化"，
		// 给 Inf 会把一个微小波动放大成灾难性数字。
		result.PercentUndefinedReason = "baseline is zero; percent change is undefined"
		return result
	}
	pct := result.Delta / result.Baseline.Value * 100
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		result.PercentUndefinedReason = "percent change is not finite"
		return result
	}
	result.PercentChange = &pct
	return result
}

// 同比/环比的基线窗口。
// 环比：紧邻当前窗口之前、等长的一段。
// 同比：按 offsets 个窗口长度回退（默认 1，即"上一个同长度周期"）。
func telemetryAnalysisBaselineWindow(startTime, endTime int64, mode string, offsets int) (int64, int64, error) {
	if endTime <= startTime {
		return 0, 0, fmt.Errorf("analysis window end time must be after start time")
	}
	if offsets <= 0 {
		offsets = 1
	}
	duration := endTime - startTime
	shift := duration * int64(offsets)
	if mode == model.TelemetryAnalysisCompareSameLast {
		// 同比额外按自然周期回退一层：这里以"周"为自然周期单位，
		// 与环比的区别在于整体平移一个自然周期而非仅仅取紧邻前一段。
		const naturalPeriod = int64(7 * 24 * time.Hour / time.Millisecond)
		shift += naturalPeriod * int64(offsets)
	}
	return startTime - shift, endTime - shift, nil
}

// FormatTelemetryComparisonPercent 把百分比渲染为文本；未定义时返回原因。
func FormatTelemetryComparisonPercent(c TelemetryPeriodComparison) string {
	if c.PercentChange == nil {
		if c.PercentUndefinedReason == "" {
			return "n/a"
		}
		return "n/a (" + c.PercentUndefinedReason + ")"
	}
	return strconv.FormatFloat(*c.PercentChange, 'f', 2, 64) + "%"
}

// telemetryAnalysisColumns 导出行列。CSV 与 Excel 共用同一套列序，
// 否则两种格式会给出互相矛盾的同一份数据。
func telemetryAnalysisColumns() []string {
	return []string{
		"device_id", "key", "aggregate", "period",
		"value", "baseline_value", "delta", "percent_change",
	}
}

// TelemetryAnalysisRowInput 一行导出的输入。
// 刻意按"设备 × 周期"逐条传入：若改成"周期 → 单一对比值"的映射再套设备列表，
// 会让所有设备共用第一台的数值——导出的表格看着齐全，数字全是假的。
type TelemetryAnalysisRowInput struct {
	DeviceID   string
	Period     string
	Comparison TelemetryPeriodComparison
}

// buildTelemetryAnalysisRows 构造导出行（不含表头）。
// 未定义的值渲染为原因文本，而不是空串或 0。
func buildTelemetryAnalysisRows(key string, kind string, inputs []TelemetryAnalysisRowInput) [][]string {
	rows := make([][]string, 0, len(inputs))
	for _, input := range inputs {
		comparison := input.Comparison
		rows = append(rows, []string{
			input.DeviceID,
			key,
			kind,
			input.Period,
			formatTelemetryAnalysisNumber(comparison.Current.Value, comparison.Current.OK),
			formatTelemetryAnalysisNumber(comparison.Baseline.Value, comparison.Baseline.OK),
			formatTelemetryAnalysisNumber(comparison.Delta, comparison.DeltaOK),
			FormatTelemetryComparisonPercent(comparison),
		})
	}
	return rows
}

// formatTelemetryAnalysisNumber 无值时输出 "n/a"，绝不输出 0——
// 把"没有数据"写成 0，会让下游把缺失当成真实的零点。
func formatTelemetryAnalysisNumber(value float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// validateTelemetryAnalysisAggregate 校验聚合方式，非法即拒绝，不静默回退。
func validateTelemetryAnalysisAggregate(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "":
		return model.TelemetryAnalysisAggAvg, nil
	case model.TelemetryAnalysisAggAvg:
		return model.TelemetryAnalysisAggAvg, nil
	case model.TelemetryAnalysisAggSum:
		return model.TelemetryAnalysisAggSum, nil
	case model.TelemetryAnalysisAggMin:
		return model.TelemetryAnalysisAggMin, nil
	case model.TelemetryAnalysisAggMax:
		return model.TelemetryAnalysisAggMax, nil
	case model.TelemetryAnalysisAggCount:
		return model.TelemetryAnalysisAggCount, nil
	case model.TelemetryAnalysisAggLast:
		return model.TelemetryAnalysisAggLast, nil
	default:
		return "", fmt.Errorf("unsupported aggregate function: %q", kind)
	}
}
