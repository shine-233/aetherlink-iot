// 文件用途：实现 C2 RBAC 角色集继承（②）——Casbin 直接授权未命中时，把评估角色集扩展为
// "绑定角色所属租户的祖先租户角色"，使子租户角色复用父（集团）租户已授予的 URL 策略。
// 核心逻辑：verifyInherited 只读拉取用户已绑定角色 → 角色所属租户 → 租户树祖先链 → 祖先租户下
// 的角色集，逐个以该角色为 subject 探测 URL 策略；任一命中即放行。所有事实源经 RoleExpander
// 抽象注入，默认 DB 实现依赖 global.TenantTree + dal，未装配时零扩展（与旧行为一致）。
// 关键注意事项：①仅走"直接拒绝后"的兜底路径，直接放行的快路径零改动、零额外延迟；
// ②纯只读，不改 g/p 策略，无缓存污染；③祖先租户角色全部纳入是文档语义（子继承父），
// 数据面隔离由各 DAL 的 Scope/Descendants 过滤负责，URL 级放行不等于跨租户取数。
// 重构建议：拒绝路径逐角色探测为 O(角色数×祖先数) DB 读；若成为热点可加角色→租户短缓存，
// 或改为登录时预展开并写 g 一次性链接（届时注意与 RemoveUserAndRole 的联动失效）。
package service

import (
	"context"

	dal "aetherlink-iot/backend/internal/dal"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// RoleExpander 提供 RBAC 角色集扩展所需的两个只读事实源：
// 租户祖先链（tenantree/hierarchy）与"租户 → 角色"归属。
type RoleExpander interface {
	// Scope 返回 tenantID 的祖先作用域 = {self} ∪ 祖先（近→远，self 恒在首位）。
	// 实现必须容忍输入租户未登记/树不可用：返回仅含自身的链，或返回错误由调用方跳过。
	Scope(ctx context.Context, tenantID string) ([]string, error)

	// RoleTenant 返回 roleID 所属租户。系统级角色（tenant_id 为空）返回 ok=false。
	RoleTenant(ctx context.Context, roleID string) (tenantID string, ok bool, err error)

	// RolesOfTenants 返回归属于任一指定租户的角色 ID 列表；空输入返回空切片、无错误。
	RolesOfTenants(ctx context.Context, tenantIDs []string) ([]string, error)
}

// roleExpanderOverride 仅供测试注入（同包测试可见）；nil 时走默认 DB 实现。
var roleExpanderOverride RoleExpander

// currentRoleExpander 返回当前生效的扩展源。默认 DB 实现仅在 global.TenantTree 装配后启用；
// 树未装配（如 tenants 表尚未建、单测环境）时返回 nil = 不扩展，保持旧授权行为。
func currentRoleExpander() RoleExpander {
	if roleExpanderOverride != nil {
		return roleExpanderOverride
	}
	if global.TenantTree == nil {
		return nil
	}
	return dbRoleExpander{}
}

// dbRoleExpander 是生产默认扩展源：租户祖先链走共享租户树，角色归属走 roles 表。
type dbRoleExpander struct{}

// Scope 委托 global.TenantTree；树缺失或查询失败时退化为仅含自身（等价于无层级信息）。
func (dbRoleExpander) Scope(ctx context.Context, tenantID string) ([]string, error) {
	tree := global.TenantTree
	if tree == nil {
		return []string{tenantID}, nil
	}
	chain, err := tree.Scope(ctx, tenantID)
	if err != nil {
		logrus.WithError(err).Debugf("tenant tree scope failed, degrade to self-only: tenant=%q", tenantID)
		return []string{tenantID}, nil
	}
	return chain, nil
}

// RoleTenant 查询 roles 表取角色归属租户；DB 未就绪/角色不存在/系统级角色均返回 ok=false。
func (dbRoleExpander) RoleTenant(ctx context.Context, roleID string) (string, bool, error) {
	if roleID == "" || global.DB == nil {
		return "", false, nil
	}
	role, err := dal.GetRoleByID(roleID)
	if err != nil {
		return "", false, err
	}
	if role.TenantID == nil || *role.TenantID == "" {
		return "", false, nil
	}
	return *role.TenantID, true, nil
}

// RolesOfTenants 委托 dal 查询；错误上抛由调用方跳过（不阻断鉴权主流程）。
func (dbRoleExpander) RolesOfTenants(ctx context.Context, tenantIDs []string) ([]string, error) {
	if len(tenantIDs) == 0 {
		return nil, nil
	}
	return dal.GetRoleIDsByTenants(tenantIDs)
}

// verifyInherited 是 C2 RBAC 继承的评估主体：仅当直接 Enforce(user,url) 拒绝后才调用。
// 语义：对用户每个已绑定角色，取其所属租户的祖先链（不含自身租户），收集所有祖先租户的角色，
// 逐个以"角色"为 subject 重新 Enforce；任一放行即返回 true。扩展失败一律跳过，绝不放大权限。
func (c *Casbin) verifyInherited(user string, url string) bool {
	ex := currentRoleExpander()
	if ex == nil {
		return false
	}
	boundRoles, ok := c.GetRoleFromUser(user)
	if !ok || len(boundRoles) == 0 {
		return false
	}
	if global.CasbinEnforcer == nil {
		return false
	}

	ctx := context.Background()
	seenTenant := make(map[string]struct{})
	seenRole := make(map[string]struct{})

	tryEnforce := func(roleID string) bool {
		if roleID == "" {
			return false
		}
		if _, dup := seenRole[roleID]; dup {
			return false
		}
		seenRole[roleID] = struct{}{}
		allowed, err := global.CasbinEnforcer.Enforce(roleID, url, "allow")
		if err == nil && allowed {
			return true
		}
		return false
	}

	for _, bound := range boundRoles {
		tenant, okRole, err := ex.RoleTenant(ctx, bound)
		if err != nil {
			logrus.WithError(err).Debugf("role tenant lookup failed, skip role: role=%q", bound)
			continue
		}
		if !okRole || tenant == "" {
			// 系统级角色/无租户角色：无祖先链可继承，跳过。
			continue
		}
		chain, err := ex.Scope(ctx, tenant)
		if err != nil {
			continue
		}
		for _, ancestorTenant := range chain {
			if ancestorTenant == tenant {
				// 自身租户的角色集合由"用户已绑定角色"覆盖；此处只补祖先租户角色。
				continue
			}
			if _, dup := seenTenant[ancestorTenant]; dup {
				continue
			}
			seenTenant[ancestorTenant] = struct{}{}
			ancestorRoles, err := ex.RolesOfTenants(ctx, []string{ancestorTenant})
			if err != nil || len(ancestorRoles) == 0 {
				continue
			}
			for _, ancestorRole := range ancestorRoles {
				if tryEnforce(ancestorRole) {
					return true
				}
			}
		}
	}
	return false
}
