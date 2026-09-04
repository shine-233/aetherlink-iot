// 文件用途：验证命令任务列表读作用域纯函数映射（ROADMAP C2 自上而下）与 nil 声明拒绝：
// TENANT_USER 保持 self-only、TENANT_ADMIN/SYS_ADMIN 非空租户 expandTenantIDScope
// （无链接回退 self-only）、空租户 [""] 保持旧行为、nil/空 fail-closed。
// 边界：设备筛选预览/提交仍锚定操作员本租户（C2 仅放列表读）；超时恢复保持单租户。
package service

import (
	"reflect"
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

func TestFleetCommandJobListScopes(t *testing.T) {
	tests := []struct {
		name   string
		claims *utils.UserClaims
		want   []string
	}{
		{
			name: "missing claims fail closed",
		},
		{
			name: "tenant user keeps self only",
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
			name: "tenant admin without hierarchy links falls back to self only",
			claims: &utils.UserClaims{
				ID:        "tenant-admin-1",
				TenantID:  "tenant-1",
				Authority: constant.TENANT_ADMIN,
			},
			want: []string{"tenant-1"},
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
			name: "system admin with tenant expands like tenant admin",
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
			got := fleetCommandJobListScopes(tt.claims)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("fleetCommandJobListScopes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
