// 文件用途：验证遥测查询、聚合窗口和读写权限边界。
// 核心逻辑：覆盖时间范围、聚合间隔、历史值格式化、设备读写权限和下行入口校验。
// 关键注意事项：遥测测试直接影响图表和诊断数据，需防止空设备、超长范围和无权限请求进入 DAL 或下行总线。
// 重构建议：拆分时间窗口、权限、历史导出和下行入口测试，补齐时区、事务和外部副作用边界。
// telemetry_data_test.go protects telemetry query and write validation rules.
//
// Purpose: test telemetry time-range processing, aggregate-window validation, timestamp formatting, access guards, and history value conversion.
// Core logic: exercises pure helpers and early service entrypoint failures without requiring a live telemetry store.
// Important notes: time windows and aggregate defaults drive chart correctness, so boundary cases around one-day limits and interval names must stay explicit.
// Refactor suggestion: split time-range, access-guard, and value-format tests when telemetry coverage expands.
package service

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func assertTelemetryErrcode(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return telemetry service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, wantCode, wantMessage)
	}
}

func assertTelemetryErrcodeVars(t *testing.T, err error, context string, wantCode int, wantVars map[string]interface{}) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return telemetry service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	for key, want := range wantVars {
		if got := appErr.Variables[key]; got != want {
			t.Fatalf("%s variable %q = %#v, want %#v", context, key, got, want)
		}
	}
}

func TestProcessTimeRangeNoAggregateExceedsOneDay(t *testing.T) {
	now := time.Now()
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "no_aggregate",
		TimeRange:       "custom",
		StartTime:       now.Add(-25*time.Hour).UnixNano() / 1e6,
		EndTime:         now.UnixNano() / 1e6,
	}

	err := processTimeRange(req)
	if err == nil {
		t.Fatal("processTimeRange should reject no_aggregate with >24h range")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("expected *errcode.Error, got %T", err)
	}
	if appErr.Code != 207001 {
		t.Fatalf("error code = %d, want 207001", appErr.Code)
	}
}

func TestProcessTimeRangeNoAggregateWithinOneDay(t *testing.T) {
	now := time.Now()
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "no_aggregate",
		TimeRange:       "custom",
		StartTime:       now.Add(-12*time.Hour).UnixNano() / 1e6,
		EndTime:         now.UnixNano() / 1e6,
	}

	err := processTimeRange(req)
	if err != nil {
		t.Fatalf("processTimeRange returned error: %v", err)
	}
}

func TestProcessTimeRangeCustomInvalidStartEnd(t *testing.T) {
	tests := []struct {
		name      string
		startTime int64
		endTime   int64
		wantErr   bool
	}{
		{
			name:      "start time zero",
			startTime: 0,
			endTime:   time.Now().UnixNano() / 1e6,
			wantErr:   true,
		},
		{
			name:      "end time zero",
			startTime: time.Now().Add(-1*time.Hour).UnixNano() / 1e6,
			endTime:   0,
			wantErr:   true,
		},
		{
			name:      "start after end",
			startTime: time.Now().UnixNano() / 1e6,
			endTime:   time.Now().Add(-1*time.Hour).UnixNano() / 1e6,
			wantErr:   true,
		},
		{
			name:      "valid custom range",
			startTime: time.Now().Add(-1*time.Hour).UnixNano() / 1e6,
			endTime:   time.Now().UnixNano() / 1e6,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &model.GetTelemetryStatisticReq{
				DeviceId:        "device-1",
				Key:             "temperature",
				AggregateWindow: "1m",
				TimeRange:       "custom",
				StartTime:       tt.startTime,
				EndTime:         tt.endTime,
			}

			err := processTimeRange(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("processTimeRange returned nil error, want error")
				}
				appErr, ok := err.(*errcode.Error)
				if !ok {
					t.Fatalf("expected *errcode.Error, got %T", err)
				}
				if appErr.Code != 207002 {
					t.Fatalf("error code = %d, want 207002", appErr.Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("processTimeRange returned error: %v", err)
			}
		})
	}
}

