package service

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildFleetCommandJobAuditSummaryUsesLatestEvent(t *testing.T) {
	first := time.Date(2026, 7, 7, 5, 0, 0, 0, time.UTC)
	latest := first.Add(2 * time.Minute)
	summary := buildFleetCommandJobAuditSummary([]*model.CommandJobEvent{
		{EventType: commandJobEventCreated, Message: "created", CreatedAt: first},
		{EventType: commandJobEventTimeout, Message: "timed out", CreatedAt: latest},
	})

	if summary.EventCount != 2 {
		t.Fatalf("event count = %d, want 2", summary.EventCount)
	}
	if summary.LatestEventType != commandJobEventTimeout || summary.LatestMessage != "timed out" {
		t.Fatalf("unexpected latest event: %#v", summary)
	}
	if !strings.Contains(summary.NextAction, "超时") || !strings.Contains(summary.NextAction, "支持证据") {
		t.Fatalf("unexpected next action: %q", summary.NextAction)
	}
}

func TestBuildFleetCommandJobAuditSummaryHandlesMissingEvents(t *testing.T) {
	summary := buildFleetCommandJobAuditSummary(nil)
	if summary.EventCount != 0 || summary.LatestEventType != "" {
		t.Fatalf("unexpected empty summary: %#v", summary)
	}
}

func TestCommandJobAuditNextActionExplainsWorkerFailure(t *testing.T) {
	action := commandJobAuditNextAction(commandJobEventWorkerFailed)
	if action == "" || action == commandJobAuditNextAction("unknown") {
		t.Fatalf("worker failure next action should be explicit, got %q", action)
	}
}

func TestCommandJobAuditNextActionExplainsDeviceAckFailure(t *testing.T) {
	action := commandJobAuditNextAction(commandJobEventDeviceAckFailed)
	if action == "" || action == commandJobAuditNextAction("unknown") {
		t.Fatalf("device ack failure next action should be explicit, got %q", action)
	}
}

func TestCommandJobAuditNextActionExplainsAmbiguousDeviceAck(t *testing.T) {
	action := commandJobAuditNextAction(commandJobEventDeviceAckAmbiguous)
	if action == "" || action == commandJobAuditNextAction("unknown") {
		t.Fatalf("ambiguous device ack next action should be explicit, got %q", action)
	}
}
