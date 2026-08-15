package storage

// These filesystem contracts are intentionally unexecuted in the 2026-07-19
// source-only pass. They use temporary directories and a SQLite DB stand-in;
// no real PostgreSQL or power-loss behavior is claimed here.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestAttributeEventFileSpoolDuplicateAndCollisionContracts(t *testing.T) {
	spool := testAttributeEventFileSpool(t, 1024*1024, 10)
	envelope, err := buildAttributeEventEnvelope(testEventMessage("alarm", json.RawMessage(`{"value":1}`)))
	if err != nil {
		t.Fatalf("build event envelope: %v", err)
	}
	first, err := spool.store(context.Background(), envelope, time.Unix(1, 0))
	if err != nil || !first.Stored {
		t.Fatalf("first store = %+v err=%v", first, err)
	}
	duplicate, err := spool.store(context.Background(), envelope, time.Unix(2, 0))
	if err != nil || !duplicate.Duplicate || duplicate.Stored {
		t.Fatalf("duplicate store = %+v err=%v", duplicate, err)
	}
	if usage := spool.usage(); usage.Records != 1 {
		t.Fatalf("duplicate changed spool usage: %+v", usage)
	}

	existing, _, err := buildAttributeEventFileSpoolRecord(envelope, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("build existing record: %v", err)
	}
	otherEnvelope, err := buildAttributeEventEnvelope(testEventMessage("alarm", json.RawMessage(`{"value":2}`)))
	if err != nil {
		t.Fatalf("build different event envelope: %v", err)
	}
	incoming, _, err := buildAttributeEventFileSpoolRecord(otherEnvelope, time.Unix(2, 0))
	if err != nil {
		t.Fatalf("build incoming record: %v", err)
	}
	// Exercise the fail-closed branch directly: a real same-name/different-body
	// case would require an SHA-256 collision, so it cannot be fabricated by
	// writing an invalid record and then calling that corruption handling proof.
	incoming.Identity = existing.Identity
	if err := acceptExistingAttributeEventSpoolRecord(existing, incoming); err == nil || !strings.Contains(err.Error(), "identity collision") {
		t.Fatalf("same filename with different checksum/payload error = %v", err)
	}
}

func TestAttributeEventFileSpoolCapacityNeverEvicts(t *testing.T) {
	spool := testAttributeEventFileSpool(t, 1024*1024, 1)
	first, _ := buildAttributeEventEnvelope(testEventMessage("first", json.RawMessage(`{}`)))
	second, _ := buildAttributeEventEnvelope(testEventMessage("second", json.RawMessage(`{}`)))
	if _, err := spool.store(context.Background(), first, time.Unix(1, 0)); err != nil {
		t.Fatalf("store first record: %v", err)
	}
	if _, err := spool.store(context.Background(), second, time.Unix(2, 0)); err == nil || !strings.Contains(err.Error(), "capacity exhausted") {
		t.Fatalf("store above capacity error = %v", err)
	}
	usage := spool.usage()
	if usage.Records != 1 {
		t.Fatalf("capacity failure evicted or added records: %+v", usage)
	}
	files, err := spool.listReplayFiles()
	if err != nil || len(files) != 1 || files[0].name != attributeEventFileSpoolFilename(first.Identity) {
		t.Fatalf("retained files = %+v err=%v, want oldest durable record", files, err)
	}
}

func TestAttributeEventFileSpoolQuarantinesCorruptionAndCountsCapacity(t *testing.T) {
	spool := testAttributeEventFileSpool(t, 1024*1024, 10)
	envelope, _ := buildAttributeEventEnvelope(testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: "auto"}}))
	if _, err := spool.store(context.Background(), envelope, time.Unix(1, 0)); err != nil {
		t.Fatalf("store initial envelope: %v", err)
	}
	path := filepath.Join(spool.directory, attributeEventFileSpoolFilename(envelope.Identity))
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatalf("corrupt committed spool record: %v", err)
	}
	result, err := spool.store(context.Background(), envelope, time.Unix(2, 0))
	if err != nil {
		t.Fatalf("replace quarantined record: %v", err)
	}
	if !result.Stored || result.Corrupt != 1 || result.Quarantined != 1 {
		t.Fatalf("corruption replacement result = %+v", result)
	}
	usage := spool.usage()
	if usage.Records != 2 || usage.QuarantinedRecords != 1 || usage.QuarantinedBytes == 0 {
		t.Fatalf("quarantine capacity usage = %+v, want one healthy + one retained quarantine", usage)
	}
}

