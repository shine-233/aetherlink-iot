package service

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
)

func TestCommandJobSupportBundleUsesCountsAndFilteredDetails(t *testing.T) {
	responseFailed := strconv.Itoa(constant.ResponseSStatusFailed)
	messageID := "msg-1"
	reason := "device rejected command"
	advice := "check firmware compatibility"
	responseAt := time.Date(2026, 7, 6, 6, 30, 0, 0, time.UTC)

	job := &model.CommandJob{
		ID:             "job-1",
		JobType:        "service",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Identify:       "reboot",
		Status:         commandJobStatusPartiallyFailed,
		RequestedCount: 1000,
		EligibleCount:  998,
		BlockedCount:   2,
		SubmittedCount: 900,
		FailedCount:    98,
	}
	supportDetails := []*model.CommandJobDetail{
		{
			DeviceID:     "retry-1",
			DeviceNumber: "DN-001",
			Name:         "retry device",
			Status:       commandJobDetailStatusFailed,
			Reason:       &reason,
			Advice:       &advice,
			CanRetry:     true,
		},
		{
			DeviceID:    "missing-log-1",
			Status:      commandJobDetailStatusSubmitted,
			MessageID:   &messageID,
			LogRecorded: false,
		},
		{
			DeviceID:       "ack-failed-1",
			Status:         commandJobDetailStatusSubmitted,
			ResponseStatus: &responseFailed,
			ResponseError:  &reason,
			ResponseAt:     &responseAt,
			LogRecorded:    true,
		},
	}

	bundle := commandJobSupportBundleFromPersistence(
		job,
		supportDetails,
		map[string]int{
			commandJobDetailStatusFailed:    98,
			commandJobDetailStatusSubmitted: 900,
			commandJobDetailStatusBlocked:   2,
		},
		42,
		12,
		30,
		4,
		7,
		nil,
	)

	if bundle.RetryableCount != 42 {
		t.Fatalf("expected retryable count from count query, got %d", bundle.RetryableCount)
	}
	if bundle.RetryReadyCount != 12 || bundle.RetryWaitingCount != 30 || bundle.RetryExhaustedCount != 4 {
		t.Fatalf(
			"expected retry policy counts from count query, got ready=%d waiting=%d exhausted=%d",
			bundle.RetryReadyCount,
			bundle.RetryWaitingCount,
			bundle.RetryExhaustedCount,
		)
	}
	if bundle.LogMissingCount != 7 {
		t.Fatalf("expected missing-log count from count query, got %d", bundle.LogMissingCount)
	}
	if bundle.ExecutionSummary == nil || len(bundle.ExecutionSummary.Checklist) == 0 {
		t.Fatalf("expected support bundle execution checklist, got %#v", bundle.ExecutionSummary)
	}
	if bundle.ExecutionSummary.Decision != "retry" {
		t.Fatalf("expected retry decision in support bundle, got %q", bundle.ExecutionSummary.Decision)
	}
	if len(bundle.RetryableDeviceIDs) != 1 || bundle.RetryableDeviceIDs[0] != "retry-1" {
		t.Fatalf("expected filtered retryable device IDs, got %#v", bundle.RetryableDeviceIDs)
	}
	if len(bundle.MissingLogDeviceIDs) != 1 || bundle.MissingLogDeviceIDs[0] != "missing-log-1" {
		t.Fatalf("expected filtered missing-log device IDs, got %#v", bundle.MissingLogDeviceIDs)
	}
	if len(bundle.FailedDevices) != 2 {
		t.Fatalf("expected failed row and device-ack-failed row, got %#v", bundle.FailedDevices)
	}
	if bundle.FailedDevices[1].ResponseStatusLabel != "device_ack_failed" {
		t.Fatalf("expected device ack failure label, got %q", bundle.FailedDevices[1].ResponseStatusLabel)
	}
}

