// 文件用途：单位换算内核的契约测试（含负向对照）。
//
// 核心逻辑：分四组——注册表不变量、已知换算精度、别名与代表单位、fail-closed 负向对照。
// 关键注意事项：本包的核心风险不是"算错"，而是**静默不换算**。
// 因此负向对照（未知单位 / 量纲不符 / 非有限数）与精度断言同等重要，缺一不可。
package units

import (
	"errors"
	"math"
	"testing"
)

const epsilon = 1e-9

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) <= epsilon*math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
}

func mustConvert(t *testing.T, value float64, from, to string) float64 {
	t.Helper()
	got, err := Convert(value, from, to)
	if err != nil {
		t.Fatalf("Convert(%v, %q, %q) 意外报错：%v", value, from, to, err)
	}
	return got
}

// TestRegistryInvariants 注册表自身的完整性——错误的单位定义会让所有换算一起错。
func TestRegistryInvariants(t *testing.T) {
	validSystems := map[System]bool{SystemMetric: true, SystemImperial: true}
	validDimensions := map[Dimension]bool{
		DimensionTemperature: true, DimensionLength: true, DimensionMass: true,
		DimensionVolume: true, DimensionArea: true, DimensionSpeed: true,
		DimensionPressure: true, DimensionEnergy: true, DimensionPower: true,
		DimensionFlow: true, DimensionTime: true, DimensionRatio: true,
	}

	if len(registry) == 0 {
		t.Fatal("单位注册表为空")
	}

	for symbol, unit := range registry {
		if unit.Symbol != symbol {
			t.Errorf("注册表键 %q 与 Unit.Symbol %q 不一致", symbol, unit.Symbol)
		}
		if unit.Scale == 0 {
			t.Errorf("单位 %q 的 Scale 为 0，会导致除零", symbol)
		}
		if math.IsNaN(unit.Scale) || math.IsInf(unit.Scale, 0) {
			t.Errorf("单位 %q 的 Scale 非有限数：%v", symbol, unit.Scale)
		}
		if math.IsNaN(unit.Offset) || math.IsInf(unit.Offset, 0) {
			t.Errorf("单位 %q 的 Offset 非有限数：%v", symbol, unit.Offset)
		}
		if !validDimensions[unit.Dimension] {
			t.Errorf("单位 %q 的量纲 %q 未在 Dimension 常量中登记", symbol, unit.Dimension)
		}
		if !validSystems[unit.System] {
			t.Errorf("单位 %q 的制式 %q 非法", symbol, unit.System)
		}
	}

	// 别名必须指向真实存在的单位。
	for alias, target := range aliasIndex {
		if _, ok := registry[target]; !ok {
			t.Errorf("别名 %q 指向不存在的单位 %q", alias, target)
		}
		if _, ok := registry[alias]; ok {
			t.Errorf("别名 %q 与正式符号重名", alias)
		}
	}

	// 每个量纲的两制式代表单位都必须已登记，且量纲/制式自洽。
	for dimension, bySystem := range canonical {
		for _, system := range []System{SystemMetric, SystemImperial} {
			symbol, ok := bySystem[system]
			if !ok {
				t.Errorf("量纲 %q 缺少 %q 制式的代表单位", dimension, system)
				continue
			}
			unit, found := registry[symbol]
			if !found {
				t.Errorf("量纲 %q 在 %q 的代表单位 %q 未在注册表中", dimension, system, symbol)
				continue
			}
			if unit.Dimension != dimension {
				t.Errorf("代表单位 %q 的量纲为 %q，与所属量纲 %q 不符", symbol, unit.Dimension, dimension)
			}
		}
	}
}

func TestTemperatureConversions(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		from  string
		to    string
		want  float64
	}{
		{"冰点 C->F", 0, "°C", "°F", 32},
		{"沸点 C->F", 100, "°C", "°F", 212},
		{"交点 C->F", -40, "°C", "°F", -40},
		{"冰点 C->K", 0, "°C", "K", 273.15},
		{"绝对零度 K->C", 0, "K", "°C", -273.15},
		{"冰点 F->K", 32, "°F", "K", 273.15},
		{"体温 F->C", 98.6, "°F", "°C", 37},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := mustConvert(t, testCase.value, testCase.from, testCase.to)
			if !almostEqual(got, testCase.want) {
				t.Fatalf("%v %s -> %s = %v，期望 %v", testCase.value, testCase.from, testCase.to, got, testCase.want)
			}
		})
	}
}

