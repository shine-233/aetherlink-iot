package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

func TestBuildPersistedFleetCommandJobKeepsFutureSchedule(t *testing.T) {
	scheduledAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	req := &model.FleetCommandJobReq{
		ScopeType:      fleetCommandScopeSelectedDevices,
		Identify:       "reboot",
		TimeoutSeconds: 90,
		ScheduledAt:    &scheduledAt,
	}
	preview := &model.FleetCommandJobPreviewResult{
		ScopeType:      fleetCommandScopeSelectedDevices,
		RequestedCount: 1,
		EligibleCount:  1,
		TimeoutSeconds: 90,
		Rows: []model.FleetCommandJobPreviewRow{{
			DeviceID: "device-1",
			Eligible: true,
		}},
	}

	job, details, err := buildPersistedFleetCommandJob(req, preview, "operator-1", &utils.UserClaims{TenantID: "tenant-1"})
	if err != nil {
		t.Fatalf("build scheduled command job: %v", err)
	}
	if job.Status != commandJobStatusScheduled || job.ScheduledAt == nil || !job.ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("scheduled job = status %q scheduled_at %v", job.Status, job.ScheduledAt)
	}
	wantTimeout := scheduledAt.Add(90 * time.Second)
	if job.TimeoutAt == nil || !job.TimeoutAt.Equal(wantTimeout) {
		t.Fatalf("timeout_at = %v, want %v", job.TimeoutAt, wantTimeout)
	}
	if len(details) != 1 || details[0].Status != commandJobDetailStatusReady {
		t.Fatalf("scheduled details = %#v", details)
	}
}

func TestFleetCommandPreviewTokenIncludesSchedule(t *testing.T) {
	first := time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	rows := []model.FleetCommandJobPreviewRow{{DeviceID: "device-1", Eligible: true}}
	base := model.FleetCommandJobReq{
		ScopeType:      fleetCommandScopeSelectedDevices,
		DeviceIDs:      []string{"device-1"},
		Identify:       "reboot",
		TimeoutSeconds: 60,
		ScheduledAt:    &first,
	}
	firstToken := fleetCommandPreviewToken(&base, rows)
	base.ScheduledAt = &second
	secondToken := fleetCommandPreviewToken(&base, rows)
	if firstToken == secondToken {
		t.Fatal("preview token did not change when scheduled_at changed")
	}
}

func TestCommandJobProgressHealthReportsScheduledState(t *testing.T) {
	job := &model.CommandJob{Status: commandJobStatusScheduled}
	if state := commandJobProgressHealthState(job, 1, 120); state != "scheduled" {
		t.Fatalf("progress state = %q, want scheduled", state)
	}
}

func TestNormalizeFleetCommandJobRejectsPastScheduleInsteadOfRunningImmediately(t *testing.T) {
	past := time.Now().UTC().Add(-time.Minute)
	req := &model.FleetCommandJobReq{
		ScopeType:   fleetCommandScopeSelectedDevices,
		Identify:    "reboot",
		ScheduledAt: &past,
	}
	if err := normalizeFleetCommandJobReq(req); err == nil {
		t.Fatal("past scheduled_at should be rejected; callers must omit it for immediate dispatch")
	}
}
