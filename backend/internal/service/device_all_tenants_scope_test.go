package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestDeviceListAllTenantsRejectsNonSystemAdministrators(t *testing.T) {
	req := &model.GetDeviceListByPageReq{AllTenants: true}
	for _, claims := range []*utils.UserClaims{
		nil,
		{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN},
		{ID: "tenant-user", TenantID: "tenant-a", Authority: constant.TENANT_USER},
	} {
		_, err := (&Device{}).GetDeviceListByPage(req, claims)
		assertDeviceConfigServiceError(
			t,
			err,
			"non-system administrator all-tenants device list",
			errcode.CodeNoPermission,
			"all-tenants device list is only available to system administrators",
		)
	}
}

func TestDeviceListDefaultScopeStillRequiresTenantForSystemAdministrator(t *testing.T) {
	_, err := (&Device{}).GetDeviceListByPage(
		&model.GetDeviceListByPageReq{},
		&utils.UserClaims{ID: "sys-admin", Authority: constant.SYS_ADMIN},
	)
	assertDeviceConfigServiceError(t, err, "default system-admin device list", errcode.CodeNoPermission, "no permission to query device list")
}

func TestDeviceListAllTenantsReturnsTenantContextForSystemAdministrator(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configA := createDeviceServiceConfig(t, db, "all-tenant-config-a", "tenant-a", "1")
	configB := createDeviceServiceConfig(t, db, "all-tenant-config-b", "tenant-b", "1")
	createDeviceServiceDevice(t, db, "all-tenant-device-a", "all-tenant-number-a", "tenant-a", configA, now)
	createDeviceServiceDevice(t, db, "all-tenant-device-b", "all-tenant-number-b", "tenant-b", configB, now)

	rsp, err := (&Device{}).GetDeviceListByPage(
		&model.GetDeviceListByPageReq{
			PageReq:    model.PageReq{Page: 1, PageSize: 10},
			AllTenants: true,
		},
		&utils.UserClaims{ID: "sys-admin", Authority: constant.SYS_ADMIN},
	)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int64(2), rsp["total"])
	rows := rsp["list"].([]model.GetDeviceListByPageRsp)
	assert.Len(t, rows, 2)
	tenantByDevice := make(map[string]string, len(rows))
	for _, row := range rows {
		tenantByDevice[row.ID] = row.ScopeTenantID
	}
	assert.Equal(t, "tenant-a", tenantByDevice["all-tenant-device-a"])
	assert.Equal(t, "tenant-b", tenantByDevice["all-tenant-device-b"])
}

func TestDeviceListScopeTenantIDIsOptInJSON(t *testing.T) {
	defaultEncoded, err := json.Marshal(model.GetDeviceListByPageRsp{TenantID: "tenant-a"})
	assert.NoError(t, err)
	assert.NotContains(t, string(defaultEncoded), "scope_tenant_id")
	assert.NotContains(t, string(defaultEncoded), "tenant-a")

	allTenantsEncoded, err := json.Marshal(model.GetDeviceListByPageRsp{ScopeTenantID: "tenant-a"})
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(allTenantsEncoded), `"scope_tenant_id":"tenant-a"`))
}

func TestDeviceOverviewAllTenantsUsesTheSameSystemAdministratorGate(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configA := createDeviceServiceConfig(t, db, "overview-config-a", "tenant-a", "1")
	configB := createDeviceServiceConfig(t, db, "overview-config-b", "tenant-b", "1")
	createDeviceServiceDevice(t, db, "overview-device-a", "overview-number-a", "tenant-a", configA, now)
	createDeviceServiceDevice(t, db, "overview-device-b", "overview-number-b", "tenant-b", configB, now)

	data, err := (&Board{}).GetDeviceOverview(
		context.Background(),
		&model.GetBoardDeviceReq{AllTenants: true},
		&utils.UserClaims{ID: "sys-admin", Authority: constant.SYS_ADMIN},
	)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int64(2), data.DeviceTotal)

	_, err = (&Board{}).GetDeviceOverview(
		context.Background(),
		&model.GetBoardDeviceReq{AllTenants: true},
		&utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN},
	)
	assertDeviceConfigServiceError(
		t,
		err,
		"tenant admin all-tenants device overview",
		errcode.CodeNoPermission,
		"all-tenants device overview is only available to system administrators",
	)
}
