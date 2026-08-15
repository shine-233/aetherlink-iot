package service

import (
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const unsupportedScopeAuthorityPermissionMessage = "unsupported account authority"

// requireSupportedScopeAuthority keeps customer-facing read scopes fail-closed:
// a tenant id alone is not authorization, and only the three JWT authorities
// understood by the service may reach a scoped DAL query.
func requireSupportedScopeAuthority(claims *utils.UserClaims, permissionMessage string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
	}
	switch claims.Authority {
	case constant.TENANT_USER, constant.TENANT_ADMIN, constant.SYS_ADMIN:
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
	}
}

// requireSystemAdminAllTenantsScope protects every explicit cross-tenant read
// seam. Every request first passes the authority allowlist; a false request
// then keeps the caller's existing tenant/owner contract, while a true request
// is never inferred from a missing tenant id or a frontend role.
func requireSystemAdminAllTenantsScope(requested bool, claims *utils.UserClaims, permissionMessage string) error {
	if requested {
		if claims == nil || claims.Authority != constant.SYS_ADMIN {
			return errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
		}
		return nil
	}
	return requireSupportedScopeAuthority(claims, unsupportedScopeAuthorityPermissionMessage)
}
