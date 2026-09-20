package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

// P2.2 同比环比核心证据。最危险的不是减法，而是把"未定义"偷换成数字：
// 基线缺失当成 0 会凭空造出 +100% 暴涨；基数为 0 时给 0 会假装没变化、给 Inf 会把噪声放大。

func TestAggregateTelemetrySeriesEmptyIsNotZero(t *testing.T) {
	result := aggregateTelemetrySeries(model.TelemetryAnalysisAggAvg, nil)
	if result.OK {
		t.Fatal("empty series must not produce a value")
	}
	if result.Value != 0 || result.OK {
		t.Fatalf("empty -> %+v; OK must be false", result)
	}
}

func TestAggregateTelemetrySeriesComputesEachKind(t *testing.T) {
	values := []float64{2, 4, 6}
	cases := map[string]float64{
		model.TelemetryAnalysisAggAvg:   4,
		model.TelemetryAnalysisAggSum:   12,
		model.TelemetryAnalysisAggMin:   2,
		model.TelemetryAnalysisAggMax:   6,
		model.TelemetryAnalysisAggCount: 3,
		model.TelemetryAnalysisAggLast:  6,
	}
	for kind, want := range cases {
		got := aggregateTelemetrySeries(kind, values)
		if !got.OK || got.Value != want {
			t.Fatalf("%s = %+v, want %v", kind, got, want)
		}
	}
}

func TestCompareTelemetryPeriodsComputesDeltaAndPercent(t *testing.T) {
	result := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, []float64{20}, []float64{10})
	if !result.DeltaOK || result.Delta != 10 {
		t.Fatalf("delta = %v (ok=%v), want 10", result.Delta, result.DeltaOK)
	}
	if result.PercentChange == nil || *result.PercentChange != 100 {
		t.Fatalf("percent = %v, want 100", result.PercentChange)
	}
}

// 基线缺失绝不等于基线为 0：那会把"没有历史数据"渲染成 +100% 暴涨。
func TestCompareTelemetryPeriodsMissingBaselineIsUndefined(t *testing.T) {
	result := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, []float64{20}, nil)
	if result.PercentChange != nil {
		t.Fatalf("percent must be nil without baseline, got %v", *result.PercentChange)
	}
	if result.DeltaOK {
		t.Fatal("delta must be undefined without baseline")
	}
	if result.PercentUndefinedReason == "" {
		t.Fatal("undefined percent must carry a reason")
	}
}

// 基数为 0 时相对变化在数学上无定义：给 0 假装没变化，给 Inf 放大噪声。
func TestCompareTelemetryPeriodsZeroBaselineIsUndefined(t *testing.T) {
	result := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, []float64{5}, []float64{0})
	if result.PercentChange != nil {
		t.Fatalf("percent must be nil when baseline is zero, got %v", *result.PercentChange)
	}
	// 绝对变化仍然可算：5 - 0 = 5。
	if !result.DeltaOK || result.Delta != 5 {
		t.Fatalf("delta = %v (ok=%v), want 5", result.Delta, result.DeltaOK)
	}
	if !strings.Contains(result.PercentUndefinedReason, "zero") {
		t.Fatalf("reason = %q, want it to mention zero baseline", result.PercentUndefinedReason)
	}
}

func TestCompareTelemetryPeriodsCurrentMissingIsUndefined(t *testing.T) {
	result := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, nil, []float64{10})
	if result.PercentChange != nil || result.DeltaOK {
		t.Fatal("missing current period must be fully undefined")
	}
}

func TestFormatTelemetryComparisonPercentRendersUndefinedAsReason(t *testing.T) {
	undefined := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, []float64{1}, []float64{0})
	got := FormatTelemetryComparisonPercent(undefined)
	if got == "0.00%" {
		t.Fatalf("undefined must not render as 0.00%%; got %q", got)
	}
	if !strings.Contains(got, "n/a") {
		t.Fatalf("undefined should render as n/a; got %q", got)
	}
	// 未定义必须带原因，否则使用者无从判断是"没数据"还是"基数为 0"。
	if !strings.Contains(got, "zero") {
		t.Fatalf("undefined should explain why; got %q", got)
	}
	defined := CompareTelemetryPeriods(model.TelemetryAnalysisAggAvg, []float64{8}, []float64{10})
	if got := FormatTelemetryComparisonPercent(defined); got != "-20.00%" {
		t.Fatalf("percent = %q, want -20.00%%", got)
	}
}