func TestAttributeEventFileSpoolReplayQuarantinesCorruptAndContinuesInStableOrder(t *testing.T) {
	spool := testAttributeEventFileSpool(t, 1024*1024, 10)
	envelopes := make([]attributeEventEnvelope, 0, 3)
	for _, identify := range []string{"one", "two", "three"} {
		envelope, err := buildAttributeEventEnvelope(testEventMessage(identify, json.RawMessage(`{"ok":true}`)))
		if err != nil {
			t.Fatalf("build %s envelope: %v", identify, err)
		}
		envelopes = append(envelopes, envelope)
		if _, err := spool.store(context.Background(), envelope, time.Now()); err != nil {
			t.Fatalf("store %s envelope: %v", identify, err)
		}
	}
	corrupt := envelopes[1]
	corruptPath := filepath.Join(spool.directory, attributeEventFileSpoolFilename(corrupt.Identity))
	if err := os.WriteFile(corruptPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("corrupt replay record: %v", err)
	}

	var replayed []string
	result, replayErr := spool.replay(context.Background(), 10, func(_ context.Context, envelope attributeEventEnvelope) error {
		replayed = append(replayed, envelope.Identity)
		return nil
	})
	if replayErr == nil || !strings.Contains(replayErr.Error(), "quarantined") {
		t.Fatalf("replay corruption error = %v", replayErr)
	}
	if result.Replayed != 2 || result.Corrupt != 1 {
		t.Fatalf("replay result = %+v", result)
	}
	wantOrder := []string{envelopes[0].Identity, envelopes[2].Identity}
	sort.Strings(wantOrder)
	if strings.Join(replayed, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("replay order = %v, want stable filename order %v", replayed, wantOrder)
	}
	if result.Usage.Records != 1 || result.Usage.QuarantinedRecords != 1 {
		t.Fatalf("usage after replay = %+v, want only retained quarantine", result.Usage)
	}
}

func TestAttributeEventFileSpoolRecoversCompleteTempRecord(t *testing.T) {
	cfg := testAttributeEventConfig(t)
	directory := cfg.AttributeEventSpoolDirectory
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create spool directory: %v", err)
	}
	envelope, _ := buildAttributeEventEnvelope(testEventMessage("recover", json.RawMessage(`{"ok":true}`)))
	_, payload, err := buildAttributeEventFileSpoolRecord(envelope, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("build recoverable record: %v", err)
	}
	temp, err := os.CreateTemp(directory, ".attribute-event-spool-*.tmp")
	if err != nil {
		t.Fatalf("create interrupted temp: %v", err)
	}
	if err := os.Chmod(temp.Name(), 0o600); err != nil {
		t.Fatalf("chmod interrupted temp: %v", err)
	}
	if _, err := temp.Write(payload); err != nil {
		t.Fatalf("write interrupted temp: %v", err)
	}
	if err := temp.Sync(); err != nil {
		t.Fatalf("sync interrupted temp: %v", err)
	}
	if err := temp.Close(); err != nil {
		t.Fatalf("close interrupted temp: %v", err)
	}

	spool := newAttributeEventFileSpool(cfg)
	if err := spool.init(); err != nil {
		t.Fatalf("recover spool: %v", err)
	}
	finalPath := filepath.Join(directory, attributeEventFileSpoolFilename(envelope.Identity))
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("recovered committed record: %v", err)
	}
	if usage := spool.usage(); usage.Records != 1 {
		t.Fatalf("recovered usage = %+v", usage)
	}
}

