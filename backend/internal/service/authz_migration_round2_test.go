// 文件用途：第二轮 authz 迁移回归矩阵（open_api_keys / plugin_registry / sys_dict /
// sys_ui_elements / secret / tenant / rdi）。
//
// 核心逻辑：每个迁移点按「角色 × 资源归属租户」笛卡尔展开，既断言允许与否，
// 也断言错误码与自定义消息——消息是接口契约的一部分，迁移不得改写。
//
// 关键注意事项：tenantReadRule 的 Scope 走层级展开（tenantVisibleScope）会读库，
// 故非 SYS_ADMIN 角色在无 DB 时跳过，避免把单元测试变成集成测试。
package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

func migrationErrMessage(err error) string {
	if e, ok := err.(*errcode.Error); ok && e.UseCustomMsg {
		return e.CustomMsg
	}
	return ""
}

// ---- 1. 开放 API key 管理规则（CreateOpenAPIKey / Update / Delete） --------------

func TestOpenAPIKeyManagerRuleRoleMatrix(t *testing.T) {
	cases := []struct {
		role   string
		claims *utils.UserClaims
		tenant string
		allow  bool
	}{
		{"nil", nil, "t1", false},
		{"nil", nil, "t2", false},
		{"SYS_ADMIN/own", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), "t1", true},
		{"SYS_ADMIN/foreign", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), "t2", true},
		{"TENANT_ADMIN/own", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1", true},
		{"TENANT_ADMIN/foreign", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t2", false},
		{"TENANT_USER/own", migrationClaims(constant.TENANT_USER, "t1", "u1"), "t1", false},
		{"TENANT_USER/foreign", migrationClaims(constant.TENANT_USER, "t1", "u1"), "t2", false},
		{"unknown/own", migrationClaims("CUSTOMER", "t1", "c1"), "t1", false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			err := openAPIKeyManagerRule.Check(tc.claims, authz.OfTenant(tc.tenant))
			if (err == nil) != tc.allow {
				t.Fatalf("Check=%v want allow=%v", err, tc.allow)
			}
			if !tc.allow && migrationErrCode(err) != errcode.CodeNoPermission {
				t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
			}
		})
	}
}

// TestOpenAPIKeyCreateGateRoleMatrix 复刻 CreateOpenAPIKey 的两段式校验：
// 先角色、后租户，两者失败时的错误载荷不同（required_role / required_tenant）。
func TestOpenAPIKeyCreateGateRoleMatrix(t *testing.T) {
	for _, role := range migrationRoles {
		for _, reqTenant := range []string{"t1", "t2"} {
			t.Run(role.name+"/"+reqTenant, func(t *testing.T) {
				roleOK := authz.HasRole(role.claims, authz.ManagerRoles...)
				tenantErr := authz.TenantRule("").Check(role.claims, authz.OfTenant(reqTenant))
				// 迁移前：`!roleOK` 与 `非 SYS_ADMIN && reqTenant != claims.TenantID` 两道门。
				wantOK := false
				if c := role.claims; c != nil {
					wantOK = roleOK && (c.Authority == constant.SYS_ADMIN || c.TenantID == reqTenant)
				}
				gotOK := roleOK && tenantErr == nil
				if gotOK != wantOK {
					t.Fatalf("create gate=%v want %v", gotOK, wantOK)
				}
				if !roleOK {
					if migrationErrCode(openAPIKeyRoleDenied(role.claims)) != errcode.CodeNoPermission {
						t.Fatalf("role denial code changed")
					}
					return
				}
				if tenantErr != nil {
					if migrationErrCode(openAPIKeyTenantDenied(reqTenant, role.claims)) != errcode.CodeNoPermission {
						t.Fatalf("tenant denial code changed")
					}
				}
			})
		}
	}
}