func TestProcessTimeRangeNamedRanges(t *testing.T) {
	namedRanges := []string{
		"last_5m",
		"last_15m",
		"last_30m",
		"last_1h",
		"last_3h",
		"last_6h",
		"last_12h",
		"last_24h",
		"last_3d",
		"last_7d",
		"last_15d",
		"last_30d",
		"last_60d",
		"last_90d",
		"last_6m",
		"last_1y",
	}

	for _, tr := range namedRanges {
		t.Run(tr, func(t *testing.T) {
			req := &model.GetTelemetryStatisticReq{
				DeviceId:        "device-1",
				Key:             "temperature",
				AggregateWindow: "1m",
				TimeRange:       tr,
			}

			err := processTimeRange(req)
			if err != nil {
				t.Fatalf("processTimeRange for %q returned error: %v", tr, err)
			}
			if req.StartTime == 0 || req.EndTime == 0 {
				t.Fatalf("processTimeRange for %q did not set start/end times", tr)
			}
			if req.StartTime >= req.EndTime {
				t.Fatalf("processTimeRange for %q: startTime(%d) >= endTime(%d)", tr, req.StartTime, req.EndTime)
			}
		})
	}
}

func TestProcessTimeRangeInvalidNamedRange(t *testing.T) {
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "1m",
		TimeRange:       "invalid_range",
	}

	err := processTimeRange(req)
	assertTelemetryErrcodeVars(t, err, "invalid named telemetry time range", 207003, map[string]interface{}{
		"time_range": "invalid_range",
	})
}

func TestGetTelemetrServeStatisticDataRejectsNilRequestBeforeAccess(t *testing.T) {
	_, err := (&TelemetryData{}).GetTelemetrServeStatisticData(nil, &utils.UserClaims{TenantID: "tenant-1"})
	assertTelemetryErrcode(t, err, "nil telemetry statistic request", errcode.CodeParamError, "request is required")
}

func TestValidateTelemetryStatisticReqRejectsMissingFieldsBeforeAccess(t *testing.T) {
	tests := []struct {
		name        string
		req         *model.GetTelemetryStatisticReq
		wantMessage string
	}{
		{
			name:        "blank device id",
			req:         &model.GetTelemetryStatisticReq{DeviceId: " ", Key: "temperature", TimeRange: "last_1h", AggregateWindow: "1m"},
			wantMessage: "device_id is required",
		},
		{
			name:        "blank key",
			req:         &model.GetTelemetryStatisticReq{DeviceId: "device-1", Key: " ", TimeRange: "last_1h", AggregateWindow: "1m"},
			wantMessage: "key is required",
		},
		{
			name:        "blank time range",
			req:         &model.GetTelemetryStatisticReq{DeviceId: "device-1", Key: "temperature", TimeRange: " ", AggregateWindow: "1m"},
			wantMessage: "time_range is required",
		},
		{
			name:        "blank aggregate window",
			req:         &model.GetTelemetryStatisticReq{DeviceId: "device-1", Key: "temperature", TimeRange: "last_1h", AggregateWindow: " "},
			wantMessage: "aggregate_window is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTelemetryStatisticReq(tt.req)
			assertTelemetryErrcode(t, err, tt.name, errcode.CodeParamError, tt.wantMessage)
		})
	}
}

