// 文件用途：service 层授权 helper 的唯一实现处，全部委托给 internal/authz。
// 核心逻辑：原先散落在 20+ 个文件、各自手写 "SYS_ADMIN || 同租户 || owner" 判断的 ensure*/require*
// helper 迁移至此；函数名与签名保持不变，以便调用方与既有测试无需感知迁移。
// 关键注意事项：每个 helper 的 fail-closed 顺序（先 claims 还是先加载）、错误码与自定义消息
// 均逐一对齐迁移前实现；not-found 掩码（用户组 / 角色）保持原样，禁止合并成统一文案。
package service

import (
	"context"
	"errors"
	"strings"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// ---- generic claim gates -----------------------------------------------------

const unsupportedScopeAuthorityPermissionMessage = "unsupported account authority"

// requireSupportedScopeAuthority keeps customer-facing read scopes fail-closed:
// a tenant id alone is not authorization, and only the three JWT authorities
// understood by the service may reach a scoped DAL query.
func requireSupportedScopeAuthority(claims *utils.UserClaims, permissionMessage string) error {
	return authz.Rule{Roles: authz.KnownRoles, Message: permissionMessage}.RequireClaims(claims)
}

// requireSystemAdminAllTenantsScope protects every explicit cross-tenant read
// seam: a true request requires SYS_ADMIN; a false request only requires a
// known authority.
func requireSystemAdminAllTenantsScope(requested bool, claims *utils.UserClaims, permissionMessage string) error {
	if requested {
		return authz.Rule{Roles: []string{authz.SysAdmin}, Message: permissionMessage}.RequireClaims(claims)
	}
	return requireSupportedScopeAuthority(claims, unsupportedScopeAuthorityPermissionMessage)
}

func ensureTenantScopedWriteClaims(claims *utils.UserClaims, action string) error {
	if claims == nil {
		return authz.NoPermission("no permission to " + action)
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return authz.NoPermission("complete tenant initialization before " + action)
	}
	return nil
}

// ---- device ownership --------------------------------------------------------

func deviceOwnerUserIDFilterForClaims(claims *utils.UserClaims) *string {
	return authz.OwnerFilter(claims)
}

// hasTelemetryTenantAccess is the device access predicate shared by telemetry,
// OTA, RDI and debug paths: SYS_ADMIN; same-tenant managers; the owning
// TENANT_USER; and, when allowSharedRead, explicit RDI share recipients (also
// cross-tenant).
func hasTelemetryTenantAccess(deviceInfo *model.Device, claims *utils.UserClaims, allowSharedRead bool) bool {
	if deviceInfo == nil {
		return false
	}
	rule := deviceWriteRule("")
	if allowSharedRead {
		rule = deviceReadRule("")
	}
	return rule.Allows(claims, deviceOwnership(deviceInfo))
}

func ensureDeviceDeleteAccess(id string, userClaims *utils.UserClaims) (*model.Device, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	rule := deviceWriteRule("no permission to delete device")
	if err := rule.RequireClaims(userClaims); err != nil {
		return nil, err
	}
	deviceInfo, err := dal.GetDeviceByIDUnscoped(id)
	if err != nil {
		return nil, err
	}
	if err := rule.Check(userClaims, authz.OfTenant(deviceInfo.TenantID).WithOwner(deviceInfo.OwnerUserID)); err != nil {
		return nil, err
	}
	return deviceInfo, nil
}

func ensurePluginDeviceTenantAccess(device *model.Device, claims *utils.UserClaims) error {
	if claims == nil {
		return authz.NoPermission("plugin device config requires api key")
	}
	if device == nil {
		return authz.NoPermission("device not found or no permission")
	}
	return deviceWriteRule("no permission to query plugin device config").
		Check(claims, authz.OfTenant(device.TenantID).WithOwner(device.OwnerUserID))
}

// ensureActivationDeviceAccess: a nil claims caller is the anonymous
// activation path and is intentionally allowed (legacy contract).
func ensureActivationDeviceAccess(device *model.Device, claims *utils.UserClaims) error {
	if claims == nil {
		return nil
	}
	return authz.Rule{}.Check(claims, authz.OfTenant(device.TenantID))
}

// ---- tenant-owned aggregates (via guards) ------------------------------------

func ensureDeviceConfigReadAccess(configID string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	return deviceConfigGuard.RequireRead(configID, claims)
}

func ensureDeviceConfigWriteAccess(configID string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	return deviceConfigGuard.RequireWrite(configID, claims)
}

func ensureSceneReadAccess(sceneID string, claims *utils.UserClaims) (*model.SceneInfo, error) {
	return sceneGuard.RequireRead(sceneID, claims)
}

func ensureSceneWriteAccess(sceneID string, claims *utils.UserClaims) (*model.SceneInfo, error) {
	return sceneGuard.RequireWrite(sceneID, claims)
}

func ensureSceneAutomationReadAccess(id string, claims *utils.UserClaims) (*model.SceneAutomation, error) {
	return sceneAutomationGuard.RequireRead(id, claims)
}

func ensureSceneAutomationWriteAccess(id string, claims *utils.UserClaims) (*model.SceneAutomation, error) {
	return sceneAutomationGuard.RequireWrite(id, claims)
}

func ensureNotificationGroupReadAccess(id string, u *utils.UserClaims) (*model.NotificationGroup, error) {
	return notificationGroupGuard.RequireRead(id, u)
}

func ensureNotificationGroupWriteAccess(id string, u *utils.UserClaims) (*model.NotificationGroup, error) {
	return notificationGroupGuard.RequireWrite(id, u)
}

func ensureServiceAccessReadAccess(id string, userClaims *utils.UserClaims) (*model.ServiceAccess, error) {
	return serviceAccessGuard.RequireRead(id, userClaims)
}

func ensureServiceAccessWriteAccess(id string, userClaims *utils.UserClaims) (*model.ServiceAccess, error) {
	return serviceAccessGuard.RequireWrite(id, userClaims)
}

func ensureDeviceTemplateReadAccess(templateID string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	return deviceTemplateGuard.RequireRead(templateID, claims)
}

func ensureDeviceTemplateWriteAccess(templateID string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	t, err := deviceTemplateGuard.RequireRead(templateID, claims)
	if err != nil {
		return nil, err
	}
	if isPublicDeviceTemplate(t) && claims.Authority == authz.TenantUser {
		return nil, errcode.New(errcode.CodeOpDenied)
	}
	if err := authz.CheckTenant(claims, t.TenantID, "no permission to modify thing model"); err != nil {
		return nil, err
	}
	return t, nil
}

func ensureDeviceModelTenantWriteAccess(tenantID string, claims *utils.UserClaims) error {
	return authz.CheckTenant(claims, tenantID, "no permission to modify device model")
}

func ensureOTAPackageAccess(packageID string, claims *utils.UserClaims) (*model.OtaUpgradePackage, error) {
	return otaPackageGuard.RequireRead(packageID, claims)
}

func otaTaskOwnerUserIDForClaims(claims *utils.UserClaims) (*string, error) {
	const msg = "no permission to access ota task"
	if err := (authz.Rule{Roles: authz.KnownRoles, Message: msg}).RequireClaims(claims); err != nil {
		return nil, err
	}
	if claims.Authority != authz.TenantUser {
		return nil, nil
	}
	ownerUserID := strings.TrimSpace(claims.ID)
	if ownerUserID == "" {
		return nil, authz.NoPermission(msg)
	}
	return &ownerUserID, nil
}

// ---- roles and user groups -------------------------------------------------

func requireRoleManager(claims *utils.UserClaims) error {
	return roleManagerRule.RequireClaims(claims)
}

// ensureRoleWriteAccess loads first (existence is permission-shaped), then
// requires a manager, then strict tenant equality (NULL-tenant system roles
// are SYS_ADMIN only).
func ensureRoleWriteAccess(id string, claims *utils.UserClaims) (model.Role, error) {
	role, err := dal.GetRoleByID(id)
	if err != nil {
		return role, err
	}
	if role.ID == "" {
		return role, authz.NoPermission("role not found or no permission")
	}
	if err := requireRoleManager(claims); err != nil {
		return role, err
	}
	if err := authz.TenantRule("no permission to manage another tenant role").Check(claims, authz.OfTenantPtr(role.TenantID)); err != nil {
		return role, err
	}
	return role, nil
}

func requireUserGroupManager(claims *utils.UserClaims) error {
	return userGroupManagerRule.RequireClaims(claims)
}

// ensureUserGroupWriteAccess masks both missing and cross-tenant groups as
// CodeNotFound so tenant admins cannot probe other tenants' group ids.
func ensureUserGroupWriteAccess(id string, claims *utils.UserClaims) (*model.UserGroup, error) {
	if err := requireUserGroupManager(claims); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "group id is required")
	}
	group, err := dal.GetUserGroupByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, dbError(err)
	}
	if err := userGroupTenantRule.Check(claims, authz.OfTenant(group.TenantID)); err != nil {
		return nil, err
	}
	return group, nil
}

