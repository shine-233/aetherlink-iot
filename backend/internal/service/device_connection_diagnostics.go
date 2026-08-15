package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/diagnostics"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

const defaultConnectionDiagnosticsDebugLogLimit = int64(20)

// DeviceConnectionDiagnosticsReq keeps the approved HTTP route's read-side
// aggregation inputs aligned with the service-owned response shape.
type DeviceConnectionDiagnosticsReq struct {
	DeviceID      string `json:"device_id"`
	DebugLogLimit int64  `json:"debug_log_limit,omitempty"`
}

type DeviceConnectionDiagnosticsResp struct {
	DeviceID       string                                `json:"device_id"`
	EvaluatedAt    time.Time                             `json:"evaluated_at"`
	Conclusion     DeviceConnectionDiagnosticsConclusion `json:"conclusion"`
	Online         DeviceConnectionOnlineSnapshot        `json:"online"`
	ReadyCheck     DeviceConnectionReadyCheck            `json:"ready_check"`
	Debug          DeviceConnectionDebugSnapshot         `json:"debug"`
	Diagnostics    DeviceConnectionDiagnosticsSnapshot   `json:"diagnostics"`
	PartialResults []DeviceConnectionDiagnosticsWarning  `json:"partial_results,omitempty"`
}

type DeviceConnectionDiagnosticsConclusion struct {
	Level       string   `json:"level"`
	Code        string   `json:"code"`
	Summary     string   `json:"summary"`
	NextActions []string `json:"next_actions"`
	Evidence    []string `json:"evidence,omitempty"`
}

type DeviceConnectionOnlineSnapshot struct {
	DeviceStatus    int        `json:"device_status"`
	IsOnline        bool       `json:"is_online"`
	LastOfflineTime *time.Time `json:"last_offline_time,omitempty"`
}

type DeviceConnectionDebugSnapshot struct {
	Enabled          bool                              `json:"enabled"`
	ExpireAt         int64                             `json:"expire_at"`
	RemainingSeconds int64                             `json:"remaining_seconds"`
	Total            int64                             `json:"total"`
	Offset           int64                             `json:"offset"`
	Limit            int64                             `json:"limit"`
	RecentLogs       []DeviceConnectionDebugLogSummary `json:"recent_logs"`
}

