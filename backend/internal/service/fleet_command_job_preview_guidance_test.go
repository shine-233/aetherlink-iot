package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestAttachFleetCommandJobPreviewGuidanceBuildsCustomerActionPlan(t *testing.T) {
	result := attachFleetCommandJobPreviewGuidance(&model.FleetCommandJobPreviewResult{
		RequestedCount: 3,
		EligibleCount:  2,
		BlockedCount:   1,
		Rows: []model.FleetCommandJobPreviewRow{
			{DeviceID: "online-1", Eligible: true, RecommendedPath: "immediate", TelemetryCurrentCount: 2},
			{DeviceID: "offline-1", Eligible: true, RecommendedPath: "jobs"},
			{DeviceID: "blocked-1", Eligible: false, RecommendedPath: "blocked", Reason: "device offline", Advice: "check connection"},
		},
	})

	if result.PathCounts.Immediate != 1 || result.PathCounts.Jobs != 1 || result.PathCounts.Blocked != 1 || result.PathCounts.Telemetry != 1 {
		t.Fatalf("unexpected path counts: %#v", result.PathCounts)
	}
	if len(result.Blockers) != 1 || result.Blockers[0].Reason != "device offline" || result.Blockers[0].Count != 1 {
		t.Fatalf("unexpected blockers: %#v", result.Blockers)
	}
	if result.NextAction != "请先核对被阻断设备的原因，再只提交可执行设备。" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
}

func TestAttachFleetCommandJobPreviewGuidanceFlagsPreviewSubsetEvidence(t *testing.T) {
	result := attachFleetCommandJobPreviewGuidance(&model.FleetCommandJobPreviewResult{
		RequestedCount: 50,
		EligibleCount:  10,
		Rows: []model.FleetCommandJobPreviewRow{
			{DeviceID: "subset-1", Eligible: true, RecommendedPath: "jobs"},
		},
	})

	if result.NextAction != "当前只是预览子集证据；请缩小筛选范围或提高上限，直到预览覆盖已确认的目标范围。" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
}
