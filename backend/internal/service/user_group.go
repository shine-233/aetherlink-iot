// 文件用途：用户组与组权限（TB-46 GPE v1）服务层——组 CRUD、成员管理、组权限元素绑定。
// 核心逻辑：管理员权限收口（requireUserGroupManager）+ 租户边界校验（组归属租户双条件）+
//
//	元素码合法性校验（资源存在且同租户）后调用 DAL 完成持久化。
//
// 关键注意事项：组权限元素码是"用户所属组→组权限→资源可见映射"的映射源，绑定前必须
//
//	fail-closed 校验元素可解析且资源落在组的租户作用域内；成员必须是 users 表内
//	登录账号（customer 客户不接入组授权，ROADMAP v1 边界）；组管理能力仅管理员，
//	组成员身份只带来资源可见性（见 group_sharing.go），不带来本组管理能力。
//
// 重构建议：成员/权限批量绑定当前为"全量替换"语义，若前端需要增量勾选，
//
//	在 service 层加 diff（keep/add/remove）而不是改 DAL 的事务替换原语。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	constant "aetherlink-iot/backend/pkg/constant"
	errcode "aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// UserGroup 用户组服务（TB-46 GPE v1）。
type UserGroup struct{}

// requireUserGroupManager 组管理能力仅 SYS_ADMIN / TENANT_ADMIN（与角色管理同口径）。
func requireUserGroupManager(claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage user groups")
	}
	if claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage user groups")
	}
	return nil
}

// ensureUserGroupWriteAccess 读取目标组并校验调用者的租户边界：
// TENANT_ADMIN 仅能管理本租户组（越权与不存在同返 404，不泄露存在性）；
// SYS_ADMIN 为平台管理员，可管理任意租户组。
func ensureUserGroupWriteAccess(id string, claims *utils.UserClaims) (*model.UserGroup, error) {
	if err := requireUserGroupManager(claims); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "group id is required")
	}
	group, err := dal.GetUserGroupByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if claims.Authority != constant.SYS_ADMIN && group.TenantID != claims.TenantID {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	return group, nil
}

// resolveUserGroupTenant 解析组的归属租户：TENANT_ADMIN 强制本租户；
// SYS_ADMIN 必须显式指定目标租户（用户组必须有租户归属，fail-closed）。
func resolveUserGroupTenant(requestedTenantID string, claims *utils.UserClaims) (string, error) {
	requested := strings.TrimSpace(requestedTenantID)
	if claims.Authority == constant.SYS_ADMIN {
		if requested == "" {
			return "", errcode.NewWithMessage(errcode.CodeParamError, "tenant_id is required for platform admin")
		}
		// GetTenantByID 未命中返回 (nil, nil)，须显式判 nil（fail-closed）。
		tenant, err := dal.GetTenantByID(requested)
		if err != nil {
			return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if tenant == nil {
			return "", errcode.NewWithMessage(errcode.CodeNotFound, "tenant not found")
		}
		return requested, nil
	}
	if claims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if requested != "" && requested != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage another tenant user group")
	}
	return claims.TenantID, nil
}

// resolveUserGroupListScopes 组列表作用域：TENANT_ADMIN 本租户 self∪子孙；
// SYS_ADMIN 显式请求租户→展开，未请求→nil（与看板列表一致的"管理员全量视图"语义）。
func resolveUserGroupListScopes(requestedTenantID string, claims *utils.UserClaims) ([]string, error) {
	if err := requireUserGroupManager(claims); err != nil {
		return nil, err
	}
	requested := strings.TrimSpace(requestedTenantID)
	if claims.Authority == constant.SYS_ADMIN {
		if requested == "" {
			return nil, nil
		}
		tenant, err := dal.GetTenantByID(requested)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if tenant == nil {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "tenant not found")
		}
		return expandTenantIDScope(requested), nil
	}
	if claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if requested != "" && requested != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query another tenant user group")
	}
	return expandTenantIDScope(claims.TenantID), nil
}

