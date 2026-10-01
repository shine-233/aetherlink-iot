// file_spool_batch_test.go locks the batch costs of the shared fileSpool:
// grouped receipt removal (one directory fsync), bounded replay selection,
// single-call batch replay, and incremental quarantine accounting.
package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// seedTelemetrySpoolFiles writes n committed telemetry records directly (no
// per-file fsync) and gives them mtimes in REVERSE index order, so the oldest
// record by mtime is the last one by index. Returns histories by index.
func seedTelemetrySpoolFiles(t *testing.T, directory string, n int) []TelemetryData {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create spool directory: %v", err)
	}
	codec := telemetrySpoolCodec{}
	base := time.Unix(1_700_000_000, 0)
	histories := make([]TelemetryData, n)
	for index := 0; index < n; index++ {
		history := testTelemetrySpoolHistory("device-batch", "k", int64(1000+index), float64(index))
		prepared, identity, err := codec.prepare(history)
		if err != nil {
			t.Fatalf("prepare record %d: %v", index, err)
		}
		payload, err := codec.encode(prepared, identity, base)
		if err != nil {
			t.Fatalf("encode record %d: %v", index, err)
		}
		path := filepath.Join(directory, fileSpoolFilename(identity))
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatalf("write record %d: %v", index, err)
		}
		modTime := base.Add(time.Duration(n-index) * time.Second)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("set record %d mtime: %v", index, err)
		}
		histories[index] = prepared
	}
	return histories
}

func countingTelemetrySpool(t *testing.T, directory string, maxRecords int) (*telemetryFileSpool, *atomic.Int64) {
	t.Helper()
	syncs := &atomic.Int64{}
	spool := &telemetryFileSpool{
		directory:      directory,
		maxBytes:       64 * 1024 * 1024,
		maxRecords:     maxRecords,
		maxRecordBytes: 1024 * 1024,
		syncDir: func(string) error {
			syncs.Add(1)
			return nil
		},
	}
	if err := spool.init(); err != nil {
		t.Fatalf("init telemetry spool: %v", err)
	}
	return spool, syncs
}

func TestFileSpoolRemoveIdentitiesGroupsOneDirectorySync(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	histories := seedTelemetrySpoolFiles(t, directory, 200)
	spool, syncs := countingTelemetrySpool(t, directory, 1000)
	if usage := spool.usage(); usage.Records != 200 {
		t.Fatalf("seeded usage = %+v, want 200 records", usage)
	}
	beforeGrouped := spool.syncer.count()
	beforeAll := syncs.Load()

	// Duplicates, never-written rows and already-missing files are not errors.
	batch := append([]TelemetryData{}, histories...)
	batch = append(batch, histories[0], testTelemetrySpoolHistory("device-batch", "missing", 1, 1))
	if err := removeTelemetryWriteAheadReceipts(spool, batch); err != nil {
		t.Fatalf("remove receipts: %v", err)
	}
	if got := spool.syncer.count() - beforeGrouped; got != 1 {
		t.Fatalf("grouped directory syncs = %d, want exactly 1 for 200 receipts", got)
	}
	if got := syncs.Load() - beforeAll; got != 1 {
		t.Fatalf("directory fsyncs = %d, want exactly 1", got)
	}
	if usage := spool.usage(); usage.Records != 0 || usage.Bytes != 0 {
		t.Fatalf("usage after removal = %+v, want empty", usage)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read spool directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("%d files left after removal, want 0", len(entries))
	}

	// Nothing left to remove: no directory fsync at all.
	if err := removeTelemetryWriteAheadReceipts(spool, histories[:3]); err != nil {
		t.Fatalf("remove already-removed receipts: %v", err)
	}
	if got := syncs.Load() - beforeAll; got != 1 {
		t.Fatalf("no-op removal synced the directory (total %d)", got)
	}
}

