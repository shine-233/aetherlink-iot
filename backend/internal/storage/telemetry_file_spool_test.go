// telemetry_file_spool_test.go locks the filesystem spool's atomicity,
// capacity, corruption-quarantine, and replay deletion contracts.
package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTelemetryFileSpoolStoreIsAtomicBoundedAndIdempotent(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 1)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)

	first, err := spool.store(history, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("store first record: %v", err)
	}
	if !first.Stored || first.Duplicate || first.Corrupt != 0 || first.Quarantined != 0 {
		t.Fatalf("first store result = %#v, want one new record", first)
	}
	duplicate, err := spool.store(history, time.Unix(2, 0))
	if err != nil {
		t.Fatalf("store duplicate record: %v", err)
	}
	if duplicate.Stored || !duplicate.Duplicate || duplicate.Corrupt != 0 || duplicate.Quarantined != 0 {
		t.Fatalf("duplicate store result = %#v, want healthy duplicate", duplicate)
	}
	if usage := spool.usage(); usage.Records != 1 || usage.Bytes < 1 {
		t.Fatalf("usage after duplicate = %#v, want one finalized record", usage)
	}

	second := testTelemetrySpoolHistory("device-1", "humidity", 1000, 50)
	_, err = spool.store(second, time.Unix(3, 0))
	if err == nil || !strings.Contains(err.Error(), "capacity exhausted") {
		t.Fatalf("store over capacity error = %v", err)
	}
	if usage := spool.usage(); usage.Records != 1 {
		t.Fatalf("capacity failure evicted committed record, usage=%#v", usage)
	}

	entries, err := os.ReadDir(spool.directory)
	if err != nil {
		t.Fatalf("read spool directory: %v", err)
	}
	for _, entry := range entries {
		if isTelemetryFileSpoolTemp(entry.Name()) {
			t.Fatalf("committed store left temp file %q", entry.Name())
		}
	}
}

func TestTelemetryFileSpoolStoreNeverTreatsCorruptDuplicateAsDurable(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record: %v", err)
	}
	path := filepath.Join(spool.directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	if err := os.WriteFile(path, []byte(`{"version":1,"checksum":"tampered"}`), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}

	replacement, err := spool.store(history, time.Unix(2, 0))
	if err != nil {
		t.Fatalf("replace corrupt duplicate: %v", err)
	}
	if !replacement.Stored || replacement.Duplicate || replacement.Corrupt != 1 || replacement.Quarantined != 1 {
		t.Fatalf("corrupt replacement store result = %#v", replacement)
	}
	if _, err := readTelemetryFileSpoolRecord(path, telemetryFileSpoolReadSafetyLimit, true); err != nil {
		t.Fatalf("replacement record is not replayable: %v", err)
	}
	quarantined, err := filepath.Glob(path + telemetryFileSpoolCorruptSuffix + "*")
	if err != nil || len(quarantined) != 1 {
		t.Fatalf("quarantined records = %#v, err=%v", quarantined, err)
	}
	finalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat replacement record: %v", err)
	}
	quarantineInfo, err := os.Stat(quarantined[0])
	if err != nil {
		t.Fatalf("stat quarantine record: %v", err)
	}
	if usage := spool.usage(); usage.Records != 2 || usage.Bytes != finalInfo.Size()+quarantineInfo.Size() || usage.QuarantinedRecords != 1 || usage.QuarantinedBytes != quarantineInfo.Size() {
		t.Fatalf("replacement/quarantine usage = %#v, want one replayable and one quarantined file", usage)
	}
}

func TestTelemetryFileSpoolStoreReportsQuarantineWhenReplacementExceedsCapacity(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 1)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record: %v", err)
	}
	path := filepath.Join(spool.directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	if err := os.WriteFile(path, []byte(`{"version":1,"checksum":"tampered"}`), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}

	result, err := spool.store(history, time.Unix(2, 0))
	if err == nil || !strings.Contains(err.Error(), "capacity exhausted") {
		t.Fatalf("corrupt replacement error = %v", err)
	}
	if result.Stored || result.Duplicate || result.Corrupt != 1 || result.Quarantined != 1 {
		t.Fatalf("corrupt capacity result = %#v, want reported quarantine", result)
	}
	usage := spool.usage()
	if usage.Records != 1 || usage.QuarantinedRecords != 1 || usage.QuarantinedBytes < 1 {
		t.Fatalf("corrupt capacity usage = %#v", usage)
	}
}

