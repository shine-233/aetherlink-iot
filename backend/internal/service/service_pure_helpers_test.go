// 文件用途：验证 service 包内无外部依赖的纯 helper 行为。
// 核心逻辑：用表驱动用例覆盖字符串、JSON、结构体映射和边界输入转换。
// 关键注意事项：纯 helper 被多个服务复用，测试需保持输入输出契约稳定并避免引入数据库依赖。
// 重构建议：按 helper 所属领域拆分测试文件，补齐 nil、空值、坏 JSON 和字段标签边界。
// service_pure_helpers_test.go covers service guardrails that can run offline.
//
// Purpose: keep pure validation, authorization, normalization, and helper routines covered without requiring database, Redis, MQTT, or HTTP services.
// Core logic: calls service-layer helpers directly and asserts early rejection, tenant scoping, payload sanitization, and defaulting rules.
// Important notes: green results here prove helper behavior only, not DAL persistence or API integration.
// Refactor suggestion: move domain clusters into smaller test files once a helper family gains enough cases to stand alone.
package service

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/metrics"
	"aetherlink-iot/backend/pkg/utils"
)

func pureHelperStringPtr(value string) *string {
	return &value
}

func pureHelperIntPtr(value int) *int {
	return &value
}

func pureHelperInt64Ptr(value int64) *int64 {
	return &value
}

func assertErrcodeError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return an errcode error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	if wantMessage != "" && (!appErr.UseCustomMsg || appErr.CustomMsg != wantMessage) {
		t.Fatalf("%s error message = %q, want %q", context, appErr.CustomMsg, wantMessage)
	}
}

func assertErrcodeDataError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return an errcode error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	data, ok := appErr.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("%s error data = %#v, want map with error", context, appErr.Data)
	}
	got, ok := data["error"].(string)
	if !ok {
		t.Fatalf("%s error data.error = %#v, want string", context, data["error"])
	}
	if got != wantMessage {
		t.Fatalf("%s error data.error = %q, want %q", context, got, wantMessage)
	}
}

func TestBuildOTAFilteredDeviceListReqUsesPackageDeviceConfig(t *testing.T) {
	search := "pump"
	groupID := "group-1"
	currentVersion := "1.0.0"
	serviceAccessID := "svc-1"
	filterConfig := "cfg-1"
	lastReportedAfter := int64(1_752_883_200_000)
	lastReportedBefore := int64(1_752_969_600_000)
	neverReported := false
	lifecycleStatus := "transmitted"
	req, err := buildOTAFilteredDeviceListReq(&model.OTAUpgradeTaskDeviceFilter{
		Search:             &search,
		GroupId:            &groupID,
		CurrentVersion:     &currentVersion,
		ServiceAccessID:    &serviceAccessID,
		DeviceConfigId:     &filterConfig,
		IsOnline:           pureHelperIntPtr(1),
		LastReportedAfter:  &lastReportedAfter,
		LastReportedBefore: &lastReportedBefore,
		NeverReported:      &neverReported,
		LifecycleStatus:    &lifecycleStatus,
	}, &model.OtaUpgradePackage{DeviceConfigID: "cfg-1"})
	if err != nil {
		t.Fatalf("buildOTAFilteredDeviceListReq returned error: %v", err)
	}

	if req.DeviceConfigId == nil || *req.DeviceConfigId != "cfg-1" {
		t.Fatalf("DeviceConfigId = %#v, want package cfg-1", req.DeviceConfigId)
	}
	if req.Page != 0 || req.PageSize != 0 {
		t.Fatalf("PageReq = page %d size %d, want unpaged full-filter scan", req.Page, req.PageSize)
	}
	if req.Search == nil || *req.Search != "pump" {
		t.Fatalf("Search = %#v, want pump", req.Search)
	}
	if req.GroupId == nil || *req.GroupId != "group-1" {
		t.Fatalf("GroupId = %#v, want group-1", req.GroupId)
	}
	if req.CurrentVersion == nil || *req.CurrentVersion != "1.0.0" {
		t.Fatalf("CurrentVersion = %#v, want 1.0.0", req.CurrentVersion)
	}
	if req.ServiceAccessID == nil || *req.ServiceAccessID != "svc-1" {
		t.Fatalf("ServiceAccessID = %#v, want svc-1", req.ServiceAccessID)
	}
	if req.IsOnline == nil || *req.IsOnline != 1 {
		t.Fatalf("IsOnline = %#v, want 1", req.IsOnline)
	}
	if req.LastReportedAfter == nil || *req.LastReportedAfter != lastReportedAfter {
		t.Fatalf("LastReportedAfter = %#v, want %d", req.LastReportedAfter, lastReportedAfter)
	}
	if req.LastReportedBefore == nil || *req.LastReportedBefore != lastReportedBefore {
		t.Fatalf("LastReportedBefore = %#v, want %d", req.LastReportedBefore, lastReportedBefore)
	}
	if req.NeverReported == nil || *req.NeverReported {
		t.Fatalf("NeverReported = %#v, want false", req.NeverReported)
	}
	if req.LifecycleStatus == nil || *req.LifecycleStatus != lifecycleStatus {
		t.Fatalf("LifecycleStatus = %#v, want %q", req.LifecycleStatus, lifecycleStatus)
	}
}

func TestNormalizeFleetSavedFilterParamsKeepsLastReportContract(t *testing.T) {
	result := normalizeFleetSavedFilterParams(map[string]interface{}{
		"last_reported_after":  float64(1_752_883_200_000),
		"last_reported_before": float64(1_752_969_600_000),
		"never_reported":       false,
		"lifecycle_status":     "transmitted",
		"unsafe":               "ignored",
	})
	if result["last_reported_after"] != float64(1_752_883_200_000) ||
		result["last_reported_before"] != float64(1_752_969_600_000) ||
		result["never_reported"] != false ||
		result["lifecycle_status"] != "transmitted" {
		t.Fatalf("normalized last report filter = %#v", result)
	}
	if _, ok := result["unsafe"]; ok {
		t.Fatalf("unsupported saved filter key was retained: %#v", result)
	}
}

func TestNormalizeFleetSavedFilterReqRejectsContradictoryLastReportFilter(t *testing.T) {
	_, err := normalizeFleetSavedFilterReq(&model.FleetSavedFilterReq{
		Name: "invalid report filter",
		DeviceFilter: map[string]interface{}{
			"never_reported":      true,
			"last_reported_after": float64(1_752_883_200_000),
		},
	})
	if err == nil {
		t.Fatal("expected contradictory last report filter to fail")
	}
}

func TestNormalizeFleetSavedFilterReqRejectsUnknownLifecycleStatus(t *testing.T) {
	_, err := normalizeFleetSavedFilterReq(&model.FleetSavedFilterReq{
		Name: "invalid lifecycle filter",
		DeviceFilter: map[string]interface{}{
			"lifecycle_status": "transfer_complete",
		},
	})
	if err == nil {
		t.Fatal("expected unknown lifecycle_status to fail")
	}
}

func TestBuildFleetCommandDeviceFilterListReqKeepsLastReportAndLifecycleContract(t *testing.T) {
	after := int64(1_752_883_200_000)
	before := int64(1_752_969_600_000)
	neverReported := false
	lifecycleStatus := "transmitted"
	req := buildFleetCommandDeviceFilterListReq(&model.FleetCommandJobDeviceFilter{
		LastReportedAfter:  &after,
		LastReportedBefore: &before,
		NeverReported:      &neverReported,
		LifecycleStatus:    &lifecycleStatus,
	}, 2, 25)
	if req.Page != 2 || req.PageSize != 25 ||
		req.LastReportedAfter == nil || *req.LastReportedAfter != after ||
		req.LastReportedBefore == nil || *req.LastReportedBefore != before ||
		req.NeverReported == nil || *req.NeverReported ||
		req.LifecycleStatus == nil || *req.LifecycleStatus != lifecycleStatus {
		t.Fatalf("fleet command last report request = %#v", req)
	}
}

func TestFleetCommandDeviceFilterIsNotEmptyForReportOrLifecycleFilters(t *testing.T) {
	reported := false
	transmitted := "transmitted"
	if fleetCommandDeviceFilterIsEmpty(&model.FleetCommandJobDeviceFilter{NeverReported: &reported}) {
		t.Fatal("explicit reported-history filter must not be treated as empty")
	}
	if fleetCommandDeviceFilterIsEmpty(&model.FleetCommandJobDeviceFilter{LifecycleStatus: &transmitted}) {
		t.Fatal("lifecycle filter must not be treated as empty")
	}
}

func TestBuildOTAFilteredDeviceListReqRejectsMismatchedDeviceConfig(t *testing.T) {
	filterConfig := "cfg-2"
	errCtx := "mismatched ota device filter config"
	_, err := buildOTAFilteredDeviceListReq(&model.OTAUpgradeTaskDeviceFilter{
		DeviceConfigId: &filterConfig,
	}, &model.OtaUpgradePackage{DeviceConfigID: "cfg-1"})

	assertErrcodeError(t, err, errCtx, errcode.CodeParamError, "device_filter.device_config_id must match ota package device_config_id")
}

func TestFilteredOTADeviceIDsDeduplicatesAndExcludes(t *testing.T) {
	ids := filteredOTADeviceIDs([]model.GetDeviceListByPageRsp{
		{ID: "dev-1"},
		{ID: "dev-2"},
		{ID: "dev-1"},
		{ID: " "},
		{ID: "dev-3"},
	}, []string{"dev-2", "dev-missing"})

	if !reflect.DeepEqual(ids, []string{"dev-1", "dev-3"}) {
		t.Fatalf("filteredOTADeviceIDs = %#v, want dev-1/dev-3", ids)
	}
}

func TestFilterOTADeviceIDsDeduplicatesAndExcludesRawIDs(t *testing.T) {
	ids := filterOTADeviceIDs([]string{"dev-1", "dev-2", "dev-1", " ", "dev-3"}, []string{"dev-2"})

	if !reflect.DeepEqual(ids, []string{"dev-1", "dev-3"}) {
		t.Fatalf("filterOTADeviceIDs = %#v, want dev-1/dev-3", ids)
	}
}

func TestOTAFilteredIDScanLimitOnlyScansEnoughForPreviewWhenOverLimit(t *testing.T) {
	limit := otaFilteredIDScanLimit(500, 7, 20, false)

	if limit != 27 {
		t.Fatalf("otaFilteredIDScanLimit preview = %d, want preview plus excluded ids", limit)
	}
}

func TestOTAFilteredIDScanLimitScansAllIDsWhenNeededForSubmit(t *testing.T) {
	limit := otaFilteredIDScanLimit(25, 3, 20, true)

	if limit != 28 {
		t.Fatalf("otaFilteredIDScanLimit submit = %d, want selected plus excluded ids", limit)
	}
}

