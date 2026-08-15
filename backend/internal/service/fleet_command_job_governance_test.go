package service

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildFleetCommandJobPreviewGovernanceSummaryWarnsOnSubsetOnlyPreview(t *testing.T) {
	result := buildFleetCommandJobPreviewGovernanceSummary(&model.FleetCommandJobPreviewResult{
		RequestedCount: 10,
		EligibleCount:  8,
		BlockedCount:   2,
		TimeoutSeconds: 45,
		Rows: []model.FleetCommandJobPreviewRow{
			{DeviceID: "device-1", Eligible: true, RecommendedPath: "jobs"},
		},
		NextAction: "提交前复核预览子集覆盖。",
	})

	if result == nil {
		t.Fatal("expected governance summary")
	}
	if result.Level != "warning" {
		t.Fatalf("expected warning level, got %q", result.Level)
	}
	if !strings.Contains(result.Summary, "目标 10 台") {
		t.Fatalf("expected requested count in summary, got %q", result.Summary)
	}
	if len(result.Items) != 5 {
		t.Fatalf("expected stable governance items, got %d", len(result.Items))
	}
	if result.Items[1].Key != "preview_coverage" || result.Items[1].State != "watch" {
		t.Fatalf("expected preview coverage watch item, got %#v", result.Items[1])
	}
	if result.Items[2].Key != "timeout" || result.Items[2].State != "watch" {
		t.Fatalf("expected short-timeout watch item, got %#v", result.Items[2])
	}
	if result.Items[4].Key != "dispatch_quota" {
		t.Fatalf("expected dispatch quota item appended last, got %#v", result.Items[4])
	}
}

func TestBuildFleetCommandJobGovernanceSummaryEscalatesRetryLimitAndMissingEvidence(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	job := &model.CommandJob{
		ID:             "job-1",
		ScopeType:      fleetCommandScopeDeviceFilter,
		Status:         commandJobStatusPartiallyFailed,
		RequestedCount: 5,
		EligibleCount:  4,
		BlockedCount:   1,
		SubmittedCount: 3,
		FailedCount:    1,
		TimeoutSeconds: 120,
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
	}
	health := &model.FleetCommandJobProgressHealth{
		State:          "needs_attention",
		PendingCount:   1,
		TerminalCount:  4,
		ElapsedSeconds: 60,
	}
	retryCounts := commandJobRetryPolicyCounts{
		Retryable: 1,
		Exhausted: 1,
	}

	result := buildFleetCommandJobGovernanceSummary(job, health, retryCounts, 2, nil)

	if result == nil {
		t.Fatal("expected governance summary")
	}
	if result.Level != "error" {
		t.Fatalf("expected error level, got %q", result.Level)
	}
	if !strings.Contains(result.NextAction, "支持包") {
		t.Fatalf("expected support-bundle next action, got %q", result.NextAction)
	}
	assertGovernanceItemState(t, result, "retry_policy", "blocked")
	assertGovernanceItemState(t, result, "evidence", "blocked")
	assertGovernanceItemState(t, result, "timeout", "blocked")
}

func assertGovernanceItemState(t *testing.T, summary *model.FleetCommandJobGovernanceSummary, key string, state string) {
	t.Helper()
	for _, item := range summary.Items {
		if item.Key == key {
			if item.State != state {
				t.Fatalf("expected %s state %q, got %q", key, state, item.State)
			}
			return
		}
	}
	t.Fatalf("missing governance item %q in %#v", key, summary.Items)
}
