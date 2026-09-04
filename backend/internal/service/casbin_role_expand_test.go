// 文件用途：验证 C2 RBAC 角色集继承（②）的授权语义——子租户角色在直接授权未命中时，
// 可复用其所属租户祖先链（Scope）上租户角色的 URL 策略；且绝不因同级/旁支租户、系统角色或
// 扩展源故障放大权限。全部用内存 Enforcer + 假 RoleExpander，不依赖数据库。
// 关键注意事项：本文件通过覆盖 roleExpanderOverride 隔离默认 DB 实现；每个用例 t.Cleanup
// 还原，避免污染同包其他 casbin 测试。
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	global "aetherlink-iot/backend/pkg/global"
)

// fakeExpander 以内存表实现 RoleExpander：角色归属、租户树（tenant→parent），
// 祖先链按 parent 链现场推导，模拟真实 tenants/roles 结构而不触碰 DB。
type fakeExpander struct {
	roleTenant   map[string]string // roleID → tenantID（""=系统级角色）
	tenantParent map[string]string // tenantID → parentTenantID（缺省/""=根）
}

func (f *fakeExpander) RoleTenant(_ context.Context, roleID string) (string, bool, error) {
	t, ok := f.roleTenant[roleID]
	if !ok || t == "" {
		return "", false, nil
	}
	return t, true, nil
}

