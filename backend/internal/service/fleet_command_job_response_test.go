package service

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
)

func TestApplyFleetCommandJobDeviceAckFailureMarksRowRetryable(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	detail := &model.CommandJobDetail{
		Status:           commandJobDetailStatusSubmitted,
		DispatchAttempts: 1,
	}

	changed := applyFleetCommandJobDeviceAckState(detail, strconv.Itoa(constant.ResponseSStatusFailed), "bad payload", now)

	if !changed || detail.Status != commandJobDetailStatusFailed {
		t.Fatalf("expected failed row state, changed=%v detail=%#v", changed, detail)
	}
	if !detail.CanRetry || detail.NextRetryAfter == nil {
		t.Fatalf("expected retryable ack failure, detail=%#v", detail)
	}
	if detail.Reason == nil || *detail.Reason == "" {
		t.Fatalf("expected customer-visible reason, detail=%#v", detail)
	}
}

func TestApplyFleetCommandJobDeviceAckSuccessCorrectsRecoveredFailure(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	retryAfter := now.Add(time.Minute)
	detail := &model.CommandJobDetail{
		Status:         commandJobDetailStatusFailed,
		CanRetry:       true,
		NextRetryAfter: &retryAfter,
		Reason:         StringPtr("restart marked this row failed"),
		Advice:         StringPtr("retry later"),
	}

	changed := applyFleetCommandJobDeviceAckState(detail, strconv.Itoa(constant.ResponseStatusOk), "", now)

	if !changed || detail.Status != commandJobDetailStatusSubmitted {
		t.Fatalf("expected submitted row state, changed=%v detail=%#v", changed, detail)
	}
	if detail.CanRetry || detail.NextRetryAfter != nil || detail.Reason != nil || detail.Advice != nil {
		t.Fatalf("expected success ack to clear retry/failure evidence, detail=%#v", detail)
	}
}

func TestApplyFleetCommandJobDeviceAckFailureOnCanceledJobDoesNotRequeue(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	detail := &model.CommandJobDetail{
		Status:           commandJobDetailStatusDispatching,
		DispatchAttempts: 1,
	}

	changed := applyFleetCommandJobDeviceAckState(detail, strconv.Itoa(constant.ResponseSStatusFailed), "late nack", now, true)

	if !changed || detail.Status != commandJobDetailStatusFailed {
		t.Fatalf("expected late ack failure evidence on canceled job, changed=%v detail=%#v", changed, detail)
	}
	if detail.CanRetry || detail.NextRetryAfter != nil {
		t.Fatalf("expected canceled job late ack to stay non-retryable, detail=%#v", detail)
	}
	if detail.Reason == nil || !strings.Contains(*detail.Reason, "canceled job") {
		t.Fatalf("expected canceled-job reason, detail=%#v", detail)
	}
	if detail.Advice == nil || !strings.Contains(*detail.Advice, "fresh preview") {
		t.Fatalf("expected fresh-preview advice, detail=%#v", detail)
	}
}

func TestCommandJobDeviceAckAmbiguousEventMessageSaysNotApplied(t *testing.T) {
	message := commandJobDeviceAckAmbiguousEventMessage("msg-1", strconv.Itoa(constant.ResponseSStatusFailed), "duplicate candidate")

	if !strings.Contains(message, "not applied") || !strings.Contains(message, "multiple command job detail candidates") {
		t.Fatalf("ambiguous ack event should explain that the response was not applied, got %q", message)
	}
	if !strings.Contains(message, "duplicate candidate") {
		t.Fatalf("ambiguous ack event should retain response error context, got %q", message)
	}
}