type DeviceConnectionDebugLogSummary struct {
	Ts        string                 `json:"ts"`
	Protocol  string                 `json:"protocol,omitempty"`
	Direction string                 `json:"direction"`
	Action    string                 `json:"action,omitempty"`
	Outcome   string                 `json:"outcome,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

type DeviceConnectionDiagnosticsSnapshot struct {
	RecentFailures []diagnostics.FailureRecord `json:"recent_failures"`
}

type DeviceConnectionDiagnosticsWarning struct {
	Component string `json:"component"`
	Reason    string `json:"reason"`
}

type DeviceConnectionReadyCheck struct {
	Ready       bool                               `json:"ready"`
	Level       string                             `json:"level"`
	Code        string                             `json:"code"`
	Summary     string                             `json:"summary"`
	NextActions []string                           `json:"next_actions"`
	Telemetry   DeviceConnectionTelemetryReadiness `json:"telemetry"`
}

type DeviceConnectionTelemetryReadiness struct {
	HasRecentCurrent bool        `json:"has_recent_current"`
	CurrentCount     int         `json:"current_count"`
	LatestKey        string      `json:"latest_key,omitempty"`
	LatestAt         *time.Time  `json:"latest_at,omitempty"`
	LatestValue      interface{} `json:"latest_value,omitempty"`
}

// GetConnectionDiagnostics aggregates existing read-only connection evidence:
// persisted online state, last offline timestamp, recent sanitized debug logs,
// and diagnostics collector failures.
func (*Device) GetConnectionDiagnostics(
	ctx context.Context,
	req DeviceConnectionDiagnosticsReq,
	claims *utils.UserClaims,
) (*DeviceConnectionDiagnosticsResp, error) {
	ctx = connectionDiagnosticsContext(ctx)
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}

	deviceInfo, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}

	return buildConnectionDiagnosticsForDevice(ctx, deviceID, req.DebugLogLimit, deviceInfo), nil
}

func buildConnectionDiagnosticsForDevice(
	ctx context.Context,
	deviceID string,
	debugLogLimit int64,
	deviceInfo *model.Device,
) *DeviceConnectionDiagnosticsResp {
	resp := &DeviceConnectionDiagnosticsResp{
		DeviceID:    deviceID,
		EvaluatedAt: time.Now().UTC(),
		Online:      buildConnectionOnlineSnapshot(deviceInfo),
		Debug:       emptyConnectionDebugSnapshot(normalizeConnectionDiagnosticsDebugLogLimit(debugLogLimit)),
		Diagnostics: DeviceConnectionDiagnosticsSnapshot{RecentFailures: []diagnostics.FailureRecord{}},
	}

	resp.Debug = collectConnectionDebugSnapshot(ctx, deviceID, resp.Debug.Limit, resp)
	resp.Diagnostics = collectConnectionDiagnosticsSnapshot(deviceID, resp)
	resp.ReadyCheck = collectConnectionReadyCheck(deviceID, resp)
	resp.Conclusion = buildConnectionDiagnosticsConclusion(resp)

	return resp
}

func connectionDiagnosticsContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func buildConnectionOnlineSnapshot(deviceInfo *model.Device) DeviceConnectionOnlineSnapshot {
	if deviceInfo == nil {
		return DeviceConnectionOnlineSnapshot{}
	}
	return DeviceConnectionOnlineSnapshot{
		DeviceStatus:    int(deviceInfo.IsOnline),
		IsOnline:        deviceInfo.IsOnline == 1,
		LastOfflineTime: deviceInfo.LastOfflineTime,
	}
}

func emptyConnectionDebugSnapshot(limit int64) DeviceConnectionDebugSnapshot {
	return DeviceConnectionDebugSnapshot{
		Offset:     0,
		Limit:      limit,
		RecentLogs: []DeviceConnectionDebugLogSummary{},
	}
}

func collectConnectionDebugSnapshot(
	ctx context.Context,
	deviceID string,
	limit int64,
	resp *DeviceConnectionDiagnosticsResp,
) DeviceConnectionDebugSnapshot {
	debug := emptyConnectionDebugSnapshot(limit)
	if global.REDIS == nil {
		addConnectionDiagnosticsWarning(resp, "device_debug", "redis_not_initialized")
		return debug
	}

	status, err := queryDeviceDebugStatus(ctx, deviceID, time.Now().Unix())
	if err != nil {
		addConnectionDiagnosticsWarning(resp, "device_debug", "status_unavailable")
	} else {
		debug.Enabled = status.Enabled
		debug.ExpireAt = status.ExpireAt
		debug.RemainingSeconds = status.RemainingSeconds
	}

	logs, err := queryDeviceDebugLogs(ctx, deviceID, 0, limit)
	if err != nil {
		addConnectionDiagnosticsWarning(resp, "device_debug", "logs_unavailable")
		return debug
	}
	debug.Total = logs.Total
	debug.Offset = logs.Offset
	debug.Limit = logs.Limit
	debug.RecentLogs = summarizeConnectionDebugLogs(logs.List)
	return debug
}

func collectConnectionDiagnosticsSnapshot(
	deviceID string,
	resp *DeviceConnectionDiagnosticsResp,
) DeviceConnectionDiagnosticsSnapshot {
	snapshot := DeviceConnectionDiagnosticsSnapshot{
		RecentFailures: []diagnostics.FailureRecord{},
	}

	data, err := diagnostics.GetInstance().GetDiagnostics(deviceID)
	if err != nil {
		if errors.Is(err, diagnostics.ErrNotInitialized) {
			addConnectionDiagnosticsWarning(resp, "diagnostics", "collector_not_initialized")
			return snapshot
		}
		addConnectionDiagnosticsWarning(resp, "diagnostics", "collector_unavailable")
		return snapshot
	}
	if data != nil && data.RecentFailures != nil {
		snapshot.RecentFailures = data.RecentFailures
	}
	return snapshot
}

func normalizeConnectionDiagnosticsDebugLogLimit(limit int64) int64 {
	if limit <= 0 {
		return defaultConnectionDiagnosticsDebugLogLimit
	}
	if limit > maxDebugLogsLimit {
		return maxDebugLogsLimit
	}
	return limit
}

func summarizeConnectionDebugLogs(logs []model.DeviceDebugLogEntry) []DeviceConnectionDebugLogSummary {
	summaries := make([]DeviceConnectionDebugLogSummary, 0, len(logs))
	for _, log := range logs {
		summaries = append(summaries, DeviceConnectionDebugLogSummary{
			Ts:        log.Ts,
			Protocol:  log.Protocol,
			Direction: log.Direction,
			Action:    log.Action,
			Outcome:   log.Outcome,
			Meta:      log.Meta,
			Error:     log.Error,
		})
	}
	return summaries
}

func addConnectionDiagnosticsWarning(
	resp *DeviceConnectionDiagnosticsResp,
	component string,
	reason string,
) {
	if resp == nil {
		return
	}
	resp.PartialResults = append(resp.PartialResults, DeviceConnectionDiagnosticsWarning{
		Component: component,
		Reason:    reason,
	})
}

func collectConnectionReadyCheck(deviceID string, resp *DeviceConnectionDiagnosticsResp) DeviceConnectionReadyCheck {
	currentCount, latest, err := dal.GetCurrentTelemetryReadiness(deviceID)
	if err != nil {
		addConnectionDiagnosticsWarning(resp, "ready_check", "telemetry_unavailable")
		return buildConnectionReadyCheck(resp, DeviceConnectionTelemetryReadiness{})
	}

	readiness := buildConnectionTelemetryReadinessSnapshot(currentCount, latest)
	return buildConnectionReadyCheck(resp, readiness)
}
