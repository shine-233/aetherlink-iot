// 文件用途：authz 迁移回归矩阵——证明 service 层手写租户/属主/分享校验迁到 internal/authz 后，
// 每个角色在每个迁移点上的判定结果与迁移前逐条一致。
//
// 核心逻辑：每个用例按「角色 × 归属/请求租户」笛卡尔展开，既断言允许与否，也断言错误码
// （消息是接口契约的一部分，迁移不得改写）。
//
// 关键注意事项：本文件只覆盖不触库的纯判定路径；需要 DB 的加载器路径由既有
// *_scope_test.go / *_owner_scope_test.go 覆盖。新增迁移点时请同步补矩阵行。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func migrationClaims(authority, tenantID, id string) *utils.UserClaims {
	return &utils.UserClaims{Authority: authority, TenantID: tenantID, ID: id}
}

func migrationErrCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := err.(*errcode.Error); ok {
		return e.Code
	}
	return -1
}

// migrationRoles 是矩阵覆盖的角色集合：三个受支持 authority + 未知 authority + nil 声明。
var migrationRoles = []struct {
	name   string
	claims *utils.UserClaims
}{
	{"nil", nil},
	{"SYS_ADMIN", migrationClaims(constant.SYS_ADMIN, "t1", "sa")},
	{"TENANT_ADMIN", migrationClaims(constant.TENANT_ADMIN, "t1", "ta")},
	{"TENANT_USER", migrationClaims(constant.TENANT_USER, "t1", "u1")},
	{"unknown authority", migrationClaims("CUSTOMER", "t1", "c1")},
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- 1. 租户级列表读作用域（4 个聚合收敛为 tenantReadListScopes） ----------------

func TestTenantReadListScopesRoleMatrix(t *testing.T) {
	cases := []struct {
		name   string
		claims *utils.UserClaims
		tenant string
		want   []string
	}{
		{"nil claims fail closed", nil, "t1", nil},
		{"sys admin empty tenant maps platform row", migrationClaims(constant.SYS_ADMIN, "", "sa"), "", []string{""}},
		{"sys admin tenant expands", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), "t1", []string{"t1"}},
		{"tenant admin tenant expands", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1", []string{"t1"}},
		{"tenant admin blank tenant maps platform row", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "  ", []string{""}},
		{"tenant user keeps self only", migrationClaims(constant.TENANT_USER, "t1", "u1"), "t1", []string{"t1"}},
		{"tenant user trims self only", migrationClaims(constant.TENANT_USER, "t1", "u1"), " t1 ", []string{"t1"}},
		{"tenant user empty tenant fail closed", migrationClaims(constant.TENANT_USER, "", "u1"), "", nil},
		{"tenant user blank tenant fail closed", migrationClaims(constant.TENANT_USER, "", "u1"), "   ", nil},
		{"unknown authority behaves like admin", migrationClaims("CUSTOMER", "t1", "c1"), "t1", []string{"t1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tenantReadListScopes(tc.tenant, tc.claims)
			if !equalStringSlices(got, tc.want) {
				t.Fatalf("tenantReadListScopes(%q) = %#v, want %#v", tc.tenant, got, tc.want)
			}
		})
	}
}

// TestTenantReadListScopesWrappersAgree 证明 4 个聚合包装器迁移后仍与共享实现一致
// （OTA 升级包 / 命令任务 / 通知历史 / 场景自动化）。
func TestTenantReadListScopesWrappersAgree(t *testing.T) {
	tenants := []string{"t1", "", "  "}
	for _, role := range migrationRoles {
		for _, tenant := range tenants {
			claims := role.claims
			if claims != nil {
				claims = migrationClaims(claims.Authority, tenant, claims.ID)
			}
			want := tenantReadListScopes(tenant, claims)
			label := role.name + "/" + tenant
			if got := otaUpgradePackageListScopes(claims); !equalStringSlices(got, want) {
				t.Fatalf("%s otaUpgradePackageListScopes=%#v want %#v", label, got, want)
			}
			if got := fleetCommandJobListScopes(claims); !equalStringSlices(got, want) {
				t.Fatalf("%s fleetCommandJobListScopes=%#v want %#v", label, got, want)
			}
			if got := notificationHistoryListScopes(claims); !equalStringSlices(got, want) {
				t.Fatalf("%s notificationHistoryListScopes=%#v want %#v", label, got, want)
			}
			if got := sceneAutomationReadScopes(tenant, claims); !equalStringSlices(got, want) {
				t.Fatalf("%s sceneAutomationReadScopes=%#v want %#v", label, got, want)
			}
		}
	}
}

func TestPlatformOrExpandedScopesWrappersAgree(t *testing.T) {
	cases := []struct {
		tenant string
		want   []string
	}{
		{"", []string{""}},
		{"t1", []string{"t1"}},
	}
	for _, tc := range cases {
		if got := platformOrExpandedScopes(tc.tenant); !equalStringSlices(got, tc.want) {
			t.Fatalf("platformOrExpandedScopes(%q)=%#v want %#v", tc.tenant, got, tc.want)
		}
		if got := emailTemplateListScopes(tc.tenant); !equalStringSlices(got, tc.want) {
			t.Fatalf("emailTemplateListScopes(%q)=%#v want %#v", tc.tenant, got, tc.want)
		}
		if got := fleetSavedFilterListScopes(tc.tenant); !equalStringSlices(got, tc.want) {
			t.Fatalf("fleetSavedFilterListScopes(%q)=%#v want %#v", tc.tenant, got, tc.want)
		}
	}
}

// ---- 2. 设备属主规则（bindChildDevice 迁移点） ----------------------------------

func TestDeviceWriteRuleRoleMatrix(t *testing.T) {
	owner := "u1"
	device := &model.Device{TenantID: "t1", OwnerUserID: &owner}
	rule := deviceWriteRule("no permission to modify device telemetry")
	cases := []struct {
		name   string
		claims *utils.UserClaims
		allow  bool
	}{
		{"nil claims", nil, false},
		{"sys admin any tenant", migrationClaims(constant.SYS_ADMIN, "t9", "sa"), true},
		{"tenant admin same tenant", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), true},
		{"tenant admin cross tenant", migrationClaims(constant.TENANT_ADMIN, "t2", "ta"), false},
		{"owner tenant user", migrationClaims(constant.TENANT_USER, "t1", "u1"), true},
		{"non owner tenant user", migrationClaims(constant.TENANT_USER, "t1", "u2"), false},
		{"blank id tenant user", migrationClaims(constant.TENANT_USER, "t1", "  "), false},
		{"cross tenant tenant user", migrationClaims(constant.TENANT_USER, "t2", "u1"), false},
		{"unknown authority", migrationClaims("CUSTOMER", "t1", "c1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rule.Allows(tc.claims, deviceOwnership(device)); got != tc.allow {
				t.Fatalf("Allows=%v want %v", got, tc.allow)
			}
		})
	}
	// nil 声明必须在加载前被拒绝，错误码与消息保持迁移前一致。
	if err := rule.RequireClaims(nil); migrationErrCode(err) != errcode.CodeNoPermission {
		t.Fatalf("RequireClaims(nil) code=%d", migrationErrCode(err))
	}
}

