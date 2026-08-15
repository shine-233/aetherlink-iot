package service

import (
	"strings"

	"aetherlink-iot/backend/internal/diagnostics"
	model "aetherlink-iot/backend/internal/model"
)

func buildConnectionDiagnosticsConclusion(
	resp *DeviceConnectionDiagnosticsResp,
) DeviceConnectionDiagnosticsConclusion {
	if resp == nil {
		return DeviceConnectionDiagnosticsConclusion{
			Level:       "unknown",
			Code:        "no_evidence",
			Summary:     "暂无连接诊断证据。",
			NextActions: []string{"设备尝试连接后，刷新诊断视图。"},
		}
	}

	if log := firstConnectionDebugError(resp.Debug.RecentLogs); log != nil {
		return DeviceConnectionDiagnosticsConclusion{
			Level:       "error",
			Code:        "recent_debug_error",
			Summary:     "近期调试日志包含连接错误。",
			NextActions: []string{"打开近期调试日志并处理报告的错误。", "检查设备凭据、主题、broker 主机、端口和 TLS 设置。"},
			Evidence:    []string{formatConnectionDebugEvidence(*log)},
		}
	}

	if failure := firstConnectionFailure(resp.Diagnostics.RecentFailures); failure != nil {
		return DeviceConnectionDiagnosticsConclusion{
			Level:       "error",
			Code:        "recent_failure",
			Summary:     "诊断采集器记录到近期连接失败。",
			NextActions: []string{"复核失败阶段和错误信息。", "重试前确认载荷格式、凭据、主题和协议设置。"},
			Evidence:    []string{formatConnectionFailureEvidence(*failure)},
		}
	}

	if !resp.Online.IsOnline {
		return DeviceConnectionDiagnosticsConclusion{
			Level:       "warning",
			Code:        "offline",
			Summary:     "设备当前离线，且没有发现更新的调试错误。",
			NextActions: []string{"从本页运行一次连接测试。", "如果设备仍未上线，请启用调试日志。", "检查 broker 地址、防火墙、凭据、主题和 TLS 设置。"},
			Evidence:    []string{"online.is_online=false"},
		}
	}

	if len(resp.PartialResults) > 0 {
		return DeviceConnectionDiagnosticsConclusion{
			Level:       "warning",
			Code:        "partial_evidence",
			Summary:     "设备看起来在线，但部分诊断证据未能采集。",
			NextActions: []string{"稍后刷新诊断。", "如果该警告持续出现，请检查调试日志存储和诊断采集器健康状态。"},
			Evidence:    formatConnectionWarnings(resp.PartialResults),
		}
	}

	return DeviceConnectionDiagnosticsConclusion{
		Level:       "ok",
		Code:        "online",
		Summary:     "设备在线，且未发现近期连接失败。",
		NextActions: []string{"发布一条测试遥测，并在遥测页确认最新值。", "验证新设备接入时，请保持调试日志开启。"},
		Evidence:    []string{"online.is_online=true"},
	}
}

func firstConnectionDebugError(logs []DeviceConnectionDebugLogSummary) *DeviceConnectionDebugLogSummary {
	for i := range logs {
		if strings.TrimSpace(logs[i].Error) != "" {
			return &logs[i]
		}
	}
	return nil
}

func firstConnectionFailure(failures []diagnostics.FailureRecord) *diagnostics.FailureRecord {
	for i := range failures {
		if strings.TrimSpace(failures[i].Error) != "" {
			return &failures[i]
		}
	}
	return nil
}

func formatConnectionDebugEvidence(log DeviceConnectionDebugLogSummary) string {
	parts := []string{"debug_log"}
	if log.Action != "" {
		parts = append(parts, "action="+log.Action)
	}
	if log.Direction != "" {
		parts = append(parts, "direction="+log.Direction)
	}
	if log.Error != "" {
		parts = append(parts, "error="+log.Error)
	}
	if code, ok := log.Meta["diagnostic_code"].(string); ok && code != "" {
		parts = append(parts, "code="+code)
	}
	if action, ok := log.Meta["recommended_action"].(string); ok && action != "" {
		parts = append(parts, "next="+action)
	}
	return strings.Join(parts, " ")
}

