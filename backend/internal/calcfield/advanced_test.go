package calcfield

import (
	"context"
	"encoding/json"
	"testing"

	types "aetherlink-iot/backend/internal/calcfield/types"
	"aetherlink-iot/backend/internal/uplink"
)

// PHASE-D-D4 BEGIN 高级类型单测(纯函数+窗口+引擎路由)

func TestParseAdvancedConfigValidationD4(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		config  string
		wantErr bool
	}{
		{"时序聚合合法", FieldTypeTimeseries, `{"source_key":"temp","func":"avg","window_seconds":60}`, false},
		{"时序聚合缺源键", FieldTypeTimeseries, `{"func":"avg","window_seconds":60}`, true},
		{"时序聚合非法函数", FieldTypeTimeseries, `{"source_key":"temp","func":"median","window_seconds":60}`, true},
		{"关联聚合合法", FieldTypeRelatedAgg, `{"source_key":"power","func":"sum","device_ids":["d1","d2"]}`, false},
		{"关联聚合缺目标", FieldTypeRelatedAgg, `{"source_key":"power","func":"sum"}`, true},
		{"圆形围栏合法", FieldTypeGeofence, `{"lat_key":"lat","lng_key":"lng","shape":"circle","lat":31.2,"lng":121.4,"radius_m":500}`, false},
		{"围栏多边形点不足", FieldTypeGeofence, `{"lat_key":"lat","lng_key":"lng","shape":"polygon","points":[[31.1,121.1],[31.2,121.2]]}`, true},
		{"传播合法", FieldTypePropagation, `{"source_key":"temp","device_ids":["d9"],"direction":"up"}`, false},
		{"传播方向非法", FieldTypePropagation, `{"source_key":"temp","device_ids":["d9"],"direction":"sideways"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFieldConfig(tc.typ, []byte(tc.config))
			if tc.wantErr && err == nil {
				t.Fatalf("应报错")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("不应报错: %v", err)
			}
		})
	}
}

func TestHaversineMetersD4(t *testing.T) {
	// 同点 0 距离;上海人民广场到外滩约 3.4km(容差 500m)。
	if d := types.HaversineMeters(31.2304, 121.4737, 31.2304, 121.4737); d > 1 {
		t.Fatalf("同点距离应≈0: %f", d)
	}
	d := types.HaversineMeters(31.2304, 121.4737, 31.2495, 121.4907)
	if d < 2500 || d > 4500 {
		t.Fatalf("已知距离段不符: %f", d)
	}
}

func TestPointInPolygonD4(t *testing.T) {
	triangle := [][]float64{{0, 0}, {0, 10}, {10, 0}}
	if !types.PointInPolygon(1, 1, triangle) {
		t.Fatalf("(1,1) 应在三角形内")
	}
	if types.PointInPolygon(9, 9, triangle) {
		t.Fatalf("(9,9) 应在三角形外")
	}
}

func TestWindowAggregationD4(t *testing.T) {
	store := &windowStore{windows: map[string][]windowSample{}}
	base := int64(1700000000000)
	for i := int64(0); i < 4; i++ {
		store.recordAndAggregate("k", base+i*1000, float64(i+1), 60, "sum")
	}
	// 1+2+3+4 = 10
	if got := store.recordAndAggregate("k", base+4999, 5, 60, "sum"); got != 15 {
		t.Fatalf("sum 窗口应为 15: %v", got)
	}
	// 窗口滑动:60s 窗口在 61s 处保留 ts>=+1s 的样本(2+3+4+5)+新样本 5 = 19。
	if got := store.recordAndAggregate("k", base+61000, 5, 60, "sum"); got != 19 {
		t.Fatalf("窗口滑动后 sum 应为 19: %v", got)
	}
	if got := store.recordAndAggregate("k", base+62000, 5, 60, "count"); got != 5 {
		t.Fatalf("count 应为 5: %v", got)
	}
}

func TestEngineAdvancedRoutingD4(t *testing.T) {
	// 围栏引擎路由:围栏内 emit true。
	rule := FieldRule{
		ID: "f1", OutputKey: "in_zone",
		Type:   FieldTypeGeofence,
		Config: []byte(`{"lat_key":"lat","lng_key":"lng","shape":"circle","lat":31.2,"lng":121.4,"radius_m":500}`),
	}
	rules := compileFieldRules([]FieldRule{rule}, nil)
	if len(rules) != 1 || rules[0].fieldType != FieldTypeGeofence {
		t.Fatalf("高级规则编译失败")
	}
	value, targets, err := evaluateAdvanced(rules[0], map[string]interface{}{"lat": 31.2, "lng": 121.4}, 1700000000000, "dev-1", "tenant-1")
	if err != nil || targets != nil {
		t.Fatalf("围栏求值异常: %v", err)
	}
	if value != true {
		t.Fatalf("围栏内应为 true: %v", value)
	}
}

func TestRecomputeDeterminismD4(t *testing.T) {
	rule := FieldRule{
		ID: "r1", OutputKey: "temp_avg",
		Type:   FieldTypeTimeseries,
		Config: []byte(`{"source_key":"temp","func":"avg","window_seconds":60}`),
	}
	samples := []HistorySample{}
	base := int64(1700000000000)
	for i := int64(0); i < 5; i++ {
		samples = append(samples, HistorySample{TS: base + i*1000, Value: float64(i + 1)})
	}
	source := &stubHistory{samples: map[string][]HistorySample{"temp": samples}}
	runOnce := func() (int64, int64, map[int64]float64) {
		emittedValues := map[int64]float64{}
		sink := &recomputeSinkStub{values: emittedValues}
		_, _, err := RecomputeRange(context.Background(), rule, "dev-1", "tenant-1", base, base+10000, source, sink, nil)
		if err != nil {
			t.Fatalf("重算失败: %v", err)
		}
		return 0, 0, emittedValues
	}
	_, _, first := runOnce()
	_, _, second := runOnce()
	if len(first) != 5 || len(second) != 5 {
		t.Fatalf("应各产出 5 条: %d/%d", len(first), len(second))
	}
	for ts, v := range first {
		if second[ts] != v {
			t.Fatalf("重算非幂等: ts=%d %v vs %v", ts, v, second[ts])
		}
	}
	// avg 窗口第 3 条 = (1+2+3)/3 = 2
	if first[base+2000] != 2 {
		t.Fatalf("滚动 avg 不符: %v", first[base+2000])
	}
}

// ---- 测试桩 ----

type recomputeSinkStub struct {
	values map[int64]float64
}

func (s *recomputeSinkStub) EnqueueDerivedTelemetry(_ context.Context, msg *uplink.DeviceMessage) bool {
	var data map[string]interface{}
	_ = json.Unmarshal(msg.Payload, &data)
	for _, v := range data {
		if f, ok := v.(float64); ok {
			s.values[msg.Timestamp] = f
		}
	}
	return true
}

type stubHistory struct {
	samples map[string][]HistorySample
}

func (s *stubHistory) ListRange(_ context.Context, _tenantID, _deviceID, key string, _from, _to int64) ([]HistorySample, error) {
	return s.samples[key], nil
}

// PHASE-D-D4 END
