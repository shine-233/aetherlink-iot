package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/storage"

	"gorm.io/gorm/clause"
)

func TestTelemetryDeadLetterReplayFailureUpdatesSchedulesRetryBeforeMaxAttempts(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	updates := telemetryDeadLetterReplayFailureUpdates(0, errors.New("boom"), now)

	if updates["status"] != storage.TelemetryDeadLetterStatusRetrying {
		t.Fatalf("status = %v, want retrying", updates["status"])
	}
	next, ok := updates["next_retry_at"].(*time.Time)
	if !ok || next == nil || !next.After(now) {
		t.Fatalf("next_retry_at = %#v, want future time", updates["next_retry_at"])
	}
	if _, ok := updates["attempts"].(clause.Expr); !ok {
		t.Fatalf("attempts = %#v, want SQL increment expression", updates["attempts"])
	}
	if updates["last_error"] != "replay failed: boom" {
		t.Fatalf("last_error = %v", updates["last_error"])
	}
	if updates["updated_at"] != now {
		t.Fatalf("updated_at = %v, want %v", updates["updated_at"], now)
	}
}

func TestTelemetryDeadLetterReplayFailureUpdatesMarksDeadAtMaxAttempts(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	updates := telemetryDeadLetterReplayFailureUpdates(storage.TelemetryDeadLetterMaxAttempts()-1, nil, now)

	if updates["status"] != storage.TelemetryDeadLetterStatusDead {
		t.Fatalf("status = %v, want dead", updates["status"])
	}
	if next, _ := updates["next_retry_at"].(*time.Time); next != nil {
		t.Fatalf("next_retry_at = %v, want nil for dead row", next)
	}
	if updates["last_error"] != "replay failed: " {
		t.Fatalf("last_error = %q, want empty cause suffix", updates["last_error"])
	}
}

func TestTelemetryDeadLetterLeaseTransitionUpdates(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)

	claim := telemetryDeadLetterClaimUpdates(now)
	if claim["status"] != storage.TelemetryDeadLetterStatusProcessing || claim["updated_at"] != now {
		t.Fatalf("claim updates = %#v", claim)
	}
	if v, ok := claim["next_retry_at"]; !ok || v != nil {
		t.Fatalf("claim must clear next_retry_at, got %#v", claim)
	}

	release := telemetryDeadLetterReleaseUpdates(now)
	if release["status"] != storage.TelemetryDeadLetterStatusRetrying ||
		release["next_retry_at"] != now ||
		release["updated_at"] != now {
		t.Fatalf("release updates = %#v", release)
	}

	resolved := telemetryDeadLetterResolvedUpdates(now)
	if resolved["status"] != storage.TelemetryDeadLetterStatusResolved {
		t.Fatalf("resolved updates = %#v", resolved)
	}
	if _, ok := resolved["attempts"]; ok {
		t.Fatalf("resolve must not touch attempts: %#v", resolved)
	}
}

func TestMarkTelemetryDeadLetterReplayFailureIncrementsAttemptsInDB(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "fail-1",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusProcessing,
		Attempts:  1,
		CreatedAt: now.Add(-time.Hour),
	})

	row := storage.TelemetryDeadLetter{ID: "fail-1", Attempts: 1}
	if err := markTelemetryDeadLetterReplayFailureContext(context.Background(), row, errors.New("disk full"), now); err != nil {
		t.Fatalf("markTelemetryDeadLetterReplayFailureContext() error = %v", err)
	}

	var got storage.TelemetryDeadLetter
	if err := db.First(&got, "id = ?", "fail-1").Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if got.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", got.Attempts)
	}
	wantStatus := storage.TelemetryDeadLetterStatusRetrying
	if storage.TelemetryDeadLetterMaxAttempts() <= 2 {
		wantStatus = storage.TelemetryDeadLetterStatusDead
	}
	if got.Status != wantStatus {
		t.Fatalf("status = %q, want %q", got.Status, wantStatus)
	}
	if !strings.Contains(got.LastError, "disk full") {
		t.Fatalf("last_error = %q, want cause", got.LastError)
	}
}

func TestDrainTelemetryDeadLetterQueryContextCanceledLeavesRowsClaimable(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Now().UTC()
	createTelemetryDeadLetterRow(t, db, storage.TelemetryDeadLetter{
		ID:        "cancel-1",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Key:       "temperature",
		TS:        1783583400000,
		Status:    storage.TelemetryDeadLetterStatusPending,
		CreatedAt: now.Add(-time.Minute),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := drainTelemetryDeadLetterQueryContext(
		ctx,
		telemetryDeadLetterReadyQuery(db.Model(&storage.TelemetryDeadLetter{}), now),
		10,
		now,
	)
	// The canceled ctx surfaces at the ready-count query, wrapped as a DB error.
	if err == nil {
		t.Fatal("drain with canceled ctx returned nil error")
	}

	var got storage.TelemetryDeadLetter
	if err := db.First(&got, "id = ?", "cancel-1").Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if got.Status != storage.TelemetryDeadLetterStatusPending {
		t.Fatalf("status = %q, canceled drain must not claim or transition rows", got.Status)
	}
	if got.Attempts != 0 {
		t.Fatalf("attempts = %d, cancellation must not count as a replay failure", got.Attempts)
	}
}