func TestValidateTelemetryStatisticReqNormalizesAggregateFunction(t *testing.T) {
	tests := []struct {
		name                  string
		req                   *model.GetTelemetryStatisticReq
		wantAggregateFunction string
		wantMessage           string
	}{
		{
			name: "defaults aggregate query to avg",
			req: &model.GetTelemetryStatisticReq{
				DeviceId:        "device-1",
				Key:             "temperature",
				TimeRange:       "last_1h",
				AggregateWindow: "1m",
			},
			wantAggregateFunction: "avg",
		},
		{
			name: "keeps supported diff aggregate",
			req: &model.GetTelemetryStatisticReq{
				DeviceId:          "device-1",
				Key:               "temperature",
				TimeRange:         "last_1h",
				AggregateWindow:   "1m",
				AggregateFunction: "diff",
			},
			wantAggregateFunction: "diff",
		},
		{
			name: "clears aggregate function for raw points",
			req: &model.GetTelemetryStatisticReq{
				DeviceId:          "device-1",
				Key:               "temperature",
				TimeRange:         "last_1h",
				AggregateWindow:   "no_aggregate",
				AggregateFunction: "avg",
			},
			wantAggregateFunction: "",
		},
		{
			name: "rejects unsupported aggregate function before DAL",
			req: &model.GetTelemetryStatisticReq{
				DeviceId:          "device-1",
				Key:               "temperature",
				TimeRange:         "last_1h",
				AggregateWindow:   "1m",
				AggregateFunction: "median",
			},
			wantMessage: "unsupported aggregate_function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTelemetryStatisticReq(tt.req)
			if tt.wantMessage != "" {
				assertTelemetryErrcode(t, err, tt.name, errcode.CodeParamError, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validateTelemetryStatisticReq returned error: %v", err)
			}
			if tt.req.AggregateFunction != tt.wantAggregateFunction {
				t.Fatalf("aggregate function = %q, want %q", tt.req.AggregateFunction, tt.wantAggregateFunction)
			}
		})
	}
}

func TestValidateTelemetryStatisticReqTrimsQueryFields(t *testing.T) {
	req := &model.GetTelemetryStatisticReq{
		DeviceId:          " device-1 ",
		Key:               " temperature ",
		TimeRange:         " last_1h ",
		AggregateWindow:   " 1m ",
		AggregateFunction: " avg ",
	}

	if err := validateTelemetryStatisticReq(req); err != nil {
		t.Fatalf("validateTelemetryStatisticReq returned error: %v", err)
	}
	if req.DeviceId != "device-1" || req.Key != "temperature" || req.TimeRange != "last_1h" || req.AggregateWindow != "1m" || req.AggregateFunction != "avg" {
		t.Fatalf("validateTelemetryStatisticReq did not trim request fields: %#v", req)
	}
}

