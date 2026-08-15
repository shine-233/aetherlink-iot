package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTelemetryDeadLetterStatusUpdates(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		action     string
		wantStatus string
		wantReset  bool
	}{
		{
			name:       "retry resets attempts and retry delay",
			action:     telemetryDeadLetterActionRetry,
			wantStatus: storage.TelemetryDeadLetterStatusPending,
			wantReset:  true,
		},
		{
			name:       "resolve marks row as handled",
			action:     telemetryDeadLetterActionResolve,
			wantStatus: storage.TelemetryDeadLetterStatusResolved,
		},
		{
			name:       "ignore marks row as terminal dead",
			action:     telemetryDeadLetterActionIgnore,
			wantStatus: storage.TelemetryDeadLetterStatusDead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, err := telemetryDeadLetterStatusUpdates(tt.action, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if updates["status"] != tt.wantStatus {
				t.Fatalf("status = %#v, want %#v", updates["status"], tt.wantStatus)
			}
			if updates["updated_at"] != now {
				t.Fatalf("updated_at = %#v, want %#v", updates["updated_at"], now)
			}
			if tt.wantReset {
				if updates["attempts"] != 0 {
					t.Fatalf("attempts = %#v, want 0", updates["attempts"])
				}
				if updates["next_retry_at"] != nil {
					t.Fatalf("next_retry_at = %#v, want nil", updates["next_retry_at"])
				}
			}
		})
	}
}

func TestTelemetryDeadLetterStatusUpdatesRejectsUnsupportedAction(t *testing.T) {
	_, err := telemetryDeadLetterStatusUpdates("archive", time.Now())
	if err == nil {
		t.Fatal("expected unsupported action to fail")
	}
}

func TestTelemetryDeadLetterStatusUpdatesRejectsReplayWithoutExecution(t *testing.T) {
	_, err := telemetryDeadLetterStatusUpdates(telemetryDeadLetterActionReplay, time.Now())
	if err == nil {
		t.Fatal("expected replay status-only update to fail")
	}
}

func TestUpdateTelemetryDeadLetterManualStatusUsesPreviousStatusCompareAndSet(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "manual-cas",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusPending,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})

	if err := updateTelemetryDeadLetterManualStatus(
		"manual-cas",
		storage.TelemetryDeadLetterStatusPending,
		telemetryDeadLetterActionResolve,
		now,
	); err != nil {
		t.Fatalf("first conditional update returned error: %v", err)
	}

	err := updateTelemetryDeadLetterManualStatus(
		"manual-cas",
		storage.TelemetryDeadLetterStatusPending,
		telemetryDeadLetterActionIgnore,
		now.Add(time.Second),
	)
	assertTelemetryDeadLetterStatusConflict(t, err, storage.TelemetryDeadLetterStatusPending)

	var row storage.TelemetryDeadLetter
	if err := db.First(&row, "id = ?", "manual-cas").Error; err != nil {
		t.Fatalf("load conditionally updated row: %v", err)
	}
	if row.Status != storage.TelemetryDeadLetterStatusResolved {
		t.Fatalf("status = %q, want resolved after stale update was rejected", row.Status)
	}
}

func TestUpdateTelemetryDeadLetterManualStatusRejectsProcessingForEveryManualAction(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 19, 12, 5, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "worker-owned",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusProcessing,
		Attempts:  1,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	})

	for _, action := range []string{
		telemetryDeadLetterActionRetry,
		telemetryDeadLetterActionResolve,
		telemetryDeadLetterActionIgnore,
	} {
		t.Run(action, func(t *testing.T) {
			err := updateTelemetryDeadLetterManualStatus(
				"worker-owned",
				storage.TelemetryDeadLetterStatusProcessing,
				action,
				now.Add(time.Second),
			)
			assertTelemetryDeadLetterStatusConflict(t, err, storage.TelemetryDeadLetterStatusProcessing)
		})
	}

	var row storage.TelemetryDeadLetter
	if err := db.First(&row, "id = ?", "worker-owned").Error; err != nil {
		t.Fatalf("load worker-owned row: %v", err)
	}
	if row.Status != storage.TelemetryDeadLetterStatusProcessing || row.Attempts != 1 || !row.UpdatedAt.Equal(now) {
		t.Fatalf("worker-owned row changed unexpectedly: %#v", row)
	}
}

