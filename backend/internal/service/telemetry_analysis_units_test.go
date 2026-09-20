// 文件用途：遥测分析单位换算（ROADMAP TB-9 接线层）的契约测试。
//
// 核心逻辑：分三层验证——换算方案的拒绝语义、双窗口换算的全有或全无、以及
// **换算发生在聚合与对比之前**这一顺序不变量。
//
// 关键注意事项：本层最危险的缺陷不是"换算算错"，而是两件更难发现的事：
//  1. 该拒绝时静默返回未换算的值（看起来像换算过）；
//  2. 换算发生在对比之后，导致 delta / 百分比仍停在源单位。
//     带偏移的单位（温度）会让这两种错误在数值上真实可辨——本文件用它做判据。
package service

import (
	"context"
	"math"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

func withTelemetryAnalysisOps(t *testing.T, ops telemetryAnalysisOperations) {
	t.Helper()
	original := telemetryAnalysisOps
	telemetryAnalysisOps = ops
	t.Cleanup(func() { telemetryAnalysisOps = original })
}

func claimsForTenant() *utils.UserClaims {
	return &utils.UserClaims{TenantID: "tenant-1"}
}

// almostEqualFloat 相对容差比较。
// 摄氏↔华氏必须经 `value*Scale + Offset` 往返，而 5/9 无法被二进制浮点精确表示，
// 0°C 会得到 31.999999999999936（相对误差 2e-15）。这是该通用形式的固有代价，
// 不是缺陷；工程单位换算不需要逐位相等。
func almostEqualFloat(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9*math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
}

// fakeAnalysisOps 注入一组取数操作：按 start 区分"当前窗口"与"基线窗口"。
func fakeAnalysisOps(current, baseline []float64) telemetryAnalysisOperations {
	toRows := func(values []float64) []map[string]interface{} {
		rows := make([]map[string]interface{}, 0, len(values))
		for _, value := range values {
			rows = append(rows, map[string]interface{}{"y": value})
		}
		return rows
	}
	return telemetryAnalysisOperations{
		access: func(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
			return &model.Device{ID: deviceID}, nil
		},
		fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
			if start == 1000 {
				return toRows(current), nil
			}
			return toRows(baseline), nil
		},
		export: ExportTelemetryAnalysis,
	}
}

func baseAnalysisQuery() model.TelemetryAnalysisQuery {
	return model.TelemetryAnalysisQuery{
		DeviceIDs: []string{"dev-1"},
		Key:       "temp",
		StartTime: 1000,
		EndTime:   2000,
		Aggregate: model.TelemetryAnalysisAggAvg,
		Compare:   model.TelemetryAnalysisComparePrevious,
	}
}

// ---------------------------------------------------------------------------
// 方案解析
// ---------------------------------------------------------------------------