// 环比窗口必须与当前窗口等长，否则拿 7 天跟 30 天比，变化量毫无意义。
func TestTelemetryAnalysisBaselineWindowIsSameLength(t *testing.T) {
	start, end := int64(1000), int64(2000)
	baseStart, baseEnd, err := telemetryAnalysisBaselineWindow(start, end, model.TelemetryAnalysisComparePrevious, 1)
	if err != nil {
		t.Fatalf("baseline window error = %v", err)
	}
	if baseEnd-baseStart != end-start {
		t.Fatalf("baseline length = %d, want %d", baseEnd-baseStart, end-start)
	}
	if baseEnd != start {
		t.Fatalf("previous period must end exactly where current starts; got %d", baseEnd)
	}
}

func TestTelemetryAnalysisBaselineWindowRejectsBadRange(t *testing.T) {
	if _, _, err := telemetryAnalysisBaselineWindow(2000, 1000, model.TelemetryAnalysisComparePrevious, 1); err == nil {
		t.Fatal("end before start must be rejected")
	}
}

func TestValidateTelemetryAnalysisAggregate(t *testing.T) {
	if got, err := validateTelemetryAnalysisAggregate(""); err != nil || got != model.TelemetryAnalysisAggAvg {
		t.Fatalf("empty should default to avg; got %q err=%v", got, err)
	}
	for _, kind := range []string{"avg", "sum", "min", "max", "count", "last"} {
		if _, err := validateTelemetryAnalysisAggregate(kind); err != nil {
			t.Fatalf("%s must be accepted; got %v", kind, err)
		}
	}
	// 非法聚合必须拒绝，不静默回退到 avg——回退会让调用方以为自己要的就是 avg。
	if _, err := validateTelemetryAnalysisAggregate("median"); err == nil {
		t.Fatal("unsupported aggregate must be rejected")
	}
}

// 导出必须把"无值"写成 n/a 而不是 0：把缺失当零，下游就会画出一条假的跌到 0 的曲线。
func TestBuildTelemetryAnalysisRowsRendersMissingAsNA(t *testing.T) {
	rows := buildTelemetryAnalysisRows("temp", model.TelemetryAnalysisAggAvg, []TelemetryAnalysisRowInput{
		{
			DeviceID:   "d1",
			Period:     "current",
			Comparison: TelemetryPeriodComparison{Current: TelemetryAggregateResult{Value: 5, OK: true}, Baseline: TelemetryAggregateResult{}},
		},
	})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row[4] != "5" {
		t.Fatalf("current value = %q, want 5", row[4])
	}
	if row[5] != "n/a" {
		t.Fatalf("baseline must render as n/a, got %q", row[5])
	}
	if row[6] != "n/a" {
		t.Fatalf("delta must render as n/a, got %q", row[6])
	}
}

// 防回归：曾把"周期 → 单一对比值"映射套到整个设备列表上，
// 导致所有设备共用第一台的数字——表格看着齐全，数值全是假的。
func TestBuildTelemetryAnalysisRowsKeepsPerDeviceValues(t *testing.T) {
	rows := buildTelemetryAnalysisRows("temp", model.TelemetryAnalysisAggAvg, []TelemetryAnalysisRowInput{
		{
			DeviceID:   "d1",
			Period:     "current",
			Comparison: TelemetryPeriodComparison{Current: TelemetryAggregateResult{Value: 10, OK: true}},
		},
		{
			DeviceID:   "d2",
			Period:     "current",
			Comparison: TelemetryPeriodComparison{Current: TelemetryAggregateResult{Value: 99, OK: true}},
		},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "d1" || rows[0][4] != "10" {
		t.Fatalf("row0 = %v, want d1/10", rows[0])
	}
	if rows[1][0] != "d2" || rows[1][4] != "99" {
		t.Fatalf("row1 = %v, want d2/99 (each device must keep its own value)", rows[1])
	}
}

// CSV 与 Excel 必须共用同一套列序，否则同一份数据在两种格式里互相矛盾。
func TestTelemetryAnalysisColumnsAreStable(t *testing.T) {
	columns := telemetryAnalysisColumns()
	if len(columns) != 8 {
		t.Fatalf("columns = %v", columns)
	}
	if columns[0] != "device_id" || columns[7] != "percent_change" {
		t.Fatalf("unexpected column order: %v", columns)
	}
}