func assertTelemetryDeadLetterStatusConflict(t *testing.T, err error, expectedStatus string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected dead-letter status conflict")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("conflict error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeOpDenied {
		t.Fatalf("conflict code = %d, want %d", appErr.Code, errcode.CodeOpDenied)
	}
	if !strings.Contains(appErr.CustomMsg, telemetryDeadLetterStatusConflict) ||
		!strings.Contains(appErr.CustomMsg, expectedStatus) {
		t.Fatalf("conflict message = %q, want status conflict mentioning %q", appErr.CustomMsg, expectedStatus)
	}
}

func TestTelemetryCurrentFromHistory(t *testing.T) {
	value := 42.5
	history := storage.TelemetryData{
		DeviceID: "device-1",
		TenantID: "tenant-1",
		Key:      "temperature",
		TS:       1783583400000,
		NumberV:  &value,
	}

	current := telemetryCurrentFromHistory(history)
	if current.DeviceID != history.DeviceID || current.TenantID != history.TenantID || current.Key != history.Key {
		t.Fatalf("current identity = %#v, want history identity", current)
	}
	if !current.TS.Equal(time.UnixMilli(history.TS)) {
		t.Fatalf("current ts = %s, want %s", current.TS, time.UnixMilli(history.TS))
	}
	if current.NumberV == nil || *current.NumberV != value {
		t.Fatalf("current value = %#v, want %v", current.NumberV, value)
	}
}

func TestNormalizeTelemetryDeadLetterDrainLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default", in: 0, want: 20},
		{name: "negative", in: -5, want: 20},
		{name: "keeps explicit", in: 10, want: 10},
		{name: "caps large value", in: 500, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTelemetryDeadLetterDrainLimit(tt.in); got != tt.want {
				t.Fatalf("limit = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClaimTelemetryDeadLetterRowsMarksProcessingAndSkipsClaimedRows(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 9, 16, 0, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "ready-1",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusPending,
		Attempts:  0,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})

	rows, totalReady, err := claimTelemetryDeadLetterRows(telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now), 10, now)
	if err != nil {
		t.Fatalf("claimTelemetryDeadLetterRows returned error: %v", err)
	}
	if totalReady != 1 || len(rows) != 1 {
		t.Fatalf("first claim totalReady=%d rows=%d, want 1/1", totalReady, len(rows))
	}

	var claimed storage.TelemetryDeadLetter
	if err := db.First(&claimed, "id = ?", "ready-1").Error; err != nil {
		t.Fatalf("load claimed row: %v", err)
	}
	if claimed.Status != storage.TelemetryDeadLetterStatusProcessing {
		t.Fatalf("status = %q, want processing", claimed.Status)
	}
	if claimed.NextRetryAt != nil {
		t.Fatalf("next_retry_at = %v, want nil", claimed.NextRetryAt)
	}

	rows, totalReady, err = claimTelemetryDeadLetterRows(telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now), 10, now)
	if err != nil {
		t.Fatalf("second claim returned error: %v", err)
	}
	if totalReady != 0 || len(rows) != 0 {
		t.Fatalf("second claim totalReady=%d rows=%d, want 0/0", totalReady, len(rows))
	}
}

