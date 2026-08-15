// 文件用途：ota_rollout_governance_preview.go 中纯映射函数 buildOTARolloutGovernanceInput 的单元测试。
// 覆盖:detail 分状态计数 → 规划器入参的映射语义(upgrading=pushed+upgrading;canceled 不计入;
// task 行的调度/限速/中止配置透传)。只测纯映射,不触 DB/PreviewRolloutGovernance 的 DAL 路径。
package service

import (
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

func TestBuildOTARolloutGovernanceInput_MapsCountsAndConfig(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	scheduled := now.Add(-time.Hour)
	timeout := now.Add(time.Hour)
	windowStart := now.Add(-30 * time.Second)
	abort := 25.0

	task := &model.OtaUpgradeTask{
		Status:                  "running",
		ScheduledAt:             &scheduled,
		TimeoutAt:               &timeout,
		RolloutRatePerMinute:    60,
		AbortFailureRatePercent: &abort,
		RateWindowStartedAt:     &windowStart,
		RateWindowDispatched:    5,
	}
	counts := map[int16]int{
		model.OtaUpgradeTaskDetailStatusPending:   10,
		model.OtaUpgradeTaskDetailStatusPushed:    3,
		model.OtaUpgradeTaskDetailStatusUpgrading: 2,
		model.OtaUpgradeTaskDetailStatusSucceeded: 7,
		model.OtaUpgradeTaskDetailStatusFailed:    1,
		model.OtaUpgradeTaskDetailStatusCanceled:  4,
	}

	in := buildOTARolloutGovernanceInput(task, counts, now)

	if in.Status != "running" {
		t.Fatalf("Status = %q, want running", in.Status)
	}
	if !in.Now.Equal(now) {
		t.Fatalf("Now = %v, want %v", in.Now, now)
	}
	if in.PendingCount != 10 {
		t.Fatalf("PendingCount = %d, want 10", in.PendingCount)
	}
	// upgrading = pushed(3) + upgrading(2)
	if in.UpgradingCount != 5 {
		t.Fatalf("UpgradingCount = %d, want 5 (pushed+upgrading)", in.UpgradingCount)
	}
	if in.SucceededCount != 7 {
		t.Fatalf("SucceededCount = %d, want 7", in.SucceededCount)
	}
	if in.FailedCount != 1 {
		t.Fatalf("FailedCount = %d, want 1", in.FailedCount)
	}
	// canceled(4) is deliberately NOT surfaced anywhere in the input.
	if in.RolloutRatePerMinute != 60 {
		t.Fatalf("RolloutRatePerMinute = %d, want 60", in.RolloutRatePerMinute)
	}
	if in.AbortFailureRatePercent == nil || *in.AbortFailureRatePercent != 25.0 {
		t.Fatalf("AbortFailureRatePercent = %v, want 25", in.AbortFailureRatePercent)
	}
	if in.RateWindowDispatched != 5 {
		t.Fatalf("RateWindowDispatched = %d, want 5", in.RateWindowDispatched)
	}
	if in.ScheduledAt == nil || !in.ScheduledAt.Equal(scheduled) {
		t.Fatalf("ScheduledAt = %v, want %v", in.ScheduledAt, scheduled)
	}
	if in.TimeoutAt == nil || !in.TimeoutAt.Equal(timeout) {
		t.Fatalf("TimeoutAt = %v, want %v", in.TimeoutAt, timeout)
	}
	if in.RateWindowStartedAt == nil || !in.RateWindowStartedAt.Equal(windowStart) {
		t.Fatalf("RateWindowStartedAt = %v, want %v", in.RateWindowStartedAt, windowStart)
	}
}

func TestBuildOTARolloutGovernanceInput_EmptyCountsAreZero(t *testing.T) {
	now := time.Now().UTC()
	task := &model.OtaUpgradeTask{Status: "running", RolloutRatePerMinute: 30}

	in := buildOTARolloutGovernanceInput(task, map[int16]int{}, now)

	if in.PendingCount != 0 || in.UpgradingCount != 0 || in.SucceededCount != 0 || in.FailedCount != 0 {
		t.Fatalf("expected all counts zero, got %+v", in)
	}
	if in.AbortFailureRatePercent != nil {
		t.Fatalf("AbortFailureRatePercent = %v, want nil", in.AbortFailureRatePercent)
	}
}

// 映射接通后,喂给纯规划器应得到一个非空动作(不 panic、决策可用)。
func TestBuildOTARolloutGovernanceInput_FeedsPlanner(t *testing.T) {
	now := time.Now().UTC()
	task := &model.OtaUpgradeTask{Status: "running", RolloutRatePerMinute: 60}
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusPending: 5}

	decision := PlanOTARolloutGovernance(buildOTARolloutGovernanceInput(task, counts, now))
	if decision.Action == "" {
		t.Fatal("expected a non-empty rollout action from the planner")
	}
}
