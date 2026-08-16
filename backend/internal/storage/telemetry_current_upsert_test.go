package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestTelemetryCurrentUpsertClauseRequiresNondecreasingTimestamp(t *testing.T) {
	upsert := TelemetryCurrentUpsertClause()
	if len(upsert.Columns) != 2 || upsert.Columns[0].Name != "device_id" || upsert.Columns[1].Name != "key" {
		t.Fatalf("conflict columns = %#v, want device_id/key", upsert.Columns)
	}
	if len(upsert.Where.Exprs) != 1 {
		t.Fatalf("upsert where expressions = %#v, want one timestamp guard", upsert.Where.Exprs)
	}
	expr, ok := upsert.Where.Exprs[0].(clause.Expr)
	if !ok {
		t.Fatalf("upsert timestamp guard = %T, want clause.Expr", upsert.Where.Exprs[0])
	}
	if expr.SQL != "EXCLUDED.ts >= telemetry_current_datas.ts" {
		t.Fatalf("upsert timestamp guard = %q", expr.SQL)
	}
}

func TestTelemetryWriterBatchInsertDoesNotRegressCurrentValue(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	seedNewerTelemetryCurrent(t, db, "batch-temperature")

	olderValue := 20.0
	writer := newTelemetryWriter(db, nil, DefaultConfig(), nil)
	err := writer.insertTelemetryBatch(
		db,
		[]TelemetryData{{
			DeviceID: "device-1",
			Key:      "batch-temperature",
			TS:       1000,
			NumberV:  &olderValue,
			TenantID: "tenant-1",
		}},
		[]TelemetryCurrentData{{
			DeviceID: "device-1",
			Key:      "batch-temperature",
			TS:       time.UnixMilli(1000),
			NumberV:  &olderValue,
			TenantID: "tenant-1",
		}},
	)
	if err != nil {
		t.Fatalf("insertTelemetryBatch returned error: %v", err)
	}

	assertNewerTelemetryCurrentPreserved(t, db, "batch-temperature")
}

func TestTelemetryWriterSingleFallbackDoesNotRegressCurrentValue(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	seedNewerTelemetryCurrent(t, db, "fallback-temperature")

	olderValue := 20.0
	history := TelemetryData{
		DeviceID: "device-1",
		Key:      "fallback-temperature",
		TS:       1000,
		NumberV:  &olderValue,
		TenantID: "tenant-1",
	}
	current := TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      "fallback-temperature",
		TS:       time.UnixMilli(1000),
		NumberV:  &olderValue,
		TenantID: "tenant-1",
	}
	writer := newTelemetryWriter(db, nil, DefaultConfig(), nil)
	written, failed := writer.fallbackInsertSingleRows(
		[]TelemetryData{history},
		buildTelemetryCurrentLookup([]TelemetryCurrentData{current}),
	)
	if written != 1 || failed != 0 {
		t.Fatalf("fallback result written=%d failed=%d, want 1/0", written, failed)
	}

	assertNewerTelemetryCurrentPreserved(t, db, "fallback-temperature")
}

func TestTelemetryFileSpoolReplayKeepsHistoryAndCurrentConsistentOnIdentityConflict(t *testing.T) {
	db := setupTelemetryCurrentUpsertTestDB(t)
	authoritativeValue := 30.0
	authoritative := TelemetryData{
		DeviceID: "device-1",
		Key:      "replay-temperature",
		TS:       1000,
		NumberV:  &authoritativeValue,
		TenantID: "tenant-1",
	}
	if err := db.Create(&authoritative).Error; err != nil {
		t.Fatalf("seed authoritative history: %v", err)
	}
	if err := db.Create(&TelemetryCurrentData{
		DeviceID: authoritative.DeviceID,
		Key:      authoritative.Key,
		TS:       time.UnixMilli(authoritative.TS),
		NumberV:  authoritative.NumberV,
		TenantID: authoritative.TenantID,
	}).Error; err != nil {
		t.Fatalf("seed authoritative current: %v", err)
	}

	conflictingValue := 20.0
	writer := newTelemetryWriter(db, nil, DefaultConfig(), nil)
	if err := writer.replayTelemetryFileSpoolRow(context.Background(), TelemetryData{
		DeviceID: authoritative.DeviceID,
		Key:      authoritative.Key,
		TS:       authoritative.TS,
		NumberV:  &conflictingValue,
		TenantID: authoritative.TenantID,
	}); err != nil {
		t.Fatalf("replay conflicting spool record: %v", err)
	}

	var history TelemetryData
	if err := db.First(&history, "device_id = ? AND key = ? AND ts = ?", authoritative.DeviceID, authoritative.Key, authoritative.TS).Error; err != nil {
		t.Fatalf("load history: %v", err)
	}
	var current TelemetryCurrentData
	if err := db.First(&current, "device_id = ? AND key = ?", authoritative.DeviceID, authoritative.Key).Error; err != nil {
		t.Fatalf("load current: %v", err)
	}
	if history.NumberV == nil || *history.NumberV != authoritativeValue {
		t.Fatalf("history value = %v, want first persisted value %v", history.NumberV, authoritativeValue)
	}
	if current.NumberV == nil || *current.NumberV != authoritativeValue {
		t.Fatalf("current value = %v, want authoritative history value %v", current.NumberV, authoritativeValue)
	}
}

func setupTelemetryCurrentUpsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	// Keep the migrated in-memory schema on the same connection used by the
	// asynchronous writer and the test's final readback.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&TelemetryData{}, &TelemetryCurrentData{}); err != nil {
		t.Fatalf("migrate telemetry tables: %v", err)
	}
	return db
}

func seedNewerTelemetryCurrent(t *testing.T, db *gorm.DB, key string) {
	t.Helper()
	newerValue := 30.0
	if err := db.Create(&TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      key,
		TS:       time.UnixMilli(2000),
		NumberV:  &newerValue,
		TenantID: "tenant-1",
	}).Error; err != nil {
		t.Fatalf("seed newer telemetry current: %v", err)
	}
}

func assertNewerTelemetryCurrentPreserved(t *testing.T, db *gorm.DB, key string) {
	t.Helper()
	var current TelemetryCurrentData
	if err := db.First(&current, "device_id = ? AND key = ?", "device-1", key).Error; err != nil {
		t.Fatalf("load telemetry current: %v", err)
	}
	if !current.TS.Equal(time.UnixMilli(2000)) {
		t.Fatalf("current timestamp = %s, want newer 2000ms", current.TS)
	}
	if current.NumberV == nil || *current.NumberV != 30 {
		t.Fatalf("current value = %v, want newer value 30", current.NumberV)
	}
}