// ---- alarms / notification history -----------------------------------------

func ensureAlarmTenantAccess(resourceTenantID string, claims *utils.UserClaims, permissionMessage string) error {
	rule := authz.Rule{Roles: authz.KnownRoles, Message: permissionMessage}
	if err := rule.RequireClaims(claims); err != nil {
		return err
	}
	return rule.Check(claims, authz.OfTenant(resourceTenantID))
}

// alarm_info is a legacy active-alert table without a device relationship.
// Until the schema records affected devices, TENANT_USER access cannot be
// narrowed to owner_user_id safely and must fail closed instead of exposing
// the whole tenant. Owner-scoped device alarms remain available via history.
func ensureActiveAlarmInfoOwnerScope(claims *utils.UserClaims) error {
	return authz.Rule{Roles: authz.ManagerRoles, Message: alarmInfoOwnerScopeMessage}.RequireClaims(claims)
}

// ensureNotificationHistoryOwnerScope rejects unauthenticated / unknown
// authorities. Tenant users pass: the DAL applies an all-devices-owned filter.
func ensureNotificationHistoryOwnerScope(claims *utils.UserClaims) error {
	return authz.Rule{Roles: authz.KnownRoles, Message: "no permission to query notification history"}.RequireClaims(claims)
}

// ---- boards ------------------------------------------------------------------

