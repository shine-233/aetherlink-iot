package service

import (
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func twinSummary(status, nextAction string, delta, unavailable, stale, desired int) model.DeviceTwinSummary {
	return model.DeviceTwinSummary{
		DesiredCount:      desired,
		DeltaCount:        delta,
		UnavailableCount:  unavailable,
		StaleDesiredCount: stale,
		ConvergenceStatus: status,
		NextAction:        nextAction,
		EvidenceBoundary:  "platform_visible_evidence_only",
	}
}

func TestBuildDeviceTwinDriftIndexRanksBySeverityAndCounts(t *testing.T) {
	index := BuildDeviceTwinDriftIndex([]DeviceTwinDriftInput{
		{DeviceID: "dev-ready", Summary: twinSummary("ready", "safe_to_continue_after_review", 0, 0, 0, 3)},
		{DeviceID: "dev-drift", DeviceName: "boiler", Summary: twinSummary("needs_review", "compare_delta_before_device_action", 2, 0, 0, 4)},
		{DeviceID: "dev-expired", Summary: twinSummary("expired_desired", "review_expired_desired_state", 0, 0, 1, 2)},
		{DeviceID: "dev-waiting", Summary: twinSummary("waiting_reported", "wait_for_reported_state", 0, 1, 0, 1)},
		{DeviceID: "dev-none", Summary: twinSummary("no_desired", "create_desired_state", 0, 0, 0, 0)},
	})

	if index.TotalDevices != 5 {
		t.Fatalf("TotalDevices = %d, want 5", index.TotalDevices)
	}
	if index.DriftDevices != 1 || index.ExpiredDevices != 1 || index.WaitingDevices != 1 || index.ReadyDevices != 1 || index.NoDesiredDevices != 1 {
		t.Fatalf("classification counts wrong: %+v", index)
	}

	// needs_review (severity 40) must rank first, ready (0) last.
	wantOrder := []string{"dev-drift", "dev-expired", "dev-waiting", "dev-none", "dev-ready"}
	if len(index.Entries) != len(wantOrder) {
		t.Fatalf("entries = %d, want %d", len(index.Entries), len(wantOrder))
	}
	for i, want := range wantOrder {
		if index.Entries[i].DeviceID != want {
			t.Fatalf("entry[%d] = %q, want %q (full order: %v)", i, index.Entries[i].DeviceID, want, driftOrder(index))
		}
	}

	if index.Entries[0].DeviceName != "boiler" || index.Entries[0].DeltaCount != 2 {
		t.Fatalf("top drift entry lost detail: %+v", index.Entries[0])
	}
}

func TestBuildDeviceTwinDriftIndexStableTieBreakByDeviceID(t *testing.T) {
	index := BuildDeviceTwinDriftIndex([]DeviceTwinDriftInput{
		{DeviceID: "dev-c", Summary: twinSummary("needs_review", "x", 1, 0, 0, 1)},
		{DeviceID: "dev-a", Summary: twinSummary("needs_review", "x", 1, 0, 0, 1)},
		{DeviceID: "dev-b", Summary: twinSummary("needs_review", "x", 1, 0, 0, 1)},
	})

	wantOrder := []string{"dev-a", "dev-b", "dev-c"}
	for i, want := range wantOrder {
		if index.Entries[i].DeviceID != want {
			t.Fatalf("entry[%d] = %q, want %q", i, index.Entries[i].DeviceID, want)
		}
	}
}

func TestBuildDeviceTwinDriftIndexEmpty(t *testing.T) {
	index := BuildDeviceTwinDriftIndex(nil)
	if index.TotalDevices != 0 || len(index.Entries) != 0 {
		t.Fatalf("empty index not empty: %+v", index)
	}
	if index.EvidenceBoundary != "platform_visible_evidence_only" {
		t.Fatalf("evidence boundary = %q", index.EvidenceBoundary)
	}
}

func driftOrder(index model.DeviceTwinDriftIndex) []string {
	ids := make([]string, 0, len(index.Entries))
	for _, e := range index.Entries {
		ids = append(ids, e.DeviceID)
	}
	return ids
}
