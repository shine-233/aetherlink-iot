// 文件用途：工程单位换算的纯逻辑内核（对标 ThingsBoard 4.1 "Units Conversion"）。
//
// 核心逻辑：每个单位声明「所属量纲 + 到该量纲基准单位的线性变换（base = value*Scale + Offset）」，
// 换算一律经由基准单位中转，因此任意两个同量纲单位之间都能一步换算，且温度这类
// **带偏移**的单位与长度这类**纯比例**单位共用同一条通路。
//
// 关键注意事项（fail closed，全部显式报错，绝不静默兜底）：
//   - 未知单位符号 → 报错，**不**当成"原样返回"。静默兜底会让"没换算"看起来像"换算成功"，
//     在仪表盘上是显示错数，比报错危险得多。
//   - 量纲不匹配（如把 m 当 kg）→ 报错。绝不按"都是数字"就硬算。
//   - 输入 NaN / ±Inf → 报错。这类值进入换算后会被传播到展示层且难以回溯。
//   - Scale 为 0 的单位在注册期即被拒绝（会导致除零）。
//
// 与 ThingsBoard 的差异：TB 的单位换算绑定在 UI 部件与用户单位制偏好上；
// 本包只做**纯换算**，单位制（metric/imperial）只用于挑选"该量纲在该制式下的代表单位"，
// 不持有任何用户偏好状态，便于在服务层与前端各取所需。
//
// 重构建议：若后续要支持非线性单位（dB、pH），需要把 Scale/Offset 换成接口，
// 届时请保留"未知单位必须报错"这条不变量。
package units

import (
	"errors"
	"fmt"
	"math"
)

// Dimension 物理量纲。同一量纲内的单位才可互转。
type Dimension string

const (
	DimensionTemperature Dimension = "temperature"
	DimensionLength      Dimension = "length"
	DimensionMass        Dimension = "mass"
	DimensionVolume      Dimension = "volume"
	DimensionArea        Dimension = "area"
	DimensionSpeed       Dimension = "speed"
	DimensionPressure    Dimension = "pressure"
	DimensionEnergy      Dimension = "energy"
	DimensionPower       Dimension = "power"
	DimensionFlow        Dimension = "flow"
	DimensionTime        Dimension = "time"
	DimensionRatio       Dimension = "ratio"
)

// System 单位制式。只用于挑选代表单位，不参与换算本身。
type System string

const (
	SystemMetric   System = "metric"
	SystemImperial System = "imperial"
)

// Unit 一个可换算的单位。
// 换算关系：基准值 = 本单位的数值 * Scale + Offset。
type Unit struct {
	Symbol    string
	Dimension Dimension
	Scale     float64
	Offset    float64
	System    System // 该单位的归属制式；两制式共用的单位填 SystemMetric
}

// ToBase 把本单位的数值转换为基准单位数值。
func (u Unit) ToBase(value float64) float64 { return value*u.Scale + u.Offset }

// FromBase 把基准单位数值转换回本单位数值。
func (u Unit) FromBase(base float64) float64 { return (base - u.Offset) / u.Scale }

var (
	// ErrUnknownUnit 未知单位符号。刻意不提供"原样返回"的宽容分支。
	ErrUnknownUnit = errors.New("units: unknown unit symbol")
	// ErrDimensionMismatch 两个单位量纲不同。
	ErrDimensionMismatch = errors.New("units: dimension mismatch")
	// ErrInvalidValue 输入不是有限数（NaN / ±Inf）。
	ErrInvalidValue = errors.New("units: value must be a finite number")
	// ErrNoCanonicalUnit 该量纲在该制式下没有登记代表单位。
	ErrNoCanonicalUnit = errors.New("units: no canonical unit for dimension and system")
)

// fahrenheitScale / fahrenheitOffset 把 °F 映射到开尔文基准：
// K = F*5/9 + (273.15 - 32*5/9)。
const fahrenheitScale = 5.0 / 9.0
const fahrenheitOffset = 273.15 - 32*fahrenheitScale

