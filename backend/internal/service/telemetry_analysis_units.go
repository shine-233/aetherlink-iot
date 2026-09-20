// 文件用途：遥测分析的工程单位换算（ROADMAP TB-9 接线层）。
//
// 核心逻辑：调用方在分析请求里给出**源单位**与**目标制式**，本层把取到的原始序列
// 换算到该制式下的代表单位，**在聚合与对比之前**完成——这样 avg/min/max/last、
// delta 与百分比变化全部落在同一单位上，不会出现"数值换了单位、变化量还是旧单位"。
//
// 关键注意事项（全部 fail closed，宁可如实说明"没换算"，也不给一个看着对的错数）：
//   - 未指定 unit_system → 完全走原路径，行为与本次改动前逐位一致。
//   - 未指定 unit、或单位不可识别、或 count 聚合、或 sum 遇上带偏移的单位（温度）——
//     一律**不换算**，并在 `unit_reason` 里写明原因。绝不静默返回未换算的值。
//   - 当前窗口与基线窗口**全有或全无**：任一条换算失败则两条都保持原样，
//     避免"当前已换算、基线没换算"这种混合结果（百分比会因此算错却看不出来）。
//
// 为什么 sum + 偏移单位要拒绝：sum(convert(x)) 与 convert(sum(x)) 在带偏移时不等价
// （n 个 0°C 之和换算成 °F 是 96，而 0°C 换算成 °F 是 32）。温度求和在物理上本就没有
// 意义，与其给一个取决于实现顺序的数，不如明确拒绝并说明。
//
// 重构建议：把"源单位"改为从设备配置链（device → device_config → device_template_id →
// device_model_telemetry.unit）服务端解析，调用方就不必再传 unit。届时本文件只需替换
// 解析来源，换算与拒绝语义保持不变。
package service

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
	units "aetherlink-iot/backend/pkg/units"
)

// telemetryUnitPlan 一次分析请求的单位换算方案。
type telemetryUnitPlan struct {
	SourceUnit string
	TargetUnit string
	// Reason 非空表示"本次不换算"，并说明原因。绝不把未换算的值冒充成换算过的。
	Reason string
}

// resolveTelemetryUnitPlan 解析单位换算方案。
//
// 参数 aggregate 是**已校验**的聚合方式（avg/sum/min/max/count/last）。
func resolveTelemetryUnitPlan(sourceUnit, aggregate string, system string) telemetryUnitPlan {
	plan := telemetryUnitPlan{SourceUnit: strings.TrimSpace(sourceUnit)}

	target := strings.TrimSpace(system)
	if target == "" {
		// 未请求换算：这是默认路径，不是"失败"，因此不写 Reason。
		return plan
	}
	if target != string(units.SystemMetric) && target != string(units.SystemImperial) {
		plan.Reason = "unit_system must be metric or imperial"
		return plan
	}
	if aggregate == model.TelemetryAnalysisAggCount {
		plan.Reason = "count is a tally, not a measurable quantity; conversion skipped"
		return plan
	}
	if plan.SourceUnit == "" {
		plan.Reason = "unit is required when unit_system is set; conversion skipped"
		return plan
	}

	unit, ok := units.Lookup(plan.SourceUnit)
	if !ok {
		plan.Reason = "unit is not a convertible unit: " + plan.SourceUnit
		return plan
	}
	if aggregate == model.TelemetryAnalysisAggSum && unit.Offset != 0 {
		plan.Reason = "sum of an offset-based unit (e.g. temperature) is not well-defined; conversion skipped"
		return plan
	}

	canonical, err := units.CanonicalUnit(unit.Dimension, units.System(target))
	if err != nil {
		plan.Reason = err.Error()
		return plan
	}
	plan.TargetUnit = canonical
	return plan
}

// convertTelemetryAnalysisWindows 对当前/基线两条序列做同一换算。
//
// 全有或全无：任一条失败则两条都原样返回，避免出现"当前已换算、基线没换算"的混合结果。
func convertTelemetryAnalysisWindows(current, baseline []float64, sourceUnit, targetUnit string) ([]float64, []float64, error) {
	if targetUnit == "" || targetUnit == sourceUnit {
		return current, baseline, nil
	}

	convertedCurrent, err := units.ConvertSeries(current, sourceUnit, targetUnit)
	if err != nil {
		return current, baseline, err
	}
	if len(baseline) == 0 {
		return convertedCurrent, baseline, nil
	}

	convertedBaseline, err := units.ConvertSeries(baseline, sourceUnit, targetUnit)
	if err != nil {
		// 基线换算失败：当前窗口也退回原样，保证两端同单位。
		return current, baseline, err
	}
	return convertedCurrent, convertedBaseline, nil
}