func TestValidateAggregateWindow(t *testing.T) {
	now := time.Date(2026, 6, 30, 5, 38, 0, 0, time.Local)

	tests := []struct {
		name            string
		startTime       int64
		endTime         int64
		aggregateWindow string
		wantErr         bool
		wantVars        map[string]interface{}
	}{
		{
			name:            "short range with small interval is valid",
			startTime:       now.Add(-1*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "30s",
			wantErr:         false,
		},
		{
			name:            "long range with small interval is invalid",
			startTime:       now.Add(-400*24*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "30s",
			wantErr:         true,
			wantVars: map[string]interface{}{
				"time_range":         "1 year",
				"min_interval":       "7d",
				"aggregate_window":   "30s",
				"current_time_range": "2025-05-26 05:38:00 to 2026-06-30 05:38:00 (400 days)",
			},
		},
		{
			name:            "long range with large interval is valid",
			startTime:       now.Add(-400*24*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "1mo",
			wantErr:         false,
		},
		{
			name:            "1 day range with 2m interval is valid",
			startTime:       now.Add(-24*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "2m",
			wantErr:         false,
		},
		{
			name:            "7 day range with 2m interval is invalid",
			startTime:       now.Add(-7*24*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "2m",
			wantErr:         true,
			wantVars: map[string]interface{}{
				"time_range":       "3 days",
				"min_interval":     "5m",
				"aggregate_window": "2m",
			},
		},
		{
			name:            "7 day range with 10m interval is valid",
			startTime:       now.Add(-7*24*time.Hour).UnixNano() / 1e6,
			endTime:         now.UnixNano() / 1e6,
			aggregateWindow: "10m",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAggregateWindow(tt.startTime, tt.endTime, tt.aggregateWindow)
			if tt.wantErr {
				assertTelemetryErrcodeVars(t, err, "invalid aggregate window", 207004, tt.wantVars)
				return
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateAggregateWindow returned error: %v", err)
			}
		})
	}
}

func TestValidateAggregateWindowRejectsUnsupportedWindowBeforeDAL(t *testing.T) {
	now := time.Now()
	err := validateAggregateWindow(now.Add(-time.Hour).UnixNano()/1e6, now.UnixNano()/1e6, "unsupported")
	assertTelemetryErrcode(t, err, "unsupported aggregate window", errcode.CodeParamError, "unsupported aggregate_window")
}

func TestFetchTelemetryDataRejectsUnsupportedAggregateWindowBeforeDAL(t *testing.T) {
	now := time.Now()
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "unsupported",
		StartTime:       now.Add(-time.Hour).UnixNano() / 1e6,
		EndTime:         now.UnixNano() / 1e6,
	}

	_, err := fetchTelemetryData(req)
	assertTelemetryErrcode(t, err, "fetch telemetry unsupported aggregate window", errcode.CodeParamError, "unsupported aggregate_window")
}

func TestIsValidInterval(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		minInterval string
		want        bool
	}{
		{
			name:        "1m >= 30s",
			current:     "1m",
			minInterval: "30s",
			want:        true,
		},
		{
			name:        "30s >= 30s",
			current:     "30s",
			minInterval: "30s",
			want:        true,
		},
		{
			name:        "1h >= 30s",
			current:     "1h",
			minInterval: "30s",
			want:        true,
		},
		{
			name:        "30s < 1m",
			current:     "30s",
			minInterval: "1m",
			want:        false,
		},
		{
			name:        "unknown current interval",
			current:     "unknown",
			minInterval: "1m",
			want:        false,
		},
		{
			name:        "unknown min interval",
			current:     "1m",
			minInterval: "unknown",
			want:        false,
		},
		{
			name:        "1mo >= 7d",
			current:     "1mo",
			minInterval: "7d",
			want:        true,
		},
		{
			name:        "1d < 7d",
			current:     "1d",
			minInterval: "7d",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidInterval(tt.current, tt.minInterval)
			if got != tt.want {
				t.Fatalf("isValidInterval(%q, %q) = %v, want %v", tt.current, tt.minInterval, got, tt.want)
			}
		})
	}
}

func TestFormatTimeUsesTelemetryDiagnosticTimestampContract(t *testing.T) {
	ts := time.Date(2023, 11, 14, 22, 13, 20, 0, time.Local).UnixNano() / 1e6

	if got := formatTime(ts); got != "2023-11-14 22:13:20" {
		t.Fatalf("formatTime() = %q, want telemetry diagnostic timestamp format", got)
	}
}

func TestTelemetryDataStructExists(t *testing.T) {
	td := &TelemetryData{}
	if td == nil {
		t.Fatal("TelemetryData struct should be instantiable")
	}
}

func TestTelemetryDataSetDownlinkBus(t *testing.T) {
	td := &TelemetryData{}
	bus := downlink.NewBus(1)
	defer bus.Close()

	td.SetDownlinkBus(bus)
	if td.downlinkBus != bus {
		t.Fatal("SetDownlinkBus should store the injected telemetry downlink bus")
	}

	td.SetDownlinkBus(nil)
	if td.downlinkBus != nil {
		t.Fatal("downlinkBus should be nil after SetDownlinkBus(nil)")
	}
}

func TestEnsureTelemetryDeviceWriteAccessNilClaims(t *testing.T) {
	_, err := ensureTelemetryDeviceWriteAccess("device-1", nil)
	assertTelemetryErrcode(t, err, "ensureTelemetryDeviceWriteAccess nil claims", errcode.CodeNoPermission, "no permission to modify device telemetry")
}

func TestLoadTelemetryDeviceForAccessMapsMissingDeviceToNotFound(t *testing.T) {
	// 错误码迁移批次（2026-08，承接 PR #123 回滚后的正式迁移）：设备主行不存在时，
	// 裸 gorm.ErrRecordNotFound 不再透传（否则会被响应中间件兜底成 100000 系统内部错误），
	// 而是显式映射为 100404 资源不存在业务码。
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "telemetry-notfound-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "existing-device", "existing-number", "tenant-a", "user-a", configID, now)

	claims := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER}

	_, err := loadTelemetryDeviceForAccess("missing-device", claims, telemetryReadPermissionMessage)
	assertTelemetryErrcode(t, err, "missing device read access", errcode.CodeNotFound, "device not found")

	_, err = loadTelemetryDeviceForAccess("missing-device", claims, telemetryWritePermissionMessage)
	assertTelemetryErrcode(t, err, "missing device write access", errcode.CodeNotFound, "device not found")

	// 完整访问守卫链对不存在的设备同样返回明确的资源不存在业务码。
	_, err = ensureTelemetryDeviceReadAccess("missing-device", claims)
	assertTelemetryErrcode(t, err, "ensure read access missing device", errcode.CodeNotFound, "device not found")

	_, err = ensureTelemetryDeviceWriteAccess("missing-device", claims)
	assertTelemetryErrcode(t, err, "ensure write access missing device", errcode.CodeNotFound, "device not found")
}

