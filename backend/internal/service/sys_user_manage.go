// 文件用途：系统用户查询、删除与租户初始化状态服务（用户生命周期编排的剩余聚合）。
// 核心逻辑：列表按登录人作用域（userListScopes）收口；删除先过管理权限再撤销 casbin
// 角色绑定、最后删主记录；租户初始化状态聚合超管/租户管理员/租户与市场注册入口。
// 创建与更新事务分别拆到 sys_user_create.go / sys_user_update.go。
// 关键注意事项：GetUserListByPage 空页显式返回 []（分页契约）；DeleteUser 拒绝删除
// SYS_ADMIN（cannot_delete_sys_admin），且角色绑定撤销先于主记录删除以降低残留授权风险。
package service

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
)

// GetUserById 按用户 ID 查询完整用户资料。
func (*User) GetUserById(id string) (*model.User, error) {
	user, err := dal.GetUsersById(id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// userListScopes 解析用户目录列表读作用域（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only（成员目录仅本租户）；TENANT_ADMIN → expandTenantIDScope
// （self∪子孙，可读层级内成员用户）；SYS_ADMIN → nil（平台级管理员目录，无租户过滤）；
// nil/未知声明 → nil，由 DAL 显式拒绝。
func userListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	switch claims.Authority {
	case constant.TENANT_USER:
		if tenantID := strings.TrimSpace(claims.TenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	case constant.TENANT_ADMIN:
		tenantID := strings.TrimSpace(claims.TenantID)
		if tenantID == "" {
			return nil
		}
		return expandTenantIDScope(tenantID)
	default:
		return nil
	}
}

// GetUserListByPage 按当前登录人的权限边界分页查询用户列表。
func (*User) GetUserListByPage(userListReq *model.UserListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetUserListByPage(userListReq, claims, userListScopes(claims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"error":     err.Error(),
		})
	}
	userListRspMap := make(map[string]interface{})
	userListRspMap["total"] = total
	// 空页显式返回 [] 而非 nil：nil 经 JSON 序列化会变成 null，破坏前端与
	// 自动化断言的分页契约（与 GetDeviceConfigListByPage 的处理保持一致）。
	if list == nil {
		list = make([]map[string]interface{}, 0)
	}
	userListRspMap["list"] = list
	return userListRspMap, nil
}

// DeleteUser 删除普通用户并清理其 Casbin 角色绑定。
func (*User) DeleteUser(id string, claims *utils.UserClaims) error {
	// 删除前先加载用户，用于权限判断并避免误删系统管理员。
	user, err := dal.GetUsersById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": id,
		})
	}

	if err := ensureUserManagementWriteAccess(user, claims, "delete_user"); err != nil {
		return err
	}

	if SafeDeref(user.Authority) == constant.SYS_ADMIN {
		return errcode.WithVars(errcode.CodeOpDenied, map[string]interface{}{
			"reason":  "cannot_delete_sys_admin",
			"user_id": id,
		})
	}

	if err := revokeUserRoleBindings(id); err != nil {
		return err
	}

	// 角色绑定已撤销后再删除用户主记录，降低残留授权风险。
	err = dal.DeleteUsersById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"user_id":   id,
			"operation": "delete_user",
		})
	}

	return nil
}

func (u *User) GetUserEmailByPhoneNumber(phoneNumber string) (string, error) {
	// 手机号登录场景需要先反查邮箱，未命中时返回业务错误码。
	user, err := dal.GetUsersByPhoneNumber(phoneNumber)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errcode.New(200013)
		}
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"message": "get_user_by_phone_number",
			"error":   err.Error(),
		})
	}
	return user.Email, nil
}

func (u *User) CheckSysAdminExists() (bool, error) {
	users := query.User
	userList, err := users.Where(users.Authority.Eq("SYS_ADMIN")).Find()
	if err != nil {
		return false, err
	}
	return len(userList) > 0, nil
}

func (u *User) GetTenantSetupState() (*model.TenantSetupStateRsp, error) {
	hasAdmin, err := u.CheckSysAdminExists()
	if err != nil {
		return nil, err
	}
	hasTenantAdmin, hasTenant, err := u.CheckTenantAdminSetupExists()
	if err != nil {
		return nil, err
	}

	baseURL := getMarketBaseURL()
	return buildTenantSetupState(hasAdmin, hasTenantAdmin, hasTenant, baseURL), nil
}

func (u *User) CheckTenantAdminSetupExists() (bool, bool, error) {
	users := query.User
	userList, err := users.Where(users.Authority.Eq(constant.TENANT_ADMIN)).Find()
	if err != nil {
		return false, false, err
	}

	hasTenantAdmin := len(userList) > 0
	hasTenant := false
	for _, user := range userList {
		if strings.TrimSpace(SafeDeref(user.TenantID)) != "" {
			hasTenant = true
			break
		}
	}
	return hasTenantAdmin, hasTenant, nil
}

func buildTenantSetupState(hasAdmin bool, hasTenantAdmin bool, hasTenant bool, baseURL string) *model.TenantSetupStateRsp {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	entry := "login"
	nextStep := "login"
	if !hasAdmin {
		entry = "register"
		nextStep = "create_super_admin"
	} else if !hasTenantAdmin || !hasTenant {
		nextStep = "create_tenant_admin"
	}
	marketRegisterURL := ""
	if baseURL != "" {
		marketRegisterURL = baseURL + "/register"
	}

	return &model.TenantSetupStateRsp{
		HasAdmin:          hasAdmin,
		HasTenantAdmin:    hasTenantAdmin,
		HasTenant:         hasTenant,
		Entry:             entry,
		NextStep:          nextStep,
		MarketBaseURL:     baseURL,
		MarketRegisterURL: marketRegisterURL,
	}
}

// GetTenantInfo 按租户 ID 读取租户对应的用户资料。
func (u *User) GetTenantInfo(tenantID string) (*model.User, error) {
	tenant, err := dal.GetTenantsById(tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
	}
	return tenant, nil
}

func (*User) GetUserSelector(req *model.UserSelectorReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetUserSelector(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "get_user_selector",
			"error":     err.Error(),
		})
	}

	result := make(map[string]interface{})
	result["total"] = total
	result["list"] = list
	return result, nil
}
