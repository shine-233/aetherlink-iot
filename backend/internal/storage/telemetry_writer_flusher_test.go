// telemetry_writer_flusher_test.go locks the dedicated-flusher pipeline:
// serialized doFlush, bounded buffer backpressure, deterministic upsert order,
// single conversion per item and one failure log line per fallback chunk.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func testFlusherConfig(batchSize int) Config {
	config := DefaultConfig()
	config.TelemetryBatchSize = batchSize
	config.TelemetryFlushInterval = 5
	config.TelemetrySpoolEnabled = false
	config.TelemetryWriteAheadSpoolEnabled = false
	return config
}

func testFlusherMessage(device string, ts int64, keys ...string) *Message {
	points := make([]TelemetryDataPoint, 0, len(keys))
	for i, key := range keys {
		points = append(points, TelemetryDataPoint{Key: key, Value: float64(i)})
	}
	return &Message{DeviceID: device, TenantID: "tenant-1", DataType: DataTypeTelemetry, Timestamp: ts, Data: points}
}

// Many producers writing while the ticker and threshold both request flushes
// must never run two doFlush calls at once, and must lose nothing.
func TestTelemetryWriterFlushesAreSerializedUnderConcurrentWrites(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	writer := newTelemetryWriter(db, logrus.New(), testFlusherConfig(7), newMetricsCollector(true))
	var inFlight, maxInFlight, flushes atomic.Int64
	writer.flushHook = func(entering bool) {
		if !entering {
			inFlight.Add(-1)
			return
		}
		flushes.Add(1)
		n := inFlight.Add(1)
		for {
			old := maxInFlight.Load()
			if n <= old || maxInFlight.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond) // widen the overlap window
	}
	if err := writer.start(context.Background()); err != nil {
		t.Fatalf("start writer: %v", err)
	}

	const producers, perProducer = 4, 60
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				if err := writer.write(testFlusherMessage(fmt.Sprintf("device-%d", p), int64(1000+i), "temperature")); err != nil {
					t.Errorf("write: %v", err)
					return
				}
			}
		}(p)
	}
	wg.Wait()
	if err := writer.stop(5 * time.Second); err != nil {
		t.Fatalf("stop writer: %v", err)
	}

	if got := maxInFlight.Load(); got != 1 {
		t.Fatalf("max concurrent doFlush = %d, want 1", got)
	}
	if flushes.Load() < 2 {
		t.Fatalf("flushes = %d, want threshold/ticker flushes to have happened", flushes.Load())
	}
	var count int64
	if err := db.Model(&TelemetryData{}).Count(&count).Error; err != nil {
		t.Fatalf("count history: %v", err)
	}
	if count != producers*perProducer {
		t.Fatalf("history rows = %d, want %d", count, producers*perProducer)
	}
	var current []TelemetryCurrentData
	if err := db.Order("device_id").Find(&current).Error; err != nil {
		t.Fatalf("load current: %v", err)
	}
	if len(current) != producers {
		t.Fatalf("current rows = %d, want %d", len(current), producers)
	}
	for _, row := range current {
		if !row.TS.Equal(time.UnixMilli(1000 + perProducer - 1)) {
			t.Fatalf("current %s ts = %s, want newest", row.DeviceID, row.TS)
		}
	}
}

