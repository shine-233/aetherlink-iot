// 文件用途：service 层所有"按 ID 加载 + 租户/拥有者/分享校验"的聚合守卫（authz.Guard）集中声明处。
// 核心逻辑：每个聚合只声明 加载器 / 归属投影 / 读写规则，ensure*Access helper 退化为一行调用；
// 关键注意事项：每个守卫的 ClaimsFirst、错误码与消息逐一复刻迁移前的 helper 语义（含 not-found 掩码），
// 修改任何规则前请同步 access_guards_test.go 的矩阵用例。
package service

import (
	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// dbLoader adapts a DAL getter so its error is wrapped in the standard
// 101001 {"sql_error": ...} envelope.
func dbLoader[T any](get func(string) (T, error)) func(string) (T, error) {
	return func(id string) (T, error) {
		res, err := get(id)
		if err != nil {
			var zero T
			return zero, dbError(err)
		}
		return res, nil
	}
}

// ---- ownership projections -------------------------------------------------

// deviceOwnership projects a device: tenant, owning user and the RDI share
// recipient list (consulted lazily, only by share-aware read rules).
func deviceOwnership(device *model.Device) authz.Owned {
	return authz.OfTenant(device.TenantID).
		WithOwner(device.OwnerUserID).
		WithShare(func(c *authz.Claims) bool {
			_, ok := rdiShareRecipientForUser(device, c)
			return ok
		})
}

// deviceWriteRule: SYS_ADMIN, same-tenant non-TENANT_USER, owning TENANT_USER.
// Roles 门禁显式限定三个 JWT authority：未知 authority（如伪造的 CUSTOMER 声明）
// 在角色闸门即被拒绝（fail-closed），与 authz_migration_test 矩阵的 "unknown authority"
// 行保持一致，而不是靠 OwnerOnly 仅收窄 TENANT_USER 的副作用漏过同租户判断。
func deviceWriteRule(message string) authz.Rule {
	rule := authz.OwnerRule(message)
	rule.Roles = authz.KnownRoles
	return rule
}

// deviceReadRule: deviceWriteRule plus explicit RDI share recipients.
func deviceReadRule(message string) authz.Rule {
	rule := authz.OwnerRule(message)
	rule.Roles = authz.KnownRoles
	rule.AllowShared = true
	return rule
}

// ---- tenant-owned aggregates ----------------------------------------------

var deviceConfigGuard = authz.Guard[*model.DeviceConfig]{
	Load:        dbLoader(dal.GetDeviceConfigByID),
	Owner:       func(c *model.DeviceConfig) authz.Owned { return authz.OfTenant(c.TenantID) },
	Read:        authz.TenantRule("no permission to query device config"),
	Write:       authz.TenantRule("no permission to modify device config"),
	ClaimsFirst: true,
}

var sceneGuard = authz.Guard[*model.SceneInfo]{
	Load:        dbLoader(dal.GetSceneInfo),
	Owner:       func(s *model.SceneInfo) authz.Owned { return authz.OfTenant(s.TenantID) },
	Read:        authz.TenantRule("no permission to query scene"),
	Write:       authz.TenantRule("no permission to modify scene"),
	ClaimsFirst: true,
}

var sceneAutomationGuard = authz.Guard[*model.SceneAutomation]{
	Load: dbLoader(func(id string) (*model.SceneAutomation, error) {
		return dal.GetSceneAutomation(id, nil)
	}),
	Owner: func(s *model.SceneAutomation) authz.Owned { return authz.OfTenant(s.TenantID) },
	Read:  authz.TenantRule("no permission to query scene automation"),
	Write: authz.TenantRule("no permission to modify scene automation"),
}

var notificationGroupGuard = authz.Guard[*model.NotificationGroup]{
	Load:  dbLoader(dal.GetNotificationGroupById),
	Owner: func(g *model.NotificationGroup) authz.Owned { return authz.OfTenant(g.TenantID) },
	Read:  authz.TenantRule("no permission to query notification group"),
	Write: authz.TenantRule("no permission to modify notification group"),
}

var serviceAccessGuard = authz.Guard[*model.ServiceAccess]{
	Load:  dbLoader(dal.GetServiceAccessByID),
	Owner: func(s *model.ServiceAccess) authz.Owned { return authz.OfTenant(s.TenantID) },
	Read:  authz.TenantRule("no permission to query service access"),
}

// deviceTemplateGuard: public templates are readable by everyone; private ones
// top-down (self plus descendant tenants). Writes are handled by
// ensureDeviceTemplateWriteAccess because public templates add an OpDenied
// branch for TENANT_USER before the strict same-tenant check.
var deviceTemplateGuard = authz.Guard[*model.DeviceTemplate]{
	Load: dbLoader(dal.GetDeviceTemplateById),
	Owner: func(t *model.DeviceTemplate) authz.Owned {
		return authz.Owned{TenantID: t.TenantID, Public: isPublicDeviceTemplate(t)}
	},
	Read: authz.Rule{
		Scope:       expandTenantIDScope,
		AllowPublic: true,
		Message:     "no permission to query thing model",
	},
	ClaimsFirst: true,
}

func isPublicDeviceTemplate(t *model.DeviceTemplate) bool {
	return t != nil && t.Flag != nil && *t.Flag == dal.DEVICE_TEMPLATE_PUBLIC
}

// otaPackageGuard: a NULL tenant_id package is platform scoped (SYS_ADMIN only).
// Load errors pass through unchanged (legacy contract).
var otaPackageGuard = authz.Guard[*model.OtaUpgradePackage]{
	Load:        dal.GetOtaUpgradePackageByID,
	Owner:       func(p *model.OtaUpgradePackage) authz.Owned { return authz.OfTenantPtr(p.TenantID) },
	Read:        authz.TenantRule("no permission to access ota package"),
	ClaimsFirst: true,
}

// ---- role gates -------------------------------------------------------------

var (
	roleManagerRule      = authz.Rule{Roles: authz.ManagerRoles, Message: "no permission to manage roles"}
	userGroupManagerRule = authz.Rule{Roles: authz.ManagerRoles, Message: "no permission to manage user groups"}
	// userGroupTenantRule masks cross-tenant groups as not found.
	userGroupTenantRule = authz.Rule{Code: errcode.CodeNotFound}
)