func TestEnsureTelemetryDeviceReadAccessCrossTenantStaysPermissionShaped(t *testing.T) {
	// 对照守卫：库内存在但越租户的设备走 permission-shaped 无权限分支，防泄漏语义不变。
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "telemetry-notfound-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "existing-device", "existing-number", "tenant-a", "user-a", configID, now)

	_, err := ensureTelemetryDeviceReadAccess("existing-device", &utils.UserClaims{
		ID:        "user-other",
		TenantID:  "tenant-other",
		Authority: constant.TENANT_USER,
	})
	assertTelemetryErrcode(t, err, "cross-tenant device read", errcode.CodeNoPermission, telemetryReadPermissionMessage)
}

func TestEnsureTelemetryDeviceWriteAccessRejectsBlankDeviceIDBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{TenantID: "tenant-1"}
	for _, deviceID := range []string{"", "   "} {
		_, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
		assertTelemetryErrcode(t, err, "blank telemetry device id", errcode.CodeParamError, "device_id is required")
	}
}

func TestEnsureTelemetryDeviceReadAccessRejectsNilClaimsAndBlankDeviceIDBeforeDAL(t *testing.T) {
	_, err := ensureTelemetryDeviceReadAccess("device-1", nil)
	assertTelemetryErrcode(t, err, "ensureTelemetryDeviceReadAccess nil claims", errcode.CodeNoPermission, "no permission to query device telemetry")

	claims := &utils.UserClaims{TenantID: "tenant-1"}
	for _, deviceID := range []string{"", "   "} {
		_, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
		assertTelemetryErrcode(t, err, "blank telemetry read device id", errcode.CodeParamError, "device_id is required")
	}
}