func TestResolveOTAUpgradeTaskMaxDevices(t *testing.T) {
	if got := resolveOTAUpgradeTaskMaxDevices(nil); got != 5000 {
		t.Fatalf("nil max = %d, want 5000", got)
	}
	if got := resolveOTAUpgradeTaskMaxDevices(pureHelperIntPtr(250)); got != 250 {
		t.Fatalf("custom max = %d, want 250", got)
	}
	if got := resolveOTAUpgradeTaskMaxDevices(pureHelperIntPtr(8000)); got != 5000 {
		t.Fatalf("oversized max = %d, want 5000", got)
	}
}

func TestPreviewOTADevicesUsesSelectedOrderAndLimit(t *testing.T) {
	preview := previewOTADevices([]model.GetDeviceListByPageRsp{
		{ID: "dev-1", Name: "Device 1"},
		{ID: "dev-2", Name: "Device 2"},
		{ID: "dev-3", Name: "Device 3"},
	}, []string{"dev-1", "dev-3"}, 1)

	if !reflect.DeepEqual(preview, []model.GetDeviceListByPageRsp{{ID: "dev-1", Name: "Device 1"}}) {
		t.Fatalf("previewOTADevices = %#v, want first selected device only", preview)
	}
}

func TestApplyOTAUpgradeTaskAuditRecordsFilterSnapshotAndCreator(t *testing.T) {
	req := &model.CreateOTAUpgradeTaskReq{
		DeviceFilter: &model.OTAUpgradeTaskDeviceFilter{
			GroupId:  pureHelperStringPtr("group-1"),
			IsOnline: pureHelperIntPtr(1),
		},
		ExpectedTotal: pureHelperInt64Ptr(42),
	}

	err := applyOTAUpgradeTaskAudit(req, &utils.UserClaims{
		ID:        "user-1",
		Authority: "TENANT_ADMIN",
	}, []string{"dev-1", "dev-2"})
	if err != nil {
		t.Fatalf("applyOTAUpgradeTaskAudit returned error: %v", err)
	}

	if req.TargetMode != "filter" {
		t.Fatalf("TargetMode = %q, want filter", req.TargetMode)
	}
	if req.TargetFilter == nil || !strings.Contains(*req.TargetFilter, "group-1") {
		t.Fatalf("TargetFilter = %#v, want serialized filter snapshot", req.TargetFilter)
	}
	if req.PreviewTotal == nil || *req.PreviewTotal != 42 {
		t.Fatalf("PreviewTotal = %#v, want 42", req.PreviewTotal)
	}
	if req.SelectedCount == nil || *req.SelectedCount != 2 {
		t.Fatalf("SelectedCount = %#v, want 2", req.SelectedCount)
	}
	if req.CreatedBy == nil || *req.CreatedBy != "user-1" {
		t.Fatalf("CreatedBy = %#v, want user-1", req.CreatedBy)
	}
	if req.CreatedByAuthority == nil || *req.CreatedByAuthority != "TENANT_ADMIN" {
		t.Fatalf("CreatedByAuthority = %#v, want TENANT_ADMIN", req.CreatedByAuthority)
	}
}

func TestApplyOTAUpgradeTaskAuditRecordsExplicitSelectionCount(t *testing.T) {
	req := &model.CreateOTAUpgradeTaskReq{}

	if err := applyOTAUpgradeTaskAudit(req, nil, []string{"dev-1", "dev-2", "dev-3"}); err != nil {
		t.Fatalf("applyOTAUpgradeTaskAudit returned error: %v", err)
	}

	if req.TargetMode != "explicit" {
		t.Fatalf("TargetMode = %q, want explicit", req.TargetMode)
	}
	if req.TargetFilter != nil {
		t.Fatalf("TargetFilter = %#v, want nil for explicit tasks", req.TargetFilter)
	}
	if req.PreviewTotal == nil || *req.PreviewTotal != 3 {
		t.Fatalf("PreviewTotal = %#v, want 3", req.PreviewTotal)
	}
	if req.SelectedCount == nil || *req.SelectedCount != 3 {
		t.Fatalf("SelectedCount = %#v, want 3", req.SelectedCount)
	}
}

func TestDeviceOwnerUserIDFilterForClaims(t *testing.T) {
	t.Run("tenant user uses trimmed owner id filter", func(t *testing.T) {
		claims := &utils.UserClaims{
			ID:        "  owner-1  ",
			Authority: constant.TENANT_USER,
		}

		got := deviceOwnerUserIDFilterForClaims(claims)
		if got == nil || *got != "owner-1" {
			t.Fatalf("deviceOwnerUserIDFilterForClaims = %#v, want owner-1", got)
		}
	})

	t.Run("tenant user with blank id gets invisible sentinel", func(t *testing.T) {
		claims := &utils.UserClaims{
			ID:        "   ",
			Authority: constant.TENANT_USER,
		}

		got := deviceOwnerUserIDFilterForClaims(claims)
		if got == nil || *got != noVisibleDeviceOwnerUserID {
			t.Fatalf("deviceOwnerUserIDFilterForClaims = %#v, want sentinel %q", got, noVisibleDeviceOwnerUserID)
		}
	})

	t.Run("non tenant user has no owner filter", func(t *testing.T) {
		claims := &utils.UserClaims{
			ID:        "admin-1",
			Authority: "TENANT_ADMIN",
		}

		if got := deviceOwnerUserIDFilterForClaims(claims); got != nil {
			t.Fatalf("deviceOwnerUserIDFilterForClaims = %#v, want nil", got)
		}
	})
}

func TestRequireDeviceTenantClaims(t *testing.T) {
	t.Run("returns tenant id when claims are valid", func(t *testing.T) {
		tenantID, err := requireDeviceTenantClaims(&utils.UserClaims{TenantID: "tenant-rdi-1", Authority: constant.TENANT_ADMIN}, "no permission")
		if err != nil {
			t.Fatalf("requireDeviceTenantClaims returned error: %v", err)
		}
		if tenantID != "tenant-rdi-1" {
			t.Fatalf("tenantID = %q, want tenant-rdi-1", tenantID)
		}
	})

	t.Run("rejects missing tenant claims", func(t *testing.T) {
		errCtx := "missing tenant claims"
		_, err := requireDeviceTenantClaims(&utils.UserClaims{TenantID: ""}, "no permission to query tenant device overview")
		assertErrcodeError(t, err, errCtx, errcode.CodeNoPermission, "no permission to query tenant device overview")
	})
}

func assertNoPermissionToQuerySystemMetrics(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject non-system-admin system metrics access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to query system metrics" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToManageRoles(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject role management access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage roles" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToManageDataPolicy(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject data policy management access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage data policy" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToCreateScene(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject scene creation access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to create scene" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToCreateSceneAutomation(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject scene automation creation access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to create scene automation" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertBoardServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string, wantVars map[string]interface{}) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return a board service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	if wantMessage != "" && (!appErr.UseCustomMsg || appErr.CustomMsg != wantMessage) {
		t.Fatalf("%s error message = %q, want %q", context, appErr.CustomMsg, wantMessage)
	}
	if wantVars != nil && !reflect.DeepEqual(appErr.Variables, wantVars) {
		t.Fatalf("%s error vars = %#v, want %#v", context, appErr.Variables, wantVars)
	}
}

func assertDeviceConfigServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return a device config service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	if wantMessage != "" && (!appErr.UseCustomMsg || appErr.CustomMsg != wantMessage) {
		t.Fatalf("%s error message = %q, want %q", context, appErr.CustomMsg, wantMessage)
	}
}

func assertNoPermissionToModifyDeviceModel(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject device model modification", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to modify device model" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToManageServicePlugins(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject service plugin management access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage service plugins" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToQueryServicePlugins(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject service plugin query access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to query service plugins" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertServicePluginError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return a service plugin error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, wantCode, wantMessage)
	}
}

func assertNoPermissionToManageUIElements(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject ui elements management access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage ui elements" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToQueryUIElements(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject ui elements query access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to query ui elements" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToManageMessagePushConfig(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject message push config management access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage message push config" {
		t.Fatalf("%s error = code %d message %q, want code %d no-permission message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertNoPermissionToQueryPluginServiceAccess(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject plugin service access query", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "plugin service access list requires api key" {
		t.Fatalf("%s error = code %d message %q, want code %d api-key-required message", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
}

func assertPluginDeviceConfigAccessError(t *testing.T, err error, context string, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should reject plugin device config access", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission, wantMessage)
	}
}

func assertProtocolPluginServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string, wantData interface{}, wantVars map[string]interface{}) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return a protocol plugin service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s error code = %d, want %d", context, appErr.Code, wantCode)
	}
	if wantMessage != "" && (!appErr.UseCustomMsg || appErr.CustomMsg != wantMessage) {
		t.Fatalf("%s error message = %q, want %q", context, appErr.CustomMsg, wantMessage)
	}
	if wantData != nil && !reflect.DeepEqual(appErr.Data, wantData) {
		t.Fatalf("%s error data = %#v, want %#v", context, appErr.Data, wantData)
	}
	if wantVars != nil && !reflect.DeepEqual(appErr.Variables, wantVars) {
		t.Fatalf("%s error vars = %#v, want %#v", context, appErr.Variables, wantVars)
	}
}

func callMessagePushConfigAndRecoverPanic(service *MessagePush, req *model.MessagePushConfigReq, claims *utils.UserClaims) (err error, panicked interface{}) {
	defer func() {
		panicked = recover()
	}()
	err = service.SetMessagePushConfig(req, claims)
	return err, nil
}

func assertOTAServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return an OTA service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, wantCode, wantMessage)
	}
}

func assertExpectedDataServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return an expected data service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, wantCode, wantMessage)
	}
}

func assertDataScriptServiceError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should return a data script service error", context)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error", context, err)
	}
	if appErr.Code != wantCode || appErr.CustomMsg != wantMessage {
		t.Fatalf("%s error = code %d message %q, want code %d message %q", context, appErr.Code, appErr.CustomMsg, wantCode, wantMessage)
	}
}

type pureHelperMetricsStorage struct {
	current  *metrics.SystemMetrics
	history  []metrics.MetricDataPoint
	combined []metrics.MetricsTimePoint
}

func (s *pureHelperMetricsStorage) SaveMetrics(time.Time, float64, float64, float64) error {
	return nil
}

func (s *pureHelperMetricsStorage) GetHistoryData(string, time.Duration) ([]metrics.MetricDataPoint, error) {
	return s.history, nil
}

func (s *pureHelperMetricsStorage) GetCurrentData() (*metrics.SystemMetrics, error) {
	return s.current, nil
}