func TestProportionalConversions(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		from  string
		to    string
		want  float64
	}{
		{"米转厘米", 1, "m", "cm", 100},
		{"英里转米", 1, "mi", "m", 1609.344},
		{"英尺转英寸", 1, "ft", "in", 12},
		{"千克转磅", 1, "kg", "lb", 2.2046226218487757},
		{"巴转千帕", 1, "bar", "kPa", 100},
		{"千瓦时转焦耳", 1, "kWh", "J", 3600000},
		{"马力转瓦", 1, "hp", "W", 745.6998715823},
		{"立方米每小时转升每秒", 1, "m3/h", "L/s", 1000.0 / 3600},
		{"公里每小时转英里每小时", 100, "km/h", "mph", 62.13711922373339},
		{"小时转秒", 1, "h", "s", 3600},
		{"百分比转 ppm", 1, "%", "ppm", 10000},
		{"公顷转平方米", 1, "ha", "m2", 10000},
		{"立方米转升", 1, "m3", "L", 1000},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := mustConvert(t, testCase.value, testCase.from, testCase.to)
			if !almostEqual(got, testCase.want) {
				t.Fatalf("%v %s -> %s = %v，期望 %v", testCase.value, testCase.from, testCase.to, got, testCase.want)
			}
		})
	}
}

// TestRoundTripIsStable 往返换算必须回到原值——带偏移的单位最容易在这里露馅。
func TestRoundTripIsStable(t *testing.T) {
	pairs := [][2]string{
		{"°C", "°F"}, {"°C", "K"}, {"°F", "K"},
		{"m", "ft"}, {"km", "mi"}, {"kg", "lb"},
		{"kPa", "psi"}, {"kWh", "BTU"}, {"kW", "hp"}, {"m3/h", "gpm"},
	}
	for _, pair := range pairs {
		for _, value := range []float64{0, 1, -17.5, 1234.5678, -273.15} {
			forward := mustConvert(t, value, pair[0], pair[1])
			back := mustConvert(t, forward, pair[1], pair[0])
			if !almostEqual(back, value) {
				t.Fatalf("往返 %v %s -> %s -> %s = %v，期望回到 %v",
					value, pair[0], pair[1], pair[0], back, value)
			}
		}
	}
}

func TestSameUnitConversionIsExact(t *testing.T) {
	// 同单位必须原样返回，不能让 Offset 引入浮点抖动。
	for _, symbol := range []string{"°C", "°F", "K", "m", "kg", "kPa", "m3/h"} {
		for _, value := range []float64{0, 37.5, -273.15, 1e12} {
			got := mustConvert(t, value, symbol, symbol)
			if got != value {
				t.Fatalf("同单位 %s 换算 %v 变成了 %v（必须逐位相等）", symbol, value, got)
			}
		}
	}
}

func TestAliasesResolve(t *testing.T) {
	cases := map[string]string{
		"degC": "°C", "celsius": "°C", "C": "°C",
		"degF": "°F", "fahrenheit": "°F",
		"meter": "m", "meters": "m", "feet": "ft", "inches": "in",
		"pound": "lb", "lbs": "lb", "liters": "L",
		"hours": "h", "seconds": "s", "percent": "%",
		"kph": "km/h", "mps": "m/s",
	}
	for alias, wantSymbol := range cases {
		unit, ok := Lookup(alias)
		if !ok {
			t.Fatalf("别名 %q 无法解析", alias)
		}
		if unit.Symbol != wantSymbol {
			t.Fatalf("别名 %q 解析为 %q，期望 %q", alias, unit.Symbol, wantSymbol)
		}
	}
}

