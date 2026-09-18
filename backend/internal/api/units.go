package api

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
	units "aetherlink-iot/backend/pkg/units"

	"github.com/gin-gonic/gin"
)

type UnitsApi struct{}

// UnitDTO 暴露给前端/调用方的单位元数据。
type UnitDTO struct {
	Symbol    string  `json:"symbol"`
	Dimension string  `json:"dimension"`
	Scale     float64 `json:"scale"`
	Offset    float64 `json:"offset"`
	System    string  `json:"system"`
}

// UnitsRegistryResponse 单位字典与量纲响应 DTO。
type UnitsRegistryResponse struct {
	Dimensions []string                      `json:"dimensions"`
	Units      []UnitDTO                     `json:"units"`
	Canonical  map[string]map[string]string  `json:"canonical"`
	Aliases    map[string]string             `json:"aliases"`
}

// ConvertUnitsRequest 单位转换请求 DTO。
type ConvertUnitsRequest struct {
	From   string    `json:"from" binding:"required"`
	To     string    `json:"to"`
	System string    `json:"system"`
	Value  *float64  `json:"value"`
	Series []float64 `json:"series"`
}

// ConvertUnitsResponse 单位转换响应 DTO。
type ConvertUnitsResponse struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Dimension string    `json:"dimension"`
	Value     *float64  `json:"value,omitempty"`
	Series    []float64 `json:"series,omitempty"`
}

// GetRegistry 获取系统支持的全部量纲、单位与代表单位映射。
// @Router /api/v1/units/registry [get]
func (*UnitsApi) GetRegistry(c *gin.Context) {
	dimensions := []string{
		string(units.DimensionTemperature),
		string(units.DimensionLength),
		string(units.DimensionMass),
		string(units.DimensionVolume),
		string(units.DimensionArea),
		string(units.DimensionSpeed),
		string(units.DimensionPressure),
		string(units.DimensionEnergy),
		string(units.DimensionPower),
		string(units.DimensionFlow),
		string(units.DimensionTime),
		string(units.DimensionRatio),
	}

	allSymbols := units.Symbols()
	unitDTOs := make([]UnitDTO, 0, len(allSymbols))
	for _, sym := range allSymbols {
		if u, ok := units.Lookup(sym); ok {
			unitDTOs = append(unitDTOs, UnitDTO{
				Symbol:    u.Symbol,
				Dimension: string(u.Dimension),
				Scale:     u.Scale,
				Offset:    u.Offset,
				System:    string(u.System),
			})
		}
	}

	canonicalMap := make(map[string]map[string]string)
	for _, dim := range dimensions {
		metric, _ := units.CanonicalUnit(units.Dimension(dim), units.SystemMetric)
		imperial, _ := units.CanonicalUnit(units.Dimension(dim), units.SystemImperial)
		canonicalMap[dim] = map[string]string{
			"metric":   metric,
			"imperial": imperial,
		}
	}

	// 别名表索引
	aliases := map[string]string{
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

	c.Set("data", UnitsRegistryResponse{
		Dimensions: dimensions,
		Units:      unitDTOs,
		Canonical:  canonicalMap,
		Aliases:    aliases,
	})
}

// Convert 执行单值或序列原子单位换算。
// @Router /api/v1/units/convert [post]
func (*UnitsApi) Convert(c *gin.Context) {
	var req ConvertUnitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
		return
	}

	fromUnit, ok := units.Lookup(req.From)
	if !ok {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "unknown from unit: "+req.From))
		return
	}

	targetUnit := strings.TrimSpace(req.To)
	if targetUnit == "" {
		system := strings.TrimSpace(req.System)
		if system != string(units.SystemMetric) && system != string(units.SystemImperial) {
			c.Error(errcode.NewWithMessage(errcode.CodeParamError, "either 'to' unit or 'system' (metric|imperial) is required"))
			return
		}
		canonical, err := units.CanonicalUnit(fromUnit.Dimension, units.System(system))
		if err != nil {
			c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
			return
		}
		targetUnit = canonical
	}

	if !units.SameDimension(fromUnit.Symbol, targetUnit) {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "dimension mismatch between "+fromUnit.Symbol+" and "+targetUnit))
		return
	}

	resp := ConvertUnitsResponse{
		From:      fromUnit.Symbol,
		To:        targetUnit,
		Dimension: string(fromUnit.Dimension),
	}

	if req.Value != nil {
		val, err := units.Convert(*req.Value, fromUnit.Symbol, targetUnit)
		if err != nil {
			c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
			return
		}
		resp.Value = &val
	}

	if len(req.Series) > 0 {
		series, err := units.ConvertSeries(req.Series, fromUnit.Symbol, targetUnit)
		if err != nil {
			if errors.Is(err, units.ErrInvalidValue) {
				c.Error(errcode.NewWithMessage(errcode.CodeParamError, "series contains invalid non-finite value"))
				return
			}
			c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
			return
		}
		resp.Series = series
	}

	c.Set("data", resp)
}
