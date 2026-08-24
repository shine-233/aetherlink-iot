package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestTelemetryWriterStopFlushesRemainingWhenPeriodicFlushIsDisabled(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	config := DefaultConfig()
	config.TelemetryBatchSize = 100
	config.TelemetryFlushInterval = 0
	config.TelemetrySpoolEnabled = false
	config.TelemetryWriteAheadSpoolEnabled = false
	writer := newTelemetryWriter(db, logrus.New(), config, newMetricsCollector())
	writer.start(context.Background())

	if err := writer.write(&Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	}); err != nil {
		t.Fatalf("write returned error: %v", err)
	}

	if err := writer.stop(time.Second); err != nil {
		t.Fatalf("first writer stop returned error: %v", err)
	}
	if err := writer.stop(time.Second); err != nil {
		t.Fatalf("second writer stop returned error: %v", err)
	}

	var count int64
	if err := db.Model(&TelemetryData{}).
		Where("device_id = ? AND key = ? AND ts = ?", "device-1", "temperature", 1000).
		Count(&count).Error; err != nil {
		t.Fatalf("count flushed telemetry: %v", err)
	}
	if count != 1 {
		t.Fatalf("flushed telemetry count = %d, want 1", count)
	}
	if err := writer.write(&Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 2000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 22.5}},
	}); err == nil {
		t.Fatal("write after stop should be rejected")
	}
}

func TestStorageStartFailureCompletesLifecycleSignals(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedPath, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("create blocking spool path: %v", err)
	}
	config := DefaultConfig()
	config.EnableMetrics = false
	config.TelemetrySpoolDirectory = blockedPath
	service := New(nil, logrus.New(), config).(*storage)

	if err := service.Start(context.Background(), make(chan *Message)); err == nil {
		t.Fatal("Start() error = nil, want spool initialization failure")
	}
	select {
	case <-service.doneCh:
	default:
		t.Fatal("storage done signal remains open after Start failure")
	}
	select {
	case <-service.telemetryWriter.doneCh:
	default:
		t.Fatal("telemetry writer done signal remains open after Start failure")
	}
	if err := service.Stop(time.Second); err != nil {
		t.Fatalf("Stop() after Start failure = %v", err)
	}
}

func TestStorageStopDrainsClosedInputChannelBeforeTelemetryWriterFlush(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	config := DefaultConfig()
	config.ChannelBufferSize = 8
	config.TelemetryBatchSize = 100
	config.TelemetryFlushInterval = 0
	config.TelemetrySpoolEnabled = false
	config.TelemetryWriteAheadSpoolEnabled = false
	service := New(db, logrus.New(), config)
	input := make(chan *Message, config.ChannelBufferSize)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := service.Start(ctx, input); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	for i := 1; i <= 3; i++ {
		input <- &Message{
			DeviceID:  "device-1",
			TenantID:  "tenant-1",
			DataType:  DataTypeTelemetry,
			Timestamp: int64(i * 1000),
			Data:      []TelemetryDataPoint{{Key: "temperature", Value: float64(20 + i)}},
		}
	}
	close(input)

	if err := service.Stop(time.Second); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	var count int64
	if err := db.Model(&TelemetryData{}).
		Where("device_id = ? AND key = ?", "device-1", "temperature").
		Count(&count).Error; err != nil {
		t.Fatalf("count drained telemetry: %v", err)
	}
	if count != 3 {
		t.Fatalf("drained telemetry count = %d, want 3", count)
	}
}

func TestStorageContextCancellationDrainsInputAndStopsTelemetryWriter(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	config := DefaultConfig()
	config.ChannelBufferSize = 2
	config.TelemetryBatchSize = 100
	config.TelemetryFlushInterval = 0
	config.TelemetrySpoolEnabled = false
	config.TelemetryWriteAheadSpoolEnabled = false
	service := New(db, logrus.New(), config).(*storage)
	input := make(chan *Message, config.ChannelBufferSize)
	input <- &Message{
		DeviceID:  "device-context-cancel",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.Start(ctx, input); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	cancel()
	waitForChannelClose(t, &service.doneCh, "storage main loop")
	waitForChannelClose(t, &service.telemetryWriter.doneCh, "telemetry writer")

	var count int64
	if err := db.Model(&TelemetryData{}).
		Where("device_id = ? AND key = ?", "device-context-cancel", "temperature").
		Count(&count).Error; err != nil {
		t.Fatalf("count drained telemetry: %v", err)
	}
	if count != 1 {
		t.Fatalf("drained telemetry count = %d, want 1", count)
	}
	if err := service.telemetryWriter.write(&Message{
		DeviceID:  "device-context-cancel",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 2000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 22.5}},
	}); err == nil {
		t.Fatal("writer should reject telemetry after context-driven shutdown")
	}
	if err := service.Stop(time.Second); err != nil {
		t.Fatalf("Stop after context cancellation returned error: %v", err)
	}
}

// waitForChannelClose 以轮询方式等待通道关闭，避免固定 time.After 在 CI 高载下超时。
func waitForChannelClose(t *testing.T, ch *chan struct{}, label string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-*ch:
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatalf("%s did not stop within 5s", label)
}