// ---- 3. RDI 告警邮箱可见性（rdiMayExposeAlarmEmails 迁移点） --------------------

func TestRDIMayExposeAlarmEmailsRoleMatrix(t *testing.T) {
	owner := "u1"
	device := &model.Device{TenantID: "t1", OwnerUserID: &owner}
	platformDevice := &model.Device{TenantID: "", OwnerUserID: &owner}
	cases := []struct {
		name   string
		claims *utils.UserClaims
		device *model.Device
		want   bool
	}{
		{"nil claims", nil, device, false},
		{"nil device", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), nil, false},
		{"sys admin cross tenant", migrationClaims(constant.SYS_ADMIN, "t9", "sa"), device, true},
		{"tenant admin same tenant", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), device, true},
		{"tenant admin cross tenant", migrationClaims(constant.TENANT_ADMIN, "t2", "ta"), device, false},
		{"owner tenant user", migrationClaims(constant.TENANT_USER, "t1", "u1"), device, true},
		{"non owner tenant user", migrationClaims(constant.TENANT_USER, "t1", "u2"), device, false},
		{"unknown authority same tenant", migrationClaims("CUSTOMER", "t1", "c1"), device, false},
		{"platform row rejects non sys admin", migrationClaims(constant.TENANT_ADMIN, "", "ta"), platformDevice, false},
		{"platform row keeps sys admin", migrationClaims(constant.SYS_ADMIN, "", "sa"), platformDevice, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rdiMayExposeAlarmEmails(tc.device, tc.claims); got != tc.want {
				t.Fatalf("rdiMayExposeAlarmEmails = %v want %v", got, tc.want)
			}
		})
	}
}