const boardModifyPermissionMessage = "no permission to modify board"

var boardManagerRule = authz.Rule{Roles: authz.ManagerRoles, Message: boardModifyPermissionMessage}

func ensureBoardWritePermission(claims *utils.UserClaims, targetTenantID *string) error {
	if err := boardManagerRule.RequireClaims(claims); err != nil {
		return err
	}
	if claims.Authority == authz.TenantAdmin && claims.TenantID == "" {
		return authz.NoPermission(boardModifyPermissionMessage)
	}
	if targetTenantID != nil {
		if *targetTenantID == "" {
			return authz.NoPermission(boardModifyPermissionMessage)
		}
		return boardManagerRule.Check(claims, authz.OfTenant(*targetTenantID))
	}
	return nil
}

// ensureBoardReadAccess 既校验看板存在，也把跨租户读取拦在 service 层。
func ensureBoardReadAccess(ctx context.Context, boardID string, claims *utils.UserClaims) (*model.Board, error) {
	rule := authz.TenantRule("no permission to query board")
	if err := rule.RequireClaims(claims); err != nil {
		return nil, err
	}
	board, err := dal.BoardQuery{}.First(ctx, query.Board.ID.Eq(boardID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "board not found")
	}
	if err != nil {
		return nil, dbError(err)
	}
	if err := rule.Check(claims, authz.OfTenant(board.TenantID)); err != nil {
		return nil, err
	}
	return board, nil
}

func ensureBoardWriteAccess(ctx context.Context, boardID string, claims *utils.UserClaims) (*model.Board, error) {
	if claims == nil {
		return nil, authz.NoPermission("no permission to query board")
	}
	if err := ensureBoardWritePermission(claims, nil); err != nil {
		return nil, err
	}
	board, err := ensureBoardReadAccess(ctx, boardID, claims)
	if err != nil {
		return nil, err
	}
	if err := ensureBoardWritePermission(claims, &board.TenantID); err != nil {
		return nil, err
	}
	return board, nil
}