func TestResolveTelemetryUnitPlan(t *testing.T) {
	cases := []struct {
		name       string
		sourceUnit string
		aggregate  string
		system     string
		wantTarget string
		wantReason bool
	}{
		{name: "未请求换算时既不换算也不报原因", sourceUnit: "°C", aggregate: "avg", system: "", wantTarget: "", wantReason: false},
		{name: "非法制式", sourceUnit: "°C", aggregate: "avg", system: "si", wantTarget: "", wantReason: true},
		{name: "count 不换算", sourceUnit: "°C", aggregate: model.TelemetryAnalysisAggCount, system: "imperial", wantTarget: "", wantReason: true},
		{name: "缺源单位", sourceUnit: "", aggregate: "avg", system: "imperial", wantTarget: "", wantReason: true},
		{name: "未知单位", sourceUnit: "furlong", aggregate: "avg", system: "imperial", wantTarget: "", wantReason: true},
		{name: "sum 遇带偏移单位（温度）拒绝", sourceUnit: "°C", aggregate: model.TelemetryAnalysisAggSum, system: "imperial", wantTarget: "", wantReason: true},
		{name: "sum 对纯比例单位可用", sourceUnit: "m", aggregate: model.TelemetryAnalysisAggSum, system: "imperial", wantTarget: "ft", wantReason: false},
		{name: "摄氏转英制", sourceUnit: "°C", aggregate: "avg", system: "imperial", wantTarget: "°F"},
		{name: "摄氏转公制（已是代表单位）", sourceUnit: "°C", aggregate: "avg", system: "metric", wantTarget: "°C"},
		{name: "千帕转英制", sourceUnit: "kPa", aggregate: "avg", system: "imperial", wantTarget: "psi"},
		{name: "流量转英制", sourceUnit: "m3/h", aggregate: "avg", system: "imperial", wantTarget: "gpm"},
		{name: "别名同样可解析", sourceUnit: "degC", aggregate: "avg", system: "imperial", wantTarget: "°F"},
		{name: "源单位含空白也能解析", sourceUnit: "  °C  ", aggregate: "avg", system: "imperial", wantTarget: "°F"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			plan := resolveTelemetryUnitPlan(testCase.sourceUnit, testCase.aggregate, testCase.system)
			if plan.TargetUnit != testCase.wantTarget {
				t.Fatalf("TargetUnit = %q，期望 %q（reason=%q）", plan.TargetUnit, testCase.wantTarget, plan.Reason)
			}
			if testCase.wantReason && plan.Reason == "" {
				t.Fatal("期望给出不换算的原因，但 Reason 为空——这会让调用方误以为已换算")
			}
			if !testCase.wantReason && plan.Reason != "" {
				t.Fatalf("不应给出原因，实际 Reason = %q", plan.Reason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 双窗口换算：全有或全无
// ---------------------------------------------------------------------------

func TestConvertTelemetryAnalysisWindows(t *testing.T) {
	t.Run("两条序列都换算", func(t *testing.T) {
		current, baseline, err := convertTelemetryAnalysisWindows([]float64{0, 10, 20}, []float64{10, 20, 30}, "°C", "°F")
		if err != nil {
			t.Fatalf("意外报错：%v", err)
		}
		wantCurrent := []float64{32, 50, 68}
		wantBaseline := []float64{50, 68, 86}
		for index := range wantCurrent {
			if !almostEqualFloat(current[index], wantCurrent[index]) {
				t.Fatalf("current[%d] = %v，期望 %v", index, current[index], wantCurrent[index])
			}
		}
		for index := range wantBaseline {
			if !almostEqualFloat(baseline[index], wantBaseline[index]) {
				t.Fatalf("baseline[%d] = %v，期望 %v", index, baseline[index], wantBaseline[index])
			}
		}
	})

	t.Run("同单位或未指定目标时原样返回", func(t *testing.T) {
		current, baseline, err := convertTelemetryAnalysisWindows([]float64{1, 2}, []float64{3}, "°C", "°C")
		if err != nil || current[0] != 1 || baseline[0] != 3 {
			t.Fatalf("同单位不应改动：current=%v baseline=%v err=%v", current, baseline, err)
		}
		current, baseline, err = convertTelemetryAnalysisWindows([]float64{1, 2}, []float64{3}, "°C", "")
		if err != nil || current[0] != 1 || baseline[0] != 3 {
			t.Fatalf("未指定目标不应改动：current=%v baseline=%v err=%v", current, baseline, err)
		}
	})

	t.Run("基线为空时只换算当前窗口", func(t *testing.T) {
		current, baseline, err := convertTelemetryAnalysisWindows([]float64{0}, nil, "°C", "°F")
		if err != nil {
			t.Fatalf("意外报错：%v", err)
		}
		if current[0] != 32 && !almostEqualFloat(current[0], 32) {
			t.Fatalf("current=%v baseline=%v", current, baseline)
		}
		if len(baseline) != 0 {
			t.Fatalf("基线应保持为空，实际 %v", baseline)
		}
	})

	// 关键不变量：任一条失败则两条都保持原样，绝不产生"当前已换算、基线没换算"的混合结果。
	t.Run("基线含非法值时两条都不换算", func(t *testing.T) {
		currentIn := []float64{0, 10}
		baselineIn := []float64{math.NaN()}
		current, baseline, err := convertTelemetryAnalysisWindows(currentIn, baselineIn, "°C", "°F")
		if err == nil {
			t.Fatal("基线含 NaN 必须报错")
		}
		if current[0] != currentIn[0] || current[1] != currentIn[1] {
			t.Fatalf("基线失败时当前窗口必须保持原样，实际 %v", current)
		}
		if !math.IsNaN(baseline[0]) {
			t.Fatalf("基线必须保持原样，实际 %v", baseline)
		}
	})

	t.Run("当前窗口含非法值时两条都不换算", func(t *testing.T) {
		currentIn := []float64{math.Inf(1)}
		baselineIn := []float64{10}
		current, baseline, err := convertTelemetryAnalysisWindows(currentIn, baselineIn, "°C", "°F")
		if err == nil {
			t.Fatal("当前窗口含 Inf 必须报错")
		}
		if !math.IsInf(current[0], 1) || baseline[0] != 10 {
			t.Fatalf("两条都必须保持原样，实际 current=%v baseline=%v", current, baseline)
		}
	})
}

// ---------------------------------------------------------------------------
// 端到端：换算发生在聚合与对比之前
// ---------------------------------------------------------------------------

// TestRunTelemetryAnalysisConvertsBeforeComparison 是本次接线最关键的一条断言。
//
// 用带偏移的摄氏→华氏：当前窗口均值 10°C、基线 20°C。
//   - 先换算再对比：50°F 对 68°F → delta -18，percent ≈ -26.4705882%
//   - 先对比再换算：percent 会停留在 -50%（10 对 20 的源单位结果）
//
// 两者的百分比数值不同，因此这条断言能真正区分"换算顺序对不对"，
// 而不是仅仅确认"换算被调用过"。
func TestRunTelemetryAnalysisConvertsBeforeComparison(t *testing.T) {
	withTelemetryAnalysisOps(t, fakeAnalysisOps([]float64{0, 10, 20}, []float64{10, 20, 30}))

	query := baseAnalysisQuery()
	query.Unit = "°C"
	query.UnitSystem = model.TelemetryAnalysisUnitSystemImperial

	result, err := RunTelemetryAnalysis(nil, query, claimsForTenant())
	if err != nil {
		t.Fatalf("分析失败：%v", err)
	}

	if result.UnitSystem != model.TelemetryAnalysisUnitSystemImperial {
		t.Fatalf("unit_system = %q", result.UnitSystem)
	}
	if result.SourceUnit != "°C" || result.TargetUnit != "°F" {
		t.Fatalf("换算单位未回传：source=%q target=%q", result.SourceUnit, result.TargetUnit)
	}
	if len(result.Devices) != 1 {
		t.Fatalf("设备数 = %d", len(result.Devices))
	}

	device := result.Devices[0]
	if device.UnitReason != "" {
		t.Fatalf("本次应完成换算，不应有 unit_reason：%q", device.UnitReason)
	}
	if math.Abs(device.Current.Value-50) > 1e-9 {
		t.Fatalf("当前窗口 = %v，期望 50°F", device.Current.Value)
	}
	if math.Abs(device.Baseline.Value-68) > 1e-9 {
		t.Fatalf("基线窗口 = %v，期望 68°F", device.Baseline.Value)
	}
	if device.Delta == nil || math.Abs(*device.Delta-(-18)) > 1e-9 {
		t.Fatalf("delta = %v，期望 -18（若为 -10 说明 delta 停在源单位）", device.Delta)
	}
	if device.Percent == nil {
		t.Fatal("percent 不应为 nil")
	}
	const wantPercent = -18.0 / 68.0 * 100
	if math.Abs(*device.Percent-wantPercent) > 1e-9 {
		t.Fatalf("percent = %v，期望 %v（若为 -50 说明换算发生在对比之后）", *device.Percent, wantPercent)
	}
}

// 负向对照：不请求换算时，结果必须与既有行为一致（不换算、不回传单位、不给原因）。
func TestRunTelemetryAnalysisWithoutUnitSystemIsUnchanged(t *testing.T) {
	withTelemetryAnalysisOps(t, fakeAnalysisOps([]float64{0, 10, 20}, []float64{10, 20, 30}))

	result, err := RunTelemetryAnalysis(nil, baseAnalysisQuery(), claimsForTenant())
	if err != nil {
		t.Fatalf("分析失败：%v", err)
	}

	if result.UnitSystem != "" || result.SourceUnit != "" || result.TargetUnit != "" {
		t.Fatalf("未请求换算时不应回传单位：%q %q %q", result.UnitSystem, result.SourceUnit, result.TargetUnit)
	}
	device := result.Devices[0]
	if device.UnitReason != "" {
		t.Fatalf("未请求换算不是「失败」，不应写 unit_reason：%q", device.UnitReason)
	}
	if math.Abs(device.Current.Value-10) > 1e-9 || math.Abs(device.Baseline.Value-20) > 1e-9 {
		t.Fatalf("数值不应被改动：current=%v baseline=%v", device.Current.Value, device.Baseline.Value)
	}
	if device.Percent == nil || math.Abs(*device.Percent-(-50)) > 1e-9 {
		t.Fatalf("percent = %v，期望 -50", device.Percent)
	}
}

// 请求了换算但无法换算时，必须如实说明原因，且数值保持源单位。
func TestRunTelemetryAnalysisReportsUnitReasonInsteadOfSilentPassThrough(t *testing.T) {
	cases := []struct {
		name      string
		unit      string
		aggregate string
		// wantValue 该聚合方式在源序列 [0,10,20] 上的期望值。
		// 不能一律按均值断言：count 得 3、sum 得 30，与"未换算"无关，是聚合本身的语义。
		wantValue float64
	}{
		{name: "缺源单位", unit: "", aggregate: model.TelemetryAnalysisAggAvg, wantValue: 10},
		{name: "未知单位", unit: "furlong", aggregate: model.TelemetryAnalysisAggAvg, wantValue: 10},
		{name: "count 不换算", unit: "°C", aggregate: model.TelemetryAnalysisAggCount, wantValue: 3},
		{name: "sum 遇温度拒绝", unit: "°C", aggregate: model.TelemetryAnalysisAggSum, wantValue: 30},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			withTelemetryAnalysisOps(t, fakeAnalysisOps([]float64{0, 10, 20}, nil))

			query := baseAnalysisQuery()
			query.Compare = model.TelemetryAnalysisCompareNone
			query.Unit = testCase.unit
			query.UnitSystem = model.TelemetryAnalysisUnitSystemImperial
			query.Aggregate = testCase.aggregate

			result, err := RunTelemetryAnalysis(nil, query, claimsForTenant())
			if err != nil {
				t.Fatalf("分析失败：%v", err)
			}

			if result.TargetUnit != "" || result.SourceUnit != "" {
				t.Fatalf("未换算时不得回传换算单位：source=%q target=%q", result.SourceUnit, result.TargetUnit)
			}
			device := result.Devices[0]
			if device.UnitReason == "" {
				t.Fatal("未换算必须给出 unit_reason，否则调用方会把源单位数值当成已换算结果")
			}
			if device.Current.OK && !almostEqualFloat(device.Current.Value, testCase.wantValue) {
				t.Fatalf("未换算时数值必须保持源单位，实际 %v，期望 %v", device.Current.Value, testCase.wantValue)
			}
		})
	}
}

// TB-9：若调用方未传 unit 但指定了 unit_system，自动从设备物模型链路解析源单位并换算。
func TestRunTelemetryAnalysisAutoResolvesUnitFromDevice(t *testing.T) {
	ops := fakeAnalysisOps([]float64{0, 10, 20}, nil)
	ops.resolveUnit = func(ctx context.Context, deviceID, key string) (string, error) {
		if deviceID == "dev-1" && key == "temp" {
			return "°C", nil
		}
		return "", nil
	}
	withTelemetryAnalysisOps(t, ops)

	query := baseAnalysisQuery()
	query.Compare = model.TelemetryAnalysisCompareNone
	query.Unit = "" // 刻意为空，依赖服务端自动解析
	query.UnitSystem = model.TelemetryAnalysisUnitSystemImperial
	query.Aggregate = model.TelemetryAnalysisAggAvg

	result, err := RunTelemetryAnalysis(context.Background(), query, claimsForTenant())
	if err != nil {
		t.Fatalf("分析失败：%v", err)
	}

	if result.SourceUnit != "°C" || result.TargetUnit != "°F" {
		t.Fatalf("自动解析与换算失败：source=%q target=%q", result.SourceUnit, result.TargetUnit)
	}
	device := result.Devices[0]
	if device.UnitReason != "" {
		t.Fatalf("换算成功时不应有 unit_reason：%q", device.UnitReason)
	}
	// 均值 10°C -> 50°F
	if !almostEqualFloat(device.Current.Value, 50) {
		t.Fatalf("期望换算后为 50°F，实际为 %v", device.Current.Value)
	}
}
