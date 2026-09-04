// 文件用途：验证场景自动化读作用域纯函数映射（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only（场景为租户级资源、无 per-user 维度）、空租户
// fail-closed / 平台 [""]、非空租户管理员 expandTenantIDScope（无链接回退 self-only）。
package service

import (
	"reflect"
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

func TestSceneAutomationReadScopes(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		claims   *utils.UserClaims
		want     []string
	}{
		{
			name: "missing claims fail closed",
		},
		{
			name:     "tenant user keeps self only",
			tenantID: "tenant-1",
			claims: &utils.UserClaims{
				ID:        "tenant-user-1",
				TenantID:  "tenant-1",
				Authority: constant.TENANT_USER,
			},
			want: []string{"tenant-1"},
		},
		{
			name: "tenant user without tenant fails closed",
			claims: &utils.UserClaims{
				ID:        "tenant-user-2",
				TenantID:  "",
				Authority: constant.TENANT_USER,
			},
		},
		{
			name: "system admin empty tenant maps platform scope",
			claims: &utils.UserClaims{
				ID:        "sys-admin-1",
				TenantID:  "",
				Authority: constant.SYS_ADMIN,
			},
			want: []string{""},
		},
		{
			name:     "tenant admin without hierarchy links falls back to self only",
			tenantID: "tenant-1",
			claims: &utils.UserClaims{
				ID:        "tenant-admin-1",
				TenantID:  "tenant-1",
				Authority: constant.TENANT_ADMIN,
			},
			want: []string{"tenant-1"},
		},
		{
			name:     "system admin with tenant expands like tenant admin",
			tenantID: "tenant-1",
			claims: &utils.UserClaims{
				ID:        "sys-admin-2",
				TenantID:  "tenant-1",
				Authority: constant.SYS_ADMIN,
			},
			want: []string{"tenant-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sceneAutomationReadScopes(tt.tenantID, tt.claims)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sceneAutomationReadScopes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSceneAutomationListScopesMissingClaimsRejected(t *testing.T) {
	_, err := sceneAutomationListScopes(nil, nil, nil)
	if err == nil {
		t.Fatal("expected permission error for nil claims, got nil")
	}
}
