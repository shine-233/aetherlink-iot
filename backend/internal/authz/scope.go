package authz

import "strings"

// NoVisibleOwnerUserID is the sentinel owner id used when a TENANT_USER has an
// empty subject id: the DAL filter then matches no rows instead of none being
// applied (fail-closed).
const NoVisibleOwnerUserID = "__aetherlink_no_visible_device_owner__"

// ListScope describes how a list query must be clamped for a subject.
type ListScope struct {
	// AllTenants means no tenant filter (SYS_ADMIN explicit cross-tenant).
	AllTenants bool
	// TenantIDs is the readable tenant set (self, or self plus descendants).
	TenantIDs []string
	// OwnerUserID, when non-nil, restricts rows to resources owned by it.
	OwnerUserID *string
}

// Options for ListScopeFor.
type ScopeOptions struct {
	// AllTenants requests the cross-tenant view; only honoured for SYS_ADMIN.
	AllTenants bool
	// Expand expands a manager's tenant into the hierarchical read set. nil
	// keeps self only.
	Expand func(tenantID string) []string
	// OwnerScoped applies the TENANT_USER owner filter.
	OwnerScoped bool
}

// OwnerFilter returns the owner-id filter for a TENANT_USER (sentinel when the
// subject id is blank) and nil for every other authority.
func OwnerFilter(c *Claims) *string {
	if c == nil || c.Authority != TenantUser {
		return nil
	}
	owner := strings.TrimSpace(c.ID)
	if owner == "" {
		owner = NoVisibleOwnerUserID
	}
	return &owner
}

// ListScopeFor computes the list scope. It returns rule's denial error for nil
// claims, unknown authorities, or AllTenants requested by a non SYS_ADMIN.
func ListScopeFor(c *Claims, opts ScopeOptions, deny Rule) (ListScope, error) {
	if c == nil || !HasRole(c, KnownRoles...) {
		return ListScope{}, deny.Deny()
	}
	if opts.AllTenants {
		if c.Authority != SysAdmin {
			return ListScope{}, deny.Deny()
		}
		return ListScope{AllTenants: true}, nil
	}
	scope := ListScope{TenantIDs: []string{c.TenantID}}
	if opts.Expand != nil && c.Authority != TenantUser && c.TenantID != "" {
		if expanded := opts.Expand(c.TenantID); len(expanded) > 0 {
			scope.TenantIDs = expanded
		}
	}
	if opts.OwnerScoped {
		scope.OwnerUserID = OwnerFilter(c)
	}
	return scope, nil
}
