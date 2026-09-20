// File purpose: P0.3 execution plane for OTA rollout governance (canary / gradual rollout).
// Core logic: reuse the pure planner PlanOTARolloutGovernance, then ACT on its decision --
// dispatch a rate-limited batch, abort on failure-rate breach, close out on timeout/complete.
// Key notes: the preview endpoint only reads; without this file the planner output is
// advisory only and the canary gate never actually gates anything.

package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

// OTARolloutGovernanceApplyResult 一次治理执行的结果。
type OTARolloutGovernanceApplyResult struct {
	Decision       model.OTARolloutGovernanceDecision `json:"decision"`
	Applied        bool                               `json:"applied"`
	Dispatched     int                                `json:"dispatched"`
	CanceledRows   int64                              `json:"canceled_rows"`
	TaskStatus     string                             `json:"task_status,omitempty"`
	Detail         string                             `json:"detail,omitempty"`
	// IsSimulation 为 false 表示本次决策已被真正执行（区别于只读预览）。
	IsSimulation   bool                               `json:"is_simulation"`
}

// otaRolloutGovernanceApplier 把执行面的副作用收敛为可注入的函数集合，
// 使"决策 → 执行"的映射可以在没有数据库 / broker 的情况下被完整验证。
type otaRolloutGovernanceApplier struct {
	plan       func(model.OTARolloutGovernanceInput) model.OTARolloutGovernanceDecision
	counts     func(taskID string) (map[int16]int, error)
	claimBatch func(taskID string, limit int) ([]*model.OtaUpgradeTaskDetail, error)
	dispatch   func(o *OTA, details []*model.OtaUpgradeTaskDetail)
	patchTask  func(taskID string, patch dal.OTARolloutTaskPatch) (int64, error)
	cancelRest func(taskID string, reason string, now time.Time) (int64, error)
	now        func() time.Time
}

var otaGovernanceApplier = otaRolloutGovernanceApplier{
	plan:       PlanOTARolloutGovernance,
	counts:     dal.CountOTAUpgradeTaskDetailStatuses,
	claimBatch: dal.ListOTAUpgradablePendingDetails,
	// 复用既有的批量下发入口，保证"治理决策"与"实际推送"走同一条路径，
	// 不会长出第二套推送逻辑。
	dispatch:   pushOTAUpgradeTaskDetails,
	patchTask:  dal.UpdateOTAUpgradeTaskRolloutState,
	cancelRest: dal.CancelOTAUpgradeTaskPendingDetails,
	now:        time.Now,
}

// ApplyRolloutGovernance 执行一次 rollout 治理：读取状态 → 规划 → 执行。
// 与 PreviewRolloutGovernance 的区别就在这里：预览把决策算出来给你看，
// 本函数把决策落到设备上——否则"金丝雀"只是个会说话的告示牌。
func (o *OTA) ApplyRolloutGovernance(taskID string, claims *utils.UserClaims) (*OTARolloutGovernanceApplyResult, error) {
	taskID = strings.TrimSpace(taskID)
	task, err := ensureOTATaskAccess(taskID, claims)
	if err != nil {
		return nil, err
	}

	counts, err := otaGovernanceApplier.counts(taskID)
	if err != nil {
		return nil, err
	}
	now := otaGovernanceApplier.now().UTC()
	decision := otaGovernanceApplier.plan(buildOTARolloutGovernanceInput(task, counts, now))

	// 规划器返回的是"模拟"标记；真正执行后必须翻转为 false，
	// 否则调用方无法区分"看了一眼"和"真的做了"。
	result := &OTARolloutGovernanceApplyResult{Decision: decision, IsSimulation: false}

	switch decision.Action {
	case OTARolloutActionDispatchBatch:
		return o.applyDispatchBatch(taskID, task, decision, result)
	case OTARolloutActionAbort:
		return o.applyAbort(taskID, decision, result, now)
	case OTARolloutActionTimeout:
		return o.applyTimeout(taskID, decision, result, now)
	case OTARolloutActionComplete:
		return o.applyComplete(taskID, decision, result)
	default:
		// wait_schedule / hold_rate_window / hold：保持现状，不做任何写入。
		result.Detail = "no action required for " + decision.Action
		return result, nil
	}
}

