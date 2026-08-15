// 文件用途：覆盖遥测存储模块 config types 行为的 Go 测试。
// 核心逻辑：验证配置默认值、数据模型、去重转换或写入批处理的关键契约，主要围绕 func TestDefaultConfigAndFlushDuration、func TestStorageModelTableNames、func TestStorageMessageAndDataPointJSONShape、func TestEventDataKeepsRawPayload 等声明展开。
// 关键注意事项：存储测试需避免误连真实数据库，重点保持数据形状和去重语义稳定。
// 重构建议：后续可增加可替换 writer 和时钟夹具，扩大并发与失败路径覆盖。

package storage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultConfigAndFlushDuration(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ChannelBufferSize != 10000 {
		t.Fatalf("ChannelBufferSize = %d, want 10000", cfg.ChannelBufferSize)
	}
	if cfg.TelemetryBatchSize != 500 {
		t.Fatalf("TelemetryBatchSize = %d, want 500", cfg.TelemetryBatchSize)
	}
	if cfg.GetFlushDuration() != time.Second {
		t.Fatalf("GetFlushDuration = %s, want 1s", cfg.GetFlushDuration())
	}
	if !cfg.EnableMetrics {
		t.Fatal("DefaultConfig should enable metrics")
	}
	if !cfg.TelemetrySpoolEnabled || cfg.TelemetrySpoolDirectory != "./data/telemetry-spool" {
		t.Fatalf("telemetry spool defaults = enabled:%v directory:%q", cfg.TelemetrySpoolEnabled, cfg.TelemetrySpoolDirectory)
	}
	if cfg.TelemetrySpoolMaxBytes != 512*1024*1024 || cfg.TelemetrySpoolMaxRecords != 100000 {
		t.Fatalf("telemetry spool capacity defaults = bytes:%d records:%d", cfg.TelemetrySpoolMaxBytes, cfg.TelemetrySpoolMaxRecords)
	}
	if cfg.TelemetrySpoolMaxRecordBytes != 16*1024*1024 {
		t.Fatalf("telemetry spool max record bytes = %d, want 16 MiB", cfg.TelemetrySpoolMaxRecordBytes)
	}
	if cfg.TelemetrySpoolReplayInterval != 30*time.Second || cfg.TelemetrySpoolReplayBatchSize != 100 || cfg.TelemetrySpoolReplayTimeout != 10*time.Second {
		t.Fatalf(
			"telemetry spool replay defaults = interval:%s batch:%d timeout:%s",
			cfg.TelemetrySpoolReplayInterval,
			cfg.TelemetrySpoolReplayBatchSize,
			cfg.TelemetrySpoolReplayTimeout,
		)
	}
	if !cfg.AttributeEventSpoolEnabled || cfg.AttributeEventSpoolDirectory != "./data/uplink-spool" {
		t.Fatalf(
			"attribute/event spool defaults = enabled:%v directory:%q",
			cfg.AttributeEventSpoolEnabled,
			cfg.AttributeEventSpoolDirectory,
		)
	}
	if cfg.AttributeEventSpoolMaxBytes != 512*1024*1024 || cfg.AttributeEventSpoolMaxRecords != 100000 || cfg.AttributeEventSpoolMaxRecordBytes != 16*1024*1024 {
		t.Fatalf(
			"attribute/event spool capacity defaults = bytes:%d records:%d record_bytes:%d",
			cfg.AttributeEventSpoolMaxBytes,
			cfg.AttributeEventSpoolMaxRecords,
			cfg.AttributeEventSpoolMaxRecordBytes,
		)
	}
	if cfg.AttributeEventSpoolReplayInterval != 30*time.Second || cfg.AttributeEventSpoolReplayBatchSize != 100 || cfg.AttributeEventSpoolReplayTimeout != 10*time.Second {
		t.Fatalf(
			"attribute/event spool replay defaults = interval:%s batch:%d timeout:%s",
			cfg.AttributeEventSpoolReplayInterval,
			cfg.AttributeEventSpoolReplayBatchSize,
			cfg.AttributeEventSpoolReplayTimeout,
		)
	}

	cfg.TelemetryFlushInterval = 0
	if cfg.GetFlushDuration() != 0 {
		t.Fatalf("GetFlushDuration disabled = %s, want 0", cfg.GetFlushDuration())
	}
	cfg.TelemetryFlushInterval = -1
	if cfg.GetFlushDuration() != 0 {
		t.Fatalf("GetFlushDuration negative = %s, want 0", cfg.GetFlushDuration())
	}
	cfg.TelemetryFlushInterval = 250
	if cfg.GetFlushDuration() != 250*time.Millisecond {
		t.Fatalf("GetFlushDuration custom = %s, want 250ms", cfg.GetFlushDuration())
	}
}

func TestStorageModelTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "telemetry history", got: (TelemetryData{}).TableName(), want: "telemetry_datas"},
		{name: "telemetry current", got: (TelemetryCurrentData{}).TableName(), want: "telemetry_current_datas"},
		{name: "attribute", got: (AttributeData{}).TableName(), want: "attribute_datas"},
		{name: "event", got: (EventDataModel{}).TableName(), want: "event_datas"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("TableName = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestStorageMessageAndDataPointJSONShape(t *testing.T) {
	msg := Message{
		DeviceID:  "dev-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1782518400000,
		Data: []TelemetryDataPoint{
			{Key: "temperature", Value: 26.5},
			{Key: "running", Value: true},
		},
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal Message returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal Message returned error: %v", err)
	}

	if decoded["device_id"] != "dev-1" || decoded["tenant_id"] != "tenant-1" || decoded["data_type"] != string(DataTypeTelemetry) {
		t.Fatalf("Message JSON identity fields mismatch: %s", string(raw))
	}
	if decoded["timestamp"].(float64) != 1782518400000 {
		t.Fatalf("Message timestamp mismatch: %s", string(raw))
	}
	data, ok := decoded["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("Message data = %#v, want two telemetry points", decoded["data"])
	}
}

func TestEventDataKeepsRawPayload(t *testing.T) {
	event := EventData{
		Identify: "dry_contact_alarm",
		Data:     json.RawMessage(`{"level":"critical","value":1}`),
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal EventData returned error: %v", err)
	}
	if string(raw) != `{"identify":"dry_contact_alarm","data":{"level":"critical","value":1}}` {
		t.Fatalf("EventData JSON = %s", string(raw))
	}
}
