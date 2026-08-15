// 文件用途：维护用户管理中的授权、角色绑定和 Casbin 同步。
// 核心逻辑：校验目标用户与操作者权限，添加、替换、恢复和撤销角色绑定。
// 关键注意事项：这是用户越权防线，资料写入、角色绑定和清理失败的顺序必须明确。
// 重构建议：拆分授权策略、角色绑定仓储和补偿逻辑，补齐事务、Casbin 故障和恢复失败测试。
package service

import (
	"fmt"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

func userClaimsAuthority(claims *utils.UserClaims) string {
	if claims == nil {
		return ""
	}
	return claims.Authority
}

func ensureUserTransformAccess(target *model.User, claims *utils.UserClaims) error {
	if target == nil || claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to transform user")
	}
	if claims.Authority == constant.SYS_ADMIN {
		return nil
	}
	if claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to transform user")
	}
	if SafeDeref(target.TenantID) != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to transform cross-tenant user")
	}
	if SafeDeref(target.Authority) != constant.TENANT_USER {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "tenant admin can only transform tenant users")
	}
	return nil
}

func ensureUserManagementWriteAccess(target *model.User, claims *utils.UserClaims, operation string) error {
	if target == nil || claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage users")
	}
	targetAuthority := SafeDeref(target.Authority)
	targetTenantID := SafeDeref(target.TenantID)

	switch claims.Authority {
	case constant.SYS_ADMIN:
		if targetAuthority == constant.SYS_ADMIN {
			return errcode.WithVars(errcode.CodeOpDenied, map[string]interface{}{
				"reason":    "cannot_manage_sys_admin",
				"user_id":   target.ID,
				"operation": operation,
			})
		}
		return nil
	case constant.TENANT_ADMIN:
		if targetTenantID != claims.TenantID {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"required_tenant": targetTenantID,
				"current_tenant":  claims.TenantID,
				"operation":       operation,
			})
		}
		if targetAuthority != constant.TENANT_USER {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "tenant admin can only manage tenant users")
		}
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage users")
	}
}

func ensureAssignableUserRoles(roleIDs []string, target *model.User, claims *utils.UserClaims) error {
	if len(roleIDs) == 0 {
		return nil
	}
	if target == nil || claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to assign user roles")
	}

	targetTenantID := SafeDeref(target.TenantID)
	if targetTenantID == "" {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "cannot assign tenant roles to a user without tenant")
	}
	if claims.Authority == constant.TENANT_ADMIN && targetTenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to assign roles across tenants")
	}
	if claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to assign user roles")
	}

	for _, roleID := range roleIDs {
		role, err := dal.GetRoleByID(roleID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"operation": "query_role",
				"role_id":   roleID,
				"error":     err.Error(),
			})
		}
		if role.ID == "" {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"reason":  "role_not_found_or_not_assignable",
				"role_id": roleID,
			})
		}
		if role.TenantID == nil || *role.TenantID != targetTenantID {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"reason":        "role_tenant_mismatch",
				"role_id":       roleID,
				"role_tenant":   SafeDeref(role.TenantID),
				"target_tenant": targetTenantID,
			})
		}
	}

	return nil
}

func ensureCasbinRoleMutationReady(roleIDs []string) error {
	if len(roleIDs) == 0 {
		return nil
	}
	if global.CasbinEnforcer == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "mutate_user_roles",
			"error":     "casbin enforcer is not initialized",
		})
	}
	return nil
}

func ensureCasbinUserRoleMutationReady(changeRequested bool) error {
	if !changeRequested {
		return nil
	}
	if global.CasbinEnforcer == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "mutate_user_roles",
			"error":     "casbin enforcer is not initialized",
		})
	}
	return nil
}

func addUserRoleBindings(userID string, roleIDs []string) error {
	if len(roleIDs) == 0 {
		return nil
	}
	ok, err := GroupApp.Casbin.AddRolesToUserWithError(userID, roleIDs)
	if err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "add_user_roles",
			"user_id":   userID,
			"role_ids":  roleIDs,
			"error":     err.Error(),
		})
	}
	if !ok {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "add_user_roles",
			"user_id":   userID,
			"role_ids":  roleIDs,
			"error":     "failed to add roles to user",
		})
	}
	return nil
}

func replaceUserRoleBindings(userID string, roleIDs []string) ([]string, error) {
	oldRoles, _ := GroupApp.Casbin.GetRoleFromUser(userID)
	if _, err := GroupApp.Casbin.RemoveUserAndRoleWithError(userID); err != nil {
		return oldRoles, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "remove_user_roles",
			"user_id":   userID,
			"error":     err.Error(),
		})
	}

	if err := addUserRoleBindings(userID, roleIDs); err != nil {
		if restoreErr := restoreUserRoleBindings(userID, oldRoles); restoreErr != nil {
			return oldRoles, fmt.Errorf("%w; restore roles failed: %v", err, restoreErr)
		}
		return oldRoles, err
	}
	return oldRoles, nil
}

func restoreUserRoleBindings(userID string, roleIDs []string) error {
	if _, err := GroupApp.Casbin.RemoveUserAndRoleWithError(userID); err != nil {
		return err
	}
	return addUserRoleBindings(userID, roleIDs)
}

func replaceUserRoleBindingsWithTx(tx *query.Query, userID string, roleIDs []string) error {
	if _, err := tx.CasbinRule.Where(tx.CasbinRule.Ptype.Eq("g"), tx.CasbinRule.V0.Eq(userID)).Delete(); err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}

	rules := make([]*model.CasbinRule, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		rules = append(rules, &model.CasbinRule{
			Ptype: StringPtr("g"),
			V0:    StringPtr(userID),
			V1:    StringPtr(roleID),
		})
	}
	return tx.CasbinRule.Create(rules...)
}

func reloadCasbinPolicyAfterRoleTransaction() error {
	if global.CasbinEnforcer == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "reload_user_roles",
			"error":     "casbin enforcer is not initialized",
		})
	}
	if global.CasbinEnforcer.GetAdapter() == nil {
		return nil
	}
	if err := global.CasbinEnforcer.LoadPolicy(); err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "reload_user_roles",
			"error":     err.Error(),
		})
	}
	return nil
}

func cleanupCreatedUserAfterRoleBindingFailure(userID string) error {
	if global.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if err := global.DB.Where("user_id = ?", userID).Delete(&model.UserAddress{}).Error; err != nil {
		return err
	}
	if err := global.DB.Where("id = ?", userID).Delete(&model.User{}).Error; err != nil {
		return err
	}
	return nil
}

func revokeUserRoleBindings(userID string) error {
	if global.CasbinEnforcer == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "remove_user_roles",
			"user_id":   userID,
			"error":     "casbin enforcer is not initialized",
		})
	}
	if _, err := GroupApp.Casbin.RemoveUserAndRoleWithError(userID); err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "remove_user_roles",
			"user_id":   userID,
			"error":     err.Error(),
		})
	}
	return nil
}
