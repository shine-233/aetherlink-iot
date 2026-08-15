package service

import "aetherlink-iot/backend/internal/model"

func buildFleetCommandJobAuditSummary(events []*model.CommandJobEvent) *model.FleetCommandJobAuditSummary {
	summary := &model.FleetCommandJobAuditSummary{
		EventCount: len(events),
		NextAction: "请把任务链接和支持包保留到客户记录中。",
	}
	if len(events) == 0 {
		summary.NextAction = "尚未加载审计事件；关闭交接前请刷新任务或打开支持包。"
		return summary
	}

	latest := events[0]
	for _, event := range events[1:] {
		if latest == nil || (event != nil && event.CreatedAt.After(latest.CreatedAt)) {
			latest = event
		}
	}
	if latest == nil {
		return summary
	}
	summary.LatestEventType = latest.EventType
	summary.LatestEventAt = &latest.CreatedAt
	summary.LatestMessage = latest.Message
	summary.NextAction = commandJobAuditNextAction(latest.EventType)
	return summary
}

func commandJobAuditNextAction(eventType string) string {
	switch eventType {
	case commandJobEventCompleted:
		return "审计轨迹显示任务已完成；请把任务链接保留到客户记录中。"
	case commandJobEventTimeout:
		return "审计轨迹以超时结束；重新发起前请复核超时行和支持证据。"
	case commandJobEventDispatchFailed:
		return "审计轨迹包含下发失败；重试前请复核设备行和重试策略。"
	case commandJobEventCanceled:
		return "审计轨迹显示任务已取消；请保留证据，如仍需执行请重新创建预览。"
	case commandJobEventWorkerFailed:
		return "审计轨迹显示后端 worker 异常停止；恢复扫描后请刷新，再复核可重试行和支持证据。"
	case commandJobEventResumed:
		return "审计轨迹显示持久化恢复已继续任务；请持续观察进度，直到各行进入终态。"
	case commandJobEventQueued:
		return "审计轨迹显示命令行已排队等待下发；请保持任务打开并观察 worker 进度。"
	case commandJobEventDeviceAckFailed:
		return "审计轨迹显示设备失败响应；关闭任务前请复核失败行、响应载荷和重试策略。"
	case commandJobEventDeviceAckAmbiguous:
		return "审计轨迹显示设备响应因匹配多条命令任务行被拒绝；复核重复 message-id 和命令日志证据前，不要基于该 ACK 关闭或重试。"
	case commandJobEventDeviceAckSuccess:
		return "审计轨迹显示设备成功响应；关闭任务前请确认所有行都已进入终态。"
	default:
		return "请复核最新审计事件，并持续刷新直到任务进入终态。"
	}
}