func TestReleaseTelemetryDeadLetterClaimsReturnsOnlyProcessingRowsToRetrying(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	claimedAt := time.Date(2026, 7, 9, 16, 5, 0, 0, time.UTC)
	releasedAt := claimedAt.Add(2 * time.Second)
	lastError := "original ingest failure"
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "cancelled-processing",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusProcessing,
		Attempts:  2,
		LastError: lastError,
		CreatedAt: claimedAt.Add(-time.Minute),
		UpdatedAt: claimedAt,
	})
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "already-resolved",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "switch_1",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusResolved,
		Attempts:  1,
		CreatedAt: claimedAt.Add(-time.Minute),
		UpdatedAt: claimedAt,
	})

	rows := []storage.TelemetryDeadLetter{
		{ID: "cancelled-processing"},
		{ID: "already-resolved"},
	}
	err := releaseTelemetryDeadLetterClaimsAfterErrorAt(rows, context.Canceled, releasedAt)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("releaseTelemetryDeadLetterClaimsAfterErrorAt error = %v, want context.Canceled", err)
	}

	var released storage.TelemetryDeadLetter
	if err := db.First(&released, "id = ?", "cancelled-processing").Error; err != nil {
		t.Fatalf("load released row: %v", err)
	}
	if released.Status != storage.TelemetryDeadLetterStatusRetrying {
		t.Fatalf("released status = %q, want retrying", released.Status)
	}
	if released.Attempts != 2 {
		t.Fatalf("released attempts = %d, want 2", released.Attempts)
	}
	if released.NextRetryAt == nil || !released.NextRetryAt.Equal(releasedAt) {
		t.Fatalf("released next_retry_at = %v, want %s", released.NextRetryAt, releasedAt)
	}
	if released.LastError != lastError {
		t.Fatalf("released last_error = %q, want preserved %q", released.LastError, lastError)
	}

	var resolved storage.TelemetryDeadLetter
	if err := db.First(&resolved, "id = ?", "already-resolved").Error; err != nil {
		t.Fatalf("load resolved row: %v", err)
	}
	if resolved.Status != storage.TelemetryDeadLetterStatusResolved || resolved.Attempts != 1 {
		t.Fatalf("resolved row changed unexpectedly: %#v", resolved)
	}
}

func TestClaimTelemetryDeadLetterRowsReclaimsStaleProcessingRows(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 9, 16, 10, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "stale-processing",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusProcessing,
		Attempts:  0,
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-telemetryDeadLetterProcessingTimeout - time.Second),
	})

	rows, totalReady, err := claimTelemetryDeadLetterRows(telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now), 10, now)
	if err != nil {
		t.Fatalf("claim stale processing returned error: %v", err)
	}
	if totalReady != 1 || len(rows) != 1 || rows[0].ID != "stale-processing" {
		t.Fatalf("stale claim totalReady=%d rows=%#v, want stale-processing", totalReady, rows)
	}
}

func TestClaimTelemetryDeadLetterForReplayRejectsFreshProcessingRow(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 9, 16, 15, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "fresh-processing",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusProcessing,
		Attempts:  0,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})

	if _, err := claimTelemetryDeadLetterForReplay("fresh-processing", now); err == nil {
		t.Fatal("expected fresh processing row to reject manual replay claim")
	}
}

func TestDrainTelemetryDeadLetterQueryReplaysClaimedRowsAndResolves(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 9, 16, 20, 0, 0, time.UTC)
	value := 26.5
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "replay-1",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		NumberV:   &value,
		Status:    storage.TelemetryDeadLetterStatusPending,
		Attempts:  0,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})

	result, err := drainTelemetryDeadLetterQuery(telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now), 10, now)
	if err != nil {
		t.Fatalf("drainTelemetryDeadLetterQuery returned error: %v", err)
	}
	if result.TotalReady != 1 || result.Attempted != 1 || result.Replayed != 1 || result.Failed != 0 {
		t.Fatalf("drain result = %#v, want one replay", result)
	}

	var row storage.TelemetryDeadLetter
	if err := db.First(&row, "id = ?", "replay-1").Error; err != nil {
		t.Fatalf("load dead letter: %v", err)
	}
	if row.Status != storage.TelemetryDeadLetterStatusResolved {
		t.Fatalf("status = %q, want resolved", row.Status)
	}

	var historyCount int64
	if err := db.Model(&storage.TelemetryData{}).Where("device_id = ? AND key = ? AND ts = ?", "device-1", "temperature", int64(1783583400000)).Count(&historyCount).Error; err != nil {
		t.Fatalf("count history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("history rows = %d, want 1", historyCount)
	}

	var current storage.TelemetryCurrentData
	if err := db.First(&current, "device_id = ? AND key = ?", "device-1", "temperature").Error; err != nil {
		t.Fatalf("load current: %v", err)
	}
	if current.NumberV == nil || *current.NumberV != value || current.TenantID != "tenant-1" {
		t.Fatalf("current = %#v, want replayed value %v tenant-1", current, value)
	}
}