func TestMergeIdentifyAndPayloadBuildsCommandWithoutParams(t *testing.T) {
	got, err := mergeIdentifyAndPayload("restart", nil)
	if err != nil {
		t.Fatalf("merge payload: %v", err)
	}

	want := map[string]any{"method": "restart"}
	if gotMap := gatewayTestDecodeJSONMap(t, got); !reflect.DeepEqual(gotMap, want) {
		t.Fatalf("unexpected merged payload: got %#v want %#v", gotMap, want)
	}
}

func TestMergeIdentifyAndPayloadAddsObjectParams(t *testing.T) {
	params := `{"speed":42,"mode":"auto"}`

	got, err := mergeIdentifyAndPayload("set_motor", &params)
	if err != nil {
		t.Fatalf("merge payload: %v", err)
	}

	gotMap := gatewayTestDecodeJSONMap(t, got)
	want := gatewayTestDecodeJSONMap(t, `{"method":"set_motor","params":{"speed":42,"mode":"auto"}}`)
	if !reflect.DeepEqual(gotMap, want) {
		t.Fatalf("unexpected merged payload: got %#v want %#v", gotMap, want)
	}
}

func TestMergeIdentifyAndPayloadAddsArrayParams(t *testing.T) {
	params := `[1,2,3]`

	got, err := mergeIdentifyAndPayload("batch_set", &params)
	if err != nil {
		t.Fatalf("merge payload: %v", err)
	}

	gotMap := gatewayTestDecodeJSONMap(t, got)
	paramsValue, ok := gotMap["params"].([]any)
	if !ok {
		t.Fatalf("params should decode as an array, got %#v", gotMap["params"])
	}
	if gotMap["method"] != "batch_set" || len(paramsValue) != 3 {
		t.Fatalf("unexpected merged payload: %#v", gotMap)
	}
}

func TestMergeIdentifyAndPayloadRejectsInvalidJSONParams(t *testing.T) {
	params := `{"speed":`

	_, err := mergeIdentifyAndPayload("set_motor", &params)
	if err == nil {
		t.Fatal("expected invalid params JSON to fail")
	}
	want := "error parsing payload JSON: unexpected end of JSON input"
	if err.Error() != want {
		t.Fatalf("merge payload error = %q, want %q", err.Error(), want)
	}
}

func TestParseExistsFromBodyAcceptsTopLevelExists(t *testing.T) {
	got, err := parseExistsFromBody([]byte(`{"exists":true}`))
	if err != nil {
		t.Fatalf("parse exists: %v", err)
	}
	if !got {
		t.Fatal("expected top-level exists=true")
	}
}

func TestParseExistsFromBodyAcceptsBooleanData(t *testing.T) {
	got, err := parseExistsFromBody([]byte(`{"data":false}`))
	if err != nil {
		t.Fatalf("parse exists: %v", err)
	}
	if got {
		t.Fatal("expected nested data boolean false")
	}
}

func TestParseExistsFromBodyAcceptsNestedDataExists(t *testing.T) {
	got, err := parseExistsFromBody([]byte(`{"data":{"exists":true}}`))
	if err != nil {
		t.Fatalf("parse exists: %v", err)
	}
	if !got {
		t.Fatal("expected nested exists=true")
	}
}

func TestParseExistsFromBodyRejectsEmptyMalformedAndMissingExists(t *testing.T) {
	cases := [][]byte{
		[]byte(``),
		[]byte(`not-json`),
		[]byte(`{"data":{"total":0}}`),
	}

	for _, body := range cases {
		_, err := parseExistsFromBody(body)
		if err == nil {
			t.Fatalf("expected parse error for %q", string(body))
		}
		if !errors.Is(err, ErrMarketInvalidResponse) {
			t.Fatalf("expected ErrMarketInvalidResponse for %q, got %v", string(body), err)
		}
	}
}

func TestCompactMarketBodyTrimsEmptyAndLongBodies(t *testing.T) {
	if got := compactMarketBody([]byte("  \n ")); got != "<empty>" {
		t.Fatalf("empty body should be marked, got %q", got)
	}
	if got := compactMarketBody([]byte("  ok  ")); got != "<redacted>" {
		t.Fatalf("non-empty body should be redacted, got %q", got)
	}

	longBody := strings.Repeat("x", 300)
	got := compactMarketBody([]byte(longBody))
	if got != "<redacted>" {
		t.Fatalf("long body should be redacted, got %q", got)
	}
}

func TestIsUserNotFoundResponseRecognizesStatusBodyAndCode(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "status 404", status: http.StatusNotFound},
		{name: "plain text", status: http.StatusBadRequest, body: "email not found"},
		{name: "localized text", status: http.StatusUnprocessableEntity, body: "user not exists"},
		{name: "business code", status: http.StatusBadRequest, body: `{"code":200015}`},
		{name: "json message", status: http.StatusUnprocessableEntity, body: `{"message":"email not found"}`},
	}

	for _, tt := range cases {
		if !isUserNotFoundResponse(tt.status, []byte(tt.body)) {
			t.Fatalf("%s should be treated as user-not-found", tt.name)
		}
	}
}

func TestIsUserNotFoundResponseRejectsServerErrorsAndUnrelatedClientErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "server error", status: http.StatusInternalServerError, body: "email not found"},
		{name: "unrelated bad request", status: http.StatusBadRequest, body: `{"message":"invalid domain"}`},
		{name: "malformed client body", status: http.StatusUnprocessableEntity, body: `{`},
	}

	for _, tt := range cases {
		if isUserNotFoundResponse(tt.status, []byte(tt.body)) {
			t.Fatalf("%s should not be treated as user-not-found", tt.name)
		}
	}
}

func TestPtrStrPReturnsNilForEmptyAndPointerForValue(t *testing.T) {
	if got := ptrStrP(""); got != nil {
		t.Fatalf("empty string should return nil, got %q", *got)
	}
	got := ptrStrP("value")
	if got == nil || *got != "value" {
		t.Fatalf("value should return pointer, got %#v", got)
	}
}

func TestGetStrAndGetStrPExtractOnlyNonEmptyStrings(t *testing.T) {
	data := map[string]any{
		"name":  "template",
		"empty": "",
		"count": 3,
	}

	if got := getStr(data, "name"); got != "template" {
		t.Fatalf("unexpected string value: %q", got)
	}
	if got := getStr(data, "count"); got != "" {
		t.Fatalf("non-string value should return empty string, got %q", got)
	}
	if got := getStrP(data, "empty"); got != nil {
		t.Fatalf("empty string pointer should be nil, got %#v", got)
	}
	got := getStrP(data, "name")
	if got == nil || *got != "template" {
		t.Fatalf("string pointer should contain template, got %#v", got)
	}
}

func TestDeviceTemplatePublishPointerHelpersReturnZeroValuesForNil(t *testing.T) {
	if got := ptrStr(nil); got != "" {
		t.Fatalf("nil string pointer should return empty string, got %q", got)
	}
	if got := ptrInt16(nil); got != 0 {
		t.Fatalf("nil int16 pointer should return zero, got %d", got)
	}
}

func TestDeviceTemplatePublishPointerHelpersDereferenceValues(t *testing.T) {
	text := "modbus"
	number := int16(7)

	if got := ptrStr(&text); got != "modbus" {
		t.Fatalf("string pointer should dereference, got %q", got)
	}
	if got := ptrInt16(&number); got != 7 {
		t.Fatalf("int16 pointer should dereference, got %d", got)
	}
}

func TestParseJSONReturnsNilForBlankAndInvalidPayload(t *testing.T) {
	if got := parseJSON(""); got != nil {
		t.Fatalf("blank JSON should return nil, got %#v", got)
	}
	if got := parseJSON("{"); got != nil {
		t.Fatalf("invalid JSON should return nil map, got %#v", got)
	}
}

func TestParseJSONDecodesObjectPayload(t *testing.T) {
	got := parseJSON(`{"baud":9600,"parity":"none"}`)
	if got["baud"] != float64(9600) || got["parity"] != "none" {
		t.Fatalf("unexpected parsed JSON: %#v", got)
	}
}

func TestGetPluginDependenciesFromProtocolReturnsEmptyWhenProtocolMissing(t *testing.T) {
	cases := []*model.DeviceConfig{
		{},
		{ProtocolType: pureHelperStringPtr("")},
	}

	for _, dc := range cases {
		if got := getPluginDependenciesFromProtocol(dc); len(got) != 0 {
			t.Fatalf("missing protocol should not declare plugin dependencies, got %#v", got)
		}
	}
}

func TestCheckMissingPluginsReturnsNilWhenMarketTemplateHasNoDependencies(t *testing.T) {
	if got := checkMissingPlugins(nil); got != nil {
		t.Fatalf("nil dependency list should not report missing plugins, got %#v", got)
	}
	if got := checkMissingPlugins([]model.PluginDependency{}); got != nil {
		t.Fatalf("empty dependency list should not report missing plugins, got %#v", got)
	}
}

func TestDecodeProtocolPluginConfigReturnsNilForMissingOrInvalidJSON(t *testing.T) {
	cases := []*string{
		nil,
		pureHelperStringPtr(""),
		pureHelperStringPtr("not-json"),
	}

	for _, raw := range cases {
		got, err := decodeProtocolPluginConfig(raw)
		if err != nil {
			t.Fatalf("decodeProtocolPluginConfig(%#v) error = %v", raw, err)
		}
		if got != nil {
			t.Fatalf("decodeProtocolPluginConfig(%#v) = %#v, want nil", raw, got)
		}
	}
}

func TestDecodeProtocolPluginConfigDecodesObject(t *testing.T) {
	got, err := decodeProtocolPluginConfig(pureHelperStringPtr(`{"host":"127.0.0.1","port":1883}`))
	if err != nil {
		t.Fatalf("decodeProtocolPluginConfig() error = %v", err)
	}
	if got["host"] != "127.0.0.1" || got["port"] != float64(1883) {
		t.Fatalf("unexpected protocol config: %#v", got)
	}
}

func TestEnsurePluginDeviceTenantAccessRequiresClaimsAndDevice(t *testing.T) {
	err := ensurePluginDeviceTenantAccess(&model.Device{TenantID: "tenant-1"}, nil)
	assertPluginDeviceConfigAccessError(t, err, "nil claims plugin device config", "plugin device config requires api key")

	err = ensurePluginDeviceTenantAccess(nil, &utils.UserClaims{TenantID: "tenant-1"})
	assertPluginDeviceConfigAccessError(t, err, "nil device plugin device config", "device not found or no permission")
}

