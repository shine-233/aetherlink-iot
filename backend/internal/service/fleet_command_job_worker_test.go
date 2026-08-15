package service

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestFleetCommandJobTimeoutRecoverableDetailStatuses(t *testing.T) {
	statuses := fleetCommandJobTimeoutRecoverableDetailStatuses()
	want := []string{commandJobDetailStatusReady, commandJobDetailStatusDispatching}

	if len(statuses) != len(want) {
		t.Fatalf("expected %d timeout-recoverable statuses, got %d", len(want), len(statuses))
	}
	for index, status := range want {
		if statuses[index] != status {
			t.Fatalf("expected timeout status %q at index %d, got %q", status, index, statuses[index])
		}
	}
}

func TestFleetCommandJobDetailSuccessClearsDispatchLease(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	leaseToken := "lease-1"
	leaseUntil := now.Add(time.Minute)
	detail := &model.CommandJobDetail{
		Status:             commandJobDetailStatusDispatching,
		DispatchAttempts:   2,
		DispatchLeaseToken: &leaseToken,
		DispatchLeaseUntil: &leaseUntil,
	}

	applyFleetCommandJobDetailSuccess(detail, &CommandDeliveryTracking{
		MessageID:   "msg-1",
		LogRecorded: true,
	}, now)

	if detail.Status != commandJobDetailStatusSubmitted {
		t.Fatalf("status = %q, want %q", detail.Status, commandJobDetailStatusSubmitted)
	}
	if detail.DispatchAttempts != 2 {
		t.Fatalf("dispatch attempts = %d, want 2", detail.DispatchAttempts)
	}
	if detail.DispatchLeaseToken != nil || detail.DispatchLeaseUntil != nil {
		t.Fatalf("dispatch lease should be cleared after successful claim writeback")
	}
	if detail.MessageID == nil || *detail.MessageID != "msg-1" {
		t.Fatalf("message id was not recorded: %#v", detail.MessageID)
	}
	if detail.SubmittedAt == nil || !detail.SubmittedAt.Equal(now) {
		t.Fatalf("submitted_at = %#v, want %s", detail.SubmittedAt, now)
	}
	if detail.CompletedAt != nil {
		t.Fatalf("completed_at should remain nil for submitted rows, got %#v", detail.CompletedAt)
	}
}

func TestFleetCommandJobDetailFailureClearsDispatchLease(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 5, 0, 0, time.UTC)
	leaseToken := "lease-2"
	leaseUntil := now.Add(time.Minute)
	detail := &model.CommandJobDetail{
		Status:             commandJobDetailStatusDispatching,
		DispatchAttempts:   2,
		DispatchLeaseToken: &leaseToken,
		DispatchLeaseUntil: &leaseUntil,
	}

	applyFleetCommandJobDetailFailure(detail, errors.New("publish failed"), now)

	if detail.Status != commandJobDetailStatusFailed {
		t.Fatalf("status = %q, want %q", detail.Status, commandJobDetailStatusFailed)
	}
	if detail.DispatchAttempts != 2 {
		t.Fatalf("dispatch attempts = %d, want 2", detail.DispatchAttempts)
	}
	if detail.DispatchLeaseToken != nil || detail.DispatchLeaseUntil != nil {
		t.Fatalf("dispatch lease should be cleared after failed claim writeback")
	}
	if !detail.CanRetry {
		t.Fatalf("failed dispatch rows should remain retryable")
	}
	if detail.NextRetryAfter == nil || !detail.NextRetryAfter.After(now) {
		t.Fatalf("next_retry_after = %#v, want a future retry time", detail.NextRetryAfter)
	}
	if detail.CompletedAt == nil || !detail.CompletedAt.Equal(now) {
		t.Fatalf("completed_at = %#v, want %s", detail.CompletedAt, now)
	}
	if detail.Reason == nil || *detail.Reason != "publish failed" {
		t.Fatalf("failure reason = %#v, want publish failed", detail.Reason)
	}
}

func TestFleetCommandJobDetailFailureStopsAfterMaxAttempts(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 10, 0, 0, time.UTC)
	detail := &model.CommandJobDetail{
		Status:           commandJobDetailStatusDispatching,
		DispatchAttempts: commandJobMaxDispatchAttempts,
	}

	applyFleetCommandJobDetailFailure(detail, errors.New("publish failed"), now)

	if detail.CanRetry {
		t.Fatalf("max-attempt failed rows should not remain retryable")
	}
	if detail.NextRetryAfter != nil {
		t.Fatalf("next_retry_after should be empty after max attempts, got %#v", detail.NextRetryAfter)
	}
	if detail.Advice == nil || *detail.Advice == "" {
		t.Fatalf("expected max-attempt advice")
	}
}