// ---- 4. 告警列表租户归一（normalizeAlarmListTenantID 迁移点） -------------------

func TestNormalizeAlarmListTenantIDRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}{
		{"nil claims", nil, "", "", true},
		{"sys admin without request is all tenants", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), "", "", false},
		{"sys admin with request", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), "t2", "t2", false},
		{"tenant admin without request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "", "t1", false},
		{"tenant admin same tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1", "t1", false},
		{"tenant admin cross tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t2", "", true},
		{"tenant user without request", migrationClaims(constant.TENANT_USER, "t1", "u1"), "", "t1", false},
		{"tenant user cross tenant request", migrationClaims(constant.TENANT_USER, "t1", "u1"), "t2", "", true},
		{"empty claims tenant", migrationClaims(constant.TENANT_ADMIN, "", "ta"), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeAlarmListTenantID(tc.request, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("tenant=%q want %q", got, tc.want)
			}
		})
	}
}

// ---- 5. 用户组租户解析（resolveUserGroupTenant 迁移点，非 SYS_ADMIN 纯路径） ----

func TestResolveUserGroupTenantRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}{
		{"tenant admin without request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "", "t1", false},
		{"tenant admin same tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1", "t1", false},
		{"tenant admin cross tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t2", "", true},
		{"tenant admin empty tenant context", migrationClaims(constant.TENANT_ADMIN, "", "ta"), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveUserGroupTenant(tc.request, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("tenant=%q want %q", got, tc.want)
			}
		})
	}
}

// ---- 6. 看板租户解析（三个 resolveBoard*Tenant 迁移点，非 SYS_ADMIN 纯路径） ----

func TestResolveBoardHomeTenantRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}{
		{"nil claims", nil, "", "", true},
		{"tenant admin without request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "", "t1", false},
		{"tenant user without request", migrationClaims(constant.TENANT_USER, "t1", "u1"), "", "t1", false},
		{"cross tenant request", migrationClaims(constant.TENANT_USER, "t1", "u1"), "t2", "", true},
		{"empty tenant context", migrationClaims(constant.TENANT_USER, "", "u1"), "", "", true},
	}
	run := func(tc struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}) {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveBoardHomeTenant(tc.request, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("tenant=%q want %q", got, tc.want)
			}
		})
	}
	for _, tc := range cases {
		run(tc)
	}
}

func TestResolveBoardListTenantRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}{
		{"nil claims", nil, "", "", true},
		{"tenant user is not a manager", migrationClaims(constant.TENANT_USER, "t1", "u1"), "", "", true},
		{"unknown authority is not a manager", migrationClaims("CUSTOMER", "t1", "c1"), "", "", true},
		{"tenant admin without request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "", "t1", false},
		{"tenant admin cross tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t2", "", true},
		{"tenant admin empty tenant context", migrationClaims(constant.TENANT_ADMIN, "", "ta"), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveBoardListTenant(&tc.request, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("tenant=%q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveBoardWriteTenantRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		request string
		want    string
		wantErr bool
	}{
		{"nil claims", nil, "", "", true},
		{"tenant user is not a manager", migrationClaims(constant.TENANT_USER, "t1", "u1"), "", "", true},
		{"tenant admin without request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "", "t1", false},
		{"tenant admin same tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t1", "t1", false},
		{"tenant admin cross tenant request", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), "t2", "", true},
		{"tenant admin empty tenant context", migrationClaims(constant.TENANT_ADMIN, "", "ta"), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveBoardWriteTenant(tc.request, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if migrationErrCode(err) != errcode.CodeNoPermission {
					t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("tenant=%q want %q", got, tc.want)
			}
		})
	}
}

// ---- 7. 用户管理授权（sys_user_authorization 迁移点） --------------------------

func TestEnsureUserTransformAccessRoleMatrix(t *testing.T) {
	tenant := "t1"
	userTarget := &model.User{ID: "u9", TenantID: &tenant, Authority: strPtr(constant.TENANT_USER)}
	adminTarget := &model.User{ID: "u8", TenantID: &tenant, Authority: strPtr(constant.TENANT_ADMIN)}
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		target  *model.User
		wantErr bool
	}{
		{"nil claims", nil, userTarget, true},
		{"nil target", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), nil, true},
		{"sys admin", migrationClaims(constant.SYS_ADMIN, "t9", "sa"), userTarget, false},
		{"tenant admin same tenant", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), userTarget, false},
		{"tenant admin cross tenant", migrationClaims(constant.TENANT_ADMIN, "t2", "ta"), userTarget, true},
		{"tenant admin cannot transform admin", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), adminTarget, true},
		{"tenant user", migrationClaims(constant.TENANT_USER, "t1", "u1"), userTarget, true},
		{"unknown authority", migrationClaims("CUSTOMER", "t1", "c1"), userTarget, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureUserTransformAccess(tc.target, tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr && migrationErrCode(err) != errcode.CodeNoPermission {
				t.Fatalf("code=%d want %d", migrationErrCode(err), errcode.CodeNoPermission)
			}
		})
	}
}

// ---- 8. 组共享管理员豁免（groupHiddenResourceIDs 迁移点，不触库分支） -----------

func TestGroupHiddenResourceIDsManagerExemption(t *testing.T) {
	managers := []struct {
		name   string
		claims *utils.UserClaims
	}{
		{"sys admin", migrationClaims(constant.SYS_ADMIN, "t1", "sa")},
		{"tenant admin", migrationClaims(constant.TENANT_ADMIN, "t1", "ta")},
	}
	for _, tc := range managers {
		t.Run(tc.name, func(t *testing.T) {
			hidden, err := groupHiddenResourceIDs([]string{"t1"}, tc.claims, model.GroupElementKindBoard)
			if err != nil || hidden != nil {
				t.Fatalf("managers must be exempt: hidden=%#v err=%v", hidden, err)
			}
		})
	}
	// 早退分支：nil 声明 / 空用户 ID / 空作用域都不查库。
	for _, claims := range []*utils.UserClaims{nil, migrationClaims(constant.TENANT_USER, "t1", "")} {
		if hidden, err := groupHiddenResourceIDs([]string{"t1"}, claims, model.GroupElementKindBoard); err != nil || hidden != nil {
			t.Fatalf("early return: hidden=%#v err=%v", hidden, err)
		}
	}
	if hidden, err := groupHiddenResourceIDs(nil, migrationClaims(constant.TENANT_USER, "t1", "u1"), model.GroupElementKindBoard); err != nil || hidden != nil {
		t.Fatalf("empty scopes: hidden=%#v err=%v", hidden, err)
	}
}