func TestEnsurePluginDeviceTenantAccessAllowsSameTenantAndSysAdmin(t *testing.T) {
	device := &model.Device{TenantID: "tenant-1"}

	if err := ensurePluginDeviceTenantAccess(device, &utils.UserClaims{TenantID: "tenant-1", Authority: "TENANT_ADMIN"}); err != nil {
		t.Fatalf("same tenant should be allowed: %v", err)
	}
	if err := ensurePluginDeviceTenantAccess(device, &utils.UserClaims{TenantID: "other", Authority: constant.SYS_ADMIN}); err != nil {
		t.Fatalf("system admin should be allowed across tenants: %v", err)
	}
}

func TestEnsurePluginDeviceTenantAccessRejectsDifferentTenant(t *testing.T) {
	device := &model.Device{TenantID: "tenant-1"}
	claims := &utils.UserClaims{TenantID: "tenant-2", Authority: "TENANT_ADMIN"}

	err := ensurePluginDeviceTenantAccess(device, claims)
	assertPluginDeviceConfigAccessError(t, err, "different tenant plugin device config", "no permission to query plugin device config")
}

func TestRequireRoleManagerAllowsSystemAndTenantAdmins(t *testing.T) {
	cases := []*utils.UserClaims{
		{Authority: constant.SYS_ADMIN},
		{Authority: constant.TENANT_ADMIN},
	}

	for _, claims := range cases {
		if err := requireRoleManager(claims); err != nil {
			t.Fatalf("authority %q should manage roles: %v", claims.Authority, err)
		}
	}
}

func TestRequireRoleManagerRejectsNilAndTenantUser(t *testing.T) {
	cases := []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_USER},
		{Authority: ""},
	}

	for _, claims := range cases {
		assertNoPermissionToManageRoles(t, requireRoleManager(claims), "requireRoleManager")
	}
}

func TestRequireDataPolicyAdminAllowsOnlySystemAdmin(t *testing.T) {
	if err := requireDataPolicyAdmin(&utils.UserClaims{Authority: constant.SYS_ADMIN}); err != nil {
		t.Fatalf("system admin should manage data policy: %v", err)
	}

	cases := []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN},
		{Authority: constant.TENANT_USER},
		{Authority: ""},
	}
	for _, claims := range cases {
		assertNoPermissionToManageDataPolicy(t, requireDataPolicyAdmin(claims), "requireDataPolicyAdmin")
	}
}

func TestAdminOnlyServiceGuardsAllowOnlySystemAdmin(t *testing.T) {
	systemAdmin := &utils.UserClaims{Authority: constant.SYS_ADMIN}
	guards := []struct {
		name        string
		fn          func(*utils.UserClaims) error
		wantMessage string
	}{
		{name: "notification services", fn: requireNotificationServicesAdmin, wantMessage: "no permission to manage notification service config"},
		{name: "message push config", fn: requireMessagePushConfigAdmin, wantMessage: "no permission to manage message push config"},
		{name: "ui elements admin", fn: requireSysUIElementsAdmin, wantMessage: "no permission to manage ui elements"},
	}

	for _, guard := range guards {
		if err := guard.fn(systemAdmin); err != nil {
			t.Fatalf("%s should allow system admin: %v", guard.name, err)
		}
		for _, claims := range []*utils.UserClaims{
			nil,
			&utils.UserClaims{Authority: constant.TENANT_ADMIN},
			&utils.UserClaims{Authority: constant.TENANT_USER},
		} {
			err := guard.fn(claims)
			assertErrcodeError(t, err, guard.name+" reject unsupported claims", errcode.CodeNoPermission, guard.wantMessage)
		}
	}
}