func TestTelemetryFileSpoolReplayDeletesOnlyAfterSuccess(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record: %v", err)
	}

	databaseUnavailable := errors.New("database unavailable")
	failed, err := spool.replay(context.Background(), 10, func(context.Context, TelemetryData) error {
		return databaseUnavailable
	})
	if !errors.Is(err, databaseUnavailable) {
		t.Fatalf("failed replay error = %v", err)
	}
	if failed.Replayed != 0 || failed.Usage.Records != 1 {
		t.Fatalf("failed replay removed record: %#v", failed)
	}

	var replayed []TelemetryData
	succeeded, err := spool.replay(context.Background(), 10, func(_ context.Context, row TelemetryData) error {
		replayed = append(replayed, row)
		return nil
	})
	if err != nil {
		t.Fatalf("successful replay: %v", err)
	}
	if succeeded.Replayed != 1 || succeeded.Usage.Records != 0 || len(replayed) != 1 {
		t.Fatalf("successful replay result=%#v rows=%#v", succeeded, replayed)
	}
	if replayed[0].DeviceID != history.DeviceID || replayed[0].Key != history.Key || replayed[0].TS != history.TS {
		t.Fatalf("replayed identity = %#v, want %#v", replayed[0], history)
	}
}

func TestTelemetryFileSpoolReplayReadsRecordsAboveLoweredWriteLimit(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	original := &telemetryFileSpool{
		directory:      directory,
		maxBytes:       1024 * 1024,
		maxRecords:     10,
		maxRecordBytes: 1024 * 1024,
	}
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := original.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record under original limit: %v", err)
	}

	lowered := &telemetryFileSpool{
		directory:      directory,
		maxBytes:       1024 * 1024,
		maxRecords:     10,
		maxRecordBytes: 64,
	}
	var replayed int
	result, err := lowered.replay(context.Background(), 1, func(context.Context, TelemetryData) error {
		replayed++
		return nil
	})
	if err != nil || result.Replayed != 1 || replayed != 1 {
		t.Fatalf("replay after lowering write limit result=%#v replayed=%d err=%v", result, replayed, err)
	}
}

func TestTelemetryFileSpoolCorruptionIsQuarantinedWithoutStarvingHealthyRows(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record: %v", err)
	}
	path := filepath.Join(spool.directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	if err := os.WriteFile(path, []byte(`{"version":1,"checksum":"tampered"}`), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}
	if err := os.Chtimes(path, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatalf("age corrupt record: %v", err)
	}
	healthy := testTelemetrySpoolHistory("device-1", "humidity", 2000, 50)
	if _, err := spool.store(healthy, time.Unix(2, 0)); err != nil {
		t.Fatalf("store healthy record: %v", err)
	}

	var replayed []TelemetryData
	result, err := spool.replay(context.Background(), 1, func(_ context.Context, row TelemetryData) error {
		replayed = append(replayed, row)
		return nil
	})
	if err == nil || result.Corrupt != 1 || result.Replayed != 1 || len(replayed) != 1 {
		t.Fatalf("corrupt replay result=%#v err=%v rows=%#v", result, err, replayed)
	}
	if replayed[0].Key != healthy.Key || replayed[0].TS != healthy.TS {
		t.Fatalf("replayed row = %#v, want healthy row %#v", replayed[0], healthy)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("corrupt record remained replayable: %v", statErr)
	}
	quarantined, globErr := filepath.Glob(path + telemetryFileSpoolCorruptSuffix + "*")
	if globErr != nil || len(quarantined) != 1 {
		t.Fatalf("quarantined records = %#v, err=%v; want one retained quarantine file", quarantined, globErr)
	}
	if usage := spool.usage(); usage.Records != 1 || usage.QuarantinedRecords != 1 || usage.QuarantinedBytes < 1 {
		t.Fatalf("quarantined record usage = %#v, want retained quarantine capacity", usage)
	}
}

func TestTelemetryFileSpoolInitRemovesOnlyIncompleteTempFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	tempPath := filepath.Join(directory, ".telemetry-spool-interrupted.tmp")
	if err := os.WriteFile(tempPath, []byte("partial"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	spool := &telemetryFileSpool{directory: directory, maxBytes: 1024, maxRecords: 10, maxRecordBytes: 512}
	if err := spool.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incomplete temp file still exists: %v", err)
	}
}

