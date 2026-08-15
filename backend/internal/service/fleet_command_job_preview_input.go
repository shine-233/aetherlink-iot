package service

import (
	"aetherlink-iot/backend/internal/model"
)

func newBlockedFleetCommandPreviewRow(deviceID string) model.FleetCommandJobPreviewRow {
	return model.FleetCommandJobPreviewRow{
		DeviceID:        deviceID,
		Status:          "blocked",
		RecommendedPath: fleetCommandPathBlocked,
	}
}

func evaluateFleetCommandPreviewInput(
	row model.FleetCommandJobPreviewRow,
	profile *commandDeviceProfile,
	req *model.FleetCommandJobReq,
	profileLoadedReadiness string,
	telemetryEvidence fleetCommandTelemetryEvidence,
) model.FleetCommandJobPreviewRow {
	if profile == nil || profile.device == nil {
		row.Reason = "设备档案不可用"
		row.Advice = "请确认设备仍然存在，并且属于当前租户。"
		return row
	}

	if row.DeviceNumber == "" {
		row.DeviceNumber = profile.device.DeviceNumber
	}
	if row.Name == "" {
		row.Name = SafeDeref(profile.device.Name)
	}
	row.Online = profile.device.IsOnline == 1
	row.Readiness = append(row.Readiness, profileLoadedReadiness)
	if row.Online {
		row.Readiness = append(row.Readiness, "设备在线")
	} else {
		row.Readiness = append(row.Readiness, "设备离线")
	}
	attachFleetCommandTelemetryEvidenceFromSnapshot(row.DeviceID, &row, telemetryEvidence)

	if _, _, err := buildCommandPayload(&model.PutMessageForCommand{
		DeviceID: row.DeviceID,
		Identify: req.Identify,
		Value:    req.Value,
	}, profile.device, profile.deviceType); err != nil {
		row.Reason = err.Error()
		row.Advice = "提交前请修正命令标识符或命令载荷。"
		return row
	}
	row.Readiness = append(row.Readiness, "命令载荷有效")

	if !row.Online {
		row.Status = "offline_jobs_recommended"
		row.RecommendedPath = fleetCommandPathJobs
		row.Reason = "设备离线；立即下发需要设备在线"
		row.Advice = "请使用受控的 Jobs/离线路径，或等待设备重新上线后再次预览。"
		return row
	}

	row.Eligible = true
	row.Status = "ready"
	row.RecommendedPath = fleetCommandPathImmediate
	if row.TelemetryCurrentCount == 0 {
		row.Status = "ready_with_caution"
		row.Advice = "设备在线且载荷有效，但还没有找到当前遥测证据。"
	} else {
		row.Advice = "可以立即下发；提交后请保留消息 ID 和设备响应证据。"
	}
	return row
}
