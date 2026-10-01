package kit

import (
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// TenantScope is the two-message tenant gate used by the "manage X" services
// (calculated field, entity version, dashboard menu): nil claims and a blank
// tenant are rejected with distinct CodeNoPermission messages, and the
// whitespace-trimmed tenant is returned for the DAL calls.
//
// Unlike Gate.NeedTenant it trims the tenant first, so "  " is rejected too.
type TenantScope struct {
	// NilMsg is returned for nil claims.
	NilMsg string
	// BlankMsg is returned when the trimmed tenant is empty.
	BlankMsg string
}

// Tenant returns the trimmed tenant ID or the matching gate error.
func (s TenantScope) Tenant(c *utils.UserClaims) (string, error) {
	if c == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, s.NilMsg)
	}
	tenantID := strings.TrimSpace(c.TenantID)
	if tenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, s.BlankMsg)
	}
	return tenantID, nil
}
