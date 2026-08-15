package service

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestAllTenantsScopeRequiresSystemAdministrator(t *testing.T) {
	for _, authority := range []string{constant.TENANT_USER, constant.TENANT_ADMIN, constant.SYS_ADMIN} {
		if err := requireSystemAdminAllTenantsScope(false, &utils.UserClaims{Authority: authority}, "denied"); err != nil {
			t.Fatalf("supported authority %q was rejected for default scope: %v", authority, err)
		}
	}
	if err := requireSystemAdminAllTenantsScope(true, &utils.UserClaims{Authority: constant.SYS_ADMIN}, "denied"); err != nil {
		t.Fatalf("system administrator all-tenant scope was rejected: %v", err)
	}
	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN, TenantID: "tenant-a"},
		{Authority: constant.TENANT_USER, TenantID: "tenant-a"},
	} {
		err := requireSystemAdminAllTenantsScope(true, claims, "denied")
		assertErrcodeError(t, err, "all-tenant scope", errcode.CodeNoPermission, "denied")
	}
}

func TestScopeAuthorityAllowlistRejectsUnknownAndEmptyAuthorities(t *testing.T) {
	for _, claims := range []*utils.UserClaims{
		nil,
		{},
		{Authority: "NEMAS_ADMIN", TenantID: "tenant-a"},
		{Authority: " TENANT_ADMIN ", TenantID: "tenant-a"},
	} {
		err := requireSystemAdminAllTenantsScope(false, claims, "denied")
		assertErrcodeError(t, err, "default read scope authority", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)
	}
	if _, err := requireDeviceTenantClaims(&utils.UserClaims{TenantID: "tenant-a", Authority: "NEMAS_ADMIN"}, "denied"); err == nil {
		t.Fatal("unknown authority reached tenant device scope")
	}
	if err := ensureAlarmTenantAccess("tenant-a", &utils.UserClaims{TenantID: "tenant-a", Authority: "NEMAS_ADMIN"}, "denied"); err == nil {
		t.Fatal("unknown authority reached tenant alarm scope")
	}
}

func TestAlarmAllTenantsEntryPointsRejectTenantRolesBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
	alarmService := &Alarm{}

	_, err := alarmService.GetAlarmHisttoryListByPage(&model.GetAlarmHisttoryListByPage{AllTenants: true}, claims)
	assertErrcodeError(t, err, "all-tenant alarm history", errcode.CodeNoPermission, "all-tenants alarm history is only available to system administrators")

	_, err = alarmService.GetAlarmHistoryMonthlyTrend(&model.AlarmHistoryMonthlyTrendReq{Year: 2026, AllTenants: true}, claims)
	assertErrcodeError(t, err, "all-tenant alarm trend", errcode.CodeNoPermission, "all-tenants alarm trend is only available to system administrators")

	_, err = alarmService.GetAlarmDeviceCounts(&model.AlarmDeviceCountsReq{AllTenants: true}, claims)
	assertErrcodeError(t, err, "all-tenant alarm counts", errcode.CodeNoPermission, "all-tenants alarm counts are only available to system administrators")
}

func TestDeviceAndBoardAllTenantsEntryPointsRejectTenantRolesBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}

	_, err := resolveDeviceListTenantScope(&model.GetDeviceListByPageReq{AllTenants: true}, claims)
	assertErrcodeError(t, err, "all-tenant device list", errcode.CodeNoPermission, "all-tenants device list is only available to system administrators")

	_, err = (&Board{}).GetDeviceOverview(context.Background(), &model.GetBoardDeviceReq{AllTenants: true}, claims)
	assertErrcodeError(t, err, "all-tenant device overview", errcode.CodeNoPermission, "all-tenants device overview is only available to system administrators")
}

func TestCustomerReadScopeEntryPointsRejectUnknownAuthorityBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{ID: "unknown-user", TenantID: "tenant-a", Authority: "NEMAS_ADMIN"}
	alarmService := &Alarm{}

	_, err := alarmService.GetAlarmHisttoryListByPage(&model.GetAlarmHisttoryListByPage{}, claims)
	assertErrcodeError(t, err, "default alarm history scope", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = alarmService.GetAlarmHistoryMonthlyTrend(&model.AlarmHistoryMonthlyTrendReq{Year: 2026}, claims)
	assertErrcodeError(t, err, "default alarm trend scope", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = alarmService.GetAlarmDeviceCounts(&model.AlarmDeviceCountsReq{}, claims)
	assertErrcodeError(t, err, "default alarm count scope", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = (&Device{}).GetDeviceListByPage(&model.GetDeviceListByPageReq{}, claims)
	assertErrcodeError(t, err, "default device list scope", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = (&Board{}).GetDeviceOverview(context.Background(), &model.GetBoardDeviceReq{}, claims)
	assertErrcodeError(t, err, "default tenant device overview scope", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = (&Board{}).GetDeviceTotal(context.Background(), claims)
	assertErrcodeError(t, err, "legacy board device total scope", errcode.CodeNoPermission, "no permission to query device total")

	_, err = (&Board{}).GetDevice(context.Background(), claims)
	assertErrcodeError(t, err, "legacy board device overview scope", errcode.CodeNoPermission, "no permission to query device overview")

	_, err = (&Device{}).GetDeviceTrend(context.Background(), claims, claims.TenantID, nil, nil)
	assertErrcodeError(t, err, "board device trend scope", errcode.CodeNoPermission, "no permission to query device trend")
}

func TestDeviceTrendRejectsTenantCrossScopeBeforeDAL(t *testing.T) {
	for _, authority := range []string{constant.TENANT_USER, constant.TENANT_ADMIN} {
		claims := &utils.UserClaims{ID: "tenant-reader", TenantID: "tenant-a", Authority: authority}
		_, err := (&Device{}).GetDeviceTrend(context.Background(), claims, "tenant-b", nil, nil)
		assertErrcodeError(t, err, "cross-tenant board device trend", errcode.CodeNoPermission, "no permission to query device trend")
	}
}