func TestTelemetryFileSpoolInitPromotesCompleteChecksummedTempRecord(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	record, payload, err := buildTelemetryFileSpoolRecord(history, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("build record: %v", err)
	}
	tempPath := filepath.Join(directory, ".telemetry-spool-complete.tmp")
	if err := os.WriteFile(tempPath, payload, 0o600); err != nil {
		t.Fatalf("write complete temp: %v", err)
	}

	spool := &telemetryFileSpool{directory: directory, maxBytes: 1024 * 1024, maxRecords: 10, maxRecordBytes: 1024 * 1024}
	if err := spool.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	finalPath := filepath.Join(directory, telemetryFileSpoolFilename(record.Identity))
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("complete temp was not promoted: %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("promoted temp still exists: %v", err)
	}
	if usage := spool.usage(); usage.Records != 1 || usage.Bytes != int64(len(payload)) {
		t.Fatalf("recovered usage = %#v", usage)
	}
}

func TestTelemetryFileSpoolInitRecoversQuarantineUsage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	spool := &telemetryFileSpool{
		directory:      directory,
		maxBytes:       1024 * 1024,
		maxRecords:     10,
		maxRecordBytes: 1024 * 1024,
	}
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("store record: %v", err)
	}
	path := filepath.Join(directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	quarantinePath := path + telemetryFileSpoolCorruptSuffix
	if err := os.Rename(path, quarantinePath); err != nil {
		t.Fatalf("move record to quarantine: %v", err)
	}
	info, err := os.Stat(quarantinePath)
	if err != nil {
		t.Fatalf("stat quarantine record: %v", err)
	}

	recovered := &telemetryFileSpool{
		directory:      directory,
		maxBytes:       1024 * 1024,
		maxRecords:     10,
		maxRecordBytes: 1024 * 1024,
	}
	if err := recovered.init(); err != nil {
		t.Fatalf("recover telemetry spool: %v", err)
	}
	usage := recovered.usage()
	if usage.Records != 1 || usage.Bytes != info.Size() || usage.QuarantinedRecords != 1 || usage.QuarantinedBytes != info.Size() {
		t.Fatalf("recovered quarantine usage = %#v, want one quarantined file of %d bytes", usage, info.Size())
	}
}

func testTelemetryFileSpool(t *testing.T, maxBytes int64, maxRecords int) *telemetryFileSpool {
	t.Helper()
	spool := &telemetryFileSpool{
		directory:      filepath.Join(t.TempDir(), "telemetry-spool"),
		maxBytes:       maxBytes,
		maxRecords:     maxRecords,
		maxRecordBytes: maxBytes,
	}
	if err := spool.init(); err != nil {
		t.Fatalf("init telemetry spool: %v", err)
	}
	return spool
}

func TestTelemetryFileSpoolRejectsPublicFilesDirectory(t *testing.T) {
	for _, directory := range []string{"./files", filepath.Join("files", "telemetry-spool")} {
		spool := &telemetryFileSpool{
			directory:      directory,
			maxBytes:       1024,
			maxRecords:     10,
			maxRecordBytes: 512,
		}
		if err := spool.validateConfig(); err == nil {
			t.Fatalf("public spool directory %q should be rejected", directory)
		}
	}
}

func TestTelemetryFileSpoolRejectsSameIdentityWithDifferentPayload(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	original := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if result, err := spool.store(original, time.Now()); err != nil || !result.Stored {
		t.Fatalf("store original result=%#v err=%v", result, err)
	}

	collision := testTelemetrySpoolHistory("device-1", "temperature", 1000, 99)
	result, err := spool.store(collision, time.Now())
	if err == nil {
		t.Fatalf("collision result=%#v err=nil, want identity collision rejection", result)
	}
	if result.Stored || result.Duplicate {
		t.Fatalf("collision result=%#v, want neither stored nor duplicate", result)
	}
	if usage := spool.usage(); usage.Records != 1 {
		t.Fatalf("usage after collision=%#v, want original record only", usage)
	}
}

func testTelemetrySpoolHistory(deviceID, key string, ts int64, value float64) TelemetryData {
	return TelemetryData{
		DeviceID: deviceID,
		TenantID: "tenant-1",
		Key:      key,
		TS:       ts,
		NumberV:  &value,
	}
}
