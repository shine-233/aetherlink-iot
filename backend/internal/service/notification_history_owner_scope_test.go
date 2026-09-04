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

func TestNotificationHistoryListScopes(t *testing.T) {
	// TENANT_USER 保持 self-only：其可见性由 DAL 的 device-owner 关系 EXISTS 钳制，
	// 即使父租户存在子孙也不做层级展开。
	if got := notificationHistoryListScopes(&utils.UserClaims{
		ID: "tenant-user-1", TenantID: "tenant-1", Authority: constant.TENANT_USER,
	}); len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("tenant-user scope = %#v, want [tenant-1]", got)
	}
	if got := notificationHistoryListScopes(&utils.UserClaims{
		ID: "tenant-user-1", TenantID: "", Authority: constant.TENANT_USER,
	}); got != nil {
		t.Fatalf("tenant-user without tenant scope = %#v, want nil (fail closed)", got)
	}
	// 空租户（SYS_ADMIN 平台空租户行）→ [""]，保持旧 tenant_id='' 行为。
	if got := notificationHistoryListScopes(&utils.UserClaims{
		ID: "sys-admin-1", TenantID: "", Authority: constant.SYS_ADMIN,
	}); len(got) != 1 || got[0] != "" {
		t.Fatalf("sys-admin platform scope = %#v, want [\"\"]", got)
	}
	// 非空租户（TENANT_ADMIN/SYS_ADMIN）→ 展开 self∪子孙；无链接时回退 self-only。
	if got := notificationHistoryListScopes(&utils.UserClaims{
		ID: "tenant-admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN,
	}); len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("tenant-admin fallback scope = %#v, want [tenant-1]", got)
	}
	if got := notificationHistoryListScopes(nil); got != nil {
		t.Fatalf("nil claims scope = %#v, want nil", got)
	}
}
