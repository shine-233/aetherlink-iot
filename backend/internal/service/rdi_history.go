// rdi_history.go reads RDI history data for frontend chart/table views.
//
// Purpose: authorize RDI device access, validate allowed history keys, and
// select telemetry or alarm-history data based on request type. Core logic
// keeps RDI-specific key filtering close to service access checks while
// delegating persistence details to DAL helpers. Important notes: broadening
// accepted keys can expose unrelated telemetry, so history changes need
// authorization and negative-key tests. Refactor suggestion: replace string
// request types with a small typed selector if more RDI history sources appear.
package service

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	rdiHistoryMaxRange          = 30 * 24 * time.Hour
	rdiHistoryMillisEpochCutoff = int64(1_000_000_000_000)
)

func (*RDI) DeviceHistory(deviceID string, req *model.RDIHistoryReq, claims *utils.UserClaims) (interface{}, error) {
	device, err := getRDIDeviceForRead(deviceID, claims)
	if err != nil {
		return nil, err
	}
	if err := validateRDIHistoryTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.Key)
	if !allowedRDIHistoryKey(key) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported RDI history key")
	}

	telemetryReq := &model.GetTelemetryHistoryDataByPageReq{
		DeviceID:     device.ID,
		Key:          key,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		ExportExcel:  req.ExportExcel,
		ExportFormat: req.ExportFormat,
		Page:         req.Page,
		PageSize:     req.PageSize,
	}
	return GroupApp.TelemetryData.GetTelemetrHistoryDataByPageV2(telemetryReq, claims)
}

func validateRDIHistoryTimeRange(startTime, endTime int64) error {
	if endTime < startTime {
		return errcode.NewWithMessage(errcode.CodeParamError, "end_time must be greater than or equal to start_time")
	}
	if startTime <= 0 || endTime <= 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "start_time and end_time must be positive")
	}
	if rdiHistoryRangeDuration(startTime, endTime) > rdiHistoryMaxRange {
		return errcode.NewWithMessage(errcode.CodeParamError, "RDI history time range must not exceed 30 days")
	}
	return nil
}

func rdiHistoryRangeDuration(startTime, endTime int64) time.Duration {
	unit := time.Second
	if startTime >= rdiHistoryMillisEpochCutoff || endTime >= rdiHistoryMillisEpochCutoff {
		unit = time.Millisecond
	}
	return time.Duration(endTime-startTime) * unit
}

func allowedRDIHistoryKey(key string) bool {
	return isAllowedValue(key,
		"temperature_1",
		"temperature_2",
		"switch_1",
		"switch_2",
		"dry_contact_output",
		"electricity_consumption",
	)
}
