package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func (c *CommandData) previewFleetCommandSelectedDevices(req *model.FleetCommandJobReq, claims *utils.UserClaims) (*model.FleetCommandJobPreviewResult, error) {
	deviceIDs := uniqueFleetDeviceIDs(req.DeviceIDs)
	if len(deviceIDs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_ids is required")
	}

	telemetryEvidence, _ := loadFleetCommandTelemetryEvidence(deviceIDs)
	rows := make([]model.FleetCommandJobPreviewRow, 0, len(deviceIDs))
	eligibleCount := 0
	for _, deviceID := range deviceIDs {
		row := c.previewFleetCommandDevice(deviceID, req, claims, telemetryEvidence)
		if row.Eligible {
			eligibleCount++
		}
		rows = append(rows, row)
	}
	previewToken := fleetCommandPreviewToken(req, rows)

	return attachFleetCommandJobPreviewGuidance(&model.FleetCommandJobPreviewResult{
		JobType:        "command",
		ScopeType:      req.ScopeType,
		PreviewToken:   previewToken,
		RequestedCount: len(deviceIDs),
		EligibleCount:  eligibleCount,
		BlockedCount:   len(deviceIDs) - eligibleCount,
		TimeoutSeconds: req.TimeoutSeconds,
		Rows:           rows,
		ScopeLimits:    fleetCommandJobScopeLimits,
		Warnings: []string{
			"预览只校验已选设备和命令载荷，不会向设备下发命令。",
			"持久化任务支持刷新、重试、取消和审计复查；设备侧业务完成仍需结合响应、遥测或设备影子证据判断。",
		},
	}), nil
}

func (c *CommandData) previewFleetCommandDevice(deviceID string, req *model.FleetCommandJobReq, claims *utils.UserClaims, telemetryEvidence fleetCommandTelemetryEvidence) model.FleetCommandJobPreviewRow {
	row := newBlockedFleetCommandPreviewRow(deviceID)
	if err := ensureCommandWriteAccess(deviceID, claims); err != nil {
		row.Reason = err.Error()
		row.Advice = "当前操作员没有这台设备的命令权限。"
		return row
	}

	profile, err := loadCommandDeviceProfile(deviceID)
	if err != nil {
		row.Reason = err.Error()
		row.Advice = "请确认设备仍然存在，并且属于当前租户。"
		return row
	}
	return evaluateFleetCommandPreviewInput(row, profile, req, "设备档案已加载", telemetryEvidence)
}

func fleetCommandPreviewToken(req *model.FleetCommandJobReq, rows []model.FleetCommandJobPreviewRow) string {
	type previewTokenRow struct {
		DeviceID        string `json:"device_id"`
		Online          bool   `json:"online"`
		Eligible        bool   `json:"eligible"`
		Status          string `json:"status"`
		RecommendedPath string `json:"recommended_path,omitempty"`
	}
	tokenRows := make([]previewTokenRow, 0, len(rows))
	for _, row := range rows {
		tokenRows = append(tokenRows, previewTokenRow{
			DeviceID:        row.DeviceID,
			Online:          row.Online,
			Eligible:        row.Eligible,
			Status:          row.Status,
			RecommendedPath: row.RecommendedPath,
		})
	}

	payload := struct {
		ScopeType      string            `json:"scope_type"`
		DeviceIDs      []string          `json:"device_ids"`
		Identify       string            `json:"identify"`
		Value          *string           `json:"value,omitempty"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
		Rows           []previewTokenRow `json:"rows"`
	}{
		ScopeType:      req.ScopeType,
		DeviceIDs:      uniqueFleetDeviceIDs(req.DeviceIDs),
		Identify:       req.Identify,
		Value:          req.Value,
		TimeoutSeconds: req.TimeoutSeconds,
		ScheduledAt:    req.ScheduledAt,
		Rows:           tokenRows,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