// TestOpenAPIKeyListTenantClampRoleMatrix 列表租户钳制：只有租户级角色被钳到自己的租户，
// SYS_ADMIN 保持空（全租户）。
func TestOpenAPIKeyListTenantClampRoleMatrix(t *testing.T) {
	cases := []struct {
		claims *utils.UserClaims
		want   string
	}{
		{migrationClaims(constant.SYS_ADMIN, "t1", "sa"), ""},
		{migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1"},
		{migrationClaims(constant.TENANT_USER, "t1", "u1"), "t1"},
		{migrationClaims("CUSTOMER", "t1", "c1"), ""},
	}
	for _, tc := range cases {
		got := ""
		if authz.HasRole(tc.claims, authz.TenantAdmin, authz.TenantUser) {
			got = tc.claims.TenantID
		}
		if got != tc.want {
			t.Fatalf("clamp=%q want %q", got, tc.want)
		}
	}
}

// ---- 2. 租户服务规则 ------------------------------------------------------------

func TestTenantManageAndCreateRuleRoleMatrix(t *testing.T) {
	cases := []struct {
		name      string
		rule      authz.Rule
		authority string
		allow     bool
	}{
		{"manage/sys", tenantManageRule, constant.SYS_ADMIN, true},
		{"manage/tenant admin", tenantManageRule, constant.TENANT_ADMIN, false},
		{"manage/tenant user", tenantManageRule, constant.TENANT_USER, false},
		{"manage/unknown", tenantManageRule, "CUSTOMER", false},
		{"create/sys", tenantCreateRule, constant.SYS_ADMIN, true},
		{"create/tenant admin", tenantCreateRule, constant.TENANT_ADMIN, true},
		{"create/tenant user", tenantCreateRule, constant.TENANT_USER, false},
		{"create/unknown", tenantCreateRule, "CUSTOMER", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.RequireClaims(migrationClaims(tc.authority, "t1", "u"))
			if (err == nil) != tc.allow {
				t.Fatalf("RequireClaims=%v want allow=%v", err, tc.allow)
			}
			if !tc.allow && migrationErrCode(err) != errcode.CodeNoPermission {
				t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
			}
		})
	}
	if err := tenantManageRule.RequireClaims(nil); err == nil {
		t.Fatal("nil claims must be denied")
	}
	if err := tenantCreateRule.RequireClaims(nil); err == nil {
		t.Fatal("nil claims must be denied")
	}
}

// TestTenantReadRuleRoleMatrix 迁移前语义：SYS_ADMIN 任意租户；其余角色按
// tenantScopeContains(claims.TenantID, target) 判定；越权按 CodeNotFound 屏蔽存在性。
func TestTenantReadRuleRoleMatrix(t *testing.T) {
	if err := tenantReadRule.Check(nil, authz.OfTenant("t1")); migrationErrCode(err) != errcode.CodeNotFound {
		t.Fatalf("nil claims must be masked as not-found: %v", err)
	}
	if err := tenantReadRule.Check(migrationClaims(constant.SYS_ADMIN, "", "sa"), authz.OfTenant("t9")); err != nil {
		t.Fatalf("sys admin reads any tenant: %v", err)
	}
	if global.DB == nil {
		t.Skip("层级作用域展开需要 DB，跳过非 SYS_ADMIN 角色")
	}
	for _, role := range migrationRoles {
		if role.claims == nil || role.claims.Authority == constant.SYS_ADMIN {
			continue
		}
		t.Run(role.name, func(t *testing.T) {
			target := role.claims.TenantID
			if err := tenantReadRule.Check(role.claims, authz.OfTenant(target)); err != nil {
				t.Fatalf("self tenant must be visible: %v", err)
			}
			if err := tenantReadRule.Check(role.claims, authz.OfTenant("definitely-other")); err == nil {
				t.Fatal("foreign tenant must be denied")
			}
		})
	}
}

// ---- 3. 平台管理员 / 租户管理员门禁 ---------------------------------------------

func TestPlatformAdminGateRoleMatrix(t *testing.T) {
	gates := []struct {
		name string
		rule authz.Rule
		want []string // 放行的 authority
		msg  string   // 非空表示 Deny 消息必须逐字保留
	}{
		{"pluginRegistryAdminRule", pluginRegistryAdminRule, []string{constant.SYS_ADMIN}, "plugin registry is platform-admin capability"},
		{"dictAdminRule", dictAdminRule, []string{constant.SYS_ADMIN}, ""},
		{"uiElementsAdminRule", uiElementsAdminRule, []string{constant.SYS_ADMIN}, "no permission to manage ui elements"},
		{"uiElementsViewerRule", uiElementsViewerRule, []string{constant.SYS_ADMIN, constant.TENANT_ADMIN}, "no permission to query ui elements"},
		{"secretRevealRule", secretRevealRule, []string{constant.SYS_ADMIN, constant.TENANT_ADMIN}, "permission denied: cannot reveal secret"},
		{"secretResealRule", secretResealRule, []string{constant.SYS_ADMIN, constant.TENANT_ADMIN}, "permission denied"},
	}
	authorities := []string{constant.SYS_ADMIN, constant.TENANT_ADMIN, constant.TENANT_USER, "CUSTOMER"}
	for _, g := range gates {
		for _, authority := range authorities {
			want := false
			for _, allowed := range g.want {
				if allowed == authority {
					want = true
				}
			}
			t.Run(g.name+"/"+authority, func(t *testing.T) {
				err := g.rule.RequireClaims(migrationClaims(authority, "t1", "u1"))
				if (err == nil) != want {
					t.Fatalf("RequireClaims=%v want allow=%v", err, want)
				}
				if want {
					return
				}
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				if g.msg != "" && migrationErrMessage(err) != g.msg {
					t.Fatalf("message=%q want %q", migrationErrMessage(err), g.msg)
				}
			})
		}
		t.Run(g.name+"/nil", func(t *testing.T) {
			if err := g.rule.RequireClaims(nil); err == nil {
				t.Fatal("nil claims must be denied")
			}
		})
	}
}