// registry 单位表。键为符号，另可通过 aliases 建立别名索引。
var registry = map[string]Unit{
	// ---- 温度（基准 K）----
	"°C": {Symbol: "°C", Dimension: DimensionTemperature, Scale: 1, Offset: 273.15, System: SystemMetric},
	"°F": {Symbol: "°F", Dimension: DimensionTemperature, Scale: fahrenheitScale, Offset: fahrenheitOffset, System: SystemImperial},
	"K":  {Symbol: "K", Dimension: DimensionTemperature, Scale: 1, Offset: 0, System: SystemMetric},

	// ---- 长度（基准 m）----
	"mm": {Symbol: "mm", Dimension: DimensionLength, Scale: 0.001, System: SystemMetric},
	"cm": {Symbol: "cm", Dimension: DimensionLength, Scale: 0.01, System: SystemMetric},
	"m":  {Symbol: "m", Dimension: DimensionLength, Scale: 1, System: SystemMetric},
	"km": {Symbol: "km", Dimension: DimensionLength, Scale: 1000, System: SystemMetric},
	"in": {Symbol: "in", Dimension: DimensionLength, Scale: 0.0254, System: SystemImperial},
	"ft": {Symbol: "ft", Dimension: DimensionLength, Scale: 0.3048, System: SystemImperial},
	"yd": {Symbol: "yd", Dimension: DimensionLength, Scale: 0.9144, System: SystemImperial},
	"mi": {Symbol: "mi", Dimension: DimensionLength, Scale: 1609.344, System: SystemImperial},

	// ---- 质量（基准 kg）----
	"g":  {Symbol: "g", Dimension: DimensionMass, Scale: 0.001, System: SystemMetric},
	"kg": {Symbol: "kg", Dimension: DimensionMass, Scale: 1, System: SystemMetric},
	"t":  {Symbol: "t", Dimension: DimensionMass, Scale: 1000, System: SystemMetric},
	"oz": {Symbol: "oz", Dimension: DimensionMass, Scale: 0.028349523125, System: SystemImperial},
	"lb": {Symbol: "lb", Dimension: DimensionMass, Scale: 0.45359237, System: SystemImperial},

	// ---- 体积（基准 L）----
	"mL":    {Symbol: "mL", Dimension: DimensionVolume, Scale: 0.001, System: SystemMetric},
	"L":     {Symbol: "L", Dimension: DimensionVolume, Scale: 1, System: SystemMetric},
	"m3":    {Symbol: "m3", Dimension: DimensionVolume, Scale: 1000, System: SystemMetric},
	"fl_oz": {Symbol: "fl_oz", Dimension: DimensionVolume, Scale: 0.0295735295625, System: SystemImperial},
	"qt":    {Symbol: "qt", Dimension: DimensionVolume, Scale: 0.946352946, System: SystemImperial},
	"gal":   {Symbol: "gal", Dimension: DimensionVolume, Scale: 3.785411784, System: SystemImperial},

	// ---- 面积（基准 m2）----
	"cm2":  {Symbol: "cm2", Dimension: DimensionArea, Scale: 0.0001, System: SystemMetric},
	"m2":   {Symbol: "m2", Dimension: DimensionArea, Scale: 1, System: SystemMetric},
	"ha":   {Symbol: "ha", Dimension: DimensionArea, Scale: 10000, System: SystemMetric},
	"km2":  {Symbol: "km2", Dimension: DimensionArea, Scale: 1000000, System: SystemMetric},
	"in2":  {Symbol: "in2", Dimension: DimensionArea, Scale: 0.00064516, System: SystemImperial},
	"ft2":  {Symbol: "ft2", Dimension: DimensionArea, Scale: 0.09290304, System: SystemImperial},
	"acre": {Symbol: "acre", Dimension: DimensionArea, Scale: 4046.8564224, System: SystemImperial},

	// ---- 速度（基准 m/s）----
	"m/s":  {Symbol: "m/s", Dimension: DimensionSpeed, Scale: 1, System: SystemMetric},
	"km/h": {Symbol: "km/h", Dimension: DimensionSpeed, Scale: 1.0 / 3.6, System: SystemMetric},
	"kn":   {Symbol: "kn", Dimension: DimensionSpeed, Scale: 0.514444, System: SystemImperial},
	"mph":  {Symbol: "mph", Dimension: DimensionSpeed, Scale: 0.44704, System: SystemImperial},

	// ---- 压力（基准 Pa）----
	"Pa":   {Symbol: "Pa", Dimension: DimensionPressure, Scale: 1, System: SystemMetric},
	"mbar": {Symbol: "mbar", Dimension: DimensionPressure, Scale: 100, System: SystemMetric},
	"kPa":  {Symbol: "kPa", Dimension: DimensionPressure, Scale: 1000, System: SystemMetric},
	"bar":  {Symbol: "bar", Dimension: DimensionPressure, Scale: 100000, System: SystemMetric},
	"MPa":  {Symbol: "MPa", Dimension: DimensionPressure, Scale: 1000000, System: SystemMetric},
	"psi":  {Symbol: "psi", Dimension: DimensionPressure, Scale: 6894.757293168, System: SystemImperial},

	// ---- 能量（基准 J）----
	"J":   {Symbol: "J", Dimension: DimensionEnergy, Scale: 1, System: SystemMetric},
	"Wh":  {Symbol: "Wh", Dimension: DimensionEnergy, Scale: 3600, System: SystemMetric},
	"kJ":  {Symbol: "kJ", Dimension: DimensionEnergy, Scale: 1000, System: SystemMetric},
	"kWh": {Symbol: "kWh", Dimension: DimensionEnergy, Scale: 3600000, System: SystemMetric},
	"BTU": {Symbol: "BTU", Dimension: DimensionEnergy, Scale: 1055.05585262, System: SystemImperial},

	// ---- 功率（基准 W）----
	"W":  {Symbol: "W", Dimension: DimensionPower, Scale: 1, System: SystemMetric},
	"kW": {Symbol: "kW", Dimension: DimensionPower, Scale: 1000, System: SystemMetric},
	"MW": {Symbol: "MW", Dimension: DimensionPower, Scale: 1000000, System: SystemMetric},
	"hp": {Symbol: "hp", Dimension: DimensionPower, Scale: 745.6998715823, System: SystemImperial},

	// ---- 流量（基准 m3/s）----
	"L/s":   {Symbol: "L/s", Dimension: DimensionFlow, Scale: 0.001, System: SystemMetric},
	"L/min": {Symbol: "L/min", Dimension: DimensionFlow, Scale: 1.0 / 60000, System: SystemMetric},
	"m3/h":  {Symbol: "m3/h", Dimension: DimensionFlow, Scale: 1.0 / 3600, System: SystemMetric},
	"m3/s":  {Symbol: "m3/s", Dimension: DimensionFlow, Scale: 1, System: SystemMetric},
	"cfm":   {Symbol: "cfm", Dimension: DimensionFlow, Scale: 0.0004719474432, System: SystemImperial},
	"gpm":   {Symbol: "gpm", Dimension: DimensionFlow, Scale: 6.30901964e-05, System: SystemImperial},

	// ---- 时间（基准 s；两制式共用）----
	"ms":  {Symbol: "ms", Dimension: DimensionTime, Scale: 0.001, System: SystemMetric},
	"s":   {Symbol: "s", Dimension: DimensionTime, Scale: 1, System: SystemMetric},
	"min": {Symbol: "min", Dimension: DimensionTime, Scale: 60, System: SystemMetric},
	"h":   {Symbol: "h", Dimension: DimensionTime, Scale: 3600, System: SystemMetric},

	// ---- 比例（基准为 1）----
	"ppm": {Symbol: "ppm", Dimension: DimensionRatio, Scale: 1e-6, System: SystemMetric},
	"%":   {Symbol: "%", Dimension: DimensionRatio, Scale: 0.01, System: SystemMetric},
}

