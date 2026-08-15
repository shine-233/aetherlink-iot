// 文件用途：维护设备分组树和设备分组关系服务。
// 核心逻辑：处理分组 CRUD、树结构构建、成员关系更新和租户范围内的访问校验。
// 关键注意事项：分组 ID 是租户边界，修改成员关系前必须同时校验分组和设备归属。
// 重构建议：拆分树构建、成员授权和关系持久化，补齐跨租户、循环结构和事务回滚测试。
// device_group.go maintains hierarchical device groups and memberships.
//
// Purpose: create, update, delete, list, and resolve device groups plus group-device relations for tenant-scoped device organization.
// Core logic: validates read/write access to groups, builds tree responses, and delegates membership persistence to DAL helpers.
// Important notes: group IDs are a tenant boundary, so relation changes must verify both group and device access before mutating memberships.
// Refactor suggestion: split tree-building and membership authorization into dedicated helpers as group features grow.
package service

import (
	"errors"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type DeviceGroup struct{}

type TreeNode struct {
	Group    *model.Group `json:"group"`
	Children []*TreeNode  `json:"children,omitempty"`
}

const deviceGroupDuplicateRelationMessage = "重复键违反唯一约束"

func mapDeviceGroupRelationWriteError(err error) error {
	if err == nil {
		return nil
	}
	if isPostgresUniqueViolation(err) {
		return errcode.NewWithMessage(errcode.CodeSystemError, deviceGroupDuplicateRelationMessage)
	}
	return err
}

func isPostgresUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}

func canCreateDeviceGroupWithCurrentVisibilityModel(claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create device group")
	}
	return nil
}

func createdGroupOwnerUserID(claims *utils.UserClaims) *string {
	return createdDeviceOwnerUserID(claims)
}

func ensureDeviceGroupReadAccess(groupID string, claims *utils.UserClaims) (*model.Group, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device group")
	}
	group, err := dal.GetDeviceGroupDetail(groupID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":    err.Error(),
			"group_id": groupID,
		})
	}
	if group == nil {
		return nil, errcode.WithVars(errcode.CodeNotFound, map[string]interface{}{
			"error":    "device_group_not_found",
			"group_id": groupID,
		})
	}
	if claims.Authority != constant.SYS_ADMIN && group.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device group")
	}
	if claims.Authority == constant.TENANT_USER {
		visible, err := dal.IsGroupVisibleToOwner(groupID, claims.TenantID, claims.ID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error":    err.Error(),
				"group_id": groupID,
			})
		}
		if !visible {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device group")
		}
	}
	return group, nil
}

func ensureDeviceGroupWriteAccess(groupID string, claims *utils.UserClaims) (*model.Group, error) {
	group, err := ensureDeviceGroupReadAccess(groupID, claims)
	if err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN && group.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device group")
	}
	return group, nil
}

func (*DeviceGroup) CreateDeviceGroup(req model.CreateDeviceGroupReq, claims *utils.UserClaims) error {
	if err := canCreateDeviceGroupWithCurrentVisibilityModel(claims); err != nil {
		return err
	}
	var deviceGroup = model.Group{}
	t := time.Now().UTC()
	deviceGroup.ID = uuid.New()
	groupTenantID := claims.TenantID

	// 处理子分组创建
	if req.ParentId != nil && *req.ParentId != "0" {
		deviceGroup.ParentID = req.ParentId

		// 父分组存在性与权限验证
		parentGroup, err := ensureDeviceGroupWriteAccess(*req.ParentId, claims)
		if err != nil {
			return err
		}
		if parentGroup == nil {
			return errcode.WithVars(errcode.CodeNotFound, map[string]interface{}{
				"error":     "parent_group_not_found",
				"parent_id": *req.ParentId,
			})
		}
		groupTenantID = parentGroup.TenantID
	}

	// 验证租户下分组名称不能重复（无论层级）
	g, err := dal.GetGroupNameExistByTenant(req.Name, groupTenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":      err.Error(),
			"group_name": req.Name,
			"tenant_id":  groupTenantID,
		})
	}
	if g != nil {
		return errcode.WithVars(203003, map[string]interface{}{
			"group_name": req.Name,
		})
	}

	// 设置分组基本信息
	deviceGroup.Tier = -1 // 暂时不计算层级
	deviceGroup.Description = req.Description
	deviceGroup.CreatedAt = t
	deviceGroup.UpdatedAt = t
	deviceGroup.Name = req.Name
	deviceGroup.Remark = req.Remark
	deviceGroup.TenantID = groupTenantID
	deviceGroup.OwnerUserID = createdGroupOwnerUserID(claims)

	// 创建分组
	if err := dal.CreateDeviceGroup(&deviceGroup); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"group_id":  deviceGroup.ID,
			"tenant_id": groupTenantID,
		})
	}

	return nil
}

func (*DeviceGroup) DeleteDeviceGroup(id string, claims *utils.UserClaims) error {
	if _, err := ensureDeviceGroupWriteAccess(id, claims); err != nil {
		return err
	}
	return dal.DeleteDeviceGroup(id)
}