// TestPlatformAdminRuleDenialMessages 断言各门禁包装函数迁移后仍返回逐字相同的错误。
func TestPlatformAdminRuleDenialMessages(t *testing.T) {
	cases := []struct {
		name string
		got  error
		msg  string
	}{
		{"requireNotificationServicesAdmin", requireNotificationServicesAdmin(migrationClaims(constant.TENANT_ADMIN, "t1", "ta")), "no permission to manage notification service config"},
		{"requireMessagePushConfigAdmin", requireMessagePushConfigAdmin(migrationClaims(constant.TENANT_ADMIN, "t1", "ta")), "no permission to manage message push config"},
		{"requireServicePluginAdmin", requireServicePluginAdmin(migrationClaims(constant.TENANT_ADMIN, "t1", "ta")), "no permission to manage service plugins"},
		{"requireSystemMonitorAdmin", requireSystemMonitorAdmin(migrationClaims(constant.TENANT_ADMIN, "t1", "ta")), "no permission to query system metrics"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if migrationErrCode(tc.got) != errcode.CodeNoPermission {
				t.Fatalf("code=%d want %d", migrationErrCode(tc.got), errcode.CodeNoPermission)
			}
			if migrationErrMessage(tc.got) != tc.msg {
				t.Fatalf("message=%q want %q", migrationErrMessage(tc.got), tc.msg)
			}
		})
	}
	if requireNotificationServicesAdmin(migrationClaims(constant.SYS_ADMIN, "", "sa")) != nil {
		t.Fatal("sys admin must pass notification config gate")
	}
	if requireMessagePushConfigAdmin(migrationClaims(constant.SYS_ADMIN, "", "sa")) != nil {
		t.Fatal("sys admin must pass push config gate")
	}
	if requireServicePluginAdmin(migrationClaims(constant.SYS_ADMIN, "", "sa")) != nil {
		t.Fatal("sys admin must pass service plugin gate")
	}
	if requireSystemMonitorAdmin(migrationClaims(constant.SYS_ADMIN, "", "sa")) != nil {
		t.Fatal("sys admin must pass system monitor gate")
	}
}

// ---- 4. RDI 未绑定控制器认领门禁 -------------------------------------------------

// TestRDIUnboundControllerClaimMatrix 迁移前语义：SYS_ADMIN 任意；tenant_id 空白
// （未绑定）的控制器可被任意角色认领；已绑定的严格按租户隔离。
func TestRDIUnboundControllerClaimMatrix(t *testing.T) {
	for _, role := range migrationRoles {
		for _, deviceTenant := range []string{"", "   ", "t1", "t2"} {
			t.Run(role.name+"/"+deviceTenant, func(t *testing.T) {
				claimable := true
				if strings.TrimSpace(deviceTenant) != "" {
					if err := authz.CheckTenant(role.claims, deviceTenant, ""); err != nil {
						claimable = false
						if migrationErrCode(err) != errcode.CodeNoPermission {
							t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
						}
					}
				}
				var want bool
				switch {
				case role.claims == nil:
					want = strings.TrimSpace(deviceTenant) == ""
				case role.claims.Authority == constant.SYS_ADMIN:
					want = true
				default:
					want = strings.TrimSpace(deviceTenant) == "" || deviceTenant == role.claims.TenantID
				}
				if claimable != want {
					t.Fatalf("claimable=%v want %v", claimable, want)
				}
			})
		}
	}
}