func TestDrainTelemetryDeadLetterQueryDoesNotRegressNewerCurrentValue(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 7, 9, 16, 25, 0, 0, time.UTC)
	newerValue := 30.0
	if err := db.Create(&storage.TelemetryCurrentData{
		DeviceID: "device-1",
		Key:      "temperature",
		TS:       time.UnixMilli(2000),
		NumberV:  &newerValue,
		TenantID: "tenant-1",
	}).Error; err != nil {
		t.Fatalf("seed newer current: %v", err)
	}

	olderValue := 20.0
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "older-replay",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1000,
		NumberV:   &olderValue,
		Status:    storage.TelemetryDeadLetterStatusPending,
		Attempts:  0,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})

	result, err := drainTelemetryDeadLetterQuery(telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now), 10, now)
	if err != nil {
		t.Fatalf("drainTelemetryDeadLetterQuery returned error: %v", err)
	}
	if result.Replayed != 1 || result.Failed != 0 {
		t.Fatalf("drain result = %#v, want one successful replay", result)
	}

	var current storage.TelemetryCurrentData
	if err := db.First(&current, "device_id = ? AND key = ?", "device-1", "temperature").Error; err != nil {
		t.Fatalf("load current: %v", err)
	}
	if !current.TS.Equal(time.UnixMilli(2000)) {
		t.Fatalf("current timestamp = %s, want newer 2000ms", current.TS)
	}
	if current.NumberV == nil || *current.NumberV != newerValue {
		t.Fatalf("current value = %v, want newer value %v", current.NumberV, newerValue)
	}

	var historyCount int64
	if err := db.Model(&storage.TelemetryData{}).
		Where("device_id = ? AND key = ? AND ts = ?", "device-1", "temperature", int64(1000)).
		Count(&historyCount).Error; err != nil {
		t.Fatalf("count replayed history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("replayed history rows = %d, want 1", historyCount)
	}
}

func TestDrainTelemetryDeadLetterRowsContextStopsBeforeReplayWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := drainTelemetryDeadLetterRowsContext(
		ctx,
		[]storage.TelemetryDeadLetter{{ID: "cancel-before-replay"}},
		1,
		time.Now().UTC(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("drainTelemetryDeadLetterRowsContext() error = %v, want context.Canceled", err)
	}
	if result == nil {
		t.Fatal("drainTelemetryDeadLetterRowsContext() result = nil")
	}
	if result.TotalReady != 1 || result.Attempted != 0 || result.Replayed != 0 || result.Failed != 0 || len(result.Items) != 0 {
		t.Fatalf("canceled drain result = %#v, want no attempted replay", result)
	}
}

func setupTelemetryDeadLetterServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&storage.TelemetryDeadLetter{},
		&storage.TelemetryData{},
		&storage.TelemetryCurrentData{},
	); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS telemetry_datas_unique_test ON telemetry_datas(device_id, key, ts)").Error; err != nil {
		t.Fatalf("create telemetry history unique index: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS telemetry_current_datas_unique_test ON telemetry_current_datas(device_id, key)").Error; err != nil {
		t.Fatalf("create telemetry current unique index: %v", err)
	}

	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}

func createTelemetryDeadLetterRow(t *testing.T, db *gorm.DB, row storage.TelemetryDeadLetter) {
	t.Helper()

	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = row.CreatedAt
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create telemetry dead letter %s: %v", row.ID, err)
	}
}
