package authz

import "aetherlink-iot/backend/pkg/errcode"

// Rule declares who may access a resource and how a denial is reported.
//
// Evaluation order (all fail-closed):
//  1. nil claims                       -> deny
//  2. Roles set and authority not in it -> deny
//  3. SYS_ADMIN                         -> allow
//  4. Public resource and AllowPublic   -> allow
//  5. platform (NoTenant) resource      -> share fallback or deny
//  6. tenant check (equality, or membership in Scope(claims.TenantID))
//     - pass + OwnerOnly + TENANT_USER non-owner -> share fallback or deny
//     - pass                                      -> allow
//     - fail                                      -> share fallback or deny
//
// "share fallback" means: allow when AllowShared and Owned.SharedWith(claims).
type Rule struct {
	// Roles restricts the authorities that may pass at all. nil means any
	// authority (the legacy "SYS_ADMIN or same tenant" contract).
	Roles []string
	// Scope expands the subject's tenant into the readable tenant set
	// (hierarchical top-down reads). nil means strict tenant equality.
	Scope func(tenantID string) []string
	// OwnerOnly narrows TENANT_USER to resources whose owner is the subject.
	OwnerOnly bool
	// AllowShared lets an explicit share grant access when the tenant/owner
	// checks failed (read paths only).
	AllowShared bool
	// AllowPublic lets any authenticated, role-allowed subject read a resource
	// flagged Public.
	AllowPublic bool
	// Code is the denial error code; 0 means errcode.CodeNoPermission. Use
	// errcode.CodeNotFound to mask existence.
	Code int
	// Message is the custom denial message; empty keeps the default text.
	Message string
}

// Deny returns the error this rule reports on denial.
func (r Rule) Deny() error { return Deny(r.Code, r.Message) }

// WithMessage returns a copy of the rule with a different denial message.
func (r Rule) WithMessage(message string) Rule {
	r.Message = message
	return r
}

// Allows reports whether the rule admits the subject for the resource.
func (r Rule) Allows(c *Claims, res TenantOwned) bool {
	if c == nil || res == nil {
		return false
	}
	if r.Roles != nil && !HasRole(c, r.Roles...) {
		return false
	}
	if c.Authority == SysAdmin {
		return true
	}
	o := res.Ownership()
	if r.AllowPublic && o.Public {
		return true
	}
	if o.NoTenant {
		return r.shared(c, o)
	}
	var tenantOK bool
	if r.Scope != nil {
		tenantOK = InScope(o.TenantID, r.Scope(c.TenantID))
	} else {
		tenantOK = o.TenantID == c.TenantID
	}
	if !tenantOK {
		return r.shared(c, o)
	}
	if r.OwnerOnly && c.Authority == TenantUser && !OwnerMatches(o.OwnerUserID, c) {
		return r.shared(c, o)
	}
	return true
}

func (r Rule) shared(c *Claims, o Owned) bool {
	return r.AllowShared && o.SharedWith != nil && o.SharedWith(c)
}

// Check returns nil when the rule admits the subject, otherwise the rule's
// denial error.
func (r Rule) Check(c *Claims, res TenantOwned) error {
	if r.Allows(c, res) {
		return nil
	}
	return r.Deny()
}

// RequireClaims fails when claims are nil (or, with roles, not in roles).
func (r Rule) RequireClaims(c *Claims) error {
	if c == nil || (r.Roles != nil && !HasRole(c, r.Roles...)) {
		return r.Deny()
	}
	return nil
}

// TenantRule is the classic "SYS_ADMIN or same tenant" rule with a message.
func TenantRule(message string) Rule {
	return Rule{Code: errcode.CodeNoPermission, Message: message}
}

// OwnerRule is the device-style rule: SYS_ADMIN, same-tenant managers and the
// owning TENANT_USER.
func OwnerRule(message string) Rule {
	return Rule{OwnerOnly: true, Code: errcode.CodeNoPermission, Message: message}
}

// CheckTenant is a convenience for the common one-off check.
func CheckTenant(c *Claims, tenantID string, message string) error {
	return TenantRule(message).Check(c, OfTenant(tenantID))
}
