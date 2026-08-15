// 文件用途：覆盖遥测存储模块 telemetry writer 行为的 Go 测试。
// 核心逻辑：验证配置默认值、数据模型、去重转换或写入批处理的关键契约，主要围绕 func TestTelemetryWriterDeduplicateAndConvertKeepsUniqueHistoryAndLatestCurrent 等声明展开。
// 关键注意事项：存储测试需避免误连真实数据库，重点保持数据形状和去重语义稳定。
// 重构建议：后续可增加可替换 writer 和时钟夹具，扩大并发与失败路径覆盖。

package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistFailedTelemetryCountsOnlyNewSpoolRecords(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	metrics := newMetricsCollector(false)
	writer := &telemetryWriter{spool: spool, metrics: metrics}
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	cause := errors.New("postgres unavailable")

	if err := writer.persistFailedTelemetry(history, cause); err != nil {
		t.Fatalf("persist first failed telemetry: %v", err)
	}
	if err := writer.persistFailedTelemetry(history, cause); err != nil {
		t.Fatalf("persist duplicate failed telemetry: %v", err)
	}

	snapshot := metrics.GetMetrics()
	if snapshot.TelemetrySpooled != 1 {
		t.Fatalf("spooled total = %d, want one newly created deterministic record", snapshot.TelemetrySpooled)
	}
	if snapshot.TelemetrySpoolFailed != 0 || snapshot.TelemetrySpoolBacklog != 1 {
		t.Fatalf("duplicate spool metrics = %#v", snapshot)
	}
}

func TestPersistFailedTelemetryPublishesQuarantineUsage(t *testing.T) {
	spool := testTelemetryFileSpool(t, 1024*1024, 10)
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	if _, err := spool.store(history, time.Unix(1, 0)); err != nil {
		t.Fatalf("seed telemetry spool: %v", err)
	}
	path := filepath.Join(spool.directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	if err := os.WriteFile(path, []byte(`{"version":1,"checksum":"tampered"}`), 0o600); err != nil {
		t.Fatalf("corrupt telemetry spool record: %v", err)
	}

	metrics := newMetricsCollector(false)
	writer := &telemetryWriter{spool: spool, metrics: metrics}
	if err := writer.persistFailedTelemetry(history, errors.New("postgres unavailable")); err != nil {
		t.Fatalf("persist replacement telemetry: %v", err)
	}

	snapshot := metrics.GetMetrics()
	if snapshot.TelemetrySpooled != 1 || snapshot.TelemetrySpoolCorrupt != 1 {
		t.Fatalf("quarantine counters = %#v", snapshot)
	}
	if snapshot.TelemetrySpoolBacklog != 2 || snapshot.TelemetrySpoolQuarantineRecords != 1 || snapshot.TelemetrySpoolQuarantineBytes < 1 {
		t.Fatalf("quarantine gauges = %#v", snapshot)
	}
}

func TestTelemetryWriterDeduplicateAndConvertKeepsUniqueHistoryAndLatestCurrent(t *testing.T) {
	writer := newTelemetryWriter(nil, nil, DefaultConfig(), nil)
	batch := []*telemetryBatchItem{
		{
			deviceID:  "device-1",
			tenantID:  "tenant-1",
			timestamp: 1000,
			points: []TelemetryDataPoint{
				{Key: "temperature", Value: 21},
				{Key: "temperature", Value: 21},
				{Key: "humidity", Value: "dry"},
			},
		},
		{
			deviceID:  "device-1",
			tenantID:  "tenant-1",
			timestamp: 2000,
			points: []TelemetryDataPoint{
				{Key: "temperature", Value: 25},
			},
		},
	}

	history, current, duplicates := writer.deduplicateAndConvert(batch)
	if duplicates != 1 {
		t.Fatalf("duplicates = %d, want 1 duplicate telemetry point", duplicates)
	}
	if len(history) != 3 {
		t.Fatalf("history rows = %d, want 3 unique rows", len(history))
	}

	var temperatureCurrent *TelemetryCurrentData
	for i := range current {
		row := current[i]
		if row.DeviceID == "device-1" && row.Key == "temperature" {
			temperatureCurrent = &row
			break
		}
	}
	if temperatureCurrent == nil {
		t.Fatal("current telemetry should include temperature row")
	}
	if !temperatureCurrent.TS.Equal(time.UnixMilli(2000)) {
		t.Fatalf("temperature current timestamp = %s, want latest 2000ms", temperatureCurrent.TS)
	}
	if temperatureCurrent.NumberV == nil || *temperatureCurrent.NumberV != 25 {
		t.Fatalf("temperature current number = %v, want latest value 25", temperatureCurrent.NumberV)
	}
	if temperatureCurrent.TenantID != "tenant-1" {
		t.Fatalf("temperature current tenant = %q, want tenant-1", temperatureCurrent.TenantID)
	}
}

func TestBuildTelemetryCurrentLookupKeepsLatestByDeviceAndKey(t *testing.T) {
	older := TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      "temperature",
		TS:       time.UnixMilli(1000),
		TenantID: "tenant-1",
	}
	latestValue := 25.0
	latest := TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      "temperature",
		TS:       time.UnixMilli(2000),
		NumberV:  &latestValue,
		TenantID: "tenant-1",
	}
	humidity := TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      "humidity",
		TS:       time.UnixMilli(1500),
		TenantID: "tenant-1",
	}

	lookup := buildTelemetryCurrentLookup([]TelemetryCurrentData{latest, humidity, older})

	if len(lookup) != 2 {
		t.Fatalf("lookup rows = %d, want 2 device/key entries", len(lookup))
	}
	temperature, ok := lookup[telemetryCurrentLookupKey("device-1", "temperature")]
	if !ok {
		t.Fatal("temperature current row should be addressable by device/key")
	}
	if !temperature.TS.Equal(latest.TS) {
		t.Fatalf("temperature timestamp = %s, want latest %s", temperature.TS, latest.TS)
	}
	if temperature.NumberV == nil || *temperature.NumberV != latestValue {
		t.Fatalf("temperature number = %v, want latest value %v", temperature.NumberV, latestValue)
	}
	if _, ok := lookup[telemetryCurrentLookupKey("device-1", "pressure")]; ok {
		t.Fatal("lookup should not fabricate current rows for history-only keys")
	}
}