// applyDispatchBatch 按限速下发一批，并累加限速窗口计数。
func (o *OTA) applyDispatchBatch(
	taskID string,
	task *model.OtaUpgradeTask,
	decision model.OTARolloutGovernanceDecision,
	result *OTARolloutGovernanceApplyResult,
) (*OTARolloutGovernanceApplyResult, error) {
	batch := decision.BatchSize
	if batch <= 0 {
		result.Detail = "planner produced a non-positive batch size; nothing dispatched"
		return result, nil
	}
	// 上限在服务层自己夹一次，不能只靠 DAL 兜底：批次上限是安全边界，
	// 依赖下层实现意味着换一个存储实现就会退化成"一次性全推"，
	// 限速窗口与金丝雀语义同时失效。
	if batch > dal.OTARolloutPendingClaimLimit {
		batch = dal.OTARolloutPendingClaimLimit
	}
	details, err := otaGovernanceApplier.claimBatch(taskID, batch)
	if err != nil {
		return nil, err
	}
	if len(details) == 0 {
		result.Detail = "no pending detail rows left to dispatch"
		return result, nil
	}

	// 先记账再下发：若先下发后记账，进程在两者之间崩溃就会重复放量，
	// 限速窗口形同虚设。窗口滚动的判定依赖 RateWindowStartedAt。
	dispatched := task.RateWindowDispatched + len(details)
	windowStart := task.RateWindowStartedAt
	if windowStart == nil {
		now := otaGovernanceApplier.now().UTC()
		windowStart = &now
	}
	if _, err := otaGovernanceApplier.patchTask(taskID, dal.OTARolloutTaskPatch{
		RateWindowDispatched: &dispatched,
		RateWindowStartedAt:  windowStart,
	}); err != nil {
		return nil, err
	}

	otaGovernanceApplier.dispatch(o, details)
	result.Applied = true
	result.Dispatched = len(details)
	result.Detail = "dispatched a rate-limited batch"
	return result, nil
}

// applyAbort 失败率越线：停止放量，把剩余 pending 明细行取消。
// 只在 task 上打个标记是不够的——下一轮治理仍会把它们捞出来推下去。
func (o *OTA) applyAbort(
	taskID string,
	decision model.OTARolloutGovernanceDecision,
	result *OTARolloutGovernanceApplyResult,
	now time.Time,
) (*OTARolloutGovernanceApplyResult, error) {
	status := "canceled"
	description := decision.Reason
	if _, err := otaGovernanceApplier.patchTask(taskID, dal.OTARolloutTaskPatch{
		Status:            status,
		StatusDescription: description,
	}); err != nil {
		return nil, err
	}
	canceled, err := otaGovernanceApplier.cancelRest(taskID, "rollout aborted: "+decision.Reason, now)
	if err != nil {
		return nil, err
	}
	result.Applied = true
	result.CanceledRows = canceled
	result.TaskStatus = status
	result.Detail = "aborted rollout and canceled remaining pending devices"
	return result, nil
}

// applyTimeout 越过绝对截止时间：停止放量并取消未收尾的设备。
func (o *OTA) applyTimeout(
	taskID string,
	decision model.OTARolloutGovernanceDecision,
	result *OTARolloutGovernanceApplyResult,
	now time.Time,
) (*OTARolloutGovernanceApplyResult, error) {
	status := "canceled"
	description := decision.Reason
	if _, err := otaGovernanceApplier.patchTask(taskID, dal.OTARolloutTaskPatch{
		Status:            status,
		StatusDescription: description,
	}); err != nil {
		return nil, err
	}
	canceled, err := otaGovernanceApplier.cancelRest(taskID, "rollout timed out: "+decision.Reason, now)
	if err != nil {
		return nil, err
	}
	result.Applied = true
	result.CanceledRows = canceled
	result.TaskStatus = status
	result.Detail = "closed out rollout on timeout and canceled pending devices"
	return result, nil
}

// applyComplete 全部收尾：标记任务完成，不触碰明细行。
func (o *OTA) applyComplete(
	taskID string,
	decision model.OTARolloutGovernanceDecision,
	result *OTARolloutGovernanceApplyResult,
) (*OTARolloutGovernanceApplyResult, error) {
	status := "completed"
	if _, err := otaGovernanceApplier.patchTask(taskID, dal.OTARolloutTaskPatch{
		Status:            status,
		StatusDescription: decision.Reason,
	}); err != nil {
		return nil, err
	}
	result.Applied = true
	result.TaskStatus = status
	result.Detail = "marked rollout complete"
	return result, nil
}