// aliasIndex 常见书写别名 → 规范符号。别名只为容忍现场写法差异，不引入新量纲。
var aliasIndex = map[string]string{
	"C": "°C", "degC": "°C", "celsius": "°C",
	"F": "°F", "degF": "°F", "fahrenheit": "°F",
	"kelvin": "K",
	"meter":  "m", "meters": "m", "metre": "m",
	"kilometer": "km", "kilometers": "km",
	"foot": "ft", "feet": "ft", "inch": "in", "inches": "in",
	"mile": "mi", "miles": "mi",
	"gram": "g", "grams": "g", "kilogram": "kg", "kilograms": "kg",
	"pound": "lb", "pounds": "lb", "lbs": "lb", "ounce": "oz",
	"liter": "L", "liters": "L", "litre": "L", "milliliter": "mL",
	"gallon": "gal", "gallons": "gal",
	"hour": "h", "hours": "h", "minute": "min", "minutes": "min",
	"second": "s", "seconds": "s",
	"percent": "%",
	"mps":     "m/s", "kph": "km/h",
	"degrees_celsius": "°C", "degrees_fahrenheit": "°F",
}

// canonical 各量纲在各制式下的代表单位。
var canonical = map[Dimension]map[System]string{
	DimensionTemperature: {SystemMetric: "°C", SystemImperial: "°F"},
	DimensionLength:      {SystemMetric: "m", SystemImperial: "ft"},
	DimensionMass:        {SystemMetric: "kg", SystemImperial: "lb"},
	DimensionVolume:      {SystemMetric: "L", SystemImperial: "gal"},
	DimensionArea:        {SystemMetric: "m2", SystemImperial: "ft2"},
	DimensionSpeed:       {SystemMetric: "m/s", SystemImperial: "mph"},
	DimensionPressure:    {SystemMetric: "kPa", SystemImperial: "psi"},
	DimensionEnergy:      {SystemMetric: "kWh", SystemImperial: "BTU"},
	DimensionPower:       {SystemMetric: "kW", SystemImperial: "hp"},
	DimensionFlow:        {SystemMetric: "m3/h", SystemImperial: "gpm"},
	DimensionTime:        {SystemMetric: "s", SystemImperial: "s"},
	DimensionRatio:       {SystemMetric: "%", SystemImperial: "%"},
}

