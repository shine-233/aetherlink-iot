package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildFleetCommandJobExecutionSummaryPrioritizesRetryReadyRows(t *testing.T) {
	now := time.Date(2026, 7, 7, 6, 0, 0, 0, time.UTC)
	job := &model.CommandJob{
		ScopeType:      fleetCommandScopeDeviceFilter,
		Status:         commandJobStatusPartiallyFailed,
		RequestedCount: 20,
		EligibleCount:  18,
		BlockedCount:   2,
		SubmittedCount: 15,
		FailedCount:    3,
		CreatedAt:      now.Add(-2 * time.Minute),
	}
	health := buildFleetCommandJobProgressHealth(job, now)

	summary := buildFleetCommandJobExecutionSummary(
		job,
		health,
		commandJobRetryPolicyCounts{Retryable: 2, Ready: 2},
		1,
		&model.FleetCommandJobAuditSummary{LatestEventType: commandJobEventDispatchFailed},
	)

	if summary.PathType != "fleet_job" || summary.PathLabel != "设备筛选批量任务" {
		t.Fatalf("unexpected path: %#v", summary)
	}
	if summary.Decision != "retry" {
		t.Fatalf("decision = %q, want retry", summary.Decision)
	}
	if summary.CanClose {
		t.Fatalf("expected retry-ready job to block closure")
	}
	if len(summary.CloseBlockers) == 0 {
		t.Fatalf("expected close blockers, got %#v", summary.CloseBlockers)
	}
	if len(summary.Evidence) < 5 {
		t.Fatalf("expected customer evidence rows, got %#v", summary.Evidence)
	}
	if len(summary.Checklist) < 5 {
		t.Fatalf("expected operator checklist, got %#v", summary.Checklist)
	}
	if summary.Checklist[2].Key != "retry" || summary.Checklist[2].State != "todo" {
		t.Fatalf("expected retry todo checklist item, got %#v", summary.Checklist)
	}
}

func TestBuildFleetCommandJobExecutionSummaryClosesCleanJob(t *testing.T) {
	now := time.Date(2026, 7, 7, 6, 0, 0, 0, time.UTC)
	job := &model.CommandJob{
		ScopeType:      fleetCommandScopeSelectedDevices,
		Status:         commandJobStatusCompleted,
		RequestedCount: 1,
		EligibleCount:  1,
		SubmittedCount: 1,
		CreatedAt:      now.Add(-30 * time.Second),
	}

	summary := buildFleetCommandJobExecutionSummary(
		job,
		buildFleetCommandJobProgressHealth(job, now),
		commandJobRetryPolicyCounts{},
		0,
		&model.FleetCommandJobAuditSummary{LatestEventType: commandJobEventCompleted},
	)

	if summary.PathType != "single_device_command" {
		t.Fatalf("path type = %q, want single_device_command", summary.PathType)
	}
	if summary.Decision != "close" {
		t.Fatalf("decision = %q, want close", summary.Decision)
	}
	if !summary.CanClose || len(summary.CloseBlockers) != 0 {
		t.Fatalf("expected clean job to be close-ready, got canClose=%v blockers=%#v", summary.CanClose, summary.CloseBlockers)
	}
	if summary.Checklist[1].Key != "progress" || summary.Checklist[1].State != "done" {
		t.Fatalf("expected terminal progress checklist item, got %#v", summary.Checklist)
	}
}

func TestBuildFleetCommandJobExecutionSummaryBlocksAmbiguousAckCloseout(t *testing.T) {
	now := time.Date(2026, 7, 7, 6, 0, 0, 0, time.UTC)
	job := &model.CommandJob{
		ScopeType:      fleetCommandScopeSelectedDevices,
		Status:         commandJobStatusCompleted,
		RequestedCount: 1,
		EligibleCount:  1,
		SubmittedCount: 1,
		CreatedAt:      now.Add(-30 * time.Second),
	}

	summary := buildFleetCommandJobExecutionSummary(
		job,
		buildFleetCommandJobProgressHealth(job, now),
		commandJobRetryPolicyCounts{},
		0,
		&model.FleetCommandJobAuditSummary{LatestEventType: commandJobEventDeviceAckAmbiguous},
	)

	if summary.CanClose || summary.Decision != "collect_evidence" {
		t.Fatalf("ambiguous ack should block closeout, got canClose=%v decision=%q", summary.CanClose, summary.Decision)
	}
	if len(summary.CloseBlockers) == 0 {
		t.Fatalf("expected ambiguous ack close blocker")
	}
	if summary.Checklist[len(summary.Checklist)-1].Key != "audit" || summary.Checklist[len(summary.Checklist)-1].State != "blocked" {
		t.Fatalf("expected blocked audit checklist item, got %#v", summary.Checklist)
	}
}