// CreateUserGroup 创建用户组（SYS_ADMIN 可经 req.TenantID 指定目标租户）。
func (*UserGroup) CreateUserGroup(req *model.CreateUserGroupReq, claims *utils.UserClaims) (*model.UserGroup, error) {
	if err := requireUserGroupManager(claims); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "group name is required")
	}
	tenantID, err := resolveUserGroupTenant(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	if req.Description != nil && strings.TrimSpace(*req.Description) == "" {
		req.Description = nil
	}
	if exists, err := dal.GetUserGroupNameExists(req.Name, tenantID, ""); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	} else if exists {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "group name already exists in tenant")
	}
	now := time.Now().UTC()
	group := &model.UserGroup{
		ID:          uuid.New(),
		Name:        strings.TrimSpace(req.Name),
		TenantID:    tenantID,
		Description: req.Description,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := dal.CreateUserGroup(group); err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return group, nil
}

// UpdateUserGroup 更新组名称/描述（至少一项）。
func (*UserGroup) UpdateUserGroup(req *model.UpdateUserGroupReq, claims *utils.UserClaims) (*model.UserGroup, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "nothing to update")
	}
	group, err := ensureUserGroupWriteAccess(req.ID, claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" && req.Description == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "nothing to update")
	}
	if strings.TrimSpace(req.Name) != "" {
		if exists, err := dal.GetUserGroupNameExists(req.Name, group.TenantID, group.ID); err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		} else if exists {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "group name already exists in tenant")
		}
	}
	updated := &model.UserGroup{ID: group.ID, TenantID: group.TenantID, Name: strings.TrimSpace(req.Name), Description: req.Description}
	ok, err := dal.UpdateUserGroupForTenant(updated)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if !ok {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	fresh, err := dal.GetUserGroupForTenant(group.ID, group.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return fresh, nil
}

// DeleteUserGroup 删除组（DAL 同事务清理成员与权限绑定行）。
func (*UserGroup) DeleteUserGroup(id string, claims *utils.UserClaims) error {
	group, err := ensureUserGroupWriteAccess(id, claims)
	if err != nil {
		return err
	}
	if err := dal.DeleteUserGroupForTenant(group.ID, group.TenantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// GetUserGroup 组详情。
func (*UserGroup) GetUserGroup(id string, claims *utils.UserClaims) (*model.UserGroup, error) {
	return ensureUserGroupWriteAccess(id, claims)
}

// GetUserGroupList 组分页列表（requestedTenantID 仅供 SYS_ADMIN 指定租户视图）。
func (*UserGroup) GetUserGroupList(req *model.GetUserGroupListReq, requestedTenantID string, claims *utils.UserClaims) (map[string]interface{}, error) {
	scopes, err := resolveUserGroupListScopes(requestedTenantID, claims)
	if err != nil {
		return nil, err
	}
	total, list, err := dal.GetUserGroupListByPage(req, scopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if list == nil {
		list = []*model.UserGroup{}
	}
	return map[string]interface{}{"total": total, "list": list}, nil
}

// GetUserGroupMembers 组成员列表（users 表内账号）。
func (*UserGroup) GetUserGroupMembers(groupID string, claims *utils.UserClaims) (*model.GetUserGroupMembersResp, error) {
	group, err := ensureUserGroupWriteAccess(groupID, claims)
	if err != nil {
		return nil, err
	}
	users, err := dal.ListUserGroupMembers(group.ID, group.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if users == nil {
		users = []model.UserGroupMemberSummary{}
	}
	return &model.GetUserGroupMembersResp{GroupID: group.ID, GroupName: group.Name, Users: users}, nil
}

// AssignUserGroupMembers 全量替换组成员。
// 成员校验：userIDs 必须全部是组归属租户内的 users 登录账号（customer 客户无 users 行，天然被拒）。
func (*UserGroup) AssignUserGroupMembers(groupID string, req *model.AssignUserGroupMembersReq, claims *utils.UserClaims) error {
	group, err := ensureUserGroupWriteAccess(groupID, claims)
	if err != nil {
		return err
	}
	userIDs := normalizeIDList(userIDsOf(req))
	if len(userIDs) > 0 {
		count, err := dal.CountUsersByIDAndTenant(userIDs, group.TenantID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if int(count) != len(userIDs) {
			return errcode.NewWithMessage(errcode.CodeParamError, "one or more users not found or do not belong to the group tenant")
		}
	}
	if err := dal.ReplaceUserGroupMembers(context.Background(), group.ID, group.TenantID, userIDs); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// GetUserGroupPermissions 组权限元素列表（附资源可读名称快照）。
func (*UserGroup) GetUserGroupPermissions(groupID string, claims *utils.UserClaims) (*model.GetUserGroupPermissionsResp, error) {
	group, err := ensureUserGroupWriteAccess(groupID, claims)
	if err != nil {
		return nil, err
	}
	rows, err := dal.ListGroupPermissions(group.ID, group.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	elements := make([]model.GroupElementInfo, 0, len(rows))
	for _, row := range rows {
		info := model.GroupElementInfo{Code: row.ElementCode}
		if kind, id, ok := model.ParseGroupElementCode(row.ElementCode); ok {
			info.Kind = kind
			info.ID = id
			info.Name = groupElementResourceName(kind, id, group.TenantID)
		}
		elements = append(elements, info)
	}
	return &model.GetUserGroupPermissionsResp{GroupID: group.ID, GroupName: group.Name, Elements: elements}, nil
}

// groupElementResourceName 读取元素资源的可读名称（读不到留空，不阻断列表展示）。
func groupElementResourceName(kind, id, tenantID string) string {
	switch kind {
	case model.GroupElementKindBoard:
		if data, err := dal.GetBoard(id, tenantID); err == nil {
			if board, ok := data.(*model.Board); ok && board != nil {
				return board.Name
			}
		}
	case model.GroupElementKindAsset:
		if asset, err := dal.GetAsset(id, []string{tenantID}); err == nil && asset != nil {
			return asset.Name
		}
	}
	return ""
}

// AssignUserGroupPermissions 全量替换组权限元素绑定。
// 元素校验：code 必须可解析（v1: board:/asset:）且资源真实存在于组归属租户（fail-closed）。
func (*UserGroup) AssignUserGroupPermissions(groupID string, req *model.AssignUserGroupPermissionsReq, claims *utils.UserClaims) error {
	group, err := ensureUserGroupWriteAccess(groupID, claims)
	if err != nil {
		return err
	}
	codes := normalizeIDList(elementCodesOf(req))
	for _, code := range codes {
		kind, id, ok := model.ParseGroupElementCode(code)
		if !ok {
			return errcode.NewWithMessage(errcode.CodeParamError, "invalid element code: "+code)
		}
		if err := validateGroupElementResource(kind, id, group.TenantID); err != nil {
			return err
		}
	}
	if err := dal.ReplaceGroupPermissions(context.Background(), group.ID, group.TenantID, codes); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// validateGroupElementResource 校验元素资源存在且属于指定租户（fail-closed）。
func validateGroupElementResource(kind, id, tenantID string) error {
	switch kind {
	case model.GroupElementKindBoard:
		count, err := dal.CountBoardsByIDAndTenant(id, tenantID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if count == 0 {
			return errcode.NewWithMessage(errcode.CodeParamError, "board not found in group tenant: "+id)
		}
	case model.GroupElementKindAsset:
		count, err := dal.CountAssetsByIDAndTenant(id, tenantID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if count == 0 {
			return errcode.NewWithMessage(errcode.CodeParamError, "asset not found in group tenant: "+id)
		}
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported element kind: "+kind)
	}
	return nil
}

// userIDsOf / elementCodesOf / normalizeIDList 入参取值与去空格去重收敛。
func userIDsOf(req *model.AssignUserGroupMembersReq) []string {
	if req == nil {
		return nil
	}
	return req.UserIDs
}

func elementCodesOf(req *model.AssignUserGroupPermissionsReq) []string {
	if req == nil {
		return nil
	}
	return req.ElementCodes
}

func normalizeIDList(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
