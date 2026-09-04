// 文件用途：验证 Casbin 权限策略服务的加载和匹配行为。
// 核心逻辑：构造策略文件或内存策略，断言角色、路径和方法维度的授权结果。
// 关键注意事项：权限测试不能依赖生产策略残留，路径归一化和角色继承变化都可能造成越权。
// 重构建议：增加隔离的策略构造器，补齐拒绝优先级、策略缺失和多租户角色边界。
package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

func TestProductionCasbinConfigHasNoHardcodedSubjectBypass(t *testing.T) {
	configPath := filepath.Join("..", "..", "configs", "casbin.conf")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	config := string(data)
	for _, forbidden := range []string{
		"admin" + "@aetherlink.local",
		"super" + "@super.cn",
		"|| " + "r." + "sub ==",
	} {
		if strings.Contains(config, forbidden) {
			t.Fatalf("casbin.conf contains hardcoded subject bypass %q", forbidden)
		}
	}
}

func setupTestCasbinEnforcer() {
	// Use an in-memory model for testing
	modelStr := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`
	casbinModel, err := model.NewModelFromString(modelStr)
	if err != nil {
		panic("failed to build test casbin model: " + err.Error())
	}

	e, err := casbin.NewEnforcer(casbinModel)
	if err != nil {
		panic("failed to create test casbin enforcer: " + err.Error())
	}
	global.CasbinEnforcer = e
}

func TestCasbinAddFunctionToRole(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	result := c.AddFunctionToRole("role-admin", []string{"/api/users", "/api/devices"})
	assert.True(t, result, "AddFunctionToRole should return true on success")

	// Verify the policies were added
	functions, ok := c.GetFunctionFromRole("role-admin")
	assert.True(t, ok)
	assert.Contains(t, functions, "/api/users")
	assert.Contains(t, functions, "/api/devices")
}

func TestCasbinAddFunctionToRoleEmpty(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	result := c.AddFunctionToRole("role-empty", []string{})
	assert.True(t, result, "AddFunctionToRole with empty functions currently returns true because Casbin treats empty policy batches as a no-op success")

	functions, ok := c.GetFunctionFromRole("role-empty")
	assert.True(t, ok)
	assert.Empty(t, functions, "empty function batch should not create any persisted allow policy")
}

func TestCasbinGetFunctionFromRole(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-test", []string{"/api/alarms", "/api/scenes"})

	functions, ok := c.GetFunctionFromRole("role-test")
	assert.True(t, ok)
	assert.Len(t, functions, 2)
	assert.Contains(t, functions, "/api/alarms")
	assert.Contains(t, functions, "/api/scenes")
}

func TestCasbinGetFunctionFromNonExistentRole(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	functions, ok := c.GetFunctionFromRole("non-existent-role")
	assert.True(t, ok)
	assert.NotNil(t, functions)
	assert.Empty(t, functions)
	assert.False(t, c.HasRole("non-existent-role"), "missing role should not appear in grouping lookups")
}

func TestCasbinRemoveRoleAndFunction(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	require.True(t, c.AddFunctionToRole("role-to-remove", []string{"/api/test"}))
	require.True(t, c.AddRolesToUser("user-with-role", []string{"role-to-remove"}))

	// Verify the user can reach the URL before the role policy is removed.
	functions, _ := c.GetFunctionFromRole("role-to-remove")
	assert.NotEmpty(t, functions)
	assert.True(t, c.Verify("user-with-role", "/api/test"))

	result := c.RemoveRoleAndFunction("role-to-remove")
	assert.True(t, result)

	functions, _ = c.GetFunctionFromRole("role-to-remove")
	assert.Empty(t, functions)
	assert.False(t, c.Verify("user-with-role", "/api/test"), "removing role policies must revoke URL access for users still bound to that role")
}

func TestCasbinAddRolesToUser(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-admin", []string{"/api/users"})
	_ = c.AddFunctionToRole("role-viewer", []string{"/api/dashboard"})

	result := c.AddRolesToUser("user-1", []string{"role-admin", "role-viewer"})
	assert.True(t, result, "AddRolesToUser should return true on success")

	roles, ok := c.GetRoleFromUser("user-1")
	assert.True(t, ok)
	assert.Contains(t, roles, "role-admin")
	assert.Contains(t, roles, "role-viewer")
}

func TestCasbinAddRolesToUserEmpty(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	result := c.AddRolesToUser("user-empty", []string{})
	assert.True(t, result, "AddRolesToUser with empty roles currently returns true because Casbin treats empty grouping batches as a no-op success")

	roles, ok := c.GetRoleFromUser("user-empty")
	assert.True(t, ok)
	assert.Empty(t, roles, "empty role batch should not create any persisted user-role mapping")
}

func TestCasbinGetRoleFromUser(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-a", []string{"/api/a"})
	_ = c.AddRolesToUser("user-x", []string{"role-a"})

	roles, ok := c.GetRoleFromUser("user-x")
	assert.True(t, ok)
	assert.Contains(t, roles, "role-a")
}

func TestCasbinGetRoleFromNonExistentUser(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	roles, ok := c.GetRoleFromUser("non-existent-user")
	assert.True(t, ok)
	assert.Empty(t, roles)
	assert.False(t, c.Verify("non-existent-user", "/api/verify"), "missing user must not verify against any protected URL")
}

func TestCasbinRemoveUserAndRole(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-rm", []string{"/api/rm"})
	_ = c.AddRolesToUser("user-rm", []string{"role-rm"})

	// Verify role was assigned
	roles, _ := c.GetRoleFromUser("user-rm")
	assert.NotEmpty(t, roles)

	// Remove user-role mapping
	result := c.RemoveUserAndRole("user-rm")
	assert.True(t, result)

	// Verify role was removed
	roles, _ = c.GetRoleFromUser("user-rm")
	assert.Empty(t, roles)
	assert.False(t, c.Verify("user-rm", "/api/rm"), "removed user-role mapping must revoke URL access")
}

func TestCasbinVerify(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-verify", []string{"/api/verify", "/api/data"})
	_ = c.AddRolesToUser("user-verify", []string{"role-verify"})

	tests := []struct {
		name string
		user string
		url  string
		want bool
	}{
		{
			name: "user with matching role and url",
			user: "user-verify",
			url:  "/api/verify",
			want: true,
		},
		{
			name: "user with matching role but wrong url",
			user: "user-verify",
			url:  "/api/forbidden",
			want: false,
		},
		{
			name: "user without any role",
			user: "user-no-role",
			url:  "/api/verify",
			want: false,
		},
		{
			name: "empty user",
			user: "",
			url:  "/api/verify",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Verify(tt.user, tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCasbinVerifyRequiresAssignedRoleAndAllowPolicy(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	require.True(t, c.AddFunctionToRole("role-device-admin", []string{"/api/devices", "/api/device/detail"}))
	require.True(t, c.AddFunctionToRole("role-alarm-viewer", []string{"/api/alarms"}))
	require.True(t, c.AddRolesToUser("user-device-admin", []string{"role-device-admin"}))

	if !c.Verify("user-device-admin", "/api/devices") {
		t.Fatal("assigned device-admin role should allow /api/devices")
	}
	if !c.Verify("user-device-admin", "/api/device/detail") {
		t.Fatal("assigned device-admin role should allow /api/device/detail")
	}
	if c.Verify("user-device-admin", "/api/alarms") {
		t.Fatal("user without alarm-viewer role should not access /api/alarms")
	}
	if c.Verify("user-without-role", "/api/devices") {
		t.Fatal("user without any role should not access role protected URL")
	}
}

func TestCasbinHasRole(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}
	_ = c.AddFunctionToRole("role-exists", []string{"/api/test"})
	_ = c.AddRolesToUser("user-has-role", []string{"role-exists"})

	assert.True(t, c.HasRole("role-exists"), "HasRole should return true for existing role")
	assert.False(t, c.HasRole("role-non-existent"), "HasRole should return false for non-existent role")
}

func TestCasbinGetUrl(t *testing.T) {
	setupTestCasbinEnforcer()

	c := &Casbin{}

	// Add a URL resource to g2 grouping
	_, err := global.CasbinEnforcer.AddNamedGroupingPolicies("g2", [][]string{
		{"/api/resource-1", "resource-group-1"},
	})
	require.NoError(t, err)

	assert.True(t, c.GetUrl("/api/resource-1"), "GetUrl should return true for registered URL")
	assert.False(t, c.GetUrl("/api/not-registered"), "GetUrl should return false for unregistered URL")
}

func TestCasbinVerifyFailsClosedWhenEnforceErrors(t *testing.T) {
	// 请求定义含 4 个 token 的模型会让三参 Enforce 直接报错（invalid request size），
	// 用于验证 Verify 在 Enforce 出错时按拒绝处理，而不是吞掉错误后放行。
	modelStr := `
[request_definition]
r = sub, obj, act, ctx

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`
	casbinModel, err := model.NewModelFromString(modelStr)
	require.NoError(t, err)
	e, err := casbin.NewEnforcer(casbinModel)
	require.NoError(t, err)

	oldEnforcer := global.CasbinEnforcer
	t.Cleanup(func() { global.CasbinEnforcer = oldEnforcer })
	global.CasbinEnforcer = e

	c := &Casbin{}
	assert.False(t, c.Verify("user-fail-closed", "/api/verify"),
		"Enforce 返回错误时必须 fail-closed 拒绝")
}

// setupPatternCasbinEnforcer 与 configs/casbin.conf 的双通道 matcher 保持一致
// （g2 分组 + urlPatternMatch 锚定模式，函数经 AddFunction 注入，与生产同源）。
func setupPatternCasbinEnforcer(t *testing.T) {
	t.Helper()
	modelStr := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (g2(r.obj, p.obj) || urlPatternMatch(r.obj, p.obj)) && r.act == p.act
`
	casbinModel, err := model.NewModelFromString(modelStr)
	require.NoError(t, err)
	e, err := casbin.NewEnforcer(casbinModel)
	require.NoError(t, err)
	// casbin v2.135 的 AddFunction 无返回值（失败以其内部 panic 暴露）。
	e.AddFunction("urlPatternMatch", utils.URLPatternCasbinFunction())

	oldEnforcer := global.CasbinEnforcer
	t.Cleanup(func() { global.CasbinEnforcer = oldEnforcer })
	global.CasbinEnforcer = e
}

