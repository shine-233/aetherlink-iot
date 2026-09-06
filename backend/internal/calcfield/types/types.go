// 文件用途:计算字段高级类型——类型常量/配置解析/纯函数几何与聚合(PHASE-D-D4)。
// 放置说明:叶子包(无 uplink 依赖),供 calcfield 引擎与 service 校验共用,规避导入环。
package types

import (
	"encoding/json"
	"fmt"
	"math"
)

const (
	TypeSimple      = "simple"
	TypeTimeseries  = "timeseries_agg"
	TypeRelatedAgg  = "related_agg"
	TypeGeofence    = "geofence"
	TypePropagation = "propagation"
)

// advancedConfig 通用高级类型配置骨架(按 type 各取所需,统一 json 反序列化)。
type AdvancedConfig struct {
	// timeseries_agg / related_agg
	SourceKey     string  `json:"source_key"`
	Func          string  `json:"func"`           // min|max|avg|count|sum
	WindowSeconds int     `json:"window_seconds"` // 时序聚合窗口
	// geofence
	LatKey  string  `json:"lat_key"`
	LngKey  string  `json:"lng_key"`
	Shape   string  `json:"shape"` // circle|polygon
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	RadiusM float64 `json:"radius_m"`
	Points  [][]float64 `json:"points"` // polygon [[lat,lng],...]
	// related_agg / propagation
	DeviceIDs []string `json:"device_ids"` // 显式关联/传播目标(资产树自动发现留集成)
	// propagation
	Direction string `json:"direction"` // up|down(目标解析语义,默认 up)
}

// parseAdvancedConfig 解析并校验高级类型配置;错误在装载期跳过该字段(与服务层保存校验双保险)。
func ParseAdvancedConfig(fieldType string, raw json.RawMessage) (*AdvancedConfig, error) {
	cfg := &AdvancedConfig{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("config is not valid json: %w", err)
		}
	}
	switch fieldType {
	case TypeTimeseries:
		if cfg.SourceKey == "" {
			return nil, fmt.Errorf("timeseries_agg requires source_key")
		}
		switch cfg.Func {
		case "min", "max", "avg", "count", "sum":
		default:
			return nil, fmt.Errorf("timeseries_agg func must be min/max/avg/count/sum")
		}
		if cfg.WindowSeconds <= 0 {
			return nil, fmt.Errorf("timeseries_agg requires positive window_seconds")
		}
		if cfg.WindowSeconds > 86400 {
			return nil, fmt.Errorf("timeseries_agg window_seconds exceeds 86400")
		}
	case TypeRelatedAgg:
		if cfg.SourceKey == "" || cfg.Func == "" {
			return nil, fmt.Errorf("related_agg requires source_key and func")
		}
		switch cfg.Func {
		case "min", "max", "avg", "count", "sum":
		default:
			return nil, fmt.Errorf("related_agg func must be min/max/avg/count/sum")
		}
		if len(cfg.DeviceIDs) == 0 {
			return nil, fmt.Errorf("related_agg requires device_ids (asset-tree auto discovery lands later)")
		}
	case TypeGeofence:
		if cfg.LatKey == "" || cfg.LngKey == "" {
			return nil, fmt.Errorf("geofence requires lat_key and lng_key")
		}
		switch cfg.Shape {
		case "circle":
			if cfg.RadiusM <= 0 {
				return nil, fmt.Errorf("geofence circle requires positive radius_m")
			}
		case "polygon":
			if len(cfg.Points) < 3 {
				return nil, fmt.Errorf("geofence polygon requires >=3 points")
			}
		default:
			return nil, fmt.Errorf("geofence shape must be circle or polygon")
		}
	case TypePropagation:
		if len(cfg.DeviceIDs) == 0 {
			return nil, fmt.Errorf("propagation requires device_ids (asset-tree auto discovery lands later)")
		}
		switch cfg.Direction {
		case "", "up", "down":
		default:
			return nil, fmt.Errorf("propagation direction must be up or down")
		}
	default:
		return nil, fmt.Errorf("unknown field type %q", fieldType)
	}
	return cfg, nil
}

// haversineMeters 两点球面距离(米),地球半径 6371008.8m。
func HaversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371008.8
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(a))
}

// pointInPolygon 射线法判定(点在多边形内部);边界情况按奇偶规则处理。
func PointInPolygon(lat, lng float64, points [][]float64) bool {
	inside := false
	j := len(points) - 1
	for i := 0; i < len(points); i++ {
		latI, lngI := points[i][0], points[i][1]
		latJ, lngJ := points[j][0], points[j][1]
		if (latI > lat) != (latJ > lat) {
			intersectLng := (lngJ-lngI)*(lat-latI)/(latJ-latI) + lngI
			if lng < intersectLng {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// aggregateFloats 聚合函数(min/max/avg/count/sum);空集 count=0,其余 0。
func AggregateFloats(values []float64, fn string, count int) float64 {
	switch fn {
	case "count":
		return float64(count)
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case "min":
		m := math.Inf(1)
		for _, v := range values {
			if v < m {
				m = v
			}
		}
		if math.IsInf(m, 1) {
			return 0
		}
		return m
	case "max":
		m := math.Inf(-1)
		for _, v := range values {
			if v > m {
				m = v
			}
		}
		if math.IsInf(m, -1) {
			return 0
		}
		return m
	default: // avg
		if count == 0 {
			return 0
		}
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(count)
	}
}

// toFloat 宽松数值转换(与引擎 decodeFlatPayload 的数值域一致)。
func ToFloat(raw interface{}) (float64, bool) {
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// ValidateFieldConfig 服务层保存校验入口:按类型校验配置合法性。
// simple(含空):配置可为空;高级类型:config 必须通过 ParseAdvancedConfig。
func ValidateFieldConfig(fieldType string, config json.RawMessage) error {
	switch fieldType {
	case "", TypeSimple:
		return nil
	default:
		_, err := ParseAdvancedConfig(fieldType, config)
		return err
	}
}

// EvaluateGeofence 返回是否在围栏内;坐标缺失返回 (false, false) 表示无法判定。
func EvaluateGeofence(cfg *AdvancedConfig, payload map[string]interface{}) (inside, ok bool) {
	rawLat, existsLat := payload[cfg.LatKey]
	rawLng, existsLng := payload[cfg.LngKey]
	if !existsLat || !existsLng {
		return false, false
	}
	lat, latOK := ToFloat(rawLat)
	lng, lngOK := ToFloat(rawLng)
	if !latOK || !lngOK {
		return false, false
	}
	switch cfg.Shape {
	case "circle":
		return HaversineMeters(lat, lng, cfg.Lat, cfg.Lng) <= cfg.RadiusM, true
	default: // polygon
		return PointInPolygon(lat, lng, cfg.Points), true
	}
}