func (*DeviceGroup) UpdateDeviceGroup(req model.UpdateDeviceGroupReq, claims *utils.UserClaims) error {
	group, err := ensureDeviceGroupWriteAccess(req.Id, claims)
	if err != nil {
		return err
	}
	// 验证分组是否冲突
	if req.Id == req.ParentId {
		return errcode.WithVars(errcode.CodeParamError, map[string]interface{}{
			"error":     "group_id_conflict",
			"message":   "old group id is same as new group id",
			"group_id":  req.Id,
			"parent_id": req.ParentId,
		})
	}
	if req.ParentId != "" && req.ParentId != "0" {
		parentGroup, err := ensureDeviceGroupWriteAccess(req.ParentId, claims)
		if err != nil {
			return err
		}
		if parentGroup.TenantID != group.TenantID {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "group and parent group tenant mismatch")
		}
	}

	// 构建更新对象
	var deviceGroup = model.Group{
		ID:          req.Id,
		ParentID:    &req.ParentId,
		UpdatedAt:   time.Now(),
		Name:        req.Name,
		Remark:      req.Remark,
		Description: req.Description,
		TenantID:    group.TenantID,
	}

	// 更新数据库
	if err := dal.UpdateDeviceGroup(&deviceGroup); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"group_id":  req.Id,
			"tenant_id": group.TenantID,
		})
	}

	return nil
}

func (*DeviceGroup) GetDeviceGroupListByPage(req model.GetDeviceGroupsListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
	total, list, err := dal.GetDeviceGroupListByPage(req, userClaims.TenantID, deviceOwnerUserIDFilterForClaims(userClaims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": userClaims.TenantID,
			"page":      req.Page,
			"page_size": req.PageSize,
		})
	}
	deviceGroupList := make(map[string]interface{})
	deviceGroupList["total"] = total
	deviceGroupList["list"] = list

	return deviceGroupList, err

}

func (*DeviceGroup) GetDeviceGroupByTree(userClaims *utils.UserClaims) (interface{}, error) {
	data, err := dal.GetDeviceGroupAll(userClaims.TenantID, deviceOwnerUserIDFilterForClaims(userClaims))
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": userClaims.TenantID,
		}), nil
	}

	return buildDeviceGroupTree(data), nil
}

// buildDeviceGroupTree preserves DAL order, treats nil and "0" parents as
// roots, and intentionally omits nodes whose referenced parent is absent.
func buildDeviceGroupTree(groups []*model.Group) []*TreeNode {
	nodeMap := make(map[string]*TreeNode, len(groups))
	rootNodes := make([]*TreeNode, 0, len(groups))

	for _, group := range groups {
		nodeMap[group.ID] = &TreeNode{Group: group}
	}

	for _, group := range groups {
		node := nodeMap[group.ID]
		if node.Group.ParentID == nil || *node.Group.ParentID == "0" {
			rootNodes = append(rootNodes, node)
		} else if parentNode, ok := nodeMap[*node.Group.ParentID]; ok {
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	return rootNodes
}

func (*DeviceGroup) GetDeviceGroupDetail(id string, claims *utils.UserClaims) (interface{}, error) {

	dataMap := make(map[string]interface{})

	data, err := ensureDeviceGroupReadAccess(id, claims)
	if err != nil {
		return nil, err
	}

	tier, err := dal.GetDeviceGroupTierById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":    err.Error(),
			"group_id": id,
		}), nil
	}

	statistics, err := dal.GetDeviceGroupStatistics(id, data.TenantID, deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":    err.Error(),
			"group_id": id,
		}), nil
	}

	dataMap["detail"] = data
	dataMap["tier"] = tier
	dataMap["statistics"] = statistics

	return dataMap, nil
}

func (*DeviceGroup) CreateDeviceGroupRelation(req model.CreateDeviceGroupRelationReq, claims *utils.UserClaims) error {
	group, err := ensureDeviceGroupWriteAccess(req.GroupId, claims)
	if err != nil {
		return err
	}
	var dataList = []*model.RGroupDevice{}
	for _, v := range req.DeviceIDList {
		device, err := ensureTelemetryDeviceWriteAccess(v, claims)
		if err != nil {
			return err
		}
		if device.TenantID != group.TenantID {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "device and group tenant mismatch")
		}
		var deviceGroupRelation = model.RGroupDevice{}
		deviceGroupRelation.DeviceID = v
		deviceGroupRelation.GroupID = req.GroupId
		deviceGroupRelation.TenantID = group.TenantID
		dataList = append(dataList, &deviceGroupRelation)
	}
	// 批量创建
	if err := dal.BatchCreateRGroupDevice(dataList); err != nil {
		return mapDeviceGroupRelationWriteError(err)
	}
	return nil
}

func (*DeviceGroup) DeleteDeviceGroupRelation(group_id, device_id string, claims *utils.UserClaims) error {
	group, err := ensureDeviceGroupWriteAccess(group_id, claims)
	if err != nil {
		return err
	}
	device, err := ensureTelemetryDeviceWriteAccess(device_id, claims)
	if err != nil {
		return err
	}
	if device.TenantID != group.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "device and group tenant mismatch")
	}
	err = dal.DeleteRGroupDevice(group_id, device_id)
	return err
}

func (*DeviceGroup) GetDeviceGroupRelation(req model.GetDeviceListByGroup, claims *utils.UserClaims) (interface{}, error) {
	group, err := ensureDeviceGroupReadAccess(req.GroupId, claims)
	if err != nil {
		return nil, err
	}
	total, list, err := dal.GetRGroupDeviceByGroupId(req, group.TenantID, deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return nil, err
	}
	devicesList := make(map[string]interface{})
	devicesList["total"] = total
	devicesList["list"] = list

	return devicesList, err
}

func (*DeviceGroup) GetDeviceGroupByDeviceId(device_id string, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(device_id, claims); err != nil {
		return nil, err
	}
	var rspData = []map[string]interface{}{}
	data, err := dal.GetRGroupDeviceByDeviceId(device_id)
	// 分组名称处理成 parent/child/current 这种层级路径。
	for i := range data {
		tier, err := dal.GetDeviceGroupTierById(data[i].GroupID)
		if err != nil {
			return nil, err
		}
		rspData = append(rspData, map[string]interface{}{
			"group_id": data[i].GroupID,
			"tier":     tier["group_path"],
		})
	}

	return rspData, err
}
