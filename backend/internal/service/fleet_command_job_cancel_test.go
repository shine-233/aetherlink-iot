package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestApplyFleetCommandJobCancelRequestStopsFurtherDispatch(t *testing.T) {
	job := &model.CommandJob{
		Status:         commandJobStatusRunning,
		CanCancel:      true,
		CanRetryFailed: true,
	}

	applyFleetCommandJobCancelRequest(job, 3)

	if job.Status != commandJobStatusCanceled {
		t.Fatalf("status = %q, want %q", job.Status, commandJobStatusCanceled)
	}
	if job.CanCancel || job.CanRetryFailed {
		t.Fatalf("cancel request should disable further cancel/retry, job=%#v", job)
	}
	if job.Remark == nil || !strings.Contains(*job.Remark, "3") || !strings.Contains(*job.Remark, "待处理") {
		t.Fatalf("cancel remark should include pending row evidence, remark=%#v", job.Remark)
	}
}

func TestCommandJobCancelEventMessageExplainsInFlightRows(t *testing.T) {
	message := commandJobCancelEventMessage(0)
	if !strings.Contains(message, "下发中") || !strings.Contains(message, "ACK") || !strings.Contains(message, "支持证据") {
		t.Fatalf("cancel event should explain in-flight row evidence, got %q", message)
	}
}
