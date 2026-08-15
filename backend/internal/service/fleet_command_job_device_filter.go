package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const defaultFleetCommandDeviceFilterPreviewSubsetLimit = 20
const maxFleetCommandDeviceFilterPreviewSubsetLimit = 50
const defaultFleetCommandDeviceFilterMaxDevices = 200
const maxFleetCommandDeviceFilterMaxDevices = 1000

var fleetCommandDeviceFilterPreviewScopeLimits = []string{
	"device_filter 预览会复用现有设备列表筛选，并在匹配数量不超过 max_devices 时返回逐设备可执行性。",
	"device_filter 提交必须携带最新预览令牌，且会拒绝超过 max_devices 的筛选范围。",
	"预览子集默认最多 20 台、最高 50 台；可执行的筛选批量任务默认最多 200 台、最高 1000 台。",
}

func (c *CommandData) previewFleetCommandDeviceFilter(req *model.FleetCommandJobReq, claims *utils.UserClaims) (*model.FleetCommandJobPreviewResult, error) {
	if req.DeviceFilter == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "预览 device_filter 批量命令时必须提供 device_filter")
	}
	if _, err := buildRawCommandData(&model.PutMessageForCommand{
		Identify: req.Identify,
		Value:    req.Value,
	}); err != nil {
		return nil, err
	}
	tenantID, err := requireDeviceTenantClaims(claims, "没有权限预览筛选批量命令")
	if err != nil {
		return nil, err
	}

	previewSubsetLimit := fleetCommandDeviceFilterPreviewSubsetLimit(req)
	maxDevices := normalizeFleetCommandDeviceFilterMaxDevices(req.MaxDevices)
	countReq := buildFleetCommandDeviceFilterListReq(req.DeviceFilter, 1, previewSubsetLimit)
	applyDeviceListOwnerFilterForClaims(countReq, claims)
	total, err := dal.CountDeviceListByFilter(countReq, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	warnings := []string{
		"预览只解析匹配设备，不会向设备下发命令。",
		"后台 Jobs 引擎和设备回调对账以后再深做；当前提交路径会立即执行已限制数量的批量下发。",
	}
	if fleetCommandDeviceFilterIsEmpty(req.DeviceFilter) {
		warnings = append(warnings, "未提供筛选字段；本次预览会匹配当前操作员可见的全部活跃租户设备。")
	}
	if req.ExpectedTotal != nil && *req.ExpectedTotal != total {
		warnings = append(warnings, fmt.Sprintf("设备筛选总数已从 %d 变为 %d；提交前请重新核对预览结果。", *req.ExpectedTotal, total))
	}

	var previewSubsetDevices []model.GetDeviceListByPageRsp
	rows := make([]model.FleetCommandJobPreviewRow, 0)
	eligibleCount := 0
	blockedCount := 0
	if total == 0 {
		rows = []model.FleetCommandJobPreviewRow{}
		previewSubsetDevices = []model.GetDeviceListByPageRsp{}
	} else if total <= int64(maxDevices) {
		allDevices, err := resolveFleetCommandDeviceFilterDevices(req.DeviceFilter, tenantID, maxDevices, claims)
		if err != nil {
			return nil, err
		}
		rows, eligibleCount = c.previewFleetCommandRowsFromDevices(allDevices, req, claims)
		blockedCount = len(rows) - eligibleCount
		previewSubsetDevices = allDevices
	} else {
		previewSubsetReq := buildFleetCommandDeviceFilterListReq(req.DeviceFilter, 1, previewSubsetLimit)
		applyDeviceListOwnerFilterForClaims(previewSubsetReq, claims)
		_, previewSubsetDevices, err := dal.PreviewCommandJobDeviceFilter(previewSubsetReq, tenantID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		normalizeFleetCommandDeviceFilterPreviewSubset(previewSubsetDevices)
		rows, eligibleCount = c.previewFleetCommandRowsFromDevices(previewSubsetDevices, req, claims)
		blockedCount = int(total) - eligibleCount
		warnings = append(warnings, fmt.Sprintf("device_filter 匹配到 %d 台设备，已超过 max_devices=%d。提交前请缩小筛选范围或提高 max_devices。", total, maxDevices))
	}

	return attachFleetCommandJobPreviewGuidance(&model.FleetCommandJobPreviewResult{
		JobType:        "command",
		ScopeType:      req.ScopeType,
		PreviewToken:   fleetCommandDeviceFilterPreviewToken(req, total, rows),
		TotalMatched:   total,
		RequestedCount: int(total),
		EligibleCount:  eligibleCount,
		BlockedCount:   blockedCount,
		TimeoutSeconds: req.TimeoutSeconds,
		Rows:           rows,
		PreviewDevices: previewSubsetDevices,
		SampleDevices:  previewSubsetDevices,
		Warnings:       warnings,
		ScopeLimits:    fleetCommandDeviceFilterPreviewScopeLimits,
	}), nil
}

func buildFleetCommandDeviceFilterListReq(filter *model.FleetCommandJobDeviceFilter, page, pageSize int) *model.GetDeviceListByPageReq {
	return &model.GetDeviceListByPageReq{
		PageReq: model.PageReq{
			Page:     page,
			PageSize: pageSize,
		},
		DeviceNumber:       filter.DeviceNumber,
		IsEnabled:          filter.IsEnabled,
		ProductID:          filter.ProductID,
		Label:              filter.Label,
		Name:               filter.Name,
		CurrentVersion:     filter.CurrentVersion,
		PIDNumber:          filter.PIDNumber,
		FirmwareVersion:    filter.FirmwareVersion,
		Description:        filter.Description,
		SharedStatus:       filter.SharedStatus,
		GroupId:            filter.GroupId,
		DeviceConfigId:     filter.DeviceConfigId,
		DeviceTemplateID:   filter.DeviceTemplateID,
		IsOnline:           filter.IsOnline,
		WarnStatus:         filter.WarnStatus,
		Search:             filter.Search,
		AccessWay:          filter.AccessWay,
		BatchNumber:        filter.BatchNumber,
		DeviceType:         filter.DeviceType,
		ServiceIdentifier:  filter.ServiceIdentifier,
		ServiceAccessID:    filter.ServiceAccessID,
		LastReportedAfter:  filter.LastReportedAfter,
		LastReportedBefore: filter.LastReportedBefore,
		NeverReported:      filter.NeverReported,
		LifecycleStatus:    filter.LifecycleStatus,
	}
}

func resolveFleetCommandDeviceFilterDevices(filter *model.FleetCommandJobDeviceFilter, tenantID string, maxDevices int, claims *utils.UserClaims) ([]model.GetDeviceListByPageRsp, error) {
	deviceListReq := buildFleetCommandDeviceFilterListReq(filter, 1, maxDevices)
	applyDeviceListOwnerFilterForClaims(deviceListReq, claims)
	deviceIDs, err := dal.ListDeviceIDsByFilter(deviceListReq, tenantID, maxDevices)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	devices, err := dal.GetDeviceListRowsByFilterAndIDs(deviceListReq, tenantID, deviceIDs)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	normalizeFleetCommandDeviceFilterPreviewSubset(devices)
	return devices, nil
}

func normalizeFleetCommandDeviceFilterPreviewSubsetLimit(limit int) int {
	if limit <= 0 {
		return defaultFleetCommandDeviceFilterPreviewSubsetLimit
	}
	if limit > maxFleetCommandDeviceFilterPreviewSubsetLimit {
		return maxFleetCommandDeviceFilterPreviewSubsetLimit
	}
	return limit
}

func fleetCommandDeviceFilterPreviewSubsetLimit(req *model.FleetCommandJobReq) int {
	if req == nil {
		return defaultFleetCommandDeviceFilterPreviewSubsetLimit
	}
	if req.SubsetLimit > 0 {
		return normalizeFleetCommandDeviceFilterPreviewSubsetLimit(req.SubsetLimit)
	}
	return normalizeFleetCommandDeviceFilterPreviewSubsetLimit(req.SampleLimit)
}

func normalizeFleetCommandDeviceFilterMaxDevices(limit int) int {
	if limit <= 0 {
		return defaultFleetCommandDeviceFilterMaxDevices
	}
	if limit > maxFleetCommandDeviceFilterMaxDevices {
		return maxFleetCommandDeviceFilterMaxDevices
	}
	return limit
}

func (c *CommandData) previewFleetCommandRowsFromDevices(devices []model.GetDeviceListByPageRsp, req *model.FleetCommandJobReq, claims *utils.UserClaims) ([]model.FleetCommandJobPreviewRow, int) {
	deviceIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		if deviceID := strings.TrimSpace(device.ID); deviceID != "" {
			deviceIDs = append(deviceIDs, deviceID)
		}
	}
	telemetryEvidence, _ := loadFleetCommandTelemetryEvidence(deviceIDs)

	rows := make([]model.FleetCommandJobPreviewRow, 0, len(devices))
	eligibleCount := 0
	for _, device := range devices {
		deviceID := strings.TrimSpace(device.ID)
		if deviceID == "" {
			continue
		}
		row := c.previewFleetCommandDeviceFilterRow(device, req, claims, telemetryEvidence)
		if row.DeviceNumber == "" {
			row.DeviceNumber = device.DeviceNumber
		}
		if row.Name == "" {
			row.Name = device.Name
		}
		if row.Eligible {
			eligibleCount++
		}
		rows = append(rows, row)
	}
	return rows, eligibleCount
}