func TestAdminOnlyServiceMethodsRejectBeforeExternalWork(t *testing.T) {
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}
	assertNotificationConfigPermission := func(t *testing.T, err error, action string) {
		t.Helper()
		if err == nil {
			t.Fatalf("tenant user should not %s notification service config", action)
		}
		appErr, ok := err.(*errcode.Error)
		if !ok {
			t.Fatalf("tenant user notification service config %s error type = %T, want *errcode.Error", action, err)
		}
		if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to manage notification service config" {
			t.Fatalf("tenant user notification service config %s error = code %d message %q, want code %d no-permission message", action, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
		}
	}

	err := (&Logo{}).UpdateLogo(&model.UpdateLogoReq{}, tenantUser)
	if err == nil {
		t.Fatal("tenant user should not update logo settings")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("tenant user logo update error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to update logo settings" {
		t.Fatalf("tenant user logo update error = code %d message %q, want code %d no-permission message", appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
	}
	_, err = (&NotificationServicesConfig{}).SaveNotificationServicesConfig(&model.SaveNotificationServicesConfigReq{}, tenantUser)
	assertNotificationConfigPermission(t, err, "save")
	_, err = (&NotificationServicesConfig{}).GetNotificationServicesConfig("EMAIL", tenantUser)
	assertNotificationConfigPermission(t, err, "read")
	err = (&NotificationServicesConfig{}).SendTestEmailByAdmin(&model.SendTestEmailReq{}, tenantUser)
	assertNotificationConfigPermission(t, err, "send test email through")
}

func TestCreateOpenAPIKeyRejectsUnsupportedRoleBeforeGeneratingKey(t *testing.T) {
	_, err := (&OpenAPIKey{}).CreateOpenAPIKey(nil, &utils.UserClaims{
		Authority: constant.TENANT_USER,
		TenantID:  "tenant-1",
	})
	if err == nil {
		t.Fatal("tenant user should not create open api keys")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("tenant user open api key error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("tenant user open api key error code = %d, want %d", appErr.Code, errcode.CodeNoPermission)
	}
	if appErr.Variables["required_role"] != "SYS_ADMIN or TENANT_ADMIN" || appErr.Variables["current_role"] != constant.TENANT_USER {
		t.Fatalf("tenant user open api key variables = %#v, want required/current role", appErr.Variables)
	}
}

func TestCreateOpenAPIKeyRejectsTenantMismatchBeforeGeneratingKey(t *testing.T) {
	_, err := (&OpenAPIKey{}).CreateOpenAPIKey(&model.CreateOpenAPIKeyReq{
		TenantID: "tenant-2",
	}, &utils.UserClaims{
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-1",
	})
	if err == nil {
		t.Fatal("tenant admin should not create open api keys for another tenant")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("tenant mismatch open api key error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("tenant mismatch open api key error code = %d, want %d", appErr.Code, errcode.CodeNoPermission)
	}
	if appErr.Variables["required_tenant"] != "tenant-2" || appErr.Variables["current_tenant"] != "tenant-1" {
		t.Fatalf("tenant mismatch open api key variables = %#v, want required tenant-2/current tenant-1", appErr.Variables)
	}
}

func TestGetPluginServiceAccessListRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := (&ServiceAccess{}).GetPluginServiceAccessList(&model.GetPluginServiceAccessListReq{
		ServiceIdentifier: "mqtt",
	}, nil)
	assertNoPermissionToQueryPluginServiceAccess(t, err, "nil api-key claims query plugin service access list")
}

func TestDeviceTopicMappingUpdateRejectsInvalidIDsBeforeDAL(t *testing.T) {
	service := &DeviceTopicMapping{}
	for _, id := range []string{"", "   ", "abc", "0", "-1"} {
		_, err := service.UpdateDeviceTopicMapping(id, &model.UpdateDeviceTopicMappingReq{}, &utils.UserClaims{})
		assertErrcodeError(t, err, "update device topic mapping invalid id "+id, errcode.CodeParamError, "invalid id")
	}
}

func TestDeviceTopicMappingDeleteRejectsInvalidIDsBeforeDAL(t *testing.T) {
	service := &DeviceTopicMapping{}
	for _, id := range []string{"", "   ", "abc", "0", "-1"} {
		err := service.DeleteDeviceTopicMapping(id, &utils.UserClaims{})
		assertErrcodeError(t, err, "delete device topic mapping invalid id "+id, errcode.CodeParamError, "invalid id")
	}
}

func TestInvalidateTopicMappingCacheRequiresRedisClient(t *testing.T) {
	oldRedis := global.REDIS
	global.REDIS = nil
	defer func() {
		global.REDIS = oldRedis
	}()

	err := invalidateTopicMappingCache(context.Background(), "device-config-1")
	if err == nil {
		t.Fatal("missing redis client should be reported")
	}
	if err.Error() != "redis client not initialized" {
		t.Fatalf("cache error = %q, want redis client not initialized", err.Error())
	}
}

func TestRequireTenantUIElementsViewerAllowsSystemAndTenantAdmins(t *testing.T) {
	cases := []*utils.UserClaims{
		{Authority: constant.SYS_ADMIN},
		{Authority: constant.TENANT_ADMIN},
	}

	for _, claims := range cases {
		if err := requireTenantUIElementsViewer(claims); err != nil {
			t.Fatalf("authority %q should view tenant ui elements: %v", claims.Authority, err)
		}
	}
}

func TestRequireTenantUIElementsViewerRejectsNilAndTenantUser(t *testing.T) {
	cases := []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_USER},
		{Authority: ""},
	}

	for _, claims := range cases {
		assertNoPermissionToQueryUIElements(t, requireTenantUIElementsViewer(claims), "requireTenantUIElementsViewer")
	}
}

func TestValidateMessagePushURLAcceptsHTTPAndHTTPSURLs(t *testing.T) {
	cases := []string{
		"https://example.com/hook",
		" http://example.com:8080/path?token=abc ",
		"HTTPS://example.com/upper-scheme",
	}

	for _, rawURL := range cases {
		if err := validateMessagePushURL(rawURL); err != nil {
			t.Fatalf("url %q should be valid: %v", rawURL, err)
		}
	}
}

func TestValidateMessagePushURLRejectsBlankInvalidAndUnsupportedSchemes(t *testing.T) {
	cases := []struct {
		rawURL      string
		wantMessage string
	}{
		{rawURL: "", wantMessage: "message push url is required"},
		{rawURL: "   ", wantMessage: "message push url is required"},
		{rawURL: "not a url", wantMessage: "message push url is invalid"},
		{rawURL: "ftp://example.com/hook", wantMessage: "message push url must use http or https"},
		{rawURL: "ws://example.com/socket", wantMessage: "message push url must use http or https"},
		{rawURL: "wss://example.com/socket", wantMessage: "message push url must use http or https"},
		{rawURL: "https:///missing-host", wantMessage: "message push url is invalid"},
	}

	for _, tc := range cases {
		err := validateMessagePushURL(tc.rawURL)
		assertErrcodeError(t, err, "message push url "+tc.rawURL, errcode.CodeParamError, tc.wantMessage)
	}
}

func TestMaskVerificationCodeMasksShortAndLongCodes(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"1":      "*",
		"12":     "**",
		"123":    "***",
		"1234":   "12*4",
		"123456": "12***6",
	}

	for input, want := range cases {
		if got := maskVerificationCode(input); got != want {
			t.Fatalf("maskVerificationCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePreferredLanguageAcceptsSupportedAliases(t *testing.T) {
	cases := map[string]string{
		"zh":       "zh-CN",
		"zh-CN":    "zh-CN",
		" zh_cn ":  "zh-CN",
		"EN":       "en-US",
		"en_us":    "en-US",
		"fr":       "fr-FR",
		"fr_fr":    "fr-FR",
		"es":       "es-ES",
		"es_es":    "es-ES",
		" ES-ES  ": "es-ES",
	}

	for input, want := range cases {
		got, err := normalizePreferredLanguage(input)
		if err != nil {
			t.Fatalf("normalizePreferredLanguage(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizePreferredLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePreferredLanguageRejectsBlankAndUnsupportedValues(t *testing.T) {
	cases := []struct {
		input       string
		wantMessage string
	}{
		{input: "", wantMessage: "prefer_lang is required"},
		{input: "   ", wantMessage: "prefer_lang is required"},
		{input: "de", wantMessage: "unsupported prefer_lang"},
		{input: "zh-TW", wantMessage: "unsupported prefer_lang"},
		{input: "english", wantMessage: "unsupported prefer_lang"},
	}

	for _, tc := range cases {
		_, err := normalizePreferredLanguage(tc.input)
		assertErrcodeError(t, err, "preferred language "+tc.input, errcode.CodeParamError, tc.wantMessage)
	}
}

func TestVerificationCodeEmailBodyUsesPreferredLanguage(t *testing.T) {
	cases := []struct {
		name     string
		language string
		want     string
	}{
		{name: "default english fallback", language: "", want: "Your verification code is 123456"},
		{name: "chinese", language: "zh-CN", want: "您的验证码是 123456"},
		{name: "french historical format", language: "fr_fr", want: "Votre code de verification est 123456"},
		{name: "spanish", language: "es-ES", want: "Su codigo de verificacion es 123456"},
		{name: "unsupported fallback", language: "de", want: "Your verification code is 123456"},
	}

	for _, tc := range cases {
		if got := verificationCodeEmailBody("123456", tc.language); got != tc.want {
			t.Fatalf("%s: verificationCodeEmailBody() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestShouldSkipMarketCheckRequiresRegisteredMatchingEmails(t *testing.T) {
	cases := []struct {
		name string
		req  *model.SuperAdminInitReq
		want bool
	}{
		{name: "nil request", req: nil, want: false},
		{name: "not market registered", req: &model.SuperAdminInitReq{Email: "admin@example.com", MarketEmail: "admin@example.com"}, want: false},
		{name: "missing request email", req: &model.SuperAdminInitReq{MarketRegistered: true, MarketEmail: "admin@example.com"}, want: false},
		{name: "missing market email", req: &model.SuperAdminInitReq{MarketRegistered: true, Email: "admin@example.com"}, want: false},
		{name: "different email", req: &model.SuperAdminInitReq{MarketRegistered: true, Email: "admin@example.com", MarketEmail: "other@example.com"}, want: false},
		{name: "same email ignores case and spaces", req: &model.SuperAdminInitReq{MarketRegistered: true, Email: " Admin@Example.com ", MarketEmail: "admin@example.com"}, want: true},
	}

	for _, tt := range cases {
		if got := shouldSkipMarketCheck(tt.req); got != tt.want {
			t.Fatalf("%s: shouldSkipMarketCheck() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUserAdditionalInfoMapHandlesNilBlankInvalidAndObject(t *testing.T) {
	if got := userAdditionalInfoMap(nil); len(got) != 0 {
		t.Fatalf("nil additional info should return empty map, got %#v", got)
	}
	if got := userAdditionalInfoMap(pureHelperStringPtr("  ")); len(got) != 0 {
		t.Fatalf("blank additional info should return empty map, got %#v", got)
	}
	if got := userAdditionalInfoMap(pureHelperStringPtr("{")); len(got) != 0 {
		t.Fatalf("invalid additional info should return empty map, got %#v", got)
	}

	got := userAdditionalInfoMap(pureHelperStringPtr(`{"theme":"dark"}`))
	if got["theme"] != "dark" {
		t.Fatalf("expected decoded additional info, got %#v", got)
	}
}

func TestWarningEmailsFromAdditionalInfoAcceptsArrayAndCommaString(t *testing.T) {
	arrayRaw := pureHelperStringPtr(`{"warning_emails":["Admin@Example.com","admin@example.com"," ops@example.com "]}`)
	if got := warningEmailsFromAdditionalInfo(arrayRaw); !reflect.DeepEqual(got, []string{"admin@example.com", "ops@example.com"}) {
		t.Fatalf("unexpected array emails: %#v", got)
	}

	stringRaw := pureHelperStringPtr(`{"warning_emails":"first@example.com, SECOND@example.com"}`)
	if got := warningEmailsFromAdditionalInfo(stringRaw); !reflect.DeepEqual(got, []string{"first@example.com", "second@example.com"}) {
		t.Fatalf("unexpected string emails: %#v", got)
	}
}

func TestWarningEmailsFromAdditionalInfoIgnoresMissingInvalidAndUnsupportedValues(t *testing.T) {
	cases := []*string{
		nil,
		pureHelperStringPtr(`{}`),
		pureHelperStringPtr(`{"warning_emails":123}`),
		pureHelperStringPtr(`{"warning_emails":["bad-address"]}`),
	}

	for _, raw := range cases {
		if got := warningEmailsFromAdditionalInfo(raw); got != nil {
			t.Fatalf("expected nil warning emails for %#v, got %#v", raw, got)
		}
	}
}

func TestNormalizeWarningEmailsTrimsLowercasesDeduplicatesAndSkipsBlank(t *testing.T) {
	got, err := normalizeWarningEmails([]string{
		" Admin@Example.com ",
		"",
		"admin@example.com",
		"Ops <ops@example.com>",
	})
	if err != nil {
		t.Fatalf("normalize warning emails: %v", err)
	}

	want := []string{"admin@example.com", "ops@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized emails: got %#v want %#v", got, want)
	}
}

func TestNormalizeWarningEmailsRejectsInvalidAddress(t *testing.T) {
	_, err := normalizeWarningEmails([]string{"not-an-email"})
	assertErrcodeError(t, err, "invalid warning email normalization", errcode.CodeParamError, "emails contains an invalid email address")
}

func TestUserUpdateRejectsWeakPasswordBeforeDAL(t *testing.T) {
	weakPassword := "short"
	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:       "user-1",
		Password: &weakPassword,
	}, &utils.UserClaims{ID: "admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	if err == nil {
		t.Fatal("weak password should be rejected before loading user")
	}
}

func TestUserEmailMethodsRejectMissingClaimsBeforeDAL(t *testing.T) {
	userService := &User{}
	_, err := userService.ChangeEmail(context.Background(), &model.ChangeEmailReq{
		NewEmail:   "new@example.com",
		VerifyCode: "123456",
	}, nil)
	assertErrcodeError(t, err, "nil claims change email", errcode.CodeNoPermission, "")

	_, err = userService.ChangeEmail(context.Background(), &model.ChangeEmailReq{
		NewEmail:   "new@example.com",
		VerifyCode: "123456",
	}, &utils.UserClaims{})
	assertErrcodeError(t, err, "blank claims user id change email", errcode.CodeNoPermission, "")

	_, err = userService.GetWarningEmails(nil)
	assertErrcodeError(t, err, "nil claims read warning emails", errcode.CodeNoPermission, "")

	_, err = userService.GetWarningEmails(&utils.UserClaims{})
	assertErrcodeError(t, err, "blank claims user id read warning emails", errcode.CodeNoPermission, "")

	_, err = userService.UpdateWarningEmails(context.Background(), &model.WarningEmailReq{}, nil)
	assertErrcodeError(t, err, "nil claims update warning emails", errcode.CodeNoPermission, "")

	_, err = userService.UpdateWarningEmails(context.Background(), &model.WarningEmailReq{}, &utils.UserClaims{})
	assertErrcodeError(t, err, "blank claims user id update warning emails", errcode.CodeNoPermission, "")
}

func TestUpdateWarningEmailsRejectsInvalidEmailBeforeDAL(t *testing.T) {
	_, err := (&User{}).UpdateWarningEmails(context.Background(), &model.WarningEmailReq{
		Emails: []string{"ops@example.com", "not-an-email"},
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	assertErrcodeError(t, err, "invalid warning email update", errcode.CodeParamError, "emails contains an invalid email address")
}

func TestUpdateWarningEmailsRejectsTenantUserBeforeDAL(t *testing.T) {
	_, err := (&User{}).UpdateWarningEmails(context.Background(), &model.WarningEmailReq{
		Emails: []string{"ops@example.com"},
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_USER})
	assertErrcodeError(t, err, "tenant user warning email update", errcode.CodeNoPermission, "no permission to update tenant warning emails")
}

func TestRequireSystemMonitorAdminAllowsOnlySystemAdmin(t *testing.T) {
	if err := requireSystemMonitorAdmin(&utils.UserClaims{Authority: constant.SYS_ADMIN}); err != nil {
		t.Fatalf("system admin should query system metrics: %v", err)
	}

	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN},
		{Authority: constant.TENANT_USER},
		{Authority: ""},
	} {
		assertNoPermissionToQuerySystemMetrics(t, requireSystemMonitorAdmin(claims), "requireSystemMonitorAdmin")
	}
}

func TestSystemMonitorMethodsRejectNonAdminBeforeMetricsManager(t *testing.T) {
	oldManager := metricsManager
	SetMetricsManager(nil)
	defer SetMetricsManager(oldManager)

	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	monitor := &SystemMonitor{}

	_, err := monitor.GetCurrentMetrics(tenantAdmin)
	assertNoPermissionToQuerySystemMetrics(t, err, "tenant admin current system metrics")
	_, err = monitor.GetHistoryData("cpu", time.Hour, tenantAdmin)
	assertNoPermissionToQuerySystemMetrics(t, err, "tenant admin system metric history")
	_, err = monitor.GetCombinedHistoryData(time.Hour, tenantAdmin)
	assertNoPermissionToQuerySystemMetrics(t, err, "tenant admin combined system metric history")
}

func TestSystemMonitorReturnsNilWhenMetricsManagerIsNotInitialized(t *testing.T) {
	oldManager := metricsManager
	SetMetricsManager(nil)
	defer SetMetricsManager(oldManager)

	admin := &utils.UserClaims{Authority: constant.SYS_ADMIN}
	monitor := &SystemMonitor{}

	current, err := monitor.GetCurrentMetrics(admin)
	if err != nil {
		t.Fatalf("GetCurrentMetrics without manager returned error: %v", err)
	}
	if current != nil {
		t.Fatalf("GetCurrentMetrics without manager = %+v, want nil", current)
	}

	history, err := monitor.GetHistoryData("cpu", time.Hour, admin)
	if err != nil {
		t.Fatalf("GetHistoryData without manager returned error: %v", err)
	}
	if history != nil {
		t.Fatalf("GetHistoryData without manager = %+v, want nil", history)
	}

	combined, err := monitor.GetCombinedHistoryData(time.Hour, admin)
	if err != nil {
		t.Fatalf("GetCombinedHistoryData without manager returned error: %v", err)
	}
	if combined != nil {
		t.Fatalf("GetCombinedHistoryData without manager = %+v, want nil", combined)
	}
}

func TestSystemMonitorDelegatesAdminQueriesToMetricsManager(t *testing.T) {
	oldManager := metricsManager
	now := time.Date(2026, 6, 27, 17, 0, 0, 0, time.UTC)
	manager := &metrics.Metrics{}
	manager.SetHistoryStorage(&pureHelperMetricsStorage{
		current: &metrics.SystemMetrics{
			CPUUsage:    12,
			MemoryUsage: 34,
			DiskUsage:   56,
			Timestamp:   now,
		},
		history: []metrics.MetricDataPoint{
			{Timestamp: now, Value: 12},
		},
	})
	SetMetricsManager(manager)
	defer SetMetricsManager(oldManager)

	admin := &utils.UserClaims{Authority: constant.SYS_ADMIN}
	monitor := &SystemMonitor{}

	current, err := monitor.GetCurrentMetrics(admin)
	if err != nil {
		t.Fatalf("GetCurrentMetrics returned error: %v", err)
	}
	if current.CPUUsage != 12 || current.MemoryUsage != 34 || current.DiskUsage != 56 || !current.Timestamp.Equal(now) {
		t.Fatalf("current metrics = %+v, want delegated values", current)
	}

	history, err := monitor.GetHistoryData("cpu", time.Hour, admin)
	if err != nil {
		t.Fatalf("GetHistoryData returned error: %v", err)
	}
	if len(history) != 1 || history[0].Value != 12 {
		t.Fatalf("history metrics = %+v, want delegated cpu point", history)
	}

	combined, err := monitor.GetCombinedHistoryData(time.Hour, admin)
	if err != nil {
		t.Fatalf("GetCombinedHistoryData returned error: %v", err)
	}
	if len(combined) != 1 || combined[0].CPUUsage != 12 || !combined[0].Timestamp.Equal(now) {
		t.Fatalf("combined metrics = %+v, want delegated combined cpu point", combined)
	}
}

func TestValidateDashboardMenuAccessTrimsTenantAndDashboardIDs(t *testing.T) {
	tenantID, dashboardID, err := validateDashboardMenuAccess(&utils.UserClaims{
		TenantID:  " tenant-1 ",
		Authority: constant.TENANT_ADMIN,
	}, " dashboard-1 ")
	if err != nil {
		t.Fatalf("validateDashboardMenuAccess returned error: %v", err)
	}
	if tenantID != "tenant-1" || dashboardID != "dashboard-1" {
		t.Fatalf("normalized tenant/dashboard = %q/%q, want tenant-1/dashboard-1", tenantID, dashboardID)
	}
}

func TestValidateDashboardMenuAccessRejectsNilBlankTenantAndBlankDashboard(t *testing.T) {
	cases := []struct {
		name        string
		claims      *utils.UserClaims
		dashboardID string
		wantCode    int
		wantMessage string
	}{
		{
			name:        "nil claims",
			claims:      nil,
			dashboardID: "dashboard-1",
			wantCode:    errcode.CodeNoPermission,
			wantMessage: "no permission to manage dashboard menu",
		},
		{
			name:        "blank tenant",
			claims:      &utils.UserClaims{TenantID: "   ", Authority: constant.TENANT_ADMIN},
			dashboardID: "dashboard-1",
			wantCode:    errcode.CodeNoPermission,
			wantMessage: "tenant dashboard menu is only available for tenant users",
		},
		{
			name:        "blank dashboard",
			claims:      &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN},
			dashboardID: "   ",
			wantCode:    errcode.CodeParamError,
			wantMessage: "dashboard_id is required",
		},
	}

	for _, tc := range cases {
		_, _, err := validateDashboardMenuAccess(tc.claims, tc.dashboardID)
		if err == nil {
			t.Fatalf("%s should be rejected with dashboard-menu access error", tc.name)
		}
		appErr, ok := err.(*errcode.Error)
		if !ok {
			t.Fatalf("%s error type = %T, want *errcode.Error", tc.name, err)
		}
		if appErr.Code != tc.wantCode || appErr.CustomMsg != tc.wantMessage {
			t.Fatalf("%s error = code %d message %q, want code %d message %q", tc.name, appErr.Code, appErr.CustomMsg, tc.wantCode, tc.wantMessage)
		}
	}
}

func TestDashboardMenuMethodsRejectInvalidAccessBeforeDAL(t *testing.T) {
	service := &DashboardMenu{}

	_, err := service.GetTenantDashboardMenu(nil, "dashboard-1")
	assertErrcodeError(t, err, "nil claims query tenant dashboard menu", errcode.CodeNoPermission, "no permission to manage dashboard menu")

	_, err = service.UpsertTenantDashboardMenu(&utils.UserClaims{TenantID: "tenant-1"}, "   ", &model.UpsertTenantDashboardMenuReq{})
	assertErrcodeError(t, err, "blank dashboard id upsert tenant dashboard menu", errcode.CodeParamError, "dashboard_id is required")

	err = service.DeleteTenantDashboardMenu(&utils.UserClaims{TenantID: ""}, "dashboard-1")
	assertErrcodeError(t, err, "blank tenant delete tenant dashboard menu", errcode.CodeNoPermission, "tenant dashboard menu is only available for tenant users")
}

func TestNormalizeAlarmListTenantIDEnforcesTenantScope(t *testing.T) {
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	got, err := normalizeAlarmListTenantID("", tenantAdmin)
	if err != nil {
		t.Fatalf("tenant admin should default to own tenant: %v", err)
	}
	if got != "tenant-1" {
		t.Fatalf("tenant admin default tenant = %q, want tenant-1", got)
	}

	got, err = normalizeAlarmListTenantID("tenant-1", tenantAdmin)
	if err != nil {
		t.Fatalf("tenant admin should query own tenant: %v", err)
	}
	if got != "tenant-1" {
		t.Fatalf("tenant admin requested tenant = %q, want tenant-1", got)
	}

	_, err = normalizeAlarmListTenantID("tenant-2", tenantAdmin)
	assertErrcodeError(t, err, "tenant admin cross-tenant alarm query", errcode.CodeNoPermission, "no permission to query alarms for another tenant")
}

func TestNormalizeAlarmListTenantIDAllowsSystemAdminAndRejectsMissingClaims(t *testing.T) {
	got, err := normalizeAlarmListTenantID(" tenant-2 ", &utils.UserClaims{Authority: constant.SYS_ADMIN})
	if err != nil {
		t.Fatalf("system admin should query requested tenant: %v", err)
	}
	if got != "tenant-2" {
		t.Fatalf("system admin tenant filter = %q, want tenant-2", got)
	}

	_, err = normalizeAlarmListTenantID("", nil)
	assertErrcodeError(t, err, "nil claims alarm query", errcode.CodeNoPermission, "no permission to query alarms")

	_, err = normalizeAlarmListTenantID("", &utils.UserClaims{Authority: constant.TENANT_ADMIN})
	assertErrcodeError(t, err, "tenant admin without tenant id alarm query", errcode.CodeNoPermission, "tenant id is required")
}

func TestValidateAlarmHistoryTypeTrimsAndWhitelistsSupportedTypes(t *testing.T) {
	for _, raw := range []string{"temperature_alarm", " switch_alarm ", "warranty_alarm", "pressure_alarm", "PT", "   "} {
		req := &model.GetAlarmHisttoryListByPage{AlarmType: pureHelperStringPtr(raw)}
		if err := validateAlarmHistoryType(req); err != nil {
			t.Fatalf("alarm type %q should be accepted: %v", raw, err)
		}
		if raw == " switch_alarm " && *req.AlarmType != "switch_alarm" {
			t.Fatalf("alarm type should be trimmed, got %q", *req.AlarmType)
		}
	}
	if err := validateAlarmHistoryType(nil); err != nil {
		t.Fatalf("nil alarm history request should be accepted: %v", err)
	}
	if err := validateAlarmHistoryType(&model.GetAlarmHisttoryListByPage{}); err != nil {
		t.Fatalf("missing alarm type should be accepted: %v", err)
	}
}

func TestValidateAlarmHistoryTypeRejectsUnsupportedValues(t *testing.T) {
	for _, raw := range []string{"unknown", "temperature", "pt", "rdi_alarm"} {
		if err := validateAlarmHistoryType(&model.GetAlarmHisttoryListByPage{AlarmType: pureHelperStringPtr(raw)}); err == nil {
			t.Fatalf("alarm type %q should be rejected", raw)
		} else {
			appErr, ok := err.(*errcode.Error)
			if !ok {
				t.Fatalf("alarm type %q error type = %T, want *errcode.Error", raw, err)
			}
			if appErr.Code != errcode.CodeParamError || appErr.CustomMsg != "unsupported alarm_type" {
				t.Fatalf("alarm type %q error = code %d message %q, want code %d unsupported_alarm_type", raw, appErr.Code, appErr.CustomMsg, errcode.CodeParamError)
			}
		}
	}
}

func TestAlarmHistoryDeviceIDsForAccessParsesJSONArraysOnly(t *testing.T) {
	if got := alarmHistoryDeviceIDsForAccess(""); len(got) != 0 {
		t.Fatalf("blank device list should produce no ids, got %#v", got)
	}
	if got := alarmHistoryDeviceIDsForAccess(`not-json`); len(got) != 0 {
		t.Fatalf("invalid device list should produce no ids, got %#v", got)
	}

	got := alarmHistoryDeviceIDsForAccess(`["device-1","device-2"]`)
	if !reflect.DeepEqual(got, []string{"device-1", "device-2"}) {
		t.Fatalf("unexpected alarm history device ids: %#v", got)
	}
}

func TestAlarmHistoryActionsRejectBlankIDBeforeAccessLookup(t *testing.T) {
	alarmService := &Alarm{}
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	_, err := alarmService.AcknowledgeAlarmHistory("   ", claims)
	assertErrcodeError(t, err, "blank alarm history id acknowledge", errcode.CodeParamError, "alarm history id is required")

	_, err = alarmService.ResetAlarmHistory("", claims)
	assertErrcodeError(t, err, "blank alarm history id reset", errcode.CodeParamError, "alarm history id is required")
}

func TestBoardCreateAndUpdateRejectInvalidInputsBeforeDAL(t *testing.T) {
	boardService := &Board{}
	ctx := context.Background()
	invalidConfig := "{"

	_, err := boardService.CreateBoard(ctx, &model.CreateBoardReq{}, nil)
	assertBoardServiceError(t, err, "nil claims create board", errcode.CodeNoPermission, "no permission to create board", nil)

	_, err = boardService.CreateBoard(ctx, &model.CreateBoardReq{Config: &invalidConfig}, &utils.UserClaims{TenantID: "tenant-1"})
	assertBoardServiceError(t, err, "invalid JSON create board", errcode.CodeParamError, "config is not a valid JSON", nil)

	_, err = boardService.UpdateBoard(ctx, &model.UpdateBoardReq{}, nil)
	assertBoardServiceError(t, err, "nil claims update board", errcode.CodeNoPermission, "no permission to modify board", nil)

	_, err = boardService.UpdateBoard(ctx, &model.UpdateBoardReq{Config: &invalidConfig}, &utils.UserClaims{TenantID: "tenant-1"})
	assertBoardServiceError(t, err, "invalid JSON update board", errcode.CodeParamError, "", map[string]interface{}{
		"field": "config",
		"error": "config is not a valid JSON",
	})
}

func TestBoardQueryMethodsRejectMissingClaimsBeforeDAL(t *testing.T) {
	boardService := &Board{}

	_, err := boardService.GetBoardListByPage(&model.GetBoardListByPageReq{}, nil)
	assertBoardServiceError(t, err, "nil claims list boards", errcode.CodeNoPermission, "no permission to query board", nil)

	_, err = boardService.GetBoard("board-1", nil)
	assertBoardServiceError(t, err, "nil claims get board", errcode.CodeNoPermission, "no permission to query board", nil)

	err = boardService.DeleteBoard("board-1", nil)
	assertBoardServiceError(t, err, "nil claims delete board", errcode.CodeNoPermission, "no permission to query board", nil)
}

func TestDeviceConfigCreateRejectsInvalidJSONFieldsBeforeDAL(t *testing.T) {
	service := &DeviceConfig{}
	invalidJSON := "{"
	claims := &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	_, err := service.CreateDeviceConfig(&model.CreateDeviceConfigReq{
		Name:           "bad-additional-info",
		DeviceType:     "1",
		AdditionalInfo: &invalidJSON,
	}, claims)
	assertDeviceConfigServiceError(t, err, "invalid additional_info create device config", errcode.CodeParamError, "additional_info is not a valid JSON")

	_, err = service.CreateDeviceConfig(&model.CreateDeviceConfigReq{
		Name:           "bad-protocol-config",
		DeviceType:     "1",
		ProtocolConfig: &invalidJSON,
	}, claims)
	assertDeviceConfigServiceError(t, err, "invalid protocol_config create device config", errcode.CodeParamError, "protocol_config is not a valid JSON")
}

func TestEnsureDeviceConfigAccessRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := ensureDeviceConfigReadAccess("config-1", nil)
	assertDeviceConfigServiceError(t, err, "nil claims read device config", errcode.CodeNoPermission, "no permission to query device config")

	_, err = ensureDeviceConfigWriteAccess("config-1", nil)
	assertDeviceConfigServiceError(t, err, "nil claims write device config", errcode.CodeNoPermission, "no permission to query device config")
}

func TestEnsureDeviceTemplateAccessRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := ensureDeviceTemplateReadAccess("template-1", nil)
	assertDeviceConfigServiceError(t, err, "nil claims read thing model", errcode.CodeNoPermission, "no permission to query thing model")

	_, err = ensureDeviceTemplateWriteAccess("template-1", nil)
	assertDeviceConfigServiceError(t, err, "nil claims write thing model", errcode.CodeNoPermission, "no permission to query thing model")
}

func TestEnsureDeviceModelTenantWriteAccessAllowsSystemAdminAndSameTenant(t *testing.T) {
	if err := ensureDeviceModelTenantWriteAccess("tenant-1", &utils.UserClaims{Authority: constant.SYS_ADMIN, TenantID: "other"}); err != nil {
		t.Fatalf("system admin should modify device model across tenants: %v", err)
	}
	if err := ensureDeviceModelTenantWriteAccess("tenant-1", &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}); err != nil {
		t.Fatalf("same tenant admin should modify device model: %v", err)
	}
}

func TestEnsureDeviceModelTenantWriteAccessRejectsMissingAndCrossTenantClaims(t *testing.T) {
	cases := []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN, TenantID: "tenant-2"},
		{Authority: constant.TENANT_USER, TenantID: "tenant-2"},
		{Authority: "", TenantID: ""},
	}
	for _, claims := range cases {
		err := ensureDeviceModelTenantWriteAccess("tenant-1", claims)
		assertNoPermissionToModifyDeviceModel(t, err, "claims device model write access")
	}
}

func TestExpectedDataCreateAndPageListRejectEmptyDeviceIDBeforeDAL(t *testing.T) {
	service := &ExpectedData{}
	claims := &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}
	payload := `{"temperature":23}`

	for _, deviceID := range []string{"", "   "} {
		_, err := service.Create(context.Background(), &model.CreateExpectedDataReq{
			DeviceID: deviceID,
			SendType: "telemetry",
			Payload:  &payload,
		}, claims)
		assertExpectedDataServiceError(t, err, "create expected data blank device id", errcode.CodeParamError, "device_id is required")

		_, err = service.PageList(context.Background(), &model.GetExpectedDataPageReq{
			DeviceID: deviceID,
		}, claims)
		assertExpectedDataServiceError(t, err, "page expected data blank device id", errcode.CodeParamError, "device_id is required")
	}
}

func TestOTAPackageAccessRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := ensureOTAPackageAccess("package-1", nil)
	assertOTAServiceError(t, err, "nil claims access ota package", errcode.CodeNoPermission, "no permission to access ota package")
}

func TestOTATaskOwnerUserIDForClaims(t *testing.T) {
	t.Run("tenant user receives trimmed owner scope", func(t *testing.T) {
		ownerUserID, err := otaTaskOwnerUserIDForClaims(&utils.UserClaims{
			ID:        "  owner-1  ",
			Authority: constant.TENANT_USER,
		})
		if err != nil {
			t.Fatalf("otaTaskOwnerUserIDForClaims returned error: %v", err)
		}
		if ownerUserID == nil || *ownerUserID != "owner-1" {
			t.Fatalf("ownerUserID = %#v, want owner-1", ownerUserID)
		}
	})

	t.Run("tenant admin keeps tenant-wide task scope", func(t *testing.T) {
		ownerUserID, err := otaTaskOwnerUserIDForClaims(&utils.UserClaims{
			ID:        "admin-1",
			Authority: constant.TENANT_ADMIN,
		})
		if err != nil {
			t.Fatalf("otaTaskOwnerUserIDForClaims returned error: %v", err)
		}
		if ownerUserID != nil {
			t.Fatalf("ownerUserID = %#v, want nil", ownerUserID)
		}
	})

	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_USER, ID: "   "},
	} {
		if _, err := otaTaskOwnerUserIDForClaims(claims); err == nil {
			t.Fatalf("claims %#v should fail closed", claims)
		} else {
			assertOTAServiceError(t, err, "invalid ota task owner scope", errcode.CodeNoPermission, "no permission to access ota task")
		}
	}
}