func TestBuildTelemetryCurrentChunkDedupesCurrentRows(t *testing.T) {
	latestValue := 25.0
	currentByKey := buildTelemetryCurrentLookup([]TelemetryCurrentData{
		{
			DeviceID: "device-1",
			Key:      "temperature",
			TS:       time.UnixMilli(2000),
			NumberV:  &latestValue,
			TenantID: "tenant-1",
		},
		{
			DeviceID: "device-1",
			Key:      "humidity",
			TS:       time.UnixMilli(1500),
			TenantID: "tenant-1",
		},
	})

	chunk := buildTelemetryCurrentChunk(
		[]TelemetryData{
			{DeviceID: "device-1", Key: "temperature", TS: 1000, TenantID: "tenant-1"},
			{DeviceID: "device-1", Key: "temperature", TS: 2000, TenantID: "tenant-1"},
			{DeviceID: "device-1", Key: "pressure", TS: 2000, TenantID: "tenant-1"},
		},
		currentByKey,
	)

	if len(chunk) != 1 {
		t.Fatalf("current chunk rows = %d, want 1 deduped current row", len(chunk))
	}
	if chunk[0].Key != "temperature" || !chunk[0].TS.Equal(time.UnixMilli(2000)) {
		t.Fatalf("current chunk row = %#v, want latest temperature current", chunk[0])
	}
	if chunk[0].NumberV == nil || *chunk[0].NumberV != latestValue {
		t.Fatalf("current chunk number = %v, want latest value %v", chunk[0].NumberV, latestValue)
	}
}

func TestBuildTelemetryDeadLetterKeepsReplayablePayload(t *testing.T) {
	value := 12.5
	now := time.Unix(1700000000, 0).UTC()
	history := TelemetryData{
		DeviceID: "device-1",
		TenantID: "tenant-1",
		Key:      "temperature",
		TS:       1234,
		NumberV:  &value,
	}

	deadLetter := buildTelemetryDeadLetter(history, errors.New("db down"), now)
	if deadLetter.ID == "" {
		t.Fatal("dead letter should have an id")
	}
	if deadLetter.DeviceID != history.DeviceID || deadLetter.TenantID != history.TenantID || deadLetter.Key != history.Key || deadLetter.TS != history.TS {
		t.Fatalf("dead letter identity fields = %#v, want history identity", deadLetter)
	}
	if deadLetter.Status != "pending" || deadLetter.Attempts != 1 || deadLetter.LastError != "db down" {
		t.Fatalf("dead letter retry fields = %#v", deadLetter)
	}
	if !deadLetter.CreatedAt.Equal(now) || !deadLetter.UpdatedAt.Equal(now) {
		t.Fatalf("dead letter timestamps = %s/%s, want %s", deadLetter.CreatedAt, deadLetter.UpdatedAt, now)
	}

	var payload TelemetryData
	if err := json.Unmarshal(deadLetter.RawPayload, &payload); err != nil {
		t.Fatalf("dead letter payload is not replayable JSON: %v", err)
	}
	if payload.DeviceID != history.DeviceID || payload.Key != history.Key || payload.TS != history.TS {
		t.Fatalf("dead letter payload = %#v, want original history", payload)
	}
	if payload.NumberV == nil || *payload.NumberV != value {
		t.Fatalf("dead letter payload number = %v, want %v", payload.NumberV, value)
	}
}