func (c *CommandData) previewFleetCommandDeviceFilterRow(device model.GetDeviceListByPageRsp, req *model.FleetCommandJobReq, claims *utils.UserClaims, telemetryEvidence fleetCommandTelemetryEvidence) model.FleetCommandJobPreviewRow {
	row := newBlockedFleetCommandPreviewRow(strings.TrimSpace(device.ID))
	row.DeviceNumber = device.DeviceNumber
	row.Name = device.Name
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		row.Reason = "没有权限预览筛选批量命令"
		row.Advice = "请使用具备租户设备权限的操作员账号登录后，再预览筛选批量命令。"
		return row
	}

	profile := fleetCommandProfileFromDeviceListRow(device)
	return evaluateFleetCommandPreviewInput(row, profile, req, "已从筛选预览加载设备档案", telemetryEvidence)
}

func fleetCommandProfileFromDeviceListRow(row model.GetDeviceListByPageRsp) *commandDeviceProfile {
	deviceConfigID := strings.TrimSpace(row.DeviceConfigID)
	deviceType := strings.TrimSpace(row.DeviceType)
	protocolType := strings.TrimSpace(row.ProtocolType)
	if deviceType == "" {
		deviceType = "1"
	}
	if protocolType == "" {
		protocolType = "MQTT"
	}

	name := row.Name
	device := &model.Device{
		ID:             strings.TrimSpace(row.ID),
		Name:           optionalString(name),
		TenantID:       strings.TrimSpace(row.TenantID),
		DeviceNumber:   row.DeviceNumber,
		DeviceConfigID: optionalString(deviceConfigID),
		ParentID:       row.ParentID,
		SubDeviceAddr:  row.SubDeviceAddr,
		IsOnline:       int16(row.IsOnline),
		AccessWay:      optionalString(row.AccessWay),
		AdditionalInfo: row.AdditionalInfo,
	}

	return &commandDeviceProfile{
		device:       device,
		deviceType:   deviceType,
		protocolType: protocolType,
	}
}