func TestEnsureTelemetryDeviceAccessAppliesTenantUserOwnerFilter(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "telemetry-owner-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "telemetry-owner-a", "telemetry-owner-number-a", "tenant-a", "user-a", configID, now)
	createDeviceServiceOwnedDevice(t, db, "telemetry-owner-b", "telemetry-owner-number-b", "tenant-a", "user-b", configID, now)

	tenantUserA := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER}
	if _, err := ensureTelemetryDeviceReadAccess("telemetry-owner-a", tenantUserA); err != nil {
		t.Fatalf("owner read access should pass: %v", err)
	}
	if _, err := ensureTelemetryDeviceWriteAccess("telemetry-owner-a", tenantUserA); err != nil {
		t.Fatalf("owner write access should pass: %v", err)
	}

	_, err := ensureTelemetryDeviceReadAccess("telemetry-owner-b", tenantUserA)
	assertTelemetryErrcode(t, err, "tenant user non-owner read", errcode.CodeNoPermission, "no permission to query device telemetry")

	_, err = ensureTelemetryDeviceWriteAccess("telemetry-owner-b", tenantUserA)
	assertTelemetryErrcode(t, err, "tenant user non-owner write", errcode.CodeNoPermission, "no permission to modify device telemetry")

	if _, err := ensureTelemetryDeviceWriteAccess("telemetry-owner-b", &utils.UserClaims{ID: "admin-a", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}); err != nil {
		t.Fatalf("tenant admin write access should pass: %v", err)
	}
}

