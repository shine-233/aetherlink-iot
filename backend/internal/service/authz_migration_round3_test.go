// 文件用途：第三轮 authz 迁移回归矩阵（scada_control.hasPermission /
// device_mqtt_debug.canUseDeviceMQTTDebug / sys_user_email.canManageTenantWarningEmails）。
//
// 核心逻辑：把原先手写的「角色白名单 + SYS_ADMIN 跨租户放行 + 其余角色租户相等」等判定
// 收编到 internal/authz 后，按「角色 × 资源归属租户」笛卡尔展开，断言迁移后判定与迁移前
// 手写实现的预期逐条一致（含 nil / 空 ID / 未知 authority 的 fail-closed 行）。
//
// 关键注意事项：scada 的角色白名单可被 SetAllowedAuthorities 覆盖，矩阵同时覆盖
// 默认白名单与自定义白名单两条路径；hasPermission 的 actor 不是 claims，而是
// Authority/TenantID 字符串，迁移时以裁剪后的值构造 authz.Claims 视图。
package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

// ---- 1. SCADA 控制权限（ScadaControlService.hasPermission） --------------------

func TestScadaControlHasPermissionRoleMatrix(t *testing.T) {
	svc := NewScadaControlService(nil, nil, nil)
	cases := []struct {
		name           string
		authority      string
		actorTenant    string
		documentTenant string
		allow          bool
	}{
		{"nil-like empty authority", "", "t1", "t1", false},
		{"SYS_ADMIN/own tenant", constant.SYS_ADMIN, "t1", "t1", true},
		{"SYS_ADMIN/foreign tenant", constant.SYS_ADMIN, "t1", "t2", true},
		{"TENANT_ADMIN/own tenant", constant.TENANT_ADMIN, "t1", "t1", true},
		{"TENANT_ADMIN/foreign tenant", constant.TENANT_ADMIN, "t1", "t2", false},
		{"TENANT_USER not in whitelist", constant.TENANT_USER, "t1", "t1", false},
		{"unknown authority", "VIEWER", "t1", "t1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := ControlActor{UserID: "u1", TenantID: tc.actorTenant, Authority: tc.authority}
			if got := svc.hasPermission(actor, tc.documentTenant); got != tc.allow {
				t.Fatalf("hasPermission=%v want %v", got, tc.allow)
			}
		})
	}
}

// SetAllowedAuthorities 覆盖后的行为：空白名单全拒；自定义白名单内角色按租户规则放行。
func TestScadaControlHasPermissionCustomAuthorities(t *testing.T) {
	svc := NewScadaControlService(nil, nil, nil)
	svc.SetAllowedAuthorities()
	actor := ControlActor{UserID: "u1", TenantID: "t1", Authority: constant.SYS_ADMIN}
	if svc.hasPermission(actor, "t1") {
		t.Fatalf("empty whitelist must deny SYS_ADMIN")
	}

	svc.SetAllowedAuthorities(constant.TENANT_USER, "  padded  ")
	if !svc.hasPermission(ControlActor{UserID: "u1", TenantID: "t1", Authority: constant.TENANT_USER}, "t1") {
		t.Fatalf("custom whitelist TENANT_USER same tenant must pass")
	}
	if svc.hasPermission(ControlActor{UserID: "u1", TenantID: "t1", Authority: constant.TENANT_USER}, "t2") {
		t.Fatalf("custom whitelist TENANT_USER foreign tenant must deny")
	}
	if !svc.hasPermission(ControlActor{UserID: "u1", TenantID: "t9", Authority: "padded"}, "t9") {
		t.Fatalf("whitelist entries are trimmed; padded authority same tenant must pass")
	}
}

// ---- 2. 设备 MQTT 调试会话（canUseDeviceMQTTDebug） -----------------------------

func TestCanUseDeviceMQTTDebugRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		allowed bool
	}{
		{"nil claims", nil, false},
		{"empty user id", migrationClaims(constant.SYS_ADMIN, "t1", ""), false},
		{"SYS_ADMIN", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), true},
		{"TENANT_ADMIN", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), true},
		{"TENANT_USER", migrationClaims(constant.TENANT_USER, "t1", "u1"), true},
		{"unknown authority", migrationClaims("CUSTOMER", "t1", "c1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canUseDeviceMQTTDebug(tc.claims); got != tc.allowed {
				t.Fatalf("canUseDeviceMQTTDebug=%v want %v", got, tc.allowed)
			}
		})
	}
}

// ---- 3. 租户告警邮箱管理（canManageTenantWarningEmails） -----------------------

func TestCanManageTenantWarningEmailsRoleMatrix(t *testing.T) {
	cases := []struct {
		name    string
		claims  *utils.UserClaims
		allowed bool
	}{
		{"nil claims", nil, false},
		{"SYS_ADMIN", migrationClaims(constant.SYS_ADMIN, "t1", "sa"), true},
		{"TENANT_ADMIN", migrationClaims(constant.TENANT_ADMIN, "t1", "ta"), true},
		{"TENANT_USER", migrationClaims(constant.TENANT_USER, "t1", "u1"), false},
		{"unknown authority", migrationClaims("CUSTOMER", "t1", "c1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canManageTenantWarningEmails(tc.claims); got != tc.allowed {
				t.Fatalf("canManageTenantWarningEmails=%v want %v", got, tc.allowed)
			}
		})
	}
}
