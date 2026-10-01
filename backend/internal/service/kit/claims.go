package kit

import (
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// Gate is the nil-claims / empty-tenant preamble that opens most tenant
// services. It deliberately emits errcode.NewWithMessage(Code, Msg) rather
// than authz.NoPermission so the historical message text is preserved.
type Gate struct {
	// Msg is the custom message, e.g. "claims required".
	Msg string
	// Code defaults to errcode.CodeNoPermission.
	Code int
	// NeedTenant also rejects claims with an empty TenantID.
	NeedTenant bool
}

// ClaimsRequired is the gate shared by the converter/integration family.
var ClaimsRequired = Gate{Msg: "claims required"}

// TenantRequired additionally rejects an empty tenant (widget bundle family).
var TenantRequired = Gate{Msg: "claims required", NeedTenant: true}

// Require returns the gate error or nil.
func (g Gate) Require(c *utils.UserClaims) error {
	if c == nil || (g.NeedTenant && c.TenantID == "") {
		code := g.Code
		if code == 0 {
			code = errcode.CodeNoPermission
		}
		return errcode.NewWithMessage(code, g.Msg)
	}
	return nil
}