func TestOTADeviceWriteAccessRejectsEmptyDeviceListBeforeDAL(t *testing.T) {
	err := ensureOTADeviceWriteAccess(nil, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	assertOTAServiceError(t, err, "nil ota device list", errcode.CodeParamError, "device_id_list is required")

	err = ensureOTADeviceWriteAccess([]string{}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	assertOTAServiceError(t, err, "empty ota device list", errcode.CodeParamError, "device_id_list is required")
}

func TestOTADeviceWriteAccessRejectsBlankDeviceIDBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}
	for _, deviceID := range []string{"", "   "} {
		err := ensureOTADeviceWriteAccess([]string{deviceID}, claims)
		assertOTAServiceError(t, err, "blank ota device id", errcode.CodeParamError, "device_id is required")
	}
}

func TestOTATaskMethodsRejectNilClaimsBeforeDAL(t *testing.T) {
	otaService := &OTA{}
	if err := otaService.CreateOTAUpgradeTask(&model.CreateOTAUpgradeTaskReq{
		OTAUpgradePackageId: "package-1",
		DeviceIdList:        []string{"device-1"},
	}, nil); err == nil {
		t.Fatal("nil claims should not create ota upgrade task")
	} else {
		assertOTAServiceError(t, err, "nil claims create ota upgrade task", errcode.CodeNoPermission, "no permission to access ota package")
	}
	if _, err := otaService.GetOTAUpgradeTaskListByPage(&model.GetOTAUpgradeTaskListByPageReq{
		OTAUpgradePackageId: "package-1",
	}, nil); err == nil {
		t.Fatal("nil claims should not query ota upgrade task list")
	} else {
		assertOTAServiceError(t, err, "nil claims query ota upgrade task list", errcode.CodeNoPermission, "no permission to access ota package")
	}
}

