// Package authz centralises the tenant / owner / share authorization rules that
// the service layer used to re-implement in dozens of ensure*Access helpers.
//
// The model is intentionally small:
//
//   - Owned describes who a loaded resource belongs to (tenant, optional owner
//     user, optional share predicate). TenantOwned is the interface form.
//   - Rule describes who may touch a resource (role allowlist, hierarchical
//     tenant scope, owner-only for TENANT_USER, share fallback) and which error
//     a denial produces (code + optional message, so not-found masking is a
//     Rule with Code: errcode.CodeNotFound).
//   - Guard[T] binds a loader, an ownership projection and read/write rules so a
//     service helper becomes a one-line RequireRead / RequireWrite call.
//   - ListScope is the list-query counterpart: which tenants and which owner the
//     DAL must clamp to.
//
// Every entry point is fail-closed: nil claims, unknown ownership and
// NULL-tenant (platform) resources are denied unless the caller is SYS_ADMIN.
package authz

import (
	"strings"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// Claims is the authenticated subject. It aliases the JWT claims type so
// callers do not need to convert.
type Claims = utils.UserClaims

// Authority constants re-exported for readability at call sites.
const (
	SysAdmin    = constant.SYS_ADMIN
	TenantAdmin = constant.TENANT_ADMIN
	TenantUser  = constant.TENANT_USER
)

// KnownRoles is the allowlist of JWT authorities the service understands.
var KnownRoles = []string{TenantUser, TenantAdmin, SysAdmin}

// ManagerRoles are the authorities that may administer tenant-level resources.
var ManagerRoles = []string{TenantAdmin, SysAdmin}

// Owned is the ownership projection of a loaded resource.
type Owned struct {
	// TenantID is the owning tenant. Compared verbatim with Claims.TenantID.
	TenantID string
	// NoTenant marks a resource whose tenant column is NULL (platform scoped).
	// Only SYS_ADMIN may access such a resource.
	NoTenant bool
	// Public marks a resource readable by every authenticated subject when the
	// rule sets AllowPublic (e.g. public thing models).
	Public bool
	// OwnerUserID is the owning user for owner-scoped resources (devices).
	OwnerUserID *string
	// SharedWith reports whether the resource is explicitly shared with the
	// subject. It is only consulted by rules with AllowShared and only after
	// the tenant/owner checks failed, so an expensive lookup stays lazy.
	SharedWith func(*Claims) bool
}

// TenantOwned is implemented by anything that can describe its ownership.
type TenantOwned interface {
	Ownership() Owned
}

// Ownership makes Owned satisfy TenantOwned.
func (o Owned) Ownership() Owned { return o }

// OfTenant returns the ownership of a resource belonging to tenantID.
func OfTenant(tenantID string) Owned { return Owned{TenantID: tenantID} }

// OfTenantPtr maps a nullable tenant column: nil means platform scoped.
func OfTenantPtr(tenantID *string) Owned {
	if tenantID == nil {
		return Owned{NoTenant: true}
	}
	return Owned{TenantID: *tenantID}
}

// WithOwner returns a copy that also carries an owning user.
func (o Owned) WithOwner(ownerUserID *string) Owned {
	o.OwnerUserID = ownerUserID
	return o
}

// WithShare returns a copy that carries a share predicate.
func (o Owned) WithShare(shared func(*Claims) bool) Owned {
	o.SharedWith = shared
	return o
}

// IsSysAdmin reports whether the subject is a system administrator.
func IsSysAdmin(c *Claims) bool { return c != nil && c.Authority == SysAdmin }

// HasRole reports whether the subject's authority is one of roles.
func HasRole(c *Claims, roles ...string) bool {
	if c == nil {
		return false
	}
	for _, role := range roles {
		if c.Authority == role {
			return true
		}
	}
	return false
}

// OwnerMatches reports whether the subject is the (non-empty) owner.
func OwnerMatches(ownerUserID *string, c *Claims) bool {
	if ownerUserID == nil || c == nil {
		return false
	}
	owner := strings.TrimSpace(*ownerUserID)
	return owner != "" && owner == strings.TrimSpace(c.ID)
}

// InScope reports whether tenantID is a member of scopes.
func InScope(tenantID string, scopes []string) bool {
	for _, scope := range scopes {
		if scope == tenantID {
			return true
		}
	}
	return false
}

// Deny builds a permission error. An empty message produces the bare code so
// the response layer renders the localized default text.
func Deny(code int, message string) error {
	if code == 0 {
		code = errcode.CodeNoPermission
	}
	if message == "" {
		return errcode.New(code)
	}
	return errcode.NewWithMessage(code, message)
}

// NoPermission is Deny(CodeNoPermission, message).
func NoPermission(message string) error { return Deny(errcode.CodeNoPermission, message) }
