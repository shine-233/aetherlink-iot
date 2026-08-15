// 文件用途：OTA rollout 治理规划器的纯单元测试（离线,不依赖 PG/broker/设备）。
package service

import (
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

func TestPlanOTARolloutGovernanceCanceledHolds(t *testing.T) {
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:       "canceled",
		Now:          time.Now().UTC(),
		PendingCount: 5,
	})
	if d.Action != OTARolloutActionHold {
		t.Fatalf("canceled 应保持 hold, got %s", d.Action)
	}
	if !d.IsSimulation {
		t.Fatalf("规划器应标记为模拟")
	}
}

func TestPlanOTARolloutGovernanceTimeoutBeatsDispatch(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		TimeoutAt:            &past,
		RolloutRatePerMinute: 100,
		PendingCount:         10,
		UpgradingCount:       3,
	})
	if d.Action != OTARolloutActionTimeout {
		t.Fatalf("越过 timeout_at 应超时, got %s", d.Action)
	}
	if d.RemainingInvalid != 13 {
		t.Fatalf("超时应汇总 pending+upgrading=13, got %d", d.RemainingInvalid)
	}
}

func TestPlanOTARolloutGovernanceAbortsOnFailureRate(t *testing.T) {
	now := time.Now().UTC()
	threshold := 25.0
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:                  "running",
		Now:                     now,
		AbortFailureRatePercent: &threshold,
		RolloutRatePerMinute:    100,
		PendingCount:            10,
		SucceededCount:          6,
		FailedCount:             4, // 4/(6+4)=40% >= 25%
	})
	if d.Action != OTARolloutActionAbort {
		t.Fatalf("失败率越过阈值应中止, got %s", d.Action)
	}
	if d.FailureRate < 39.9 || d.FailureRate > 40.1 {
		t.Fatalf("失败率应约为 40%%, got %.2f", d.FailureRate)
	}
}

func TestPlanOTARolloutGovernanceDoesNotAbortWithoutSample(t *testing.T) {
	now := time.Now().UTC()
	threshold := 25.0
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:                  "running",
		Now:                     now,
		AbortFailureRatePercent: &threshold,
		RolloutRatePerMinute:    100,
		PendingCount:            10,
		// 无任何设备进入终态 → 不应因失败率中止
	})
	if d.Action == OTARolloutActionAbort {
		t.Fatalf("没有终态样本时不应中止, got %s", d.Action)
	}
}

func TestPlanOTARolloutGovernanceWaitsForSchedule(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		ScheduledAt:          &future,
		RolloutRatePerMinute: 100,
		PendingCount:         10,
	})
	if d.Action != OTARolloutActionWaitSchedule {
		t.Fatalf("未到计划时间应等待, got %s", d.Action)
	}
}

func TestPlanOTARolloutGovernanceDispatchesRateBoundedBatch(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 60,
		RateWindowDispatched: 50,
		PendingCount:         100,
	})
	if d.Action != OTARolloutActionDispatchBatch {
		t.Fatalf("窗口内应下发, got %s", d.Action)
	}
	if d.BatchSize != 10 {
		t.Fatalf("窗口剩余 60-50=10, 批量应为 10, got %d", d.BatchSize)
	}
}

func TestPlanOTARolloutGovernanceBatchCappedByPending(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 60,
		RateWindowDispatched: 0,
		PendingCount:         5,
	})
	if d.Action != OTARolloutActionDispatchBatch {
		t.Fatalf("应下发, got %s", d.Action)
	}
	if d.BatchSize != 5 {
		t.Fatalf("批量不应超过待下发数 5, got %d", d.BatchSize)
	}
}

func TestPlanOTARolloutGovernanceHoldsWhenRateWindowFull(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 60,
		RateWindowDispatched: 60,
		PendingCount:         100,
	})
	if d.Action != OTARolloutActionHoldRateWindow {
		t.Fatalf("窗口已满应保持, got %s", d.Action)
	}
}

func TestPlanOTARolloutGovernanceCompletesWhenDrained(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 60,
		PendingCount:         0,
		UpgradingCount:       0,
		SucceededCount:       10,
	})
	if d.Action != OTARolloutActionComplete {
		t.Fatalf("无待下发且无升级中应完成, got %s", d.Action)
	}
}

func TestPlanOTARolloutGovernanceHoldsForUpgradingCallback(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 60,
		PendingCount:         0,
		UpgradingCount:       4,
	})
	if d.Action != OTARolloutActionHold {
		t.Fatalf("无待下发但仍有升级中应保持等待回调, got %s", d.Action)
	}
}

func TestPlanOTARolloutGovernanceNonPositiveRateFallsBack(t *testing.T) {
	now := time.Now().UTC()
	d := PlanOTARolloutGovernance(model.OTARolloutGovernanceInput{
		Status:               "running",
		Now:                  now,
		RolloutRatePerMinute: 0,
		PendingCount:         10,
	})
	if d.Action != OTARolloutActionDispatchBatch {
		t.Fatalf("非正限速应回退为每分钟 1 台并下发, got %s", d.Action)
	}
	if d.BatchSize != 1 {
		t.Fatalf("回退限速批量应为 1, got %d", d.BatchSize)
	}
	if len(d.Warnings) == 0 {
		t.Fatalf("非正限速应产生告警")
	}
}