func TestTelemetryWriteEntrypointsRejectBlankDeviceIDBeforeDALOrDownlink(t *testing.T) {
	claims := &utils.UserClaims{TenantID: "tenant-1"}
	ctx := context.Background()

	if err := (&TelemetryData{}).DeleteTelemetrData(&model.DeleteTelemetryDataReq{
		DeviceID: "   ",
		Key:      "temperature",
	}, claims); err != nil {
		assertTelemetryErrcode(t, err, "DeleteTelemetrData blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("DeleteTelemetrData should reject blank device id before DAL")
	}

	if err := (&AttributeData{}).AttributePutMessage(ctx, "operator-1", &model.AttributePutMessage{
		DeviceID: "   ",
		Value:    `{"switch":true}`,
	}, "manual", claims); err != nil {
		assertTelemetryErrcode(t, err, "AttributePutMessage blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("AttributePutMessage should reject blank device id before device cache or downlink")
	}

	if err := (&AttributeData{}).AttributeGetMessage(claims, &model.AttributeGetMessageReq{
		DeviceID: "   ",
		Keys:     []string{"temperature"},
	}); err != nil {
		assertTelemetryErrcode(t, err, "AttributeGetMessage blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("AttributeGetMessage should reject blank device id before DAL or downlink")
	}

	value := `{"mode":"auto"}`
	if err := (&CommandData{}).CommandPutMessage(ctx, "operator-1", &model.PutMessageForCommand{
		DeviceID: "   ",
		Identify: "set_mode",
		Value:    &value,
	}, "manual", claims); err != nil {
		assertTelemetryErrcode(t, err, "CommandPutMessage blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("CommandPutMessage should reject blank device id before device cache or downlink")
	}

	if tracking, err := (&CommandData{}).CommandPutMessageWithTracking(ctx, "operator-1", &model.PutMessageForCommand{
		DeviceID: "   ",
		Identify: "set_mode",
		Value:    &value,
	}, "manual", claims); err != nil {
		if tracking != nil {
			t.Fatal("CommandPutMessageWithTracking should not return tracking data when validation fails")
		}
		assertTelemetryErrcode(t, err, "CommandPutMessageWithTracking blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("CommandPutMessageWithTracking should reject blank device id before device cache or downlink")
	}
}

func TestTelemetryReadEntrypointsRejectBlankDeviceIDBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{TenantID: "tenant-1"}
	service := &TelemetryData{}

	if _, err := service.GetCurrentTelemetrData("   ", claims); err != nil {
		assertTelemetryErrcode(t, err, "GetCurrentTelemetrData blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetCurrentTelemetrData should reject blank device id before DAL")
	}

	if _, err := service.GetCurrentTelemetrDataKeys(&model.GetTelemetryCurrentDataKeysReq{
		DeviceID: "   ",
		Keys:     []string{"temperature"},
	}, claims); err != nil {
		assertTelemetryErrcode(t, err, "GetCurrentTelemetrDataKeys blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetCurrentTelemetrDataKeys should reject blank device id before DAL")
	}

	if _, err := service.GetCurrentTelemetrDataForWs("   ", claims); err != nil {
		assertTelemetryErrcode(t, err, "GetCurrentTelemetrDataForWs blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetCurrentTelemetrDataForWs should reject blank device id before DAL")
	}

	if _, err := service.GetCurrentTelemetrDataKeysForWs("   ", []string{"temperature"}, claims); err != nil {
		assertTelemetryErrcode(t, err, "GetCurrentTelemetrDataKeysForWs blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetCurrentTelemetrDataKeysForWs should reject blank device id before DAL")
	}

	if _, err := service.GetCurrentTelemetrDetailData("   ", claims); err != nil {
		assertTelemetryErrcode(t, err, "GetCurrentTelemetrDetailData blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetCurrentTelemetrDetailData should reject blank device id before DAL")
	}

	if _, err := (&AttributeData{}).GetAttributeDataList("   ", claims); err != nil {
		assertTelemetryErrcode(t, err, "GetAttributeDataList blank device id", errcode.CodeParamError, "device_id is required")
	} else {
		t.Fatal("GetAttributeDataList should reject blank device id before DAL")
	}
}

func TestProcessTimeRangeNoAggregateExactlyOneDay(t *testing.T) {
	now := time.Now()
	// Exactly 24 hours should be allowed
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "no_aggregate",
		TimeRange:       "custom",
		StartTime:       now.Add(-24*time.Hour).UnixNano() / 1e6,
		EndTime:         now.UnixNano() / 1e6,
	}

	err := processTimeRange(req)
	if err != nil {
		t.Fatalf("processTimeRange for exactly 24h should not error: %v", err)
	}
}

func TestProcessTimeRangeSetsDefaultAggregateFunction(t *testing.T) {
	// Default aggregate function normalization happens before time-range processing.
	req := &model.GetTelemetryStatisticReq{
		DeviceId:          "device-1",
		Key:               "temperature",
		AggregateWindow:   "1m",
		AggregateFunction: "",
		TimeRange:         "last_1h",
	}

	err := processTimeRange(req)
	if err != nil {
		t.Fatalf("processTimeRange returned error: %v", err)
	}
}

func TestGetTelemetryStatisticDataByDeviceIdsValidation(t *testing.T) {
	td := &TelemetryData{}
	claims := &utils.UserClaims{
		ID:        "user-1",
		Authority: "TENANT_USER",
		TenantID:  "tenant-1",
	}

	tests := []struct {
		name     string
		req      *model.GetTelemetryStatisticByDeviceIdReq
		wantErr  bool
		wantCode int
	}{
		{
			name: "mismatched device IDs and keys count",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds: []string{"device-1"},
				Keys:      []string{"key-1", "key-2"},
			},
			wantErr:  true,
			wantCode: errcode.CodeParamError,
		},
		{
			name: "empty device IDs and keys",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds: []string{},
				Keys:      []string{},
			},
			wantErr:  true,
			wantCode: errcode.CodeParamError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := td.GetTelemetryStatisticDataByDeviceIds(tt.req, claims)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("GetTelemetryStatisticDataByDeviceIds returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("GetTelemetryStatisticDataByDeviceIds returned nil error, want error")
			}
			appErr, ok := err.(*errcode.Error)
			if !ok {
				t.Fatalf("expected *errcode.Error, got %T", err)
			}
			if appErr.Code != tt.wantCode {
				t.Fatalf("error code = %d, want %d", appErr.Code, tt.wantCode)
			}
		})
	}
}