func TestTelemetryDeadLetterIDMatchesHistoryIdentityDeterministically(t *testing.T) {
	history := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	sameIdentityDifferentValue := testTelemetrySpoolHistory("device-1", "temperature", 1000, 99)
	differentKey := testTelemetrySpoolHistory("device-1", "humidity", 1000, 21.5)
	differentTimestamp := testTelemetrySpoolHistory("device-1", "temperature", 2000, 21.5)

	id := telemetryDeadLetterID(history)
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("dead-letter id = %q, want UUID-shaped 36-character identity", id)
	}
	if got := telemetryDeadLetterID(sameIdentityDifferentValue); got != id {
		t.Fatalf("same history identity id = %q, want %q", got, id)
	}
	if got := telemetryDeadLetterID(differentKey); got == id {
		t.Fatalf("different key reused dead-letter id %q", id)
	}
	if got := telemetryDeadLetterID(differentTimestamp); got == id {
		t.Fatalf("different timestamp reused dead-letter id %q", id)
	}
}

func TestRecordTelemetryDeadLetterKeepsFirstWriterForDuplicateIdentity(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	if err := db.AutoMigrate(&TelemetryDeadLetter{}); err != nil {
		t.Fatalf("migrate telemetry dead-letter table: %v", err)
	}
	writer := &telemetryWriter{db: db}
	first := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	duplicate := testTelemetrySpoolHistory("device-1", "temperature", 1000, 99)

	if err := writer.recordTelemetryDeadLetter(first, errors.New("first failure")); err != nil {
		t.Fatalf("record first telemetry dead-letter: %v", err)
	}
	if err := writer.recordTelemetryDeadLetter(duplicate, errors.New("duplicate failure")); err != nil {
		t.Fatalf("record duplicate telemetry dead-letter: %v", err)
	}

	var rows []TelemetryDeadLetter
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load telemetry dead-letters: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("dead-letter rows = %d, want one deterministic record", len(rows))
	}
	if rows[0].ID != telemetryDeadLetterID(first) || rows[0].LastError != "first failure" {
		t.Fatalf("dead-letter metadata = %#v, want first writer", rows[0])
	}
	if rows[0].NumberV == nil || *rows[0].NumberV != 21.5 {
		t.Fatalf("dead-letter value = %v, want first value 21.5", rows[0].NumberV)
	}
}

func TestRecordTelemetryDeadLetterReusesLegacyRandomIDForSameHistoryIdentity(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	if err := db.AutoMigrate(&TelemetryDeadLetter{}); err != nil {
		t.Fatalf("migrate telemetry dead-letter table: %v", err)
	}
	first := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	legacy := buildTelemetryDeadLetter(first, errors.New("legacy failure"), time.Unix(1, 0).UTC())
	legacy.ID = "legacy-random-id"
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy telemetry dead-letter: %v", err)
	}

	duplicate := testTelemetrySpoolHistory("device-1", "temperature", 1000, 99)
	if err := (&telemetryWriter{db: db}).recordTelemetryDeadLetter(duplicate, errors.New("new failure")); err != nil {
		t.Fatalf("record duplicate for legacy dead-letter: %v", err)
	}

	var rows []TelemetryDeadLetter
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load telemetry dead-letters: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != legacy.ID || rows[0].LastError != "legacy failure" {
		t.Fatalf("legacy duplicate rows = %#v, want unchanged first legacy row", rows)
	}
}

