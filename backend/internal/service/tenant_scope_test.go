package service

import "testing"

// setupTenantScopeDB 使用 sqlite 装载 tenants 链接表（仅 parent_tenant_id 语义需要）。
// tenants 真实列较多，这里只建测试所需列；其余函数在链接不可用时回退 self-only，不影响本测试意图。
func TestInheritedAuthorityRolesExpandsAncestors(t *testing.T) {
	// 纯函数路径（无链接表）：TENANT_USER 不扩展、管理员无祖先不扩展。
	if got := InheritedAuthorityRoles("TENANT_USER", "t2"); len(got) != 1 || got[0] != "TENANT_USER" {
		t.Fatalf("tenant user must not expand: %v", got)
	}
	if got := InheritedAuthorityRoles("SYS_ADMIN", ""); len(got) != 1 || got[0] != "SYS_ADMIN" {
		t.Fatalf("sys admin without tenant must not expand: %v", got)
	}
	// 纯展开：注入祖先链 → base role + role@ancestor。
	got := inheritedAuthorityRolesFor("TENANT_ADMIN", []string{"t-parent", "t-root"})
	want := []string{"TENANT_ADMIN", "TENANT_ADMIN@t-parent", "TENANT_ADMIN@t-root"}
	if len(got) != len(want) {
		t.Fatalf("expand len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expand[%d]=%q want %q", i, got[i], want[i])
		}
	}
}
