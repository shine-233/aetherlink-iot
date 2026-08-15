package service

import (
	"context"
	"strings"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

type DeviceConnectionGuideReq struct {
	DeviceID        string `json:"device_id"`
	DebugLogLimit   int64  `json:"debug_log_limit,omitempty"`
	CommandLogLimit int    `json:"command_log_limit,omitempty"`
}

type connectionGuideDiagnosticsBundle struct {
	diagnostics    *DeviceConnectionDiagnosticsResp
	diagnosticsErr error
	twin           *model.DeviceTwinState
	twinErr        error
	command        *CommandDeliveryDiagnosticsResp
	commandErr     error
}

func (d *Device) GetConnectionGuide(
	ctx context.Context,
	req DeviceConnectionGuideReq,
	lang string,
	claims *utils.UserClaims,
) (*model.DeviceConnectionGuideResp, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	connectCtx, err := loadDeviceConnectionGuideContext(ctx, deviceID, claims)
	if err != nil {
		return nil, err
	}

	profile, profileErr := resolveDeviceConnectRuntimeProfile(connectCtx)
	resp := newDeviceConnectionGuideResponse(deviceID, profile)

	d.applyConnectionGuideAccessSection(ctx, deviceID, connectCtx, profile, profileErr, lang, resp)
	d.applyConnectionGuideDiagnosticsSection(ctx, req, deviceID, connectCtx.device, claims, resp)
	applyConnectionGuideReadinessFallback(resp)
	applyConnectionGuideStepStatuses(resp)

	return resp, nil
}

func newDeviceConnectionGuideResponse(deviceID string, profile deviceConnectProfile) *model.DeviceConnectionGuideResp {
	return &model.DeviceConnectionGuideResp{
		DeviceID:    deviceID,
		EvaluatedAt: time.Now().UTC(),
		Access: model.DeviceConnectionGuideAccess{
			Protocol:       guideProtocol(profile),
			CredentialMode: profile.voucherType,
			TLS:            buildConnectionGuideTLSHint(),
		},
		NextSteps: buildConnectionGuideSteps(),
	}
}

func (d *Device) applyConnectionGuideAccessSection(
	ctx context.Context,
	deviceID string,
	connectCtx deviceConnectContext,
	profile deviceConnectProfile,
	profileErr error,
	lang string,
	resp *model.DeviceConnectionGuideResp,
) {
	if profileErr != nil {
		appendConnectionGuideWarning(resp, "access_profile", profileErr)
	} else if stableProfile, err := buildDeviceConnectionGuideProfile(ctx, deviceID, connectCtx, profile); err != nil {
		appendConnectionGuideWarning(resp, "connection_profile", err)
	} else {
		resp.Access.ConnectionProfile = stableProfile
	}

	form, err := buildDeviceConnectionGuideCredentialForm(d, connectCtx)
	if err != nil {
		appendConnectionGuideWarning(resp, "credential_form", err)
	} else {
		resp.Access.CredentialForm = form
	}

	if profileErr == nil {
		connectionInfo, err := buildDeviceConnectionGuideInfo(ctx, deviceID, connectCtx, profile, lang)
		if err != nil {
			appendConnectionGuideWarning(resp, "connection_info", err)
		} else {
			resp.Access.ConnectionInfo = connectionInfo
		}
	}
	if resp.Access.Protocol != "MQTT" {
		resp.Access.HTTPHint = &model.DeviceConnectionGuideHTTPHint{
			Available: true,
			Summary:   "可通过插件连接载荷使用 HTTP 或协议插件接入。",
		}
	}
}

func (d *Device) applyConnectionGuideDiagnosticsSection(
	ctx context.Context,
	req DeviceConnectionGuideReq,
	deviceID string,
	device *model.Device,
	claims *utils.UserClaims,
	resp *model.DeviceConnectionGuideResp,
) {
	results := d.collectConnectionGuideDiagnostics(ctx, req, deviceID, device, claims)

	if results.diagnosticsErr != nil {
		appendConnectionGuideWarning(resp, "connection_diagnostics", results.diagnosticsErr)
	} else {
		resp.Readiness = buildConnectionGuideReadiness(results.diagnostics)
		resp.LastError = buildConnectionGuideLastError(results.diagnostics)
		if results.diagnostics == nil {
			appendConnectionGuideWarningReason(resp, "connection_diagnostics", "no diagnostics result")
		}
	}

	if results.twinErr != nil {
		appendConnectionGuideWarning(resp, "device_twin", results.twinErr)
	} else if results.twin == nil {
		appendConnectionGuideWarningReason(resp, "device_twin", "no twin result")
	} else {
		resp.TwinSummary = &results.twin.Summary
	}

	if results.commandErr != nil {
		appendConnectionGuideWarning(resp, "command_delivery", results.commandErr)
	} else if results.command == nil {
		appendConnectionGuideWarningReason(resp, "command_delivery", "no command delivery result")
	} else {
		resp.CommandSummary = buildConnectionGuideCommandSummary(results.command)
	}
}

func (d *Device) collectConnectionGuideDiagnostics(
	ctx context.Context,
	req DeviceConnectionGuideReq,
	deviceID string,
	device *model.Device,
	claims *utils.UserClaims,
) connectionGuideDiagnosticsBundle {
	var wg sync.WaitGroup
	var results connectionGuideDiagnosticsBundle

	wg.Add(3)
	go func() {
		defer wg.Done()
		if device != nil {
			results.diagnostics = buildConnectionDiagnosticsForDevice(ctx, deviceID, req.DebugLogLimit, device)
			return
		}
		results.diagnostics, results.diagnosticsErr = d.GetConnectionDiagnostics(ctx, DeviceConnectionDiagnosticsReq{
			DeviceID:      deviceID,
			DebugLogLimit: req.DebugLogLimit,
		}, claims)
	}()
	go func() {
		defer wg.Done()
		results.twin, results.twinErr = GroupApp.DeviceTwin.GetDeviceTwin(deviceID, claims)
	}()
	go func() {
		defer wg.Done()
		results.command, results.commandErr = GroupApp.CommandData.GetCommandDeliveryDiagnostics(ctx, CommandDeliveryDiagnosticsReq{
			DeviceID: deviceID,
			Limit:    req.CommandLogLimit,
		}, claims)
	}()
	wg.Wait()
	return results
}

func appendConnectionGuideWarning(resp *model.DeviceConnectionGuideResp, component string, err error) {
	if err == nil {
		return
	}
	resp.PartialResults = append(resp.PartialResults, guideWarning(component, err))
}

func appendConnectionGuideWarningReason(resp *model.DeviceConnectionGuideResp, component string, reason string) {
	if strings.TrimSpace(reason) == "" {
		return
	}
	resp.PartialResults = append(resp.PartialResults, model.DeviceConnectionGuideWarning{
		Component: component,
		Reason:    reason,
	})
}

func applyConnectionGuideReadinessFallback(resp *model.DeviceConnectionGuideResp) {
	if resp.Readiness.Code == "" {
		resp.Readiness = buildFallbackConnectionGuideReadiness(resp.PartialResults)
	}
}

func applyConnectionGuideStepStatuses(resp *model.DeviceConnectionGuideResp) {
	for index := range resp.NextSteps {
		switch resp.NextSteps[index].Key {
		case "credentials":
			resp.NextSteps[index].Status = connectionGuideCredentialsStepStatus(resp)
		case "publish_telemetry":
			resp.NextSteps[index].Status = connectionGuideTelemetryStepStatus(resp)
		case "ready_check":
			resp.NextSteps[index].Status = connectionGuideReadyStepStatus(resp)
		case "control_loop":
			resp.NextSteps[index].Status = connectionGuideControlLoopStepStatus(resp)
		}
	}
}

func connectionGuideCredentialsStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if resp == nil {
		return "todo"
	}
	if resp.Access.ConnectionProfile != nil || resp.Access.ConnectionInfo != nil {
		return "done"
	}
	if connectionGuideHasWarning(resp, "access_profile") ||
		connectionGuideHasWarning(resp, "connection_profile") ||
		connectionGuideHasWarning(resp, "connection_info") ||
		connectionGuideHasWarning(resp, "credential_form") {
		return "warning"
	}
	return "todo"
}

func connectionGuideTelemetryStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if resp == nil {
		return "todo"
	}
	if resp.Readiness.LatestTelemetryAt != nil {
		return "done"
	}
	if resp.Readiness.Level == "error" || resp.Readiness.Level == "warning" || len(resp.PartialResults) > 0 {
		return "warning"
	}
	return "todo"
}

func connectionGuideReadyStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if resp == nil {
		return "todo"
	}
	if resp.Readiness.Ready || resp.Readiness.Level == "ok" {
		return "done"
	}
	if resp.Readiness.Code != "" || len(resp.PartialResults) > 0 || resp.LastError != nil {
		return "warning"
	}
	return "todo"
}

func connectionGuideControlLoopStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if resp == nil {
		return "todo"
	}
	twinStatus := connectionGuideTwinStepStatus(resp)
	commandStatus := connectionGuideCommandStepStatus(resp)
	if twinStatus == "done" && commandStatus == "done" {
		return "done"
	}
	if twinStatus == "warning" || commandStatus == "warning" {
		return "warning"
	}
	if twinStatus == "done" || commandStatus == "done" {
		return "warning"
	}
	return "todo"
}

func connectionGuideTwinStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if connectionGuideHasWarning(resp, "device_twin") {
		return "warning"
	}
	if resp.TwinSummary == nil {
		return "todo"
	}
	if resp.TwinSummary.DeltaCount > 0 || resp.TwinSummary.UnavailableCount > 0 {
		return "warning"
	}
	if resp.TwinSummary.DesiredCount > 0 || resp.TwinSummary.ReportedCount > 0 || resp.TwinSummary.MatchedCount > 0 {
		return "done"
	}
	return "todo"
}