func TestConvertToSystem(t *testing.T) {
	cases := []struct {
		name       string
		value      float64
		from       string
		system     System
		wantValue  float64
		wantSymbol string
	}{
		{"米转英制", 1, "m", SystemImperial, 3.280839895013123, "ft"},
		{"英尺转公制", 1, "ft", SystemMetric, 0.3048, "m"},
		{"摄氏转英制", 100, "°C", SystemImperial, 212, "°F"},
		{"华氏转公制", 32, "°F", SystemMetric, 0, "°C"},
		{"千帕转英制", 100, "kPa", SystemImperial, 14.503773773021683, "psi"},
		{"时间两制式同单位", 90, "min", SystemImperial, 5400, "s"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gotValue, gotSymbol, err := ConvertToSystem(testCase.value, testCase.from, testCase.system)
			if err != nil {
				t.Fatalf("ConvertToSystem 意外报错：%v", err)
			}
			if gotSymbol != testCase.wantSymbol {
				t.Fatalf("目标单位 = %q，期望 %q", gotSymbol, testCase.wantSymbol)
			}
			if !almostEqual(gotValue, testCase.wantValue) {
				t.Fatalf("换算值 = %v，期望 %v", gotValue, testCase.wantValue)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 负向对照：本包的核心风险是"静默不换算"，这组用例是防它退化回兜底实现的锁。
// ---------------------------------------------------------------------------

func TestConvertRejectsUnknownUnit(t *testing.T) {
	if _, err := Convert(1, "unknown_unit", "m"); !errors.Is(err, ErrUnknownUnit) {
		t.Fatalf("未知源单位必须报 ErrUnknownUnit，实际：%v", err)
	}
	if _, err := Convert(1, "m", "unknown_unit"); !errors.Is(err, ErrUnknownUnit) {
		t.Fatalf("未知目标单位必须报 ErrUnknownUnit，实际：%v", err)
	}
	// 空串不是合法单位，不能当成"不需要换算"。
	if _, err := Convert(1, "", "m"); !errors.Is(err, ErrUnknownUnit) {
		t.Fatalf("空单位必须报 ErrUnknownUnit，实际：%v", err)
	}
}

func TestConvertRejectsDimensionMismatch(t *testing.T) {
	pairs := [][2]string{{"m", "kg"}, {"°C", "kPa"}, {"kWh", "m/s"}, {"%", "L"}}
	for _, pair := range pairs {
		if _, err := Convert(1, pair[0], pair[1]); !errors.Is(err, ErrDimensionMismatch) {
			t.Fatalf("%s -> %s 必须报 ErrDimensionMismatch，实际：%v", pair[0], pair[1], err)
		}
	}
}

func TestConvertRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := Convert(value, "m", "ft"); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("输入 %v 必须报 ErrInvalidValue，实际：%v", value, err)
		}
	}
}

func TestCanonicalUnitRejectsUnknownInputs(t *testing.T) {
	if _, err := CanonicalUnit(Dimension("nope"), SystemMetric); !errors.Is(err, ErrNoCanonicalUnit) {
		t.Fatalf("未知量纲必须报 ErrNoCanonicalUnit，实际：%v", err)
	}
	if _, err := CanonicalUnit(DimensionLength, System("nope")); !errors.Is(err, ErrNoCanonicalUnit) {
		t.Fatalf("未知制式必须报 ErrNoCanonicalUnit，实际：%v", err)
	}
}

// TestConvertSeriesIsAllOrNothing 序列里任何一个值不合法，整条都必须失败。
// 部分成功的序列会混合两种单位，比整条失败更难发现。
func TestConvertSeriesIsAllOrNothing(t *testing.T) {
	ok, err := ConvertSeries([]float64{0, 100, -40}, "°C", "°F")
	if err != nil {
		t.Fatalf("合法序列不应报错：%v", err)
	}
	want := []float64{32, 212, -40}
	for index := range want {
		if !almostEqual(ok[index], want[index]) {
			t.Fatalf("第 %d 个值 = %v，期望 %v", index, ok[index], want[index])
		}
	}

	if _, err := ConvertSeries([]float64{1, math.NaN(), 3}, "m", "ft"); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("含 NaN 的序列必须整条失败，实际：%v", err)
	}
	// 单位本身错误时，即使序列为空也必须暴露——空序列不能成为绕过校验的后门。
	if _, err := ConvertSeries(nil, "m", "kg"); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("空序列也必须校验单位，实际：%v", err)
	}
	if _, err := ConvertSeries([]float64{}, "bogus", "m"); !errors.Is(err, ErrUnknownUnit) {
		t.Fatalf("空序列也必须校验未知单位，实际：%v", err)
	}
}

func TestSameDimensionAndIsKnown(t *testing.T) {
	if !SameDimension("m", "ft") {
		t.Fatal("m 与 ft 应同量纲")
	}
	if SameDimension("m", "kg") {
		t.Fatal("m 与 kg 不应同量纲")
	}
	if SameDimension("m", "bogus") || SameDimension("bogus", "m") {
		t.Fatal("含未知单位时 SameDimension 必须为 false")
	}
	if !IsKnown("kPa") || IsKnown("bogus") {
		t.Fatal("IsKnown 判定错误")
	}
	if _, ok := DimensionOf("kWh"); !ok {
		t.Fatal("kWh 应可解析量纲")
	}
	if dimension, _ := DimensionOf("kWh"); dimension != DimensionEnergy {
		t.Fatalf("kWh 量纲 = %q，期望 %q", dimension, DimensionEnergy)
	}
}
