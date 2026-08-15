// telemetry_write_ahead_test.go locks the pre-buffer durability contract.
//
// The in-memory batch buffer has no on-disk trace, so a SIGKILL between
// enqueue and flush loses up to TelemetryBatchSize points. The write-ahead
// receipt closes that window: a record lands in the existing file spool
// before the point enters the buffer, and is retired only after the primary
// database write is confirmed. These tests assert both halves, plus the
// fail-closed behaviour that prevents an unspooled point entering memory.
package storage

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestTelemetryWriteAheadStartFailsWhenTelemetrySpoolIsDisabled(t *testing.T) {
	config := DefaultConfig()
	config.TelemetryWriteAheadSpoolEnabled = true
	config.TelemetrySpoolEnabled = false
	writer := newTelemetryWriter(nil, logrus.New(), config, newMetricsCollector(false))

	if err := writer.start(context.Background()); err == nil {
		t.Fatal("writer start succeeded with write-ahead enabled and telemetry spool disabled")
	}
}

func testWriteAheadWriter(t *testing.T, enabled bool, maxRecords int) *telemetryWriter {
	t.Helper()
	config := DefaultConfig()
	config.TelemetryWriteAheadSpoolEnabled = enabled
	config.TelemetryBatchSize = 100
	return &telemetryWriter{
		logger:  logrus.New(),
		config:  config,
		metrics: newMetricsCollector(false),
		spool:   testTelemetryFileSpool(t, 1024*1024, maxRecords),
		buffer:  make([]*telemetryBatchItem, 0, config.TelemetryBatchSize),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

func testWriteAheadMessage(deviceID string, ts int64, value float64) *Message {
	return &Message{
		DeviceID:  deviceID,
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: ts,
		Data: []TelemetryDataPoint{
			{Key: "temperature", Value: value},
		},
	}
}

// A point must be durable on disk before it is visible in the buffer, so a
// crash before flush leaves recoverable evidence.
func TestTelemetryWriteAheadPersistsReceiptBeforeBuffering(t *testing.T) {
	writer := testWriteAheadWriter(t, true, 10)

	if err := writer.write(testWriteAheadMessage("device-1", 1000, 21.5)); err != nil {
		t.Fatalf("write telemetry: %v", err)
	}

	usage := writer.spool.usage()
	if usage.Records != 1 {
		t.Fatalf("spool usage after buffered write = %#v, want one write-ahead record", usage)
	}

	writer.bufferMu.Lock()
	buffered := len(writer.buffer)
	receipts := 0
	if buffered == 1 {
		receipts = len(writer.buffer[0].writeAhead)
	}
	writer.bufferMu.Unlock()

	if buffered != 1 {
		t.Fatalf("buffered items = %d, want 1", buffered)
	}
	if receipts != 1 {
		t.Fatalf("write-ahead receipts on buffered item = %d, want 1", receipts)
	}

	// Simulate a crash: the process dies without ever flushing. Nothing
	// removes the receipt, so the record must still be on disk and countable
	// by the existing replay accounting.
	if usage := writer.spool.usage(); usage.Records != 1 || usage.Bytes < 1 {
		t.Fatalf("usage after simulated crash = %#v, want the receipt retained", usage)
	}
}

// Receipts must be retired only after the primary write is confirmed,
// otherwise a successful flush would leave the spool growing forever.
func TestTelemetryWriteAheadReceiptReleasedAfterConfirmedWrite(t *testing.T) {
	writer := testWriteAheadWriter(t, true, 10)
	item, err := telemetryBatchItemFromMessage(testWriteAheadMessage("device-1", 1000, 21.5))
	if err != nil {
		t.Fatalf("build batch item: %v", err)
	}
	item.writeAhead, err = writer.storeWriteAheadReceipts(item)
	if err != nil {
		t.Fatalf("store write-ahead receipt: %v", err)
	}
	if len(item.writeAhead) != 1 {
		t.Fatalf("write-ahead receipts = %d, want 1", len(item.writeAhead))
	}
	if usage := writer.spool.usage(); usage.Records != 1 {
		t.Fatalf("usage before release = %#v, want one record", usage)
	}
	if metrics := writer.metrics.GetMetrics(); metrics.TelemetrySpoolBacklog != 1 {
		t.Fatalf("backlog metric before release = %d, want 1", metrics.TelemetrySpoolBacklog)
	}

	writer.releaseWriteAheadReceipts([]*telemetryBatchItem{item})

	if usage := writer.spool.usage(); usage.Records != 0 {
		t.Fatalf("usage after release = %#v, want the receipt retired", usage)
	}
	if metrics := writer.metrics.GetMetrics(); metrics.TelemetrySpoolBacklog != 0 {
		t.Fatalf("backlog metric after release = %d, want 0", metrics.TelemetrySpoolBacklog)
	}
	if len(item.writeAhead) != 0 {
		t.Fatalf("receipts still attached after release = %d, want 0", len(item.writeAhead))
	}

	// Releasing twice must stay quiet: replay may already have removed the
	// same deterministic identity.
	writer.releaseWriteAheadReceipts([]*telemetryBatchItem{item})
}

// The feature is enabled by default; operators can explicitly opt out when
// accepting the producer-to-consumer durability gap is intentional.
func TestTelemetryWriteAheadEnabledByDefaultAndSupportsExplicitOptOut(t *testing.T) {
	if !DefaultConfig().TelemetryWriteAheadSpoolEnabled {
		t.Fatal("write-ahead spool must be enabled by default")
	}

	writer := testWriteAheadWriter(t, false, 10)
	if err := writer.write(testWriteAheadMessage("device-1", 1000, 21.5)); err != nil {
		t.Fatalf("write telemetry: %v", err)
	}
	if usage := writer.spool.usage(); usage.Records != 0 {
		t.Fatalf("disabled write-ahead still wrote receipts: %#v", usage)
	}
}

// A saturated spool must reject live telemetry before it enters the buffer.
func TestTelemetryWriteAheadFailsClosedWhenSpoolCannotAccept(t *testing.T) {
	writer := testWriteAheadWriter(t, true, 1)

	if err := writer.write(testWriteAheadMessage("device-1", 1000, 21.5)); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Second distinct point exceeds the one-record capacity.
	if err := writer.write(testWriteAheadMessage("device-2", 2000, 22.5)); err == nil {
		t.Fatal("second write must fail when the write-ahead spool is full")
	}

	writer.bufferMu.Lock()
	buffered := len(writer.buffer)
	secondReceipts := 0
	if buffered == 2 {
		secondReceipts = len(writer.buffer[1].writeAhead)
	}
	writer.bufferMu.Unlock()

	if buffered != 1 {
		t.Fatalf("buffered items = %d, want only the durably prepared point", buffered)
	}
	if secondReceipts != 0 {
		t.Fatalf("second item receipts = %d, want no buffered second point", secondReceipts)
	}
	if usage := writer.spool.usage(); usage.Records != 1 {
		t.Fatalf("capacity refusal changed committed usage: %#v", usage)
	}
}
