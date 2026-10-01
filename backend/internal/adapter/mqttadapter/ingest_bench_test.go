// 文件用途：MQTT 上行接入热路径基准（每条设备消息都会经过）。
// 运行：go test ./internal/adapter/mqttadapter/ -run '^$' -bench . -benchmem -count=3
package mqttadapter

import (
	"io"
	"testing"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/sirupsen/logrus"
)

// benchWirePayload 是典型直连设备遥测：values 为 base64 编码的 JSON 对象。
var benchWirePayload = []byte(`{"device_id":"7f3c2a10-1b2c-4d5e-8f90-123456789abc","values":"eyJ2b2x0YWdlIjoyMjAuNSwiY3VycmVudCI6My4yLCJ0ZW1wZXJhdHVyZSI6MjYuNSwiaHVtaWRpdHkiOjYxLCJvbmxpbmUiOnRydWUsImZ3IjoiMS4yLjMiLCJwZiI6MC45NywiZnJlcSI6NTAuMDF9"}`)

func benchAdapter() *Adapter {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.WarnLevel)
	return NewAdapter(nil, nil, logger)
}

func BenchmarkVerifyPayload(b *testing.B) {
	a := benchAdapter()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := a.verifyPayload(benchWirePayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMQTTUplinkSourceID(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if mqttUplinkSourceID("tenant-a", "7f3c2a10-1b2c-4d5e-8f90-123456789abc", "attribute", "msg-000123") == "" {
			b.Fatal("empty source id")
		}
	}
}

func BenchmarkParseAttributeOrEventTopic(b *testing.B) {
	a := benchAdapter()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := a.parseAttributeOrEventTopic("devices/attributes/msg-000123"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBusPublishUplinkMessage 测量 adapter 构造的 UplinkMessage 进入 Bus 的成本
// （包含消费端 drain，不含下游业务）。
func BenchmarkBusPublishUplinkMessage(b *testing.B) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	bus := uplink.NewBus(uplink.BusConfig{BufferSize: 1024}, logger)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range bus.SubscribeTelemetry() {
		}
	}()
	payload := []byte(`{"voltage":220.5,"current":3.2,"temperature":26.5,"humidity":61,"online":true,"fw":"1.2.3","pf":0.97,"freq":50.01}`)
	b.ReportAllocs()
	for b.Loop() {
		msg := &UplinkMessage{
			Type:      "telemetry",
			DeviceID:  "7f3c2a10-1b2c-4d5e-8f90-123456789abc",
			TenantID:  "tenant-a",
			Timestamp: 1700000000000,
			Payload:   payload,
			Metadata: map[string]interface{}{
				"device_id":       "7f3c2a10-1b2c-4d5e-8f90-123456789abc",
				"topic":           "devices/telemetry",
				"source_protocol": "mqtt",
			},
		}
		if err := bus.Publish(msg); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	bus.Close()
	<-done
}
