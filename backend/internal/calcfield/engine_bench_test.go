// 文件用途：计算字段引擎热路径基准——每条遥测消息都会经过 processMessage。
// 运行：go test ./internal/calcfield/ -run '^$' -bench . -benchmem -count=3
package calcfield

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/sirupsen/logrus"
)

type discardStorage struct{ n int }

func (d *discardStorage) EnqueueDerivedTelemetry(_ context.Context, _ *uplink.DeviceMessage) bool {
	d.n++
	return true
}

func newBenchEngine(fields []FieldRule) (*Engine, *discardStorage) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	storage := &discardStorage{}
	engine := NewEngine(nil, storage, logger)
	engine.templateSource = stubSource{templateID: "tpl-1", fields: fields}
	return engine, storage
}

var benchTelemetryPayload = []byte(`{"voltage":220.5,"current":3.2,"temperature":26.5,"humidity":61,"online":true,"fw":"1.2.3","pf":0.97,"freq":50.01}`)

func benchMessage() *uplink.DeviceMessage {
	return &uplink.DeviceMessage{
		Type:      uplink.MessageTypeTelemetry,
		DeviceID:  "dev-1",
		TenantID:  "tenant-a",
		Timestamp: 1700000000000,
		Payload:   benchTelemetryPayload,
	}
}

// 典型场景：模板挂了 3 条 simple 规则。
func BenchmarkProcessMessageThreeRules(b *testing.B) {
	engine, _ := newBenchEngine([]FieldRule{
		{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
		{ID: "f-2", OutputKey: "apparent_va", Expression: "voltage * current / pf"},
		{ID: "f-3", OutputKey: "hot", Expression: "temperature > 30 && humidity > 50"},
	})
	msg := benchMessage()
	engine.processMessage(msg) // 预热缓存
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.processMessage(msg)
	}
}

// 最常见场景：设备模板没有任何计算字段——这条路径应接近零开销。
func BenchmarkProcessMessageNoRules(b *testing.B) {
	engine, _ := newBenchEngine(nil)
	msg := benchMessage()
	engine.processMessage(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.processMessage(msg)
	}
}

func BenchmarkEvaluateRule(b *testing.B) {
	rules := compileFieldRules([]FieldRule{{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current / pf"}}, nil)
	payload := decodeFlatPayload(benchTelemetryPayload)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := evaluateRule(rules[0], payload); !ok {
			b.Fatal("evaluate failed")
		}
	}
}

func BenchmarkDecodeFlatPayload(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(decodeFlatPayload(benchTelemetryPayload)) == 0 {
			b.Fatal("empty")
		}
	}
}
