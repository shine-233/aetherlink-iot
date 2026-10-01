// 文件用途：遥测上行热路径基准（每条直连设备遥测都会经过 processMessage）。
// 运行：go test ./internal/uplink/ -run '^$' -bench Telemetry -benchmem -count=3
package uplink

import (
	"io"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

var benchTelemetryPayload = []byte(`{"voltage":220.5,"current":3.2,"temperature":26.5,"humidity":61,"online":true,"fw":"1.2.3","pf":0.97,"freq":50.01}`)

// benchTelemetrySink 防止编译器消除副作用参数。
var benchTelemetrySink int

func benchTelemetryUplink(b *testing.B) *TelemetryUplink {
	b.Helper()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	f := NewTelemetryUplink(TelemetryUplinkConfig{Logger: logger, Shards: 1})
	b.Cleanup(f.Stop)
	device := &model.Device{ID: "7f3c2a10-1b2c-4d5e-8f90-123456789abc", TenantID: "tenant-a", IsOnline: 1}
	f.loadDevice = func(string) (*model.Device, error) { return device, nil }
	f.sideEffects = func(_ *model.Device, points []storage.TelemetryDataPoint, triggerParam []string, triggerValues map[string]interface{}, ts int64) {
		benchTelemetrySink += len(points) + len(triggerParam) + len(triggerValues) + int(ts&1)
	}
	return f
}

// BenchmarkTelemetryProcessMessage 测量单条直连遥测从解析到副作用入口的成本。
func BenchmarkTelemetryProcessMessage(b *testing.B) {
	f := benchTelemetryUplink(b)
	msg := &DeviceMessage{
		Type:      "telemetry",
		DeviceID:  "7f3c2a10-1b2c-4d5e-8f90-123456789abc",
		TenantID:  "tenant-a",
		Timestamp: 1700000000000,
		Payload:   benchTelemetryPayload,
		Metadata:  map[string]interface{}{"device_id": "7f3c2a10-1b2c-4d5e-8f90-123456789abc"},
	}
	b.ReportAllocs()
	for b.Loop() {
		f.processMessage(msg)
	}
}

// BenchmarkTelemetryConvertPoints 只测 map -> 存储点/触发参数的转换。
func BenchmarkTelemetryConvertPoints(b *testing.B) {
	data := map[string]interface{}{
		"voltage": 220.5, "current": 3.2, "temperature": 26.5, "humidity": 61.0,
		"online": true, "fw": "1.2.3", "pf": 0.97, "freq": 50.01,
	}
	b.ReportAllocs()
	for b.Loop() {
		points, params, values := convertTelemetryMapToPoints(normalizeLegacyRDITelemetryAliases(data))
		benchTelemetrySink += len(points) + len(params) + len(values)
	}
}
