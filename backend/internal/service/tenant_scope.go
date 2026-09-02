// 文件用途：租户层级作用域展开（ROADMAP C2）——服务层公共 helper。
// 核心逻辑：由 tenants.parent_tenant_id 链接（dal.ListTenantParentLinks）经 hierarchy.Scope
//   计算 self∪祖先；链接不可用/异常时回退 self-only，保证老库与脏数据下读路径可用。
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