func TestSceneReferenceValidationRejectsEmptyTypedTargetsBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}
	for _, actionType := range []string{
		model.AUTOMATE_ACTION_TYPE_ONE,
		model.AUTOMATE_ACTION_TYPE_MULTIPLE,
		model.AUTOMATE_ACTION_TYPE_ALARM,
	} {
		err := validateSceneActionReferences([]model.SceneActionsReq{{ActionType: actionType}}, claims, "tenant-1")
		if err == nil {
			t.Fatalf("scene action type %q should reject an empty action target before DAL", actionType)
		}
	}
}

func TestSceneReferenceValidationIgnoresUnknownActionTypes(t *testing.T) {
	err := validateSceneActionReferences([]model.SceneActionsReq{
		{ActionType: "unknown"},
	}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
	if err != nil {
		t.Fatalf("unknown scene action type should be ignored by action validation: %v", err)
	}
}

func TestSceneCreateRejectsNilClaimsBeforeReferenceOrDALWork(t *testing.T) {
	_, err := (&Scene{}).CreateScene(model.CreateSceneReq{}, nil)
	assertNoPermissionToCreateScene(t, err, "nil claims create scene")
}

func TestSceneAutomationReferenceValidationSkipsEmptyTargetsAndRejectsServiceActions(t *testing.T) {
	triggerSource := ""
	err := validateSceneAutomationReferences([][]model.Condition{{
		{TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &triggerSource},
		{TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE, TriggerSource: &triggerSource},
	}}, []model.Action{
		{ActionType: model.AUTOMATE_ACTION_TYPE_ONE},
		{ActionType: model.AUTOMATE_ACTION_TYPE_MULTIPLE},
		{ActionType: model.AUTOMATE_ACTION_TYPE_SCENE},
		{ActionType: model.AUTOMATE_ACTION_TYPE_ALARM},
	}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
	if err != nil {
		t.Fatalf("empty automation references should be ignored: %v", err)
	}

	err = validateSceneAutomationReferences(nil, []model.Action{
		{ActionType: model.AUTOMATE_ACTION_TYPE_SERVICE, ActionTarget: "service-1"},
	}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
	if err == nil {
		t.Fatal("service action should be rejected because it is not supported")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("service action error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeParamError || appErr.CustomMsg != "service action is not supported" {
		t.Fatalf("service action error = code %d message %q, want code %d unsupported-service message", appErr.Code, appErr.CustomMsg, errcode.CodeParamError)
	}
}

func TestSceneAutomationReferenceValidationRejectsEmptyEventParamMatchConditions(t *testing.T) {
	triggerParamType := "event"
	triggerValue := `{"match_mode":"field","conditions":[]}`
	err := validateSceneAutomationReferences([][]model.Condition{{
		{
			TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE,
			TriggerParamType:      &triggerParamType,
			TriggerValue:          &triggerValue,
		},
	}}, nil, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
	assertErrcodeError(t, err, "empty event param match conditions", errcode.CodeParamError, "event trigger_value must contain at least one condition")
}

func TestSceneAutomationReferenceValidationRejectsBlankEventParamMatchField(t *testing.T) {
	triggerParamType := "event"
	triggerValue := `{"match_mode":"field","conditions":[{"field":" ","operator":"=","value":"ok"}]}`
	err := validateSceneAutomationReferences([][]model.Condition{{
		{
			TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE,
			TriggerParamType:      &triggerParamType,
			TriggerValue:          &triggerValue,
		},
	}}, nil, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
	assertErrcodeError(t, err, "blank event param match field", errcode.CodeParamError, "event trigger_value condition field is required")
}

func TestSceneAutomationReferenceValidationRejectsInvalidEventParamMatchValues(t *testing.T) {
	triggerParamType := "event"
	cases := []struct {
		name         string
		triggerValue string
		wantMessage  string
	}{
		{
			name:         "blank equal value",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"=","value":" "}]}`,
			wantMessage:  "event trigger_value condition value is required",
		},
		{
			name:         "exists not boolean",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"exists","value":"true"}]}`,
			wantMessage:  "event trigger_value exists operator requires a boolean value",
		},
		{
			name:         "between missing max",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"between","value":[1]}]}`,
			wantMessage:  "event trigger_value between operator requires two ordered numeric values",
		},
		{
			name:         "in empty values",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"in","value":[]}]}`,
			wantMessage:  "event trigger_value in operator requires a non-empty list",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			triggerValue := tc.triggerValue
			err := validateSceneAutomationReferences([][]model.Condition{{
				{
					TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE,
					TriggerParamType:      &triggerParamType,
					TriggerValue:          &triggerValue,
				},
			}}, nil, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}, "tenant-1")
			assertErrcodeError(t, err, tc.name, errcode.CodeParamError, tc.wantMessage)
		})
	}
}