func normalizeFleetCommandDeviceFilterPreviewSubset(devices []model.GetDeviceListByPageRsp) {
	for i := range devices {
		devices[i].DeviceStatus = devices[i].IsOnline
		devices[i].PIDNumber = devices[i].DeviceNumber
		devices[i].SharedStatus = rdiDeviceSharedStatus(devices[i].AdditionalInfo)
		if devices[i].WarnStatus == "N" || devices[i].WarnStatus == "" {
			devices[i].WarnStatus = "N"
		} else {
			devices[i].WarnStatus = "Y"
		}
	}
}

func fleetCommandDeviceFilterIsEmpty(filter *model.FleetCommandJobDeviceFilter) bool {
	if filter == nil {
		return true
	}
	return !hasFleetCommandFilterValue(filter.DeviceNumber) &&
		!hasFleetCommandFilterValue(filter.IsEnabled) &&
		!hasFleetCommandFilterValue(filter.ProductID) &&
		!hasFleetCommandFilterValue(filter.Label) &&
		!hasFleetCommandFilterValue(filter.Name) &&
		!hasFleetCommandFilterValue(filter.CurrentVersion) &&
		!hasFleetCommandFilterValue(filter.PIDNumber) &&
		!hasFleetCommandFilterValue(filter.FirmwareVersion) &&
		!hasFleetCommandFilterValue(filter.Description) &&
		!hasFleetCommandFilterValue(filter.SharedStatus) &&
		!hasFleetCommandFilterValue(filter.GroupId) &&
		!hasFleetCommandFilterValue(filter.DeviceConfigId) &&
		!hasFleetCommandFilterValue(filter.DeviceTemplateID) &&
		filter.IsOnline == nil &&
		!hasFleetCommandFilterValue(filter.WarnStatus) &&
		!hasFleetCommandFilterValue(filter.Search) &&
		!hasFleetCommandFilterValue(filter.AccessWay) &&
		!hasFleetCommandFilterValue(filter.BatchNumber) &&
		!hasFleetCommandFilterValue(filter.DeviceType) &&
		!hasFleetCommandFilterValue(filter.ServiceIdentifier) &&
		!hasFleetCommandFilterValue(filter.ServiceAccessID) &&
		filter.LastReportedAfter == nil &&
		filter.LastReportedBefore == nil &&
		filter.NeverReported == nil &&
		!hasFleetCommandFilterValue(filter.LifecycleStatus)
}