func TestFileSpoolRemoveIdentitiesRejectsEscapingIdentity(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	outside := filepath.Join(filepath.Dir(spool.directory), "victim.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	err := spool.removeIdentities([]string{"../victim", "a/b", ".hidden", ""})
	if err == nil {
		t.Fatal("escaping identities were accepted")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("file outside the spool was touched: %v", statErr)
	}
}

func TestFileSpoolReplayLimitSelectsOldestFromLargeBacklog(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	histories := seedTelemetrySpoolFiles(t, directory, 500)
	spool, syncs := countingTelemetrySpool(t, directory, 1000)

	candidates, truncated, err := spool.listReplayCandidates(5, nil)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if !truncated || len(candidates) != 5 {
		t.Fatalf("candidates=%d truncated=%v, want 5 truncated", len(candidates), truncated)
	}

	var replayed []TelemetryData
	before := syncs.Load()
	result, err := spool.replay(context.Background(), 5, func(_ context.Context, row TelemetryData) error {
		replayed = append(replayed, row)
		return nil
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if result.Attempted != 5 || result.Replayed != 5 || result.Corrupt != 0 {
		t.Fatalf("replay result = %+v", result)
	}
	// mtimes run in reverse index order: the 5 oldest are indexes 499..495.
	for position, row := range replayed {
		want := histories[499-position]
		if row.TS != want.TS {
			t.Fatalf("replayed[%d].TS = %d, want %d (oldest mtime first)", position, row.TS, want.TS)
		}
	}
	if got := syncs.Load() - before; got != 1 {
		t.Fatalf("replay pass directory fsyncs = %d, want 1 grouped", got)
	}
	if usage := spool.usage(); usage.Records != 495 {
		t.Fatalf("usage after replay = %+v, want 495", usage)
	}

	full, err := spool.listReplayFiles()
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(full) != 495 {
		t.Fatalf("unbounded listing = %d, want 495", len(full))
	}
	for index := 1; index < len(full); index++ {
		if fileSpoolReplayBefore(full[index], full[index-1], true) {
			t.Fatalf("unbounded listing out of order at %d", index)
		}
	}
}

func TestFileSpoolReplayBatchCallsOnceAndRetiresAll(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	histories := seedTelemetrySpoolFiles(t, directory, 40)
	spool, syncs := countingTelemetrySpool(t, directory, 1000)

	// Corrupt the two oldest records: the batch must still fill its quota of
	// healthy rows from later selection rounds.
	for _, index := range []int{39, 38} {
		_, identity, _ := telemetrySpoolCodec{}.prepare(histories[index])
		path := filepath.Join(directory, fileSpoolFilename(identity))
		modTime := time.Unix(1_600_000_000, 0)
		if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
			t.Fatalf("corrupt record: %v", err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("set corrupt mtime: %v", err)
		}
	}

	failing := errors.New("database down")
	calls := 0
	result, err := spool.replayBatch(context.Background(), 10, func(_ context.Context, rows []TelemetryData) error {
		calls++
		return failing
	})
	if !errors.Is(err, failing) || calls != 1 || result.Replayed != 0 || result.Corrupt != 2 {
		t.Fatalf("failed batch result=%+v calls=%d err=%v", result, calls, err)
	}
	if usage := spool.usage(); usage.Records != 40 || usage.QuarantinedRecords != 2 {
		t.Fatalf("failed batch removed records: %+v", usage)
	}

	calls = 0
	var got []TelemetryData
	before := syncs.Load()
	result, err = spool.replayBatch(context.Background(), 10, func(_ context.Context, rows []TelemetryData) error {
		calls++
		got = append(got, rows...)
		return nil
	})
	if err != nil {
		t.Fatalf("batch replay: %v", err)
	}
	if calls != 1 || len(got) != 10 || result.Replayed != 10 || result.Attempted != 10 {
		t.Fatalf("batch result=%+v calls=%d rows=%d", result, calls, len(got))
	}
	for position, row := range got {
		if want := histories[37-position].TS; row.TS != want {
			t.Fatalf("batch[%d].TS = %d, want %d", position, row.TS, want)
		}
	}
	if delta := syncs.Load() - before; delta != 1 {
		t.Fatalf("batch replay directory fsyncs = %d, want 1", delta)
	}
	assertFileSpoolUsageMatchesRefresh(t, spool)
	if usage := spool.usage(); usage.Records != 30 || usage.QuarantinedRecords != 2 {
		t.Fatalf("usage after batch = %+v, want 28 healthy + 2 quarantined", usage)
	}
}

func TestFileSpoolQuarantineKeepsUsageEqualToFullRefresh(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "telemetry-spool")
	histories := seedTelemetrySpoolFiles(t, directory, 12)
	spool, _ := countingTelemetrySpool(t, directory, 1000)

	// Corruption that also resizes the file: incremental accounting cannot
	// know the original size, so the pass must reconcile once.
	_, identity, _ := telemetrySpoolCodec{}.prepare(histories[11])
	corruptPath := filepath.Join(directory, fileSpoolFilename(identity))
	if err := os.WriteFile(corruptPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}
	oldest := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(corruptPath, oldest, oldest); err != nil {
		t.Fatalf("set corrupt mtime: %v", err)
	}
	result, err := spool.replay(context.Background(), 3, func(context.Context, TelemetryData) error { return nil })
	if err == nil || result.Corrupt != 1 || result.Replayed != 3 {
		t.Fatalf("replay result=%+v err=%v", result, err)
	}
	assertFileSpoolUsageMatchesRefresh(t, spool)

	// Store-path quarantine (corrupt predecessor of the same identity).
	_, identity, _ = telemetrySpoolCodec{}.prepare(histories[0])
	if err := os.WriteFile(filepath.Join(directory, fileSpoolFilename(identity)), []byte("broken"), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}
	stored, err := spool.store(context.Background(), histories[0], time.Now())
	if err != nil || !stored.Stored || stored.Quarantined != 1 {
		t.Fatalf("store replacement result=%+v err=%v", stored, err)
	}
	if err := spool.reconcileUsage(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	assertFileSpoolUsageMatchesRefresh(t, spool)
}

func assertFileSpoolUsageMatchesRefresh(t *testing.T, spool *telemetryFileSpool) {
	t.Helper()
	incremental := spool.usage()
	spool.mu.Lock()
	dirty := spool.usageDirty
	err := spool.refreshUsageLocked()
	spool.mu.Unlock()
	if err != nil {
		t.Fatalf("refresh usage: %v", err)
	}
	if dirty {
		t.Fatalf("usage still marked dirty after the pass")
	}
	if refreshed := spool.usage(); refreshed != incremental {
		t.Fatalf("incremental usage %+v != full refresh %+v", incremental, refreshed)
	}
}