func TestGetTelemetryStatisticDataByDeviceIdsRejectsNilAndBlankPairsBeforeDAL(t *testing.T) {
	td := &TelemetryData{}
	claims := &utils.UserClaims{TenantID: "tenant-1"}

	tests := []struct {
		name        string
		req         *model.GetTelemetryStatisticByDeviceIdReq
		wantMessage string
	}{
		{
			name:        "nil request",
			req:         nil,
			wantMessage: "request is required",
		},
		{
			name: "blank device id",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds:       []string{" "},
				Keys:            []string{"temperature"},
				TimeType:        "hour",
				AggregateMethod: "avg",
			},
			wantMessage: "device_id is required",
		},
		{
			name: "blank key",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds:       []string{"device-1"},
				Keys:            []string{" "},
				TimeType:        "hour",
				AggregateMethod: "avg",
			},
			wantMessage: "key is required",
		},
		{
			name: "unsupported time type",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds:       []string{"device-1"},
				Keys:            []string{"temperature"},
				TimeType:        "quarter",
				AggregateMethod: "avg",
			},
			wantMessage: "unsupported time_type",
		},
		{
			name: "unsupported aggregate method",
			req: &model.GetTelemetryStatisticByDeviceIdReq{
				DeviceIds:       []string{"device-1"},
				Keys:            []string{"temperature"},
				TimeType:        "hour",
				AggregateMethod: "median",
			},
			wantMessage: "unsupported aggregate_method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := td.GetTelemetryStatisticDataByDeviceIds(tt.req, claims)
			assertTelemetryErrcode(t, err, tt.name, errcode.CodeParamError, tt.wantMessage)
		})
	}
}

func TestHistoryTelemetryValueToString(t *testing.T) {
	tests := []struct {
		name string
		data *model.TelemetryData
		want string
	}{
		{
			name: "nil data",
			data: nil,
			want: "",
		},
		{
			name: "string value",
			data: &model.TelemetryData{StringV: StringPtr("hello")},
			want: "hello",
		},
		{
			name: "number value",
			data: &model.TelemetryData{NumberV: Float64Ptr(42.5)},
			want: "42.5",
		},
		{
			name: "bool value true",
			data: &model.TelemetryData{BoolV: BoolPtr(true)},
			want: "true",
		},
		{
			name: "bool value false",
			data: &model.TelemetryData{BoolV: BoolPtr(false)},
			want: "false",
		},
		{
			name: "all nil values",
			data: &model.TelemetryData{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := historyTelemetryValueToString(tt.data)
			if got != tt.want {
				t.Fatalf("historyTelemetryValueToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildHistoryTelemetryListReturnsEmptyArrayForNoRows(t *testing.T) {
	list := buildHistoryTelemetryList(nil)
	if list == nil {
		t.Fatal("buildHistoryTelemetryList(nil) returned nil; paged API list must be an empty array")
	}
	if len(list) != 0 {
		t.Fatalf("buildHistoryTelemetryList(nil) length = %d, want 0", len(list))
	}
}

// Helper functions for test convenience
func Float64Ptr(v float64) *float64 {
	return &v
}

func BoolPtr(v bool) *bool {
	return &v
}

func TestAggregateRuleStructure(t *testing.T) {
	rule := AggregateRule{
		Days:         30,
		MinInterval:  "1h",
		FriendlyDesc: "30天",
	}
	if rule.Days != 30 {
		t.Fatalf("Days = %d, want 30", rule.Days)
	}
	if rule.MinInterval != "1h" {
		t.Fatalf("MinInterval = %q, want %q", rule.MinInterval, "1h")
	}
}

func TestProcessTimeRangeNoAggregateJustOverOneDay(t *testing.T) {
	now := time.Now()
	// Just over 24 hours should be rejected
	req := &model.GetTelemetryStatisticReq{
		DeviceId:        "device-1",
		Key:             "temperature",
		AggregateWindow: "no_aggregate",
		TimeRange:       "custom",
		StartTime:       now.Add(-24*time.Hour-time.Millisecond).UnixNano() / 1e6,
		EndTime:         now.UnixNano() / 1e6,
	}

	err := processTimeRange(req)
	if err == nil {
		t.Fatal("processTimeRange should reject no_aggregate with >24h range")
	}
}
