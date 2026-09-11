// 文件用途：批次作业的进度消费与回滚（ROADMAP P0.3）。
// 核心逻辑：设备上报进度以事件形式消费并去重；回滚创建新批次并在原批次留下指向新批次的审计事件。
// 关键注意事项：
//  1. 进度事件必须幂等：同一 (设备, 百分比, 状态, 错误) 重复上报只产生一条记录，
//     不能用事件数量冒充处理进度。
//  2. 终态/已取消作业不接受进度上报：给死掉的批次写进度等于伪造进展。
//  3. 回滚不是"把原批次改回去"，而是创建一个新批次（新的下发动作），
//     原批次历史保持只读，回滚关系通过审计事件显式留痕。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// 进度与回滚事件类型。
const (
	commandJobEventProgress = "progress"
	commandJobEventRollback = "rollback"
)

// FleetCommandJobProgressEvent 设备上报的进度事件。
type FleetCommandJobProgressEvent struct {
	JobID    string
	TenantID string
	DeviceID string
	// Percent 0-100，越界即拒绝，不接受"负进度"或"超额进度"。
	Percent int
	// Status 设备侧作业状态；为空表示仅进度更新。
	Status string
	Error  string
	At     time.Time
}

// progressDedupeToken 生成确定性去重令牌。
// 刻意不包含 At：设备重复上报同一进度不应产生第二条记录，
// 否则重传会把"一次进度"放大成"多次进度"，用事件数量伪造推进速度。
func progressDedupeToken(e FleetCommandJobProgressEvent) string {
	return fmt.Sprintf("progress|device=%s|percent=%d|status=%s|error=%s",
		e.DeviceID, e.Percent, e.Status, e.Error)
}

// ConsumeFleetCommandJobProgress 消费一条设备进度上报。
// 返回 nil 表示已接受（含"重复上报被幂等忽略"的情况）。
func (c *CommandData) ConsumeFleetCommandJobProgress(ctx context.Context, e FleetCommandJobProgressEvent) error {
	_ = ctx
	if strings.TrimSpace(e.JobID) == "" || strings.TrimSpace(e.DeviceID) == "" || strings.TrimSpace(e.TenantID) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "job_id, tenant_id and device_id are required")
	}
	if e.Percent < 0 || e.Percent > 100 {
		return errcode.NewWithMessage(errcode.CodeParamError, "percent must be between 0 and 100")
	}

	job, err := loadFleetCommandJobWithFreshTimeout(e.JobID, e.TenantID)
	if err != nil {
		return err
	}
	// 终态或已取消的批次不接受进度：给它写进度等于伪造进展。
	if isTerminalCommandJobStatus(job.Status) {
		return commandJobProgressRejectedError(job.Status)
	}

	token := progressDedupeToken(e)
	exists, err := dal.HasCommandJobEventMessage(e.JobID, e.TenantID, commandJobEventProgress, token)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if exists {
		// 幂等：重复上报直接接受但不新增记录。
		return nil
	}

	deviceID := e.DeviceID
	recordFleetCommandJobEvent(e.JobID, e.TenantID, nil, &deviceID, commandJobEventProgress, token)
	return nil
}

func commandJobProgressRejectedError(status string) error {
	return errcode.NewWithMessage(
		errcode.CodeOpDenied,
		fmt.Sprintf("command job status %s does not accept progress reports", status),
	)
}

// RollbackFleetCommandJob 基于原批次创建一个回滚批次。
// 回滚只允许发生在已结束（completed/partially_failed/failed）的批次上；
// 仍在运行/暂停/已取消的批次必须先结束或取消，不允许"边跑边回滚"。
func (c *CommandData) RollbackFleetCommandJob(
	ctx context.Context,
	jobID, operatorID string,
	req *model.FleetCommandJobReq,
	claims *utils.UserClaims,
	includeRows ...bool,
) (*model.FleetCommandJobSubmitResult, error) {
	original, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	switch original.Status {
	case commandJobStatusCompleted, commandJobStatusPartiallyFailed, commandJobStatusFailed:
		// 只有已结束的批次可回滚。
	default:
		return nil, errcode.NewWithMessage(
			errcode.CodeOpDenied,
			fmt.Sprintf("command job status %s cannot be rolled back; finish or cancel it first", original.Status),
		)
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "rollback payload is required")
	}

	// 回滚是新批次：不修改原批次任何字段，历史保持只读。
	result, err := c.SubmitFleetCommandJob(ctx, operatorID, req, claims, includeRows...)
	if err != nil {
		return nil, err
	}

	// 双端留痕：原批次记录回滚目标，新批次记录回滚来源，便于双向追溯。
	recordFleetCommandJobEvent(
		original.ID, claims.TenantID, nil, nil,
		commandJobEventRollback,
		fmt.Sprintf("rollback job=%s identify=%s", result.JobID, result.Identify),
	)
	if result.JobID != "" {
		recordFleetCommandJobEvent(
			result.JobID, claims.TenantID, nil, nil,
			commandJobEventRollback,
			fmt.Sprintf("rollback of job=%s", original.ID),
		)
	}
	return result, nil
}
