package service

import "testing"

// TestInheritedRoleMarkers 覆盖 RBAC 继承接缝（自上而下：总部管理员对子树打域角色标记）。
func TestInheritedRoleMarkers(t *testing.T) {
	got := inheritedRoleMarkersFor("TENANT_ADMIN", []string{"t-east", "t-east-s"})
	want := []string{"TENANT_ADMIN", "TENANT_ADMIN@t-east", "TENANT_ADMIN@t-east-s"}
	if len(got) != len(want) {
		t.Fatalf("markers len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("markers[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

// TestInheritedAuthorityRolesFallbacks 覆盖无子孙/非管理员路径（无需 DB 链接）。
func TestInheritedAuthorityRolesFallbacks(t *testing.T) {
	if got := InheritedAuthorityRoles("TENANT_USER", "t-x"); len(got) != 1 || got[0] != "TENANT_USER" {
		t.Fatalf("tenant user must not expand: %v", got)
	}
	if got := InheritedAuthorityRoles("SYS_ADMIN", ""); len(got) != 1 || got[0] != "SYS_ADMIN" {
		t.Fatalf("sys admin without tenant must not expand: %v", got)
	}
	if got := InheritedAuthorityRoles("TENANT_ADMIN", "t-hq"); len(got) != 1 || got[0] != "TENANT_ADMIN" {
		t.Fatalf("admin without resolvable links must fallback to self role: %v", got)
	}
}
