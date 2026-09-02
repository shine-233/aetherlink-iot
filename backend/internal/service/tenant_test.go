// 文件用途：租户客户层级（ROADMAP C2）的纯逻辑单测——不依赖数据库。
// 核心逻辑：锁定 BFS 层级范围解析、scope 归属判定与创建租户的入参守卫语义。
// 关键注意事项：DB 路径（dal.tenant.go）由集成/E2E 覆盖；此处仅覆盖无 DB 的
//              纯函数与服务层守卫，保证语义在迁移/重构后不被静默改动。
package service

import (
	"sort"
	"testing"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestResolveTenantScopeFromMap(t *testing.T) {
	// 树：A → {B, D}，B → {E}（无环，干净语义）。
	children := map[string][]string{
		"A": {"B", "D"},
		"B": {"E"},
	}

	cases := []struct {
		name string
		root string
		want []string
	}{
		{name: "root includes self and descendants", root: "A", want: []string{"A", "B", "D", "E"}},
		{name: "middle node", root: "B", want: []string{"B", "E"}},
		{name: "leaf node", root: "D", want: []string{"D"}},
		{name: "unregistered root degrades to self", root: "ZZZ", want: []string{"ZZZ"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dal.ResolveTenantScopeFromMap(children, tc.root)
			sort.Strings(got)
			sort.Strings(tc.want)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveTenantScopeFromMapCycleGuarded(t *testing.T) {
	// 环：E 指回 A（B → E → A → B）。BFS 必须正常终止而非死循环，
	// 且沿有向边可达的节点（B,E,A,D）全部入范围——seen 防护仅防环，不阻断祖先。
	cycled := map[string][]string{
		"A": {"B", "D"},
		"B": {"E"},
		"E": {"A"},
	}
	got := dal.ResolveTenantScopeFromMap(cycled, "B")
	want := []string{"B", "E", "A", "D"}
	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got)
}

func TestResolveTenantScopeFromMapEmptyGraph(t *testing.T) {
	got := dal.ResolveTenantScopeFromMap(nil, "x")
	assert.Equal(t, []string{"x"}, got)
}

func TestTenantIDInScope(t *testing.T) {
	scope := []string{"root", "child-a", "child-b"}
	assert.True(t, tenantIDInScope("root", scope))
	assert.True(t, tenantIDInScope("child-b", scope))
	assert.False(t, tenantIDInScope("other", scope))
	assert.False(t, tenantIDInScope("", scope))
}

func TestAssertTenantManageability(t *testing.T) {
	sysAdmin := &utils.UserClaims{Authority: "SYS_ADMIN"}

	// SYS_ADMIN 全量放行（不触 DB）。
	assert.NoError(t, assertTenantManageability(sysAdmin, "any-tenant"))

	// nil claims 拒绝。
	err := assertTenantManageability(nil, "root")
	assertCode(t, err, errcode.CodeNoPermission)

	// 空 tenant claims 拒绝（requireNonEmptyTenantID 提前分支，不触 DB）。
	emptyTenant := &utils.UserClaims{Authority: "TENANT_ADMIN", TenantID: ""}
	err = assertTenantManageability(emptyTenant, "root")
	assertCode(t, err, errcode.CodeNoPermission)
	// 带非空 tenantID 的守卫会触 DB（TenantScope），本单测不覆盖该路径。
}

func TestCreateTenantValidation(t *testing.T) {
	sysAdmin := &utils.UserClaims{Authority: "SYS_ADMIN"}
	tenantAdmin := &utils.UserClaims{Authority: "TENANT_ADMIN", TenantID: "root"}

	// 空 name → 参数错误。
	_, err := (&TenantService{}).CreateTenant(&model.CreateTenantReq{Name: "  "}, sysAdmin)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok, "expected *errcode.Error, got %T", err)
	assert.Equal(t, 100005, appErr.Code, "empty name should be rejected as field-error")

	// 非超管创建根租户（parent 为空）→ 无权限（不触 DB）。
	_, err = (&TenantService{}).CreateTenant(&model.CreateTenantReq{Name: "child"}, tenantAdmin)
	assertCode(t, err, errcode.CodeNoPermission)

	// 超管创建根租户：入参合法但写库路径不在此单测范围。
	// TENANT_ADMIN 指定 parent 会走 DB scope，本单测不覆盖。
}

// assertCode 断言错误为 *errcode.Error 且错误码匹配。
func assertCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	if !assert.Error(t, err, "expected error") {
		return
	}
	appErr, ok := err.(*errcode.Error)
	if !assert.True(t, ok, "expected *errcode.Error, got %T", err) {
		return
	}
	assert.Equal(t, wantCode, appErr.Code)
}