package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	constant "aetherlink-iot/backend/pkg/constant"
	errcode "aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type RolePermissionService struct{}

var RolePermission RolePermissionService

// ListPermissions 获取系统标准权限字典
func (RolePermissionService) ListPermissions(ctx context.Context, module string, claims *utils.UserClaims) ([]model.SysPermission, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	return dal.ListPermissions(ctx, module)
}

// GetRolePermissions 获取指定角色的权限配置
func (RolePermissionService) GetRolePermissions(ctx context.Context, roleID string, claims *utils.UserClaims) (*model.RolePermissionsResp, error) {
	role, err := ensureRoleWriteAccess(roleID, claims)
	if err != nil {
		return nil, err
	}

	tenantID := ""
	if role.TenantID != nil {
		tenantID = *role.TenantID
	}

	rolePerms, err := dal.GetRolePermissions(ctx, roleID, tenantID)
	if err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(rolePerms))
	for _, rp := range rolePerms {
		codes = append(codes, rp.PermissionCode)
	}

	perms, err := dal.GetPermissionsByCodes(ctx, codes)
	if err != nil {
		return nil, err
	}

	return &model.RolePermissionsResp{
		RoleID:      role.ID,
		RoleName:    role.Name,
		TenantID:    tenantID,
		Codes:       codes,
		Permissions: perms,
	}, nil
}

// AssignRolePermissions 为指定角色分配权限点并同步 Casbin p 策略
func (RolePermissionService) AssignRolePermissions(ctx context.Context, roleID string, req *model.AssignRolePermissionsReq, claims *utils.UserClaims) error {
	role, err := ensureRoleWriteAccess(roleID, claims)
	if err != nil {
		return err
	}

	tenantID := ""
	if role.TenantID != nil {
		tenantID = *role.TenantID
	}

	if req == nil {
		req = &model.AssignRolePermissionsReq{PermissionCodes: []string{}}
	}

	// 校验所有传入的权限 code 是否合法
	if len(req.PermissionCodes) > 0 {
		perms, err := dal.GetPermissionsByCodes(ctx, req.PermissionCodes)
		if err != nil {
			return err
		}
		if len(perms) != len(req.PermissionCodes) {
			foundMap := make(map[string]bool, len(perms))
			for _, p := range perms {
				foundMap[p.Code] = true
			}
			var missing []string
			for _, c := range req.PermissionCodes {
				if !foundMap[c] {
					missing = append(missing, c)
				}
			}
			return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("invalid permission codes: %s", strings.Join(missing, ", ")))
		}
	}

	// 1. 在数据库事务中原子替换角色权限
	if err := dal.ReplaceRolePermissions(ctx, roleID, tenantID, req.PermissionCodes); err != nil {
		logrus.Errorf("failed to replace role permissions: %v", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	// 2. 收集所有相关的 API 匹配模式
	var allPatterns []string
	if len(req.PermissionCodes) > 0 {
		perms, err := dal.GetPermissionsByCodes(ctx, req.PermissionCodes)
		if err == nil {
			for _, p := range perms {
				var patterns []string
				if err := json.Unmarshal(p.ApiPatterns, &patterns); err == nil {
					allPatterns = append(allPatterns, patterns...)
				}
			}
		}
	}

	// 3. 同步 Casbin p 策略
	if global.CasbinEnforcer != nil {
		// 先移除该角色旧有的 p 策略
		_, _ = global.CasbinEnforcer.RemoveFilteredNamedPolicy("p", 0, roleID)

		// 增加新的 p 策略
		if len(allPatterns) > 0 {
			var newPolicies [][]string
			seen := make(map[string]bool)
			for _, pat := range allPatterns {
				pat = strings.TrimSpace(pat)
				if pat == "" || seen[pat] {
					continue
				}
				seen[pat] = true
				newPolicies = append(newPolicies, []string{roleID, pat, "allow"})
			}
			if len(newPolicies) > 0 {
				_, _ = global.CasbinEnforcer.AddNamedPolicies("p", newPolicies)
			}
		}
		_ = global.CasbinEnforcer.LoadPolicy()
	}

	return nil
}

// GetRoleUsers 获取分配了该角色的用户列表
func (RolePermissionService) GetRoleUsers(ctx context.Context, roleID string, claims *utils.UserClaims) (*model.RoleUsersResp, error) {
	role, err := ensureRoleWriteAccess(roleID, claims)
	if err != nil {
		return nil, err
	}

	tenantID := ""
	if role.TenantID != nil {
		tenantID = *role.TenantID
	}

	users, err := dal.GetUsersByRoleID(ctx, roleID, tenantID)
	if err != nil {
		return nil, err
	}

	return &model.RoleUsersResp{
		RoleID:   role.ID,
		RoleName: role.Name,
		Users:    users,
	}, nil
}

// AssignRoleUsers 为角色批量绑定用户（同步 Casbin g 规则）
func (RolePermissionService) AssignRoleUsers(ctx context.Context, roleID string, req *model.AssignRoleUsersReq, claims *utils.UserClaims) error {
	role, err := ensureRoleWriteAccess(roleID, claims)
	if err != nil {
		return err
	}

	tenantID := ""
	if role.TenantID != nil {
		tenantID = *role.TenantID
	}

	if req == nil {
		req = &model.AssignRoleUsersReq{UserIDs: []string{}}
	}

	// 校验所有目标用户是否存在且属于同一租户
	if len(req.UserIDs) > 0 {
		var count int64
		q := global.DB.WithContext(ctx).Table(model.TableNameUser).Where("id IN ?", req.UserIDs)
		if claims.Authority != constant.SYS_ADMIN && tenantID != "" {
			q = q.Where("tenant_id = ?", tenantID)
		}
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if int(count) != len(req.UserIDs) {
			return errcode.NewWithMessage(errcode.CodeParamError, "one or more users not found or do not belong to the tenant")
		}
	}

	// 更新 Casbin g 绑定
	if global.CasbinEnforcer != nil {
		// 1. 先查出当前所有拥有该角色的用户并移除对应的绑定
		currUsers, _ := dal.GetUsersByRoleID(ctx, roleID, tenantID)
		for _, u := range currUsers {
			_, _ = global.CasbinEnforcer.RemoveNamedGroupingPolicy("g", u.ID, roleID)
		}

		// 2. 为新用户列表增加 g 规则
		if len(req.UserIDs) > 0 {
			var rules [][]string
			for _, uid := range req.UserIDs {
				rules = append(rules, []string{uid, roleID})
			}
			_, _ = global.CasbinEnforcer.AddNamedGroupingPolicies("g", rules)
		}
		_ = global.CasbinEnforcer.LoadPolicy()
	}

	return nil
}
