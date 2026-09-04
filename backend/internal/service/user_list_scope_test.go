// 文件用途：验证用户目录列表读作用域纯函数映射（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only、TENANT_ADMIN expandTenantIDScope（无链接回退 self-only）、
// SYS_ADMIN 平台级管理员目录（nil scopes，无租户过滤）、nil/未知声明 fail-closed。
package service

import (
	"reflect"
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

func TestUserListScopes(t *testing.T) {
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
			name: "tenant admin without tenant fails closed",
			claims: &utils.UserClaims{
				ID:        "tenant-admin-2",
				TenantID:  "",
				Authority: constant.TENANT_ADMIN,
			},
		},
		{
			name: "system admin keeps platform admin directory",
			claims: &utils.UserClaims{
				ID:        "sys-admin-1",
				TenantID:  "",
				Authority: constant.SYS_ADMIN,
			},
		},
		{
			name: "unknown authority fails closed",
			claims: &utils.UserClaims{
				ID:        "unexpected-1",
				TenantID:  "tenant-1",
				Authority: "UNEXPECTED_ROLE",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := userListScopes(tt.claims)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("userListScopes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
