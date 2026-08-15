package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestEnsureNotificationHistoryOwnerScope(t *testing.T) {
	tests := []struct {
		name    string
		claims  *utils.UserClaims
		wantErr bool
	}{
		{name: "missing claims fail closed", wantErr: true},
		{
			name: "unknown authority fails closed",
			claims: &utils.UserClaims{
				ID:        "unexpected-role-1",
				TenantID:  "tenant-1",
				Authority: "UNEXPECTED_ROLE",
			},
			wantErr: true,
		},
		{
			name: "tenant user continues to device owner scoped query",
			claims: &utils.UserClaims{
				ID:        "tenant-user-1",
				TenantID:  "tenant-1",
				Authority: constant.TENANT_USER,
			},
		},
		{
			name: "tenant admin keeps tenant wide audit access",
			claims: &utils.UserClaims{
				ID:        "tenant-admin-1",
				TenantID:  "tenant-1",
				Authority: constant.TENANT_ADMIN,
			},
		},
		{
			name: "system admin remains allowed",
			claims: &utils.UserClaims{
				ID:        "sys-admin-1",
				TenantID:  "tenant-1",
				Authority: constant.SYS_ADMIN,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureNotificationHistoryOwnerScope(tt.claims)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected permission error, got nil")
				}
				appErr, ok := err.(*errcode.Error)
				if !ok {
					t.Fatalf("expected *errcode.Error, got %T", err)
				}
				if appErr.Code != errcode.CodeNoPermission {
					t.Fatalf("permission error code = %d, want %d", appErr.Code, errcode.CodeNoPermission)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRedactNotificationHistoryForTenantUser(t *testing.T) {
	content := "device alarm body"
	remark := "smtp failure details"
	rows := []*model.NotificationHistory{
		{
			SendContent: &content,
			SendTarget:  "tenant-admin@example.com",
			Remark:      &remark,
		},
	}

	redactNotificationHistoryForTenantUser(rows, &utils.UserClaims{Authority: constant.TENANT_USER})
	if rows[0].SendTarget != "" || rows[0].SendContent != nil || rows[0].Remark != nil {
		t.Fatalf("tenant-user notification history still exposes sensitive delivery fields: %#v", rows[0])
	}
}

func TestRedactNotificationHistoryKeepsAdminAuditFields(t *testing.T) {
	content := "device alarm body"
	remark := "smtp failure details"
	rows := []*model.NotificationHistory{
		{
			SendContent: &content,
			SendTarget:  "tenant-admin@example.com",
			Remark:      &remark,
		},
	}

	redactNotificationHistoryForTenantUser(rows, &utils.UserClaims{Authority: constant.TENANT_ADMIN})
	if rows[0].SendTarget == "" || rows[0].SendContent == nil || rows[0].Remark == nil {
		t.Fatalf("tenant-admin notification audit fields were redacted: %#v", rows[0])
	}
}
