package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestBuildFleetCommandJobHandoffSummaryIncludesCustomerNextAction(t *testing.T) {
	summary := buildFleetCommandJobHandoffSummary(
		&model.CommandJob{
			ID:             "job-1",
			Status:         commandJobStatusRunning,
			RequestedCount: 10,
			SubmittedCount: 6,
			FailedCount:    1,
			BlockedCount:   2,
		},
		&model.FleetCommandJobProgressHealth{
			State:      "timeout_risk",
			NextAction: "Watch closely and prepare support evidence.",
		},
		&model.FleetCommandJobExecutionSummary{
			CanClose:      false,
			CloseBlockers: []string{"3 devices are still pending."},
			NextAction:    "Review close blockers before handoff.",
		},
	)

	for _, expected := range []string{
		"命令任务 job-1 当前状态 running",
		"已提交 6/10 台",
		"失败 1 台",
		"阻断 2 台",
		"健康状态=timeout_risk",
		"close_ready=no",
		"3 devices are still pending.",
		"Review close blockers before handoff.",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary %q does not contain %q", summary, expected)
		}
	}
}
