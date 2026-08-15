package service

import (
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func ensureTenantScopedWriteClaims(claims *utils.UserClaims, action string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to "+action)
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "complete tenant initialization before "+action)
	}
	return nil
}
