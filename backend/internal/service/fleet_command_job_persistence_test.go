package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestAdoptPersistedCanceledCommandJobPreventsStaleRefreshOverwrite(t *testing.T) {
	stale := &model.CommandJob{ID: "job-1", TenantID: "tenant-1", Status: commandJobStatusRunning}
	latest := &model.CommandJob{ID: stale.ID, TenantID: stale.TenantID, Status: commandJobStatusCanceled}

	got := adoptPersistedCanceledCommandJob(stale, latest)
	if got != latest || got.Status != commandJobStatusCanceled {
		t.Fatalf("expected persisted canceled snapshot, got %#v", got)
	}
}

func TestAdoptPersistedCanceledCommandJobKeepsActiveSnapshotWithoutCancel(t *testing.T) {
	stale := &model.CommandJob{ID: "job-2", TenantID: "tenant-1", Status: commandJobStatusRunning}
	latest := &model.CommandJob{ID: stale.ID, TenantID: stale.TenantID, Status: commandJobStatusRunning}

	if got := adoptPersistedCanceledCommandJob(stale, latest); got != stale {
		t.Fatalf("active snapshot should remain unchanged, got %#v", got)
	}
}
