// 文件用途：租户层级作用域展开（ROADMAP C2）——服务层公共 helper。
// 核心逻辑：由 tenants.parent_tenant_id 链接（dal.ListTenantParentLinks）经 hierarchy.ScopeDown
//
//	计算 self∪子孙（自上而下：总部/父级可下钻查看自身与全部子孙，子级租户仅自身）；
//	链接不可用/异常时回退 self-only，保证老库与脏数据下读路径可用。
//
// RBAC 继承接缝：InheritedAuthorityRoles 为 tenant-qualified 策略预留"角色@子孙租户"展开。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/hierarchy"
)

// tenantParentLinks 读取 tenants.parent_tenant_id 映射（失败返回空 map，由调用方回退 self-only）。
func tenantParentLinks() map[string]string {
	parent := map[string]string{}
	if links := dal.ListTenantParentLinks(); links != nil {
		if pm, err := hierarchy.BuildParentMap(links); err == nil {
			return pm
		}
	}
	return parent
}

// expandTenantIDScope 返回 tenantID 的自上而下可读租户作用域（self+子孙；self 为空返回 nil）。
func expandTenantIDScope(self string) []string {
	if self == "" {
		return nil
	}
	scope, err := hierarchy.ScopeDown(self, tenantParentLinks())
	if err != nil {
		scope = nil
	}
	return scope
}

// InheritedAuthorityRoles 返回层级可见的角色集（RBAC 继承接缝，自上而下）。
// 语义：默认保留自身 authority；TENANT_ADMIN/SYS_ADMIN 且存在子孙租户时，
//
//	追加 "<authority>@<descendantTenantID>" 域角色（供 tenant-qualified 策略对子树授权）。
//
// 说明：当前 Casbin 角色策略为全局角色名（TENANT_ADMIN 等），同角色天然跨租户生效；
//
//	总部/父级管理员对子租户资源的实际可见性由数据作用域（ScopeDown）保证，角色集展开为其策略预留。
func InheritedAuthorityRoles(authority, tenantSelf string) []string {
	if authority == "TENANT_ADMIN" || authority == "SYS_ADMIN" {
		if tenantSelf != "" {
			if desc, err := hierarchy.Descendants(tenantSelf, tenantParentLinks()); err == nil {
				return inheritedRoleMarkersFor(authority, desc)
			}
		}
	}
	return []string{authority}
}

// inheritedRoleMarkersFor 纯展开：base role + role@descendant（供测试注入子树链）。
func inheritedRoleMarkersFor(authority string, descendants []string) []string {
	out := []string{authority}
	for _, d := range descendants {
		out = append(out, authority+"@"+d)
	}
	return out
}
