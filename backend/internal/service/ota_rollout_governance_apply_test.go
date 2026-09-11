package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

// P0.3 执行面证据：规划器早已存在，但只有只读预览端点；
// 决策算得出来却没人执行，金丝雀的门控实际不生效。
// 本文件用可注入的 applier 验证"决策 -> 执行"的映射真的发生了。

type governanceApplySpy struct {
	claimed   []int
	dispatched [][]*model.OtaUpgradeTaskDetail
	patches    []dal.OTARolloutTaskPatch
	cancels    []string
}

func withGovernanceApplier(t *testing.T, counts map[int16]int, task *model.OtaUpgradeTask, pending []*model.OtaUpgradeTaskDetail) (*governanceApplySpy, *OTARolloutGovernanceApplyResult) {
	t.Helper()
	spy := &governanceApplySpy{}
	original := otaGovernanceApplier
	t.Cleanup(func() { otaGovernanceApplier = original })

	otaGovernanceApplier.counts = func(string) (map[int16]int, error) { return counts, nil }
	otaGovernanceApplier.claimBatch = func(_ string, limit int) ([]*model.OtaUpgradeTaskDetail, error) {
		spy.claimed = append(spy.claimed, limit)
		if limit > len(pending) {
			limit = len(pending)
		}
		return pending[:limit], nil
	}
	otaGovernanceApplier.dispatch = func(_ *OTA, details []*model.OtaUpgradeTaskDetail) {
		spy.dispatched = append(spy.dispatched, details)
	}
	otaGovernanceApplier.patchTask = func(_ string, patch dal.OTARolloutTaskPatch) (int64, error) {
		spy.patches = append(spy.patches, patch)
		return 1, nil
	}
	otaGovernanceApplier.cancelRest = func(_ string, reason string, _ time.Time) (int64, error) {
		spy.cancels = append(spy.cancels, reason)
		return 7, nil
	}
	otaGovernanceApplier.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	// 直接复用私有的执行函数，绕开 ensureOTATaskAccess 的 DB 依赖。
	automate := &OTA{}
	decision := PlanOTARolloutGovernance(buildOTARolloutGovernanceInput(task, counts, otaGovernanceApplier.now()))
	result := &OTARolloutGovernanceApplyResult{Decision: decision, IsSimulation: false}
	var err error
	switch decision.Action {
	case OTARolloutActionDispatchBatch:
		result, err = automate.applyDispatchBatch("task-1", task, decision, result)
	case OTARolloutActionAbort:
		result, err = automate.applyAbort("task-1", decision, result, otaGovernanceApplier.now())
	case OTARolloutActionTimeout:
		result, err = automate.applyTimeout("task-1", decision, result, otaGovernanceApplier.now())
	case OTARolloutActionComplete:
		result, err = automate.applyComplete("task-1", decision, result)
	default:
		result.Detail = "no action required for " + decision.Action
	}
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	return spy, result
}

// 金丝雀的核心：按限速放一批，且限速窗口计数必须累加，否则下一轮会重复放量。
func TestApplyRolloutGovernanceDispatchesRateLimitedBatch(t *testing.T) {
	rate := 5
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusPending: 100}
	task := &model.OtaUpgradeTask{RolloutRatePerMinute: rate, RateWindowDispatched: 2}
	pending := make([]*model.OtaUpgradeTaskDetail, 10)
	for i := range pending {
		pending[i] = &model.OtaUpgradeTaskDetail{ID: "d"}
	}
	spy, result := withGovernanceApplier(t, counts, task, pending)

	if result.Decision.Action != OTARolloutActionDispatchBatch {
		t.Fatalf("action = %q, want dispatch_batch", result.Decision.Action)
	}
	if !result.Applied || result.Dispatched == 0 {
		t.Fatalf("canary must actually dispatch; applied=%v dispatched=%d", result.Applied, result.Dispatched)
	}
	// 窗口剩余 3（rate 5 - 已发 2），即使有 10 条 pending 也只应取 3 条。
	if len(spy.claimed) == 0 || spy.claimed[0] != 3 {
		t.Fatalf("claim limit = %v, want [3] (rate window remaining)", spy.claimed)
	}
	if len(spy.dispatched) == 0 || len(spy.dispatched[0]) != 3 {
		t.Fatalf("dispatched count = %v, want 3", len(spy.dispatched[0]))
	}
	// 计数必须先记账：2 + 3 = 5。
	if len(spy.patches) == 0 || spy.patches[0].RateWindowDispatched == nil || *spy.patches[0].RateWindowDispatched != 5 {
		t.Fatalf("rate window counter not accumulated: %+v", spy.patches)
	}
	if result.IsSimulation {
		t.Fatal("applied result must not be marked as simulation")
	}
}