func TestCommandJobResultPreservesIdentifyForDetailAndSummaryResponses(t *testing.T) {
	job := &model.CommandJob{
		ID:             "job-identify",
		JobType:        "command",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Identify:       "test_dry_contact",
		Status:         commandJobStatusCompleted,
		RequestedCount: 1,
		EligibleCount:  1,
		SubmittedCount: 1,
	}
	detail := &model.CommandJobDetail{
		ID:           "detail-identify",
		CommandJobID: job.ID,
		DeviceID:     "device-identify",
		Status:       commandJobDetailStatusSubmitted,
		Eligible:     true,
	}
	result := commandJobResultFromPersistence(job, []*model.CommandJobDetail{detail}, map[string]int{
		commandJobDetailStatusSubmitted: 1,
	}, nil)
	if result.Identify != job.Identify {
		t.Fatalf("detail identify = %q, want %q", result.Identify, job.Identify)
	}
	summary := commandJobResultSummaryFromPersistence(job, map[string]int{
		commandJobDetailStatusSubmitted: 1,
	}, 0, 0, 0, 0, 0, nil)
	if summary.Identify != job.Identify {
		t.Fatalf("summary identify = %q, want %q", summary.Identify, job.Identify)
	}
}

func TestCommandJobResultDerivesCanRetryFailedFromFreshRetryReadyDetails(t *testing.T) {
	job := &model.CommandJob{
		ID:             "job-retry-ready",
		JobType:        "command",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Identify:       "e2e_forced_failure",
		Status:         commandJobStatusPartiallyFailed,
		CanRetryFailed: false, // stale persisted summary reproduced by the ACK race
		RequestedCount: 1,
		FailedCount:    1,
	}
	nextRetryAfter := time.Now().UTC().Add(-time.Second)
	detail := &model.CommandJobDetail{
		ID:               "detail-retry-ready",
		CommandJobID:     job.ID,
		DeviceID:         "device-retry-ready",
		Status:           commandJobDetailStatusFailed,
		CanRetry:         true,
		DispatchAttempts: 1,
		NextRetryAfter:   &nextRetryAfter,
	}

	result := commandJobResultFromPersistence(job, []*model.CommandJobDetail{detail}, map[string]int{
		commandJobDetailStatusFailed: 1,
	}, nil)
	if !result.CanRetryFailed {
		t.Fatalf("detail response must derive can_retry_failed from retry-ready details, got %#v", result)
	}

	summary := commandJobResultSummaryFromPersistence(job, map[string]int{
		commandJobDetailStatusFailed: 1,
	}, 1, 1, 0, 0, 0, nil)
	if !summary.CanRetryFailed {
		t.Fatalf("summary response must derive can_retry_failed from retry_ready_count, got %#v", summary)
	}
}

func TestCommandJobSupportBundleShowsCanceledInFlightRowsWithoutRetryAdvice(t *testing.T) {
	messageID := "msg-in-flight"
	job := &model.CommandJob{
		ID:             "job-canceled",
		JobType:        "service",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Identify:       "reboot",
		Status:         commandJobStatusCanceled,
		RequestedCount: 1,
		EligibleCount:  1,
		FailedCount:    0,
	}
	supportDetails := []*model.CommandJobDetail{
		{
			DeviceID:         "in-flight-1",
			DeviceNumber:     "DN-IF",
			Name:             "in flight",
			Status:           commandJobDetailStatusDispatching,
			MessageID:        &messageID,
			DispatchAttempts: 1,
			CanRetry:         true,
		},
	}

	bundle := commandJobSupportBundleFromPersistence(
		job,
		supportDetails,
		map[string]int{commandJobDetailStatusDispatching: 1},
		1,
		1,
		0,
		0,
		0,
		nil,
	)

	if len(bundle.FailedDevices) != 1 {
		t.Fatalf("expected canceled in-flight row in support evidence, got %#v", bundle.FailedDevices)
	}
	diagnostic := bundle.FailedDevices[0].DiagnosticSummary
	if diagnostic == nil || diagnostic.Code != "cancel_in_flight" {
		t.Fatalf("expected cancel_in_flight diagnostic, got %#v", diagnostic)
	}
	for _, action := range bundle.NextActions {
		if strings.Contains(strings.ToLower(action), "retry the ready devices") {
			t.Fatalf("canceled support bundle should not suggest retrying ready devices, actions=%#v", bundle.NextActions)
		}
	}
}