func TestSceneAutomationCreateRejectsNilClaimsBeforeTransaction(t *testing.T) {
	_, err := (&SceneAutomation{}).CreateSceneAutomation(&model.CreateSceneAutomationReq{}, nil)
	assertNoPermissionToCreateSceneAutomation(t, err, "nil claims create scene automation")
}

func TestSceneAutomationSwitchTargetTogglesOnlyWhenTargetIsBlank(t *testing.T) {
	tests := []struct {
		name             string
		currentEnabled   string
		requestedTarget  string
		wantSwitchTarget string
	}{
		{name: "blank target disables enabled automation", currentEnabled: "Y", requestedTarget: "", wantSwitchTarget: "N"},
		{name: "blank target enables disabled automation", currentEnabled: "N", requestedTarget: "", wantSwitchTarget: "Y"},
		{name: "blank target enables empty state fallback", currentEnabled: "", requestedTarget: "", wantSwitchTarget: "Y"},
		{name: "explicit enable is preserved even when already enabled", currentEnabled: "Y", requestedTarget: "Y", wantSwitchTarget: "Y"},
		{name: "explicit disable is preserved even when already disabled", currentEnabled: "N", requestedTarget: "N", wantSwitchTarget: "N"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sceneAutomationSwitchTarget(tc.currentEnabled, tc.requestedTarget); got != tc.wantSwitchTarget {
				t.Fatalf("sceneAutomationSwitchTarget(%q, %q) = %q, want %q", tc.currentEnabled, tc.requestedTarget, got, tc.wantSwitchTarget)
			}
		})
	}
}

func TestServicePluginGuardsAllowExpectedRoles(t *testing.T) {
	systemAdmin := &utils.UserClaims{Authority: constant.SYS_ADMIN}
	if err := requireServicePluginAdmin(systemAdmin); err != nil {
		t.Fatalf("system admin should manage service plugins: %v", err)
	}

	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN},
		{Authority: constant.TENANT_USER},
	} {
		assertNoPermissionToManageServicePlugins(t, requireServicePluginAdmin(claims), "requireServicePluginAdmin")
	}

	for _, claims := range []*utils.UserClaims{
		systemAdmin,
		{Authority: constant.TENANT_ADMIN},
		{Authority: constant.TENANT_USER},
	} {
		if err := requireServicePluginViewer(claims); err != nil {
			t.Fatalf("claims %#v should query service plugins: %v", claims, err)
		}
	}
	assertNoPermissionToQueryServicePlugins(t, requireServicePluginViewer(nil), "requireServicePluginViewer")
}

func TestPublicServicePluginInfoReturnsOnlyPublicFields(t *testing.T) {
	version := "1.2.3"
	description := "MQTT access"
	serviceConfig := `{"secret":"hidden"}`
	remark := "public remark"

	got := publicServicePluginInfo(&model.ServicePlugin{
		ID:                "plugin-1",
		Name:              "MQTT",
		ServiceIdentifier: "MQTT",
		ServiceType:       1,
		Version:           &version,
		Description:       &description,
		ServiceConfig:     &serviceConfig,
		Remark:            &remark,
	})

	wantKeys := []string{"id", "name", "service_identifier", "service_type", "version", "description", "remark"}
	if len(got) != len(wantKeys) {
		t.Fatalf("publicServicePluginInfo keys = %#v, want exactly public keys %#v", got, wantKeys)
	}
	for _, key := range wantKeys {
		if _, ok := got[key]; !ok {
			t.Fatalf("publicServicePluginInfo missing key %q in %#v", key, got)
		}
	}
	if _, ok := got["service_config"]; ok {
		t.Fatalf("publicServicePluginInfo leaked service_config: %#v", got)
	}
	if got["id"] != "plugin-1" ||
		got["name"] != "MQTT" ||
		got["service_identifier"] != "MQTT" ||
		got["service_type"] != int32(1) ||
		got["version"] != &version ||
		got["description"] != &description ||
		got["remark"] != &remark {
		t.Fatalf("unexpected public plugin info: %#v", got)
	}
	if empty := publicServicePluginInfo(nil); len(empty) != 0 {
		t.Fatalf("nil plugin should return empty map, got %#v", empty)
	}
}

func TestServicePluginMethodsRejectUnauthorizedBeforeDAL(t *testing.T) {
	service := &ServicePlugin{}
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}

	_, err := service.Create(&model.CreateServicePluginReq{Name: "plugin"}, tenantAdmin)
	assertNoPermissionToManageServicePlugins(t, err, "tenant admin create service plugin")

	_, err = service.List(&model.GetServicePluginByPageReq{}, tenantUser)
	assertNoPermissionToManageServicePlugins(t, err, "tenant user list service plugins")

	_, err = service.Get("plugin-1", tenantAdmin)
	assertNoPermissionToManageServicePlugins(t, err, "tenant admin get service plugin detail")

	err = service.Update(&model.UpdateServicePluginReq{ID: "plugin-1"}, tenantAdmin)
	assertNoPermissionToManageServicePlugins(t, err, "tenant admin update service plugin")

	err = service.Delete("plugin-1", tenantUser)
	assertNoPermissionToManageServicePlugins(t, err, "tenant user delete service plugin")

	_, err = service.GetServiceSelect(&model.GetServiceSelectReq{}, nil)
	assertNoPermissionToQueryServicePlugins(t, err, "nil claims get service select")

	_, err = service.GetServicePluginByServiceIdentifier(" MQTT ", nil)
	assertNoPermissionToQueryServicePlugins(t, err, "nil claims query service plugin by identifier")
}

func TestServicePluginHeartbeatRejectsBlankIdentifierBeforeDAL(t *testing.T) {
	service := &ServicePlugin{}
	for _, rawIdentifier := range []string{"", "   "} {
		req := &model.HeartbeatReq{ServiceIdentifier: rawIdentifier}
		if err := service.Heartbeat(req); err == nil {
			t.Fatalf("Heartbeat(%q) should reject blank service_identifier", rawIdentifier)
		} else {
			assertServicePluginError(t, err, "blank service plugin heartbeat identifier", errcode.CodeParamError, "service_identifier is required")
		}
		if req.ServiceIdentifier != "" {
			t.Fatalf("Heartbeat should trim service_identifier to blank, got %q", req.ServiceIdentifier)
		}
	}
}

func TestServicePluginMQTTProtocolFormNeedsNoPluginLookup(t *testing.T) {
	for _, tc := range []struct {
		name         string
		protocolType string
		deviceType   string
	}{
		{name: "canonical", protocolType: "MQTT", deviceType: "1"},
		{name: "lowercase", protocolType: "mqtt", deviceType: "1"},
		{name: "trimmed", protocolType: " MQTT ", deviceType: " 1 "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := (&ServicePlugin{}).GetProtocolPluginFormByProtocolType(tc.protocolType, tc.deviceType)
			if err != nil {
				t.Fatalf("MQTT protocol form should not error: %v", err)
			}
			if data != nil {
				t.Fatalf("MQTT protocol form = %#v, want nil built-in form", data)
			}
		})
	}
}

func TestNormalizePluginFormLookupRejectsMissingRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name         string
		protocolType string
		deviceType   string
		formType     string
		message      string
	}{
		{name: "protocol type", protocolType: " ", deviceType: "1", formType: string(constant.CONFIG_FORM), message: "protocol_type is required"},
		{name: "device type", protocolType: "http", deviceType: " ", formType: string(constant.CONFIG_FORM), message: "device_type is required"},
		{name: "form type", protocolType: "http", deviceType: "1", formType: " ", message: "form_type is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := normalizePluginFormLookup(tc.protocolType, tc.deviceType, tc.formType)
			assertServicePluginError(t, err, tc.name, errcode.CodeParamError, tc.message)
		})
	}
}