// 失败率越线：必须同时停掉剩余 pending，只打 task 标记等于没中止。
func TestApplyRolloutGovernanceAbortCancelsRemainingPending(t *testing.T) {
	threshold := 50.0
	counts := map[int16]int{
		model.OtaUpgradeTaskDetailStatusFailed:    10,
		model.OtaUpgradeTaskDetailStatusSucceeded: 5,
		model.OtaUpgradeTaskDetailStatusPending:   80,
	}
	task := &model.OtaUpgradeTask{AbortFailureRatePercent: &threshold, RolloutRatePerMinute: 10}
	spy, result := withGovernanceApplier(t, counts, task, nil)

	if result.Decision.Action != OTARolloutActionAbort {
		t.Fatalf("action = %q, want abort", result.Decision.Action)
	}
	if spy.patches == nil || spy.patches[0].Status != "canceled" {
		t.Fatalf("task must be marked canceled; patches=%+v", spy.patches)
	}
	if len(spy.cancels) != 1 {
		t.Fatalf("remaining pending must be canceled; cancels=%v", spy.cancels)
	}
	if result.CanceledRows != 7 {
		t.Fatalf("canceled rows = %d, want 7", result.CanceledRows)
	}
	// 中止时绝不能再下发。
	if len(spy.dispatched) != 0 {
		t.Fatalf("abort must not dispatch any device; dispatched=%d batches", len(spy.dispatched))
	}
}

// 越过绝对截止时间：停止放量并取消未收尾设备。
func TestApplyRolloutGovernanceTimeoutStopsDispatch(t *testing.T) {
	past := time.Unix(1600000000, 0).UTC()
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusPending: 20}
	task := &model.OtaUpgradeTask{TimeoutAt: &past, RolloutRatePerMinute: 10}
	pending := make([]*model.OtaUpgradeTaskDetail, 20)
	spy, result := withGovernanceApplier(t, counts, task, pending)

	if result.Decision.Action != OTARolloutActionTimeout {
		t.Fatalf("action = %q, want timeout", result.Decision.Action)
	}
	if len(spy.dispatched) != 0 {
		t.Fatal("timeout must not dispatch")
	}
	if len(spy.cancels) != 1 {
		t.Fatalf("timeout must cancel remaining devices; cancels=%v", spy.cancels)
	}
}

// 全部收尾：只改 task 状态，不碰明细行。
func TestApplyRolloutGovernanceCompleteOnlyTouchesTask(t *testing.T) {
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusSucceeded: 10}
	task := &model.OtaUpgradeTask{RolloutRatePerMinute: 10}
	spy, result := withGovernanceApplier(t, counts, task, nil)

	if result.Decision.Action != OTARolloutActionComplete {
		t.Fatalf("action = %q, want complete", result.Decision.Action)
	}
	if spy.patches == nil || spy.patches[0].Status != "completed" {
		t.Fatalf("task must be marked completed; patches=%+v", spy.patches)
	}
	if len(spy.cancels) != 0 || len(spy.dispatched) != 0 {
		t.Fatal("complete must not touch detail rows or dispatch")
	}
}

// 限速窗口已满：保持等待，不写任何东西。
func TestApplyRolloutGovernanceHoldDoesNotWrite(t *testing.T) {
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusPending: 50}
	task := &model.OtaUpgradeTask{RolloutRatePerMinute: 2, RateWindowDispatched: 5}
	spy, result := withGovernanceApplier(t, counts, task, nil)

	if result.Decision.Action != OTARolloutActionHoldRateWindow {
		t.Fatalf("action = %q, want hold_rate_window", result.Decision.Action)
	}
	if len(spy.patches) != 0 || len(spy.dispatched) != 0 || len(spy.cancels) != 0 {
		t.Fatalf("hold must not write anything; patches=%v dispatched=%d cancels=%v",
			spy.patches, len(spy.dispatched), spy.cancels)
	}
	if result.Applied {
		t.Fatal("hold must not be reported as applied")
	}
}

// 批次上限：即便规划器给出超大批次，也不能一次性把整批 pending 全捞出内存，
// 否则"限速"变成"一次性全推"，金丝雀失去意义。
func TestApplyRolloutGovernanceClampsOversizedBatch(t *testing.T) {
	counts := map[int16]int{model.OtaUpgradeTaskDetailStatusPending: 100000}
	task := &model.OtaUpgradeTask{RolloutRatePerMinute: 100000, RateWindowDispatched: 0}
	pending := make([]*model.OtaUpgradeTaskDetail, dal.OTARolloutPendingClaimLimit)
	spy, _ := withGovernanceApplier(t, counts, task, pending)
	if len(spy.claimed) == 0 || spy.claimed[0] > dal.OTARolloutPendingClaimLimit {
		t.Fatalf("claim limit = %v, want <= %d", spy.claimed, dal.OTARolloutPendingClaimLimit)
	}
}
