// 文件用途：维护角色、权限菜单和用户授权相关服务。
// 核心逻辑：处理角色 CRUD、权限绑定、角色列表和用户管理流程需要的角色校验。
// 关键注意事项：角色服务是权限边界，跨租户角色、默认角色和删除级联需谨慎处理。
// 重构建议：拆分角色仓储与授权策略接口，补齐权限、事务、Casbin 同步和删除副作用测试。
package service

import (
	"fmt"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type Role struct{}

func requireRoleManager(claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage roles")
	}
	if claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage roles")
	}
	return nil
}

func ensureRoleWriteAccess(id string, claims *utils.UserClaims) (model.Role, error) {
	role, err := dal.GetRoleByID(id)
	if err != nil {
		return role, err
	}
	if role.ID == "" {
		return role, errcode.NewWithMessage(errcode.CodeNoPermission, "role not found or no permission")
	}
	if err := requireRoleManager(claims); err != nil {
		return role, err
	}
	if claims.Authority != constant.SYS_ADMIN {
		if role.TenantID == nil || *role.TenantID != claims.TenantID {
			return role, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage another tenant role")
		}
	}
	return role, nil
}

func (*Role) CreateRole(createRoleReq *model.CreateRoleReq, userClaims *utils.UserClaims) error {
	if err := requireRoleManager(userClaims); err != nil {
		return err
	}

	var role = model.Role{}

	role.ID = uuid.New()
	role.Name = createRoleReq.Name
	role.Description = createRoleReq.Description

	t := time.Now().UTC()
	role.CreatedAt = &t
	role.UpdatedAt = &t
	role.TenantID = &userClaims.TenantID

	err := dal.CreateRole(&role)

	if err != nil {
		logrus.Error(err)
	}

	return err
}

func (*Role) UpdateRole(updateRoleReq *model.UpdateRoleReq, userClaims *utils.UserClaims) (model.Role, error) {
	oldRole, err := ensureRoleWriteAccess(updateRoleReq.Id, userClaims)
	if err != nil {
		return oldRole, err
	}

	var role = model.Role{}
	role.ID = updateRoleReq.Id
	if updateRoleReq.Description != nil {
		role.Description = updateRoleReq.Description
	}
	if updateRoleReq.Name != "" {
		role.Name = updateRoleReq.Name
	}
	tenantID := ""
	if oldRole.TenantID != nil {
		tenantID = *oldRole.TenantID
	}
	info, err := dal.UpdateRole(&role, tenantID)
	if err != nil {
		logrus.Error(err)
	}

	if info.RowsAffected == 0 {
		return role, fmt.Errorf("no data updated")
	}

	role, err = dal.GetRoleByID(role.ID)
	if err != nil {
		logrus.Error(err)
	}

	return role, err
}

func (*Role) DeleteRole(id string, userClaims *utils.UserClaims) error {
	role, err := ensureRoleWriteAccess(id, userClaims)
	if err != nil {
		return err
	}
	tenantID := ""
	if role.TenantID != nil {
		tenantID = *role.TenantID
	}
	err = dal.DeleteRole(id, tenantID)
	return err
}

func (*Role) GetRoleListByPage(params *model.GetRoleListByPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {

	total, list, err := dal.GetRoleListByPage(params, userClaims.TenantID)
	if err != nil {
		return nil, err
	}
	roleListRsp := make(map[string]interface{})
	roleListRsp["total"] = total
	roleListRsp["list"] = list

	return roleListRsp, err
}
