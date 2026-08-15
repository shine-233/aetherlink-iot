package service

import (
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/model"
)

func buildFleetCommandJobHandoffSummary(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	execution *model.FleetCommandJobExecutionSummary,
) string {
	if job == nil {
		return ""
	}

	healthState := "unknown"
	nextAction := "客户交接前，请复核任务详情和支持证据。"
	if health != nil {
		healthState = health.State
		if health.NextAction != "" {
			nextAction = health.NextAction
		}
	}
	closeReadiness := "close_ready=unknown"
	closeBlockers := ""
	healthNextAction := nextAction
	if execution != nil {
		if execution.CanClose {
			closeReadiness = "close_ready=yes"
		} else {
			closeReadiness = "close_ready=no"
		}
		if len(execution.CloseBlockers) > 0 {
			closeBlockers = " 阻断项：" + strings.Join(execution.CloseBlockers, " ")
		}
		if execution.NextAction != "" {
			nextAction = execution.NextAction
		}
	}
	// Keep the progress-health action in the handoff even when governance
	// supplies a more specific execution action. Consumers use both as
	// independent evidence for customer follow-up.
	if healthNextAction != "" && healthNextAction != nextAction {
		nextAction = nextAction + " 进度下一步：" + healthNextAction
	}

	return fmt.Sprintf(
		"命令任务 %s 当前状态 %s：已提交 %d/%d 台，失败 %d 台，阻断 %d 台，健康状态=%s，%s。%s 下一步：%s",
		job.ID,
		job.Status,
		job.SubmittedCount,
		job.RequestedCount,
		job.FailedCount,
		job.BlockedCount,
		healthState,
		closeReadiness,
		closeBlockers,
		nextAction,
	)
}