func TestGetUrlRecognizesParamRoutePattern(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	c := &Casbin{}
	_, err := global.CasbinEnforcer.AddNamedGroupingPolicies("g2", [][]string{
		{"api/v1/device/:id", "device-resource"},
		{"api/v1/devices", "device-list-resource"},
	})
	require.NoError(t, err)

	assert.True(t, c.GetUrl("api/v1/device/123"), "参数路由请求路径应经锚定模式判定为已登记")
	assert.True(t, c.GetUrl("api/v1/device/:id"), "模式串本身应精确命中")
	assert.True(t, c.GetUrl("api/v1/devices"), "静态路由精确命中不受影响")
	assert.False(t, c.GetUrl("api/v1/devicesXYZ"), "锚定匹配不得子串误命中")
	assert.False(t, c.GetUrl("api/v1/other/123"), "未登记路径不得误判")
}

func TestVerifyEnforcesParamRouteViaPatternPolicy(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	c := &Casbin{}
	_, err := global.CasbinEnforcer.AddNamedGroupingPolicies("g", [][]string{
		{"user-1", "role-1"},
	})
	require.NoError(t, err)
	// p.obj 直接写模式——urlPatternMatch 通道使参数路由可被真实保护
	_, err = global.CasbinEnforcer.AddNamedPolicies("p", [][]string{
		{"role-1", "api/v1/device/:id", "allow"},
		{"role-1", "api/v1/devices", "allow"},
	})
	require.NoError(t, err)

	assert.True(t, c.Verify("user-1", "api/v1/device/123"), "模式策略应放行参数路由请求")
	assert.False(t, c.Verify("user-no-role", "api/v1/device/123"), "无角色用户应被拒绝")
	assert.True(t, c.Verify("user-1", "api/v1/devices"), "静态路由既有语义不受影响")
	assert.False(t, c.Verify("user-1", "api/v1/device/123/secret"), "模式不得越过段边界（越权放大回归）")
}
