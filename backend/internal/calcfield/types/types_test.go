// 文件用途：calcfield/types 叶子包纯函数的单元测试（此前该包零测试，但承载运行时
// 地理围栏判定、时序/关联聚合与保存校验，属高风险未覆盖路径）。
package types

import (
	"math"
	"testing"
)

func TestHaversineMeters(t *testing.T) {
	if d := HaversineMeters(31.2304, 121.4737, 31.2304, 121.4737); d != 0 {
		t.Fatalf("same point distance = %v, want 0", d)
	}
	// 赤道上经度差 1 度 ≈ 2πR/360 ≈ 111195.08m
	d := HaversineMeters(0, 0, 0, 1)
	if math.Abs(d-111195.08) > 1 {
		t.Fatalf("1 degree on equator = %v, want ~111195", d)
	}
	if HaversineMeters(10, 20, 30, 40) != HaversineMeters(30, 40, 10, 20) {
		t.Fatal("distance must be symmetric")
	}
}

func TestPointInPolygon(t *testing.T) {
	square := [][]float64{{0, 0}, {0, 10}, {10, 10}, {10, 0}}
	cases := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"center", 5, 5, true},
		{"outside east", 5, 15, false},
		{"outside south", -1, 5, false},
		{"near corner inside", 0.1, 0.1, true},
	}
	for _, tc := range cases {
		if got := PointInPolygon(tc.lat, tc.lng, square); got != tc.want {
			t.Errorf("%s: PointInPolygon(%v,%v)=%v want %v", tc.name, tc.lat, tc.lng, got, tc.want)
		}
	}
	// 凹多边形（U 形）：缺口处应判为外部
	u := [][]float64{{0, 0}, {0, 10}, {10, 10}, {10, 7}, {3, 7}, {3, 3}, {10, 3}, {10, 0}}
	if PointInPolygon(5, 5, u) {
		t.Error("point in U-notch must be outside")
	}
	if !PointInPolygon(1, 5, u) {
		t.Error("point in U-base must be inside")
	}
	if PointInPolygon(1, 1, nil) {
		t.Error("empty polygon must contain nothing")
	}
}

func TestAggregateFloats(t *testing.T) {
	vals := []float64{3, -1, 7, 1}
	cases := map[string]float64{"sum": 10, "min": -1, "max": 7, "count": 4, "avg": 2.5, "": 2.5}
	for fn, want := range cases {
		if got := AggregateFloats(vals, fn, len(vals)); got != want {
			t.Errorf("AggregateFloats(%q)=%v want %v", fn, got, want)
		}
	}
	for _, fn := range []string{"sum", "min", "max", "count", "avg"} {
		if got := AggregateFloats(nil, fn, 0); got != 0 {
			t.Errorf("empty %s = %v, want 0", fn, got)
		}
	}
}

func TestToFloat(t *testing.T) {
	ok := map[string]struct {
		in   interface{}
		want float64
	}{
		"float64": {2.5, 2.5}, "int": {3, 3}, "int64": {int64(-4), -4}, "true": {true, 1}, "false": {false, 0},
	}
	for name, tc := range ok {
		got, valid := ToFloat(tc.in)
		if !valid || got != tc.want {
			t.Errorf("%s: ToFloat(%v)=(%v,%v)", name, tc.in, got, valid)
		}
	}
	for _, bad := range []interface{}{"1", nil, []int{1}, float32(1)} {
		if _, valid := ToFloat(bad); valid {
			t.Errorf("ToFloat(%#v) must be rejected", bad)
		}
	}
}

func TestEvaluateGeofence(t *testing.T) {
	circle := &AdvancedConfig{Shape: "circle", LatKey: "lat", LngKey: "lng", Lat: 0, Lng: 0, RadiusM: 1000}
	if inside, ok := EvaluateGeofence(circle, map[string]interface{}{"lat": 0.0, "lng": 0.005}); !ok || !inside {
		t.Fatalf("~556m from center must be inside 1km circle, got inside=%v ok=%v", inside, ok)
	}
	if inside, ok := EvaluateGeofence(circle, map[string]interface{}{"lat": 0.0, "lng": 0.02}); !ok || inside {
		t.Fatalf("~2.2km from center must be outside, got inside=%v ok=%v", inside, ok)
	}
	poly := &AdvancedConfig{Shape: "polygon", LatKey: "y", LngKey: "x", Points: [][]float64{{0, 0}, {0, 10}, {10, 10}, {10, 0}}}
	if inside, ok := EvaluateGeofence(poly, map[string]interface{}{"y": 5, "x": int64(5)}); !ok || !inside {
		t.Fatalf("int coords inside polygon: inside=%v ok=%v", inside, ok)
	}
	// 坐标缺失 / 非数值：无法判定（fail-closed，不得误报为"在围栏外"）
	for _, p := range []map[string]interface{}{{"lat": 1.0}, {"lng": 1.0}, {"lat": "1", "lng": 1.0}, {}} {
		if inside, ok := EvaluateGeofence(circle, p); ok || inside {
			t.Errorf("payload %v must be undecidable, got inside=%v ok=%v", p, inside, ok)
		}
	}
}

func TestValidateFieldConfig(t *testing.T) {
	for _, ft := range []string{"", TypeSimple} {
		if err := ValidateFieldConfig(ft, nil); err != nil {
			t.Errorf("simple type %q must accept empty config: %v", ft, err)
		}
	}
	if err := ValidateFieldConfig("no-such-type", []byte(`{}`)); err == nil {
		t.Error("unknown field type must be rejected")
	}
	if err := ValidateFieldConfig(TypeGeofence, []byte(`{not json`)); err == nil {
		t.Error("malformed geofence config must be rejected")
	}
}
