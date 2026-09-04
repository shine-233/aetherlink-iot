package service

import "testing"

// TestTenantIDInScopes 覆盖读守卫的纯成员判断：模板归属落在自上而下作用域内才可读。
func TestTenantIDInScopes(t *testing.T) {
	cases := []struct {
		resource string
		scopes   []string
		want     bool
	}{
		{"child", []string{"hq", "child"}, true},     // 子租户模板对总部可见（自上而下）
		{"hq", []string{"hq", "child"}, true},        // 自身
		{"hq", []string{"child"}, false},             // 子级不能读父级模板
		{"tenant-x", []string{"hq", "child"}, false}, // 旁支租户不可见
		{"child", nil, false},                        // 空作用域 fail-closed
		{"", []string{"hq"}, false},                  // 空资源租户
	}
	for i, c := range cases {
		if got := tenantIDInScopes(c.resource, c.scopes); got != c.want {
			t.Fatalf("case %d tenantIDInScopes(%q, %v)=%v want %v", i, c.resource, c.scopes, got, c.want)
		}
	}
}