func connectionGuideCommandStepStatus(resp *model.DeviceConnectionGuideResp) string {
	if connectionGuideHasWarning(resp, "command_delivery") {
		return "warning"
	}
	if resp.CommandSummary == nil {
		return "todo"
	}
	if resp.CommandSummary.Level == "ok" {
		return "done"
	}
	if resp.CommandSummary.Level == "warning" || resp.CommandSummary.Level == "error" || resp.CommandSummary.Code != "" {
		return "warning"
	}
	return "todo"
}

func connectionGuideHasWarning(resp *model.DeviceConnectionGuideResp, component string) bool {
	if resp == nil {
		return false
	}
	for _, warning := range resp.PartialResults {
		if warning.Component == component {
			return true
		}
	}
	return false
}

func loadDeviceConnectionGuideContext(ctx context.Context, deviceID string, claims *utils.UserClaims) (deviceConnectContext, error) {
	device, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return deviceConnectContext{}, err
	}
	if device.DeviceConfigID == nil {
		return deviceConnectContext{device: device}, nil
	}

	deviceConfig, err := loadDeviceConnectConfig(ctx, deviceID, *device.DeviceConfigID)
	if err != nil {
		return deviceConnectContext{}, err
	}
	return deviceConnectContext{
		device:       device,
		deviceConfig: deviceConfig,
	}, nil
}

func buildConnectionGuideReadiness(resp *DeviceConnectionDiagnosticsResp) model.DeviceConnectionGuideReadiness {
	if resp == nil {
		return model.DeviceConnectionGuideReadiness{
			Level:       "unknown",
			Code:        "no_diagnostics",
			Summary:     "暂无连接诊断结果。",
			NextActions: []string{"设备尝试连接后，请刷新接入指南。"},
		}
	}

	return model.DeviceConnectionGuideReadiness{
		Level:             resp.Conclusion.Level,
		Code:              resp.Conclusion.Code,
		Summary:           resp.Conclusion.Summary,
		Online:            resp.Online.IsOnline,
		Ready:             resp.ReadyCheck.Ready,
		LatestTelemetryAt: resp.ReadyCheck.Telemetry.LatestAt,
		NextActions:       resp.Conclusion.NextActions,
		Evidence:          resp.Conclusion.Evidence,
	}
}

func buildConnectionGuideLastError(resp *DeviceConnectionDiagnosticsResp) *model.DeviceConnectionGuideLastError {
	if resp == nil || resp.Conclusion.Level == "ok" {
		return nil
	}
	if resp.Conclusion.Code == "" {
		return nil
	}
	return &model.DeviceConnectionGuideLastError{
		Code:     resp.Conclusion.Code,
		Summary:  resp.Conclusion.Summary,
		Evidence: resp.Conclusion.Evidence,
	}
}

func buildConnectionGuideCommandSummary(resp *CommandDeliveryDiagnosticsResp) *model.DeviceConnectionGuideCommand {
	if resp == nil {
		return nil
	}
	summary := &model.DeviceConnectionGuideCommand{
		Level:       resp.Conclusion.Level,
		Code:        resp.Conclusion.Code,
		Summary:     resp.Conclusion.Summary,
		NextActions: resp.Conclusion.NextActions,
	}
	if resp.LatestLog != nil {
		summary.LatestStatus = resp.LatestLog.StatusLabel
		summary.LatestMessageID = resp.LatestLog.MessageID
	}
	return summary
}

func buildConnectionGuideSteps() []model.DeviceConnectionGuideStep {
	return []model.DeviceConnectionGuideStep{
		{
			Key:         "credentials",
			Title:       "创建并保存设备凭据",
			Description: "复制任何测试命令前，请先填写凭据表单并确认连接信息。",
			Status:      "todo",
		},
		{
			Key:         "publish_telemetry",
			Title:       "发布一条测试遥测",
			Description: "运行 MQTT 或 HTTP 测试命令，并确认最新遥测时间。",
			Status:      "todo",
		},
		{
			Key:         "ready_check",
			Title:       "检查在线状态和最近错误",
			Description: "使用就绪诊断查看在线状态、最新遥测、调试日志和失败证据。",
			Status:      "todo",
		},
		{
			Key:         "control_loop",
			Title:       "验证设备影子和命令响应",
			Description: "对比期望/上报状态，并检查命令下发响应、超时或错误证据。",
			Status:      "todo",
		},
	}
}

func buildFallbackConnectionGuideReadiness(warnings []model.DeviceConnectionGuideWarning) model.DeviceConnectionGuideReadiness {
	return model.DeviceConnectionGuideReadiness{
		Level:       "unknown",
		Code:        "partial_guide",
		Summary:     "接入指南已生成，但就绪证据不完整。",
		NextActions: []string{"打开 Ready Check 查看详细诊断。", "设备发布遥测后再重试。"},
		Evidence:    formatConnectionGuideWarnings(warnings),
	}
}

func guideWarning(component string, err error) model.DeviceConnectionGuideWarning {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return model.DeviceConnectionGuideWarning{
		Component: component,
		Reason:    reason,
	}
}

func formatConnectionGuideWarnings(warnings []model.DeviceConnectionGuideWarning) []string {
	evidence := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		evidence = append(evidence, warning.Component+": "+warning.Reason)
	}
	return evidence
}
