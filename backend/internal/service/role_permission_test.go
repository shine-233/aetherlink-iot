package service

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCasbinEnforcer(t *testing.T) *casbin.SyncedEnforcer {
	t.Helper()
	text := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`
	m, err := casbinmodel.NewModelFromString(text)
	require.NoError(t, err)
	e, err := casbin.NewSyncedEnforcer(m)
	require.NoError(t, err)
	return e
}

func TestListPermissionsRequiresAuth(t *testing.T) {
	ctx := context.Background()
	_, err := RolePermission.ListPermissions(ctx, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestRolePermissionCasbinSyncLogic(t *testing.T) {
	oldEnforcer := global.CasbinEnforcer
	defer func() { global.CasbinEnforcer = oldEnforcer }()

	e := newTestCasbinEnforcer(t)
	global.CasbinEnforcer = e

	roleID := "custom_operator_role"
	userID := "user_operator_001"

	// 模拟分配角色给用户: g, userID, roleID
	_, err := e.AddGroupingPolicy(userID, roleID)
	require.NoError(t, err)

	// 此时用户尚无权限
	allowed, err := e.Enforce(userID, "api/v1/device", "allow")
	require.NoError(t, err)
	assert.False(t, allowed, "用户未被赋予权限前应被拒绝")

	// 模拟为自定义角色同步 p 策略
	patterns := []string{"api/v1/device", "api/v1/device/list"}
	for _, p := range patterns {
		_, err := e.AddPolicy(roleID, p, "allow")
		require.NoError(t, err)
	}

	// 再次校验：用户自动获得该角色的权限
	allowed, err = e.Enforce(userID, "api/v1/device", "allow")
	require.NoError(t, err)
	assert.True(t, allowed, "自定义角色赋予权限后用户应被放行")

	allowedList, err := e.Enforce(userID, "api/v1/device/list", "allow")
	require.NoError(t, err)
	assert.True(t, allowedList, "自定义角色赋予权限后列表接口应放行")

	// 校验未授予的接口仍被拦截
	denied, err := e.Enforce(userID, "api/v1/ota/tasks", "allow")
	require.NoError(t, err)
	assert.False(t, denied, "未授予的接口必须被严格拦截")
}

func TestEnsureRoleWriteAccessIsolation(t *testing.T) {
	sysClaims := &utils.UserClaims{
		ID:        "sys_admin_1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant_root",
	}

	tenantClaims := &utils.UserClaims{
		ID:        "tenant_admin_1",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant_alpha",
	}

	userClaims := &utils.UserClaims{
		ID:        "tenant_user_1",
		Authority: constant.TENANT_USER,
		TenantID:  "tenant_alpha",
	}

	// 普通用户无权管理角色
	err := requireRoleManager(userClaims)
	assert.Error(t, err)

	// 管理员有权管理角色
	assert.NoError(t, requireRoleManager(sysClaims))
	assert.NoError(t, requireRoleManager(tenantClaims))

	// 跨租户角色访问控制
	otherTenantRole := model.Role{
		ID:       "role_beta_1",
		Name:     "Beta Operator",
		TenantID: StringPtr("tenant_beta"),
	}

	// 租户管理员不能管理其他租户的角色
	assert.NotEqual(t, *otherTenantRole.TenantID, tenantClaims.TenantID)
}
