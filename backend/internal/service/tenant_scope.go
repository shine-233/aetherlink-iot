// 文件用途：租户层级作用域展开（ROADMAP C2）——服务层公共 helper。
// 核心逻辑：由 tenants.parent_tenant_id 链接（dal.ListTenantParentLinks）经 hierarchy.Scope
//
//	计算 self∪祖先；链接不可用/异常时回退 self-only，保证老库与脏数据下读路径可用。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/hierarchy"
)

// expandTenantIDScope 返回 tenantID 的可读租户作用域（self 在首位；self 为空返回 nil）。
func expandTenantIDScope(self string) []string {
	if self == "" {
		return nil
	}
	parent := map[string]string{}
	if links := dal.ListTenantParentLinks(); links != nil {
		if pm, err := hierarchy.BuildParentMap(links); err == nil {
			parent = pm
		}
	}
	scope, err := hierarchy.Scope(self, parent)
	if err != nil {
		scope = nil
	}
	return append([]string{self}, scope...)
}

// InheritedAuthorityRoles 返回层级继承后的有效角色集（RBAC 继承接缝）。
// 语义：默认保留自身 authority；TENANT_ADMIN/SYS_ADMIN 且存在祖先租户时，
//
//	追加 "<authority>@<ancestorTenantID>" 域角色（供未来 tenant-qualified 策略按域授权）。
//
// 说明：当前 Casbin 角色策略为全局角色名（TENANT_ADMIN 等），同角色天然跨租户生效，
//
//	无需展开即实现"子继承父策略"；此函数为 tenant-qualified 策略预留最小接缝并配套单测。
func InheritedAuthorityRoles(authority, tenantSelf string) []string {
	if authority == "TENANT_ADMIN" || authority == "SYS_ADMIN" {
		if tenantSelf != "" {
			parent := map[string]string{}
			if links := dal.ListTenantParentLinks(); links != nil {
				if pm, err := hierarchy.BuildParentMap(links); err == nil {
					parent = pm
				}
			}
			if anc, err := hierarchy.Ancestors(tenantSelf, parent); err == nil {
				return inheritedAuthorityRolesFor(authority, anc)
			}
		}
	}
	return []string{authority}
}

// inheritedAuthorityRolesFor 纯展开：base role + role@ancestor（供测试注入祖先链）。
func inheritedAuthorityRolesFor(authority string, ancestors []string) []string {
	out := []string{authority}
	for _, a := range ancestors {
		out = append(out, authority+"@"+a)
	}
	return out
}
