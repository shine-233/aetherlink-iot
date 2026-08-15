package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildFleetCommandJobProgressHealthShowsTimeoutRisk(t *testing.T) {
	now := time.Date(2026, 7, 7, 5, 0, 0, 0, time.UTC)
	timeoutAt := now.Add(10 * time.Second)
	job := &model.CommandJob{
		Status:         commandJobStatusRunning,
		RequestedCount: 10,
		SubmittedCount: 5,
		FailedCount:    1,
		BlockedCount:   1,
		CreatedAt:      now.Add(-50 * time.Second),
		TimeoutAt:      &timeoutAt,
	}

	health := buildFleetCommandJobProgressHealth(job, now)

	if health.State != "timeout_risk" {
		t.Fatalf("state = %q, want timeout_risk", health.State)
	}
	if health.PendingCount != 3 || health.TerminalCount != 7 {
		t.Fatalf("unexpected counts: %#v", health)
	}
	if health.TimeoutRemainingSeconds != 10 || health.ElapsedSeconds != 50 {
		t.Fatalf("unexpected timing: %#v", health)
	}
}

func TestBuildFleetCommandJobProgressHealthShowsAttentionForFailedJob(t *testing.T) {
	now := time.Date(2026, 7, 7, 5, 0, 0, 0, time.UTC)
	job := &model.CommandJob{
		Status:         commandJobStatusPartiallyFailed,
		RequestedCount: 4,
		SubmittedCount: 2,
		FailedCount:    1,
		BlockedCount:   1,
		CreatedAt:      now.Add(-2 * time.Minute),
	}

	health := buildFleetCommandJobProgressHealth(job, now)

	if health.State != "needs_attention" {
		t.Fatalf("state = %q, want needs_attention", health.State)
	}
	if health.PendingCount != 0 {
		t.Fatalf("pending = %d, want 0", health.PendingCount)
	}
}