func TestAttributeEventSpoolMetricsTrackFailureCorruptionAndQuarantine(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	if err := db.Migrator().DropTable(&EventDataModel{}); err != nil {
		t.Fatalf("drop event table: %v", err)
	}
	if err := db.Migrator().DropTable(&uplinkStorageDeadLetter{}); err != nil {
		t.Fatalf("drop attribute/event dead-letter table: %v", err)
	}
	cfg := testAttributeEventConfig(t)
	metrics := newMetricsCollector(false)
	ingress := newAttributeEventIngress(db, logrus.New(), cfg, metrics)
	message := testEventMessage("metric", json.RawMessage(`{}`))
	if _, err := ingress.accept(context.Background(), message); err != nil {
		t.Fatalf("spool first metric envelope: %v", err)
	}
	envelope, _ := buildAttributeEventEnvelope(message)
	path := filepath.Join(ingress.spool.directory, attributeEventFileSpoolFilename(envelope.Identity))
	if err := os.WriteFile(path, []byte(`broken`), 0o600); err != nil {
		t.Fatalf("corrupt metric envelope: %v", err)
	}
	if _, err := ingress.accept(context.Background(), message); err != nil {
		t.Fatalf("replace corrupt metric envelope: %v", err)
	}
	got := metrics.GetMetrics()
	if got.AttributeEventSpooled != 2 || got.AttributeEventSpoolCorrupt != 1 ||
		got.AttributeEventSpoolBacklog != 2 || got.AttributeEventSpoolQuarantineRecords != 1 {
		t.Fatalf("corruption metrics = %+v", got)
	}

	// Fill the one-record variant and prove a two-boundary failure increments
	// the failure counter without evicting the existing envelope.
	failureCfg := testAttributeEventConfig(t)
	failureCfg.AttributeEventSpoolMaxRecords = 1
	failureMetrics := newMetricsCollector(false)
	failureIngress := newAttributeEventIngress(db, logrus.New(), failureCfg, failureMetrics)
	if _, err := failureIngress.accept(context.Background(), testEventMessage("first", json.RawMessage(`{}`))); err != nil {
		t.Fatalf("fill failure spool: %v", err)
	}
	_, err := failureIngress.accept(context.Background(), testEventMessage("second", json.RawMessage(`{}`)))
	if err == nil || !strings.Contains(err.Error(), "capacity exhausted") {
		t.Fatalf("two-boundary capacity error = %v", err)
	}
	if got := failureMetrics.GetMetrics(); got.AttributeEventSpoolFailed != 1 || got.AttributeEventSpoolBacklog != 1 {
		t.Fatalf("failure metrics = %+v", got)
	}
}

func TestAttributeEventSpoolRejectsPublicOrTelemetrySharedDirectory(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(cfg *Config) { cfg.AttributeEventSpoolDirectory = filepath.Join("files", "attribute-event-spool") },
		func(cfg *Config) { cfg.AttributeEventSpoolDirectory = cfg.TelemetrySpoolDirectory },
		func(cfg *Config) {
			cfg.AttributeEventSpoolDirectory = filepath.Join(cfg.TelemetrySpoolDirectory, "nested")
		},
	} {
		cfg := testAttributeEventConfig(t)
		mutate(&cfg)
		if err := newAttributeEventFileSpool(cfg).init(); err == nil {
			t.Fatalf("spool init accepted unsafe directory %q", cfg.AttributeEventSpoolDirectory)
		}
	}
}

func testAttributeEventFileSpool(t *testing.T, maxBytes int64, maxRecords int) *attributeEventFileSpool {
	t.Helper()
	cfg := testAttributeEventConfig(t)
	cfg.AttributeEventSpoolMaxBytes = maxBytes
	cfg.AttributeEventSpoolMaxRecords = maxRecords
	cfg.AttributeEventSpoolMaxRecordBytes = maxBytes
	spool := newAttributeEventFileSpool(cfg)
	if err := spool.init(); err != nil {
		t.Fatalf("init attribute/event file spool: %v", err)
	}
	return spool
}