func TestRecordTelemetryDeadLetterRejectsDeterministicIdentityCollision(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	if err := db.AutoMigrate(&TelemetryDeadLetter{}); err != nil {
		t.Fatalf("migrate telemetry dead-letter table: %v", err)
	}
	target := testTelemetrySpoolHistory("device-1", "temperature", 1000, 21.5)
	collision := buildTelemetryDeadLetter(
		testTelemetrySpoolHistory("device-2", "humidity", 2000, 50),
		errors.New("seed collision"),
		time.Unix(1, 0).UTC(),
	)
	collision.ID = telemetryDeadLetterID(target)
	if err := db.Create(&collision).Error; err != nil {
		t.Fatalf("seed deterministic identity collision: %v", err)
	}

	err := (&telemetryWriter{db: db}).recordTelemetryDeadLetter(target, errors.New("target failure"))
	if err == nil || err.Error() != "telemetry dead-letter deterministic identity collision" {
		t.Fatalf("collision error = %v, want deterministic identity collision", err)
	}
}

func TestTelemetryDeadLetterDrainPlanSelectsReadyReplayRows(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	value := 12.5
	payload, err := json.Marshal(TelemetryData{
		DeviceID: "device-1",
		TenantID: "tenant-1",
		Key:      "temperature",
		TS:       1234,
		NumberV:  &value,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	waitUntil := now.Add(time.Minute)

	plan := buildTelemetryDeadLetterDrainPlan([]TelemetryDeadLetter{
		{
			ID:         "ready",
			Status:     TelemetryDeadLetterStatusPending,
			Attempts:   1,
			RawPayload: payload,
		},
		{
			ID:          "waiting",
			Status:      TelemetryDeadLetterStatusRetrying,
			Attempts:    1,
			NextRetryAt: &waitUntil,
			RawPayload:  payload,
		},
		{
			ID:         "resolved",
			Status:     TelemetryDeadLetterStatusResolved,
			Attempts:   1,
			RawPayload: payload,
		},
		{
			ID:         "exhausted",
			Status:     TelemetryDeadLetterStatusPending,
			Attempts:   telemetryDeadLetterMaxAttempts,
			RawPayload: payload,
		},
	}, now, 10)

	if len(plan.Ready) != 1 {
		t.Fatalf("ready rows = %d, want 1", len(plan.Ready))
	}
	ready := plan.Ready[0]
	if ready.DeadLetter.ID != "ready" {
		t.Fatalf("ready id = %q, want ready", ready.DeadLetter.ID)
	}
	if ready.History.DeviceID != "device-1" || ready.History.Key != "temperature" || ready.History.TS != 1234 {
		t.Fatalf("ready history = %#v, want replay payload", ready.History)
	}
	if ready.History.NumberV == nil || *ready.History.NumberV != value {
		t.Fatalf("ready number = %v, want %v", ready.History.NumberV, value)
	}
	if len(plan.Skipped) != 3 {
		t.Fatalf("skipped rows = %#v, want waiting/resolved/exhausted", plan.Skipped)
	}
	wantReasons := map[string]string{
		"waiting":   "retry_waiting",
		"resolved":  "terminal_status",
		"exhausted": "retry_exhausted",
	}
	for _, skipped := range plan.Skipped {
		if wantReasons[skipped.ID] != skipped.Reason {
			t.Fatalf("skip reason for %q = %q, want %q", skipped.ID, skipped.Reason, wantReasons[skipped.ID])
		}
	}
}

func TestTelemetryDataFromDeadLetterFallsBackToColumns(t *testing.T) {
	value := "ok"
	history, err := telemetryDataFromDeadLetter(TelemetryDeadLetter{
		ID:         "fallback",
		DeviceID:   "device-1",
		TenantID:   "tenant-1",
		Key:        "state",
		TS:         1234,
		StringV:    &value,
		RawPayload: json.RawMessage(`{`),
	})
	if err != nil {
		t.Fatalf("fallback dead letter should be replayable: %v", err)
	}
	if history.DeviceID != "device-1" || history.TenantID != "tenant-1" || history.Key != "state" || history.TS != 1234 {
		t.Fatalf("history = %#v, want column fallback identity", history)
	}
	if history.StringV == nil || *history.StringV != value {
		t.Fatalf("history string = %v, want %q", history.StringV, value)
	}
}

func TestNextTelemetryDeadLetterRetryAtUsesBoundedBackoff(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()

	first := nextTelemetryDeadLetterRetryAt(1, now)
	if first == nil || !first.Equal(now.Add(time.Minute)) {
		t.Fatalf("first retry = %v, want one minute later", first)
	}

	capped := nextTelemetryDeadLetterRetryAt(10, now)
	if capped == nil || !capped.Equal(now.Add(telemetryDeadLetterMaxBackoff)) {
		t.Fatalf("capped retry = %v, want max backoff", capped)
	}
}