func (f *fakeExpander) Scope(_ context.Context, tenantID string) ([]string, error) {
	chain := []string{tenantID}
	cur := tenantID
	for {
		parent, ok := f.tenantParent[cur]
		if !ok || parent == "" {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	return chain, nil
}

func (f *fakeExpander) RolesOfTenants(_ context.Context, tenantIDs []string) ([]string, error) {
	hit := make(map[string]struct{})
	for _, t := range tenantIDs {
		for role, tenant := range f.roleTenant {
			if tenant == t {
				hit[role] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(hit))
	for role := range hit {
		out = append(out, role)
	}
	return out, nil
}

// setRoleExpander 覆盖扩展源并在用例结束时还原。
func setRoleExpander(t *testing.T, ex RoleExpander) {
	t.Helper()
	old := roleExpanderOverride
	roleExpanderOverride = ex
	t.Cleanup(func() { roleExpanderOverride = old })
}

// errExpander 让 RoleTenant 恒报错，用于验证扩展源故障时绝不放大权限。
type errExpander struct{ RoleExpander }

func (errExpander) RoleTenant(context.Context, string) (string, bool, error) {
	return "", false, errors.New("simulated expander failure")
}

// seedExpandFixture 构造集团 G → 分部 D → 子租户 C 的三层租户树：
// roleC1 属 tenant-C 并拥有 /api/child 策略且绑定 userC；roleD1 属 tenant-D 拥有 /api/div；
// roleG1 属 tenant-G 拥有 /api/group；roleSys 为系统级角色（无租户）。
func seedExpandFixture(t *testing.T) *fakeExpander {
	t.Helper()
	setupTestCasbinEnforcer()

	ex := &fakeExpander{
		roleTenant: map[string]string{
			"roleC1":  "tenant-C",
			"roleD1":  "tenant-D",
			"roleG1":  "tenant-G",
			"roleSys": "", // 系统级角色：无租户归属
		},
		tenantParent: map[string]string{
			"tenant-D": "tenant-G",
			"tenant-C": "tenant-D",
		},
	}
	setRoleExpander(t, ex)

	c := &Casbin{}
	require.True(t, c.AddRolesToUser("userC", []string{"roleC1", "roleSys"}))
	require.True(t, c.AddFunctionToRole("roleC1", []string{"/api/child"}))
	require.True(t, c.AddFunctionToRole("roleD1", []string{"/api/div"}))
	require.True(t, c.AddFunctionToRole("roleG1", []string{"/api/group"}))
	return ex
}

func TestVerifyDirectAllowUnaffectedByExpansion(t *testing.T) {
	seedExpandFixture(t)
	c := &Casbin{}
	// 直接命中（用户自身角色策略）仍由快路径放行，与是否启用扩展无关。
	assert.True(t, c.Verify("userC", "/api/child"))
}

func TestVerifyInheritsAncestorRolePolicy(t *testing.T) {
	seedExpandFixture(t)
	c := &Casbin{}

	// userC 未绑定 /api/div：直接 Enforce(userC) 必须拒绝 —— 放行只能来自继承扩展。
	allowedDirect, err := global.CasbinEnforcer.Enforce("userC", "/api/div", "allow")
	require.NoError(t, err)
	assert.False(t, allowedDirect, "userC 直接授权必须拒绝，证明下述放行来自祖先角色继承")
	assert.True(t, c.Verify("userC", "/api/div"), "子租户角色应继承祖先租户 D 的角色策略")

	// 跨两层：C → D → G，roleG1 的 /api/group 也应对 userC 放行。
	assert.True(t, c.Verify("userC", "/api/group"), "子租户角色应跨两层继承集团租户角色策略")
}

func TestVerifyInheritanceDoesNotLeakSelfTenantUnboundRoles(t *testing.T) {
	seedExpandFixture(t)
	ex := roleExpanderOverride.(*fakeExpander)
	// 同租户 C 下另一角色 roleC2（拥有 /api/sibling）——非 userC 绑定，不应借继承放行。
	ex.roleTenant["roleC2"] = "tenant-C"
	c := &Casbin{}
	require.True(t, c.AddFunctionToRole("roleC2", []string{"/api/sibling"}))

	assert.False(t, c.Verify("userC", "/api/sibling"),
		"同一租户内未绑定角色不得因继承被放行（继承只沿祖先租户方向补充）")
}

func TestVerifyInheritanceDoesNotLeakSiblingTenantRoles(t *testing.T) {
	seedExpandFixture(t)
	ex := roleExpanderOverride.(*fakeExpander)
	// 旁支租户 S（同样挂在集团 G 下，但不在 C 的祖先链上）的角色 roleS1 拥有 /api/sibling-s。
	ex.tenantParent["tenant-S"] = "tenant-G"
	ex.roleTenant["roleS1"] = "tenant-S"
	c := &Casbin{}
	require.True(t, c.AddFunctionToRole("roleS1", []string{"/api/sibling-s"}))

	assert.False(t, c.Verify("userC", "/api/sibling-s"),
		"旁支租户（不在祖先链上）的角色策略不得被继承")
}

func TestVerifyInheritanceSkipsSystemRoles(t *testing.T) {
	seedExpandFixture(t)
	c := &Casbin{}
	// userC 同时绑定系统级角色 roleSys（无租户归属）→ 不得引发任何扩展，也不影响自身授权。
	assert.True(t, c.Verify("userC", "/api/child"), "系统角色绑定不影响自身角色直接授权")
}

func TestVerifyNoExpansionWhenNoExpander(t *testing.T) {
	seedExpandFixture(t)
	c := &Casbin{}
	// 关闭扩展源（模拟树未装配/旧部署）→ 继承失效，跨租户 URL 拒绝，与 C2 前行为一致。
	setRoleExpander(t, nil)

	assert.False(t, c.Verify("userC", "/api/div"))
	assert.True(t, c.Verify("userC", "/api/child"), "自身角色直接授权不依赖扩展源")
}

func TestVerifyNoExpansionForUnboundUser(t *testing.T) {
	seedExpandFixture(t)
	c := &Casbin{}
	// 未绑定任何角色的用户不得因扩展获得任何 URL 权限。
	assert.False(t, c.Verify("user-nobody", "/api/child"))
	assert.False(t, c.Verify("user-nobody", "/api/div"))
	assert.False(t, c.Verify("user-nobody", "/api/group"))
}

func TestVerifyInheritanceIgnoresExpanderErrors(t *testing.T) {
	seedExpandFixture(t)
	// RoleTenant 恒报错 → 所有绑定角色跳过继承，绝不放大权限。
	setRoleExpander(t, errExpander{})
	c := &Casbin{}

	assert.False(t, c.Verify("userC", "/api/div"), "扩展源故障必须拒绝而非放行")
	assert.True(t, c.Verify("userC", "/api/child"), "自身租户直接授权不受扩展源故障影响")
}

func TestVerifyInheritedRequiresRealAncestry(t *testing.T) {
	seedExpandFixture(t)
	ex := roleExpanderOverride.(*fakeExpander)
	c := &Casbin{}
	require.True(t, c.AddRolesToUser("userD", []string{"roleD1"}))
	// 断开 D 的父链：集团 G 不再位于 D 的祖先链上。
	delete(ex.tenantParent, "tenant-D")

	assert.False(t, c.Verify("userD", "/api/group"), "祖先链断开后不得再继承集团角色策略")
	assert.True(t, c.Verify("userD", "/api/div"), "自身角色策略不受影响")
}

// TestNoExpanderWithoutTenantTree 只读契约：global.TenantTree 未装配时默认扩展源必须为 nil（零扩展）。
func TestNoExpanderWithoutTenantTree(t *testing.T) {
	old := global.TenantTree
	t.Cleanup(func() { global.TenantTree = old })
	global.TenantTree = nil
	setRoleExpander(t, nil)

	assert.Nil(t, global.TenantTree)
	assert.Nil(t, currentRoleExpander(), "树未装配时默认扩展源必须为 nil（零扩展）")
}