func hasFleetCommandFilterValue(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

func fleetCommandDeviceFilterPreviewToken(req *model.FleetCommandJobReq, total int64, rows []model.FleetCommandJobPreviewRow) string {
	rowTokens := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DeviceID) == "" {
			continue
		}
		rowTokens = append(rowTokens, map[string]interface{}{
			"device_id":        row.DeviceID,
			"eligible":         row.Eligible,
			"status":           row.Status,
			"recommended_path": row.RecommendedPath,
		})
	}

	payload := struct {
		ScopeType      string                             `json:"scope_type"`
		DeviceFilter   *model.FleetCommandJobDeviceFilter `json:"device_filter,omitempty"`
		ExpectedTotal  *int64                             `json:"expected_total,omitempty"`
		Identify       string                             `json:"identify"`
		Value          *string                            `json:"value,omitempty"`
		TimeoutSeconds int                                `json:"timeout_seconds"`
		ScheduledAt    *time.Time                         `json:"scheduled_at,omitempty"`
		TotalMatched   int64                              `json:"total_matched"`
		SubsetLimit    int                                `json:"subset_limit"`
		SampleLimit    int                                `json:"sample_limit"`
		MaxDevices     int                                `json:"max_devices"`
		Rows           []map[string]interface{}           `json:"rows"`
	}{
		ScopeType:      req.ScopeType,
		DeviceFilter:   req.DeviceFilter,
		ExpectedTotal:  req.ExpectedTotal,
		Identify:       req.Identify,
		Value:          req.Value,
		TimeoutSeconds: req.TimeoutSeconds,
		ScheduledAt:    req.ScheduledAt,
		TotalMatched:   total,
		SubsetLimit:    fleetCommandDeviceFilterPreviewSubsetLimit(req),
		SampleLimit:    fleetCommandDeviceFilterPreviewSubsetLimit(req),
		MaxDevices:     normalizeFleetCommandDeviceFilterMaxDevices(req.MaxDevices),
		Rows:           rowTokens,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
