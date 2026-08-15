package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildOTAUpgradeTaskSupportBundleKeepsTaskLevelFailureEvidence(t *testing.T) {
	selectedCount := 3
	previewTotal := int64(4)
	task := &model.OtaUpgradeTask{
		ID:                  "task-1",
		Name:                "Field rollout",
		OtaUpgradePackageID: "pkg-1",
		TargetMode:          "filter",
		PreviewTotal:        &previewTotal,
		SelectedCount:       &selectedCount,
		CreatedAt:           time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC),
	}
	rows := []map[string]interface{}{
		{
			"id":                 "detail-1",
			"device_id":          "dev-1",
			"device_number":      "DN-1",
			"name":               "Pump 1",
			"current_version":    "1.0",
			"version":            "2.0",
			"steps":              45,
			"status":             model.OtaUpgradeTaskDetailStatusFailed,
			"status_description": "verify failed",
		},
		{
			"id":                 "detail-2",
			"device_id":          "dev-2",
			"status":             "5",
			"status_description": "verify failed",
		},
		{
			"id":        "detail-3",
			"device_id": "dev-3",
			"status":    model.OtaUpgradeTaskDetailStatusSucceeded,
		},
	}

	failureGroups := normaliseOTAFailureGroups([]model.OTAUpgradeTaskFailureGroup{{Reason: "verify failed", Count: 2}})
	bundle := buildOTAUpgradeTaskSupportBundle(task, rows, []map[string]interface{}{{"status": "5", "count": 2}}, failureGroups, 3, 2)

	if bundle.TaskID != "task-1" || bundle.PackageID != "pkg-1" {
		t.Fatalf("unexpected task identity in bundle: %#v", bundle)
	}
	if bundle.TotalRows != 3 || bundle.FailedCount != 2 {
		t.Fatalf("unexpected counts: total=%d failed=%d", bundle.TotalRows, bundle.FailedCount)
	}
	if len(bundle.FailedDevices) != 2 {
		t.Fatalf("expected two failed devices, got %#v", bundle.FailedDevices)
	}
	if bundle.FailedDevices[0].ReadyCheckURL != "/device/details?d_id=dev-1&tab=ready-check&source=ota&ota_task_id=task-1&ota_detail_id=detail-1" {
		t.Fatalf("unexpected ready check url: %q", bundle.FailedDevices[0].ReadyCheckURL)
	}
	if len(bundle.FailureGroups) != 1 || bundle.FailureGroups[0].Reason != "verify failed" || bundle.FailureGroups[0].Count != 2 {
		t.Fatalf("unexpected failure groups: %#v", bundle.FailureGroups)
	}
	if len(bundle.EvidenceBoundary) == 0 || len(bundle.NextActions) == 0 {
		t.Fatalf("expected boundary and next actions: %#v", bundle)
	}
}

func TestBuildOTAUpgradeTaskSupportBundleSeparatesSamplesFromTaskCounts(t *testing.T) {
	task := &model.OtaUpgradeTask{ID: "task-large", Name: "Large rollout", OtaUpgradePackageID: "pkg-1"}
	rows := []map[string]interface{}{
		{
			"id":                 "detail-sample",
			"device_id":          "dev-sample",
			"status":             model.OtaUpgradeTaskDetailStatusFailed,
			"status_description": "offline",
		},
	}
	failureGroups := normaliseOTAFailureGroups([]model.OTAUpgradeTaskFailureGroup{
		{Reason: "offline", Count: 120},
		{Reason: "", Count: 3},
	})

	bundle := buildOTAUpgradeTaskSupportBundle(task, rows, []map[string]interface{}{{"status": "5", "count": 123}}, failureGroups, 500, 123)

	if bundle.TotalRows != 500 || bundle.FailedCount != 123 {
		t.Fatalf("expected task-level counts, got total=%d failed=%d", bundle.TotalRows, bundle.FailedCount)
	}
	if len(bundle.FailedDevices) != 1 {
		t.Fatalf("expected only sampled failed device rows in bundle, got %#v", bundle.FailedDevices)
	}
	if len(bundle.FailureGroups) != 2 || bundle.FailureGroups[0].Count != 120 || bundle.FailureGroups[1].Reason != "No failure reason returned" {
		t.Fatalf("expected full-task failure groups, got %#v", bundle.FailureGroups)
	}
}