// write must not run the DB transaction on the caller's goroutine, and must
// block only once the buffer reaches the high-water mark.
func TestTelemetryWriterWriteAppliesHighWaterBackpressure(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	config := testFlusherConfig(2)
	config.TelemetryFlushInterval = 0
	writer := newTelemetryWriter(db, logrus.New(), config, newMetricsCollector(false))
	release := make(chan struct{})
	entered := make(chan struct{}, 16)
	writer.flushHook = func(entering bool) {
		if entering {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
		}
	}
	if err := writer.start(context.Background()); err != nil {
		t.Fatalf("start writer: %v", err)
	}

	// First batch reaches the threshold; the flusher takes it and stalls.
	for i := 0; i < 2; i++ {
		if err := writer.write(testFlusherMessage("device-1", int64(i), "k")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("flusher never started")
	}

	// While the flush is stalled, writes up to the high-water mark return
	// immediately: no inline flush on the producer goroutine.
	for i := 0; i < writer.highWaterMark(); i++ {
		done := make(chan error, 1)
		go func(i int) { done <- writer.write(testFlusherMessage("device-1", int64(100+i), "k")) }(i)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("write below high-water: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("write %d blocked below the high-water mark", i)
		}
	}

	blocked := make(chan error, 1)
	go func() { blocked <- writer.write(testFlusherMessage("device-1", 999, "k")) }()
	select {
	case err := <-blocked:
		t.Fatalf("write above high-water returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release) // flusher drains; producer resumes
	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("blocked write: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("producer stayed blocked after the flusher drained the buffer")
	}
	if err := writer.stop(5 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}
	var count int64
	db.Model(&TelemetryData{}).Count(&count)
	if want := int64(2 + writer.highWaterMark() + 1); count != want {
		t.Fatalf("history rows = %d, want %d", count, want)
	}
}

// A producer parked at the high-water mark must be released by stop.
func TestTelemetryWriterStopReleasesBlockedProducer(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	config := testFlusherConfig(1)
	config.TelemetryFlushInterval = 0
	writer := newTelemetryWriter(db, logrus.New(), config, newMetricsCollector(false))
	release := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	writer.flushHook = func(entering bool) {
		if entering {
			once.Do(func() { close(entered); <-release })
		}
	}
	if err := writer.start(context.Background()); err != nil {
		t.Fatalf("start writer: %v", err)
	}
	// 1 item taken by the stalled flusher, then fill to the high-water mark.
	for i := 0; i <= writer.highWaterMark(); i++ {
		if err := writer.write(testFlusherMessage("device-1", int64(i), "k")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if i == 0 {
			<-entered // the flusher took item 0 and is stalled
		}
	}
	blocked := make(chan error, 1)
	go func() { blocked <- writer.write(testFlusherMessage("device-1", 999, "k")) }()
	time.Sleep(20 * time.Millisecond)
	writer.requestStop()
	select {
	case err := <-blocked:
		if err == nil || !strings.Contains(err.Error(), "stopped") {
			t.Fatalf("blocked write after stop = %v, want stopped error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not release the blocked producer")
	}
	close(release)
	if err := writer.stop(5 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestTelemetryWriterDeduplicateAndConvertSortsForDeterministicLockOrder(t *testing.T) {
	writer := newTelemetryWriter(nil, nil, testFlusherConfig(100), nil)
	batch := []*telemetryBatchItem{}
	for _, msg := range []*Message{
		testFlusherMessage("device-b", 3000, "z", "a"),
		testFlusherMessage("device-a", 2000, "m", "c"),
		testFlusherMessage("device-b", 1000, "a", "z"),
		testFlusherMessage("device-a", 1000, "c"),
	} {
		item, err := telemetryBatchItemFromMessage(msg)
		if err != nil {
			t.Fatalf("item: %v", err)
		}
		batch = append(batch, item)
	}
	history, current, duplicates := writer.deduplicateAndConvert(batch)
	if duplicates != 0 {
		t.Fatalf("duplicates = %d, want 0", duplicates)
	}
	var gotHistory []string
	for _, row := range history {
		gotHistory = append(gotHistory, fmt.Sprintf("%s/%s/%d", row.DeviceID, row.Key, row.TS))
	}
	wantHistory := "device-a/c/1000 device-a/c/2000 device-a/m/2000 device-b/a/1000 device-b/a/3000 device-b/z/1000 device-b/z/3000"
	if strings.Join(gotHistory, " ") != wantHistory {
		t.Fatalf("history order = %v, want %s", gotHistory, wantHistory)
	}
	var gotCurrent []string
	for _, row := range current {
		gotCurrent = append(gotCurrent, fmt.Sprintf("%s/%s/%d", row.DeviceID, row.Key, row.TS.UnixMilli()))
	}
	wantCurrent := "device-a/c/2000 device-a/m/2000 device-b/a/3000 device-b/z/3000"
	if strings.Join(gotCurrent, " ") != wantCurrent {
		t.Fatalf("current order = %v, want %s", gotCurrent, wantCurrent)
	}
}

// Write-ahead, flush and dedup must share one conversion per item.
func TestTelemetryWriterConvertsEachItemOnce(t *testing.T) {
	var conversions atomic.Int64
	telemetryConversionHook = func(*telemetryBatchItem) { conversions.Add(1) }
	t.Cleanup(func() { telemetryConversionHook = nil })

	writer := testWriteAheadWriter(t, true, 10)
	writer.db = setupTelemetryCurrentUpsertTestDB(t)
	for i := 0; i < 3; i++ {
		if err := writer.write(testFlusherMessage(fmt.Sprintf("device-%d", i), 1000, "temperature", "humidity")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if usage := writer.spool.usage(); usage.Records != 6 {
		t.Fatalf("write-ahead records = %d, want 6", usage.Records)
	}
	writer.flush()

	if got := conversions.Load(); got != 3 {
		t.Fatalf("conversions = %d, want exactly one per item (3)", got)
	}
	var count int64
	writer.db.Model(&TelemetryData{}).Count(&count)
	if count != 6 {
		t.Fatalf("history rows = %d, want 6", count)
	}
	if usage := writer.spool.usage(); usage.Records != 0 {
		t.Fatalf("write-ahead records after confirmed flush = %d, want 0", usage.Records)
	}
}

// A failing chunk logs a single summary line, not one JSON preview per row.
func TestTelemetryFallbackSingleRowsLogsOncePerChunk(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	if err := db.Migrator().DropTable(&TelemetryCurrentData{}); err != nil {
		t.Fatalf("drop current table: %v", err)
	}
	var out bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&out)
	writer := newTelemetryWriter(db, logger, testFlusherConfig(100), newMetricsCollector(false))

	item, _ := telemetryBatchItemFromMessage(testFlusherMessage("device-1", 1000, "a", "b", "c"))
	history, current, _ := writer.deduplicateAndConvert([]*telemetryBatchItem{item})
	written, failed := writer.fallbackInsertSingleRows(history, buildTelemetryCurrentLookup(current))
	if written != 0 || failed != 3 {
		t.Fatalf("written=%d failed=%d, want 0/3", written, failed)
	}
	if got := strings.Count(out.String(), "single insert failed"); got != 1 {
		t.Fatalf("single insert failure log lines = %d, want 1:\n%s", got, out.String())
	}
	if !strings.Contains(out.String(), "total=3") {
		t.Fatalf("summary should carry the failed count:\n%s", out.String())
	}
}
