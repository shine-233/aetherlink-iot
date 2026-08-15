package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestActiveAlarmInfoOwnerScopeFailsClosedForTenantUsers(t *testing.T) {
	for _, claims := range []*utils.UserClaims{
		nil,
		{ID: "tenant-user-1", TenantID: "tenant-1", Authority: constant.TENANT_USER},
		{ID: "unexpected-role-1", TenantID: "tenant-1", Authority: "UNEXPECTED_ROLE"},
	} {
		err := ensureActiveAlarmInfoOwnerScope(claims)
		appErr, ok := err.(*errcode.Error)
		if !ok || appErr.Code != errcode.CodeNoPermission {
			t.Fatalf("tenant-user active alarm info access error = %#v, want no-permission", err)
		}
	}
}

func TestActiveAlarmInfoOwnerScopeKeepsManagerRolesAvailable(t *testing.T) {
	for _, claims := range []*utils.UserClaims{
		{ID: "tenant-admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN},
		{ID: "sys-admin-1", Authority: constant.SYS_ADMIN},
	} {
		if err := ensureActiveAlarmInfoOwnerScope(claims); err != nil {
			t.Fatalf("manager active alarm info access was rejected: %v", err)
		}
	}
}