func formatConnectionFailureEvidence(failure diagnostics.FailureRecord) string {
	parts := []string{"failure"}
	if failure.Stage != "" {
		parts = append(parts, "stage="+string(failure.Stage))
	}
	if failure.Direction != "" {
		parts = append(parts, "direction="+string(failure.Direction))
	}
	if failure.Error != "" {
		parts = append(parts, "error="+failure.Error)
	}
	return strings.Join(parts, " ")
}

func formatConnectionWarnings(warnings []DeviceConnectionDiagnosticsWarning) []string {
	evidence := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		component := strings.TrimSpace(warning.Component)
		if component == "" {
			component = "diagnostics"
		}
		reason := strings.TrimSpace(warning.Reason)
		if reason == "" {
			reason = "partial_result"
		}
		evidence = append(evidence, component+":"+reason)
	}
	return evidence
}

func buildConnectionTelemetryReadinessSnapshot(
	currentCount int64,
	latest *model.TelemetryCurrentData,
) DeviceConnectionTelemetryReadiness {
	readiness := DeviceConnectionTelemetryReadiness{
		CurrentCount:     int(currentCount),
		HasRecentCurrent: currentCount > 0,
	}
	if latest == nil {
		return readiness
	}

	readiness.LatestKey = latest.Key
	readiness.LatestAt = &latest.T
	readiness.LatestValue = connectionTelemetryCurrentValue(latest)
	return readiness
}

func buildConnectionTelemetryReadiness(telemetry []*model.TelemetryCurrentData) DeviceConnectionTelemetryReadiness {
	readiness := DeviceConnectionTelemetryReadiness{
		CurrentCount:     len(telemetry),
		HasRecentCurrent: len(telemetry) > 0,
	}
	if len(telemetry) == 0 || telemetry[0] == nil {
		return readiness
	}

	latest := telemetry[0]
	readiness.LatestKey = latest.Key
	readiness.LatestAt = &latest.T
	readiness.LatestValue = connectionTelemetryCurrentValue(latest)
	return readiness
}

func connectionTelemetryCurrentValue(data *model.TelemetryCurrentData) interface{} {
	if data == nil {
		return nil
	}
	if data.BoolV != nil {
		return *data.BoolV
	}
	if data.NumberV != nil {
		return *data.NumberV
	}
	if data.StringV != nil {
		return *data.StringV
	}
	return nil
}

func buildConnectionReadyCheck(
	resp *DeviceConnectionDiagnosticsResp,
	telemetry DeviceConnectionTelemetryReadiness,
) DeviceConnectionReadyCheck {
	if resp == nil {
		return DeviceConnectionReadyCheck{
			Ready:       false,
			Level:       "unknown",
			Code:        "no_diagnostics",
			Summary:     "暂时无法评估连接就绪状态。",
			NextActions: []string{"设备尝试连接后刷新诊断。"},
			Telemetry:   telemetry,
		}
	}

	if !resp.Online.IsOnline {
		return DeviceConnectionReadyCheck{
			Ready:       false,
			Level:       "warning",
			Code:        "offline",
			Summary:     "设备仍然离线，因此尚未就绪。",
			NextActions: []string{"从本页运行连接测试。", "如果仍未上线，请启用调试日志。"},
			Telemetry:   telemetry,
		}
	}

	if !telemetry.HasRecentCurrent {
		return DeviceConnectionReadyCheck{
			Ready:       false,
			Level:       "warning",
			Code:        "no_current_telemetry",
			Summary:     "设备已在线，但尚未收到当前遥测。",
			NextActions: []string{"发布一条测试遥测。", "刷新本诊断视图，并确认最新遥测值出现。"},
			Telemetry:   telemetry,
		}
	}

	return DeviceConnectionReadyCheck{
		Ready:       true,
		Level:       "ok",
		Code:        "ready",
		Summary:     "设备已在线，并且已收到当前遥测。",
		NextActions: []string{"打开遥测页签查看实时值。", "继续配置命令、规则、告警或看板。"},
		Telemetry:   telemetry,
	}
}