// Lookup 按符号（含别名）查找单位。空白与大小写不敏感的写法会先被规范化。
func Lookup(symbol string) (Unit, bool) {
	if unit, ok := registry[symbol]; ok {
		return unit, true
	}
	if canonicalSymbol, ok := aliasIndex[symbol]; ok {
		unit, found := registry[canonicalSymbol]
		return unit, found
	}
	return Unit{}, false
}

// IsKnown 判断符号是否可识别。
func IsKnown(symbol string) bool {
	_, ok := Lookup(symbol)
	return ok
}

// DimensionOf 返回符号所属量纲。
func DimensionOf(symbol string) (Dimension, bool) {
	unit, ok := Lookup(symbol)
	if !ok {
		return "", false
	}
	return unit.Dimension, true
}

// Symbols 返回全部已登记符号（不含量纲分组，顺序不保证）。
func Symbols() []string {
	out := make([]string, 0, len(registry))
	for symbol := range registry {
		out = append(out, symbol)
	}
	return out
}

// CanonicalUnit 返回该量纲在该制式下的代表单位符号。
func CanonicalUnit(dimension Dimension, system System) (string, error) {
	bySystem, ok := canonical[dimension]
	if !ok {
		return "", fmt.Errorf("%w: dimension=%s", ErrNoCanonicalUnit, dimension)
	}
	symbol, ok := bySystem[system]
	if !ok {
		return "", fmt.Errorf("%w: dimension=%s system=%s", ErrNoCanonicalUnit, dimension, system)
	}
	return symbol, nil
}

// Convert 把 value 从 from 单位换算为 to 单位。
// 任一环节不成立都返回错误，绝不返回"未换算的原值"。
func Convert(value float64, from, to string) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%w: %v", ErrInvalidValue, value)
	}

	fromUnit, ok := Lookup(from)
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownUnit, from)
	}
	toUnit, ok := Lookup(to)
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownUnit, to)
	}
	if fromUnit.Dimension != toUnit.Dimension {
		return 0, fmt.Errorf("%w: %s(%s) -> %s(%s)",
			ErrDimensionMismatch, fromUnit.Symbol, fromUnit.Dimension, toUnit.Symbol, toUnit.Dimension)
	}

	// 同一单位直接返回，避免 Offset 参与造成的浮点抖动。
	if fromUnit.Symbol == toUnit.Symbol {
		return value, nil
	}

	converted := toUnit.FromBase(fromUnit.ToBase(value))
	if math.IsNaN(converted) || math.IsInf(converted, 0) {
		return 0, fmt.Errorf("%w: 换算结果非有限数", ErrInvalidValue)
	}
	return converted, nil
}

// ConvertToSystem 把 value 从 from 换算到 system 制式下该量纲的代表单位。
// 返回换算后的数值与目标单位符号。
func ConvertToSystem(value float64, from string, system System) (float64, string, error) {
	fromUnit, ok := Lookup(from)
	if !ok {
		return 0, "", fmt.Errorf("%w: %q", ErrUnknownUnit, from)
	}
	target, err := CanonicalUnit(fromUnit.Dimension, system)
	if err != nil {
		return 0, "", err
	}
	converted, err := Convert(value, from, target)
	if err != nil {
		return 0, "", err
	}
	return converted, target, nil
}

// ConvertSeries 对整条序列做同一换算。
//
// 语义：**全有或全无**——序列里任何一个值不合法，整条序列都不返回结果。
// 部分换算成功的序列会混合两种单位，比整条失败更难发现。
func ConvertSeries(values []float64, from, to string) ([]float64, error) {
	// 先做一次单位校验，让"单位本身就错"这类错误在空序列上也能暴露。
	if _, err := Convert(0, from, to); err != nil {
		return nil, err
	}

	out := make([]float64, len(values))
	for index, value := range values {
		converted, err := Convert(value, from, to)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个值换算失败: %w", index, err)
		}
		out[index] = converted
	}
	return out, nil
}

// SameDimension 判断两个符号是否同量纲（任一未知即 false）。
func SameDimension(left, right string) bool {
	leftUnit, ok := Lookup(left)
	if !ok {
		return false
	}
	rightUnit, ok := Lookup(right)
	if !ok {
		return false
	}
	return leftUnit.Dimension == rightUnit.Dimension
}
