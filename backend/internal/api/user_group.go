// 文件用途：用户组与组权限（TB-46 GPE v1）HTTP Handler——接收请求、提取 claims 并分派服务。
// 核心逻辑：组 CRUD / 成员查询与批量绑定 / 组权限元素查询与批量绑定的参数绑定与错误透传。
// 关键注意事项：接口层只透传 claims，不重复实现权限与租户判定（由 service 层收口）；
//
//	claims 经 MustGet("claims") 提取，依赖 JWT 中间件先于 CasbinRBAC 注入
//	（与 role.go 的组管理同口径），避免本层重复实现认证兜底逻辑。
//
// 重构建议：组详情与写操作共用的 ensureUserGroupWriteAccess 已在 service 层收口，
//
//	若后续增加"组所有者"等细粒度能力，扩展 service 层判定而不是在 Handler 加分支。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// UserGroupApi 用户组与组权限接口入口。
type UserGroupApi struct{}

// CreateUserGroup 创建用户组。
// 参数绑定：JSON body → model.CreateUserGroupReq（name 必填；tenant_id 仅 SYS_ADMIN 生效）。
// Service 调用链：api.CreateUserGroup -> service.GroupApp.UserGroup.CreateUserGroup -> dal.CreateUserGroup。
// @Router  /api/v1/user_group [post]
func (*UserGroupApi) CreateUserGroup(c *gin.Context) {
	var req model.CreateUserGroupReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.CreateUserGroup(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateUserGroup 更新用户组（名称/描述至少一项）。
// @Router  /api/v1/user_group [put]
func (*UserGroupApi) UpdateUserGroup(c *gin.Context) {
	var req model.UpdateUserGroupReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.UpdateUserGroup(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteUserGroup 删除用户组（级联清理成员与权限绑定）。
// @Router  /api/v1/user_group/{id} [delete]
func (*UserGroupApi) DeleteUserGroup(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.UserGroup.DeleteUserGroup(id, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetUserGroup 用户组详情。
// @Router  /api/v1/user_group/{id} [get]
func (*UserGroupApi) GetUserGroup(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.GetUserGroup(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetUserGroupList 用户组分页列表。
// 参数绑定：query/form → model.GetUserGroupListReq；tenant_id 仅 SYS_ADMIN 指定租户视图生效。
// @Router  /api/v1/user_groups [get]
func (*UserGroupApi) GetUserGroupList(c *gin.Context) {
	var req model.GetUserGroupListReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.GetUserGroupList(&req, c.Query("tenant_id"), userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetUserGroupMembers 组成员列表。
// @Router  /api/v1/user_group/{id}/users [get]
func (*UserGroupApi) GetUserGroupMembers(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.GetUserGroupMembers(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// AssignUserGroupMembers 全量替换组成员（user_ids 传空数组=清空成员）。
// @Router  /api/v1/user_group/{id}/users [post]
func (*UserGroupApi) AssignUserGroupMembers(c *gin.Context) {
	id := c.Param("id")
	var req model.AssignUserGroupMembersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()}))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.UserGroup.AssignUserGroupMembers(id, &req, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}

// GetUserGroupPermissions 组权限元素列表。
// @Router  /api/v1/user_group/{id}/permissions [get]
func (*UserGroupApi) GetUserGroupPermissions(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.UserGroup.GetUserGroupPermissions(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// AssignUserGroupPermissions 全量替换组权限元素绑定（element_codes 传空数组=清空绑定）。
// @Router  /api/v1/user_group/{id}/permissions [post]
func (*UserGroupApi) AssignUserGroupPermissions(c *gin.Context) {
	id := c.Param("id")
	var req model.AssignUserGroupPermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()}))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.UserGroup.AssignUserGroupPermissions(id, &req, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}
