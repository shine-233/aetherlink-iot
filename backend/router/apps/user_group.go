// 文件用途：用户组与组权限（TB-46 GPE v1）路由定义。
// 核心逻辑：挂载 /api/v1/user_group[...] 路由组并关联 UserGroupApi（Casbin 登记见 131.sql）。
// 关键注意事项：组内路径变更必须同步 131.sql 之后的迁移 Casbin 登记与
//
//	router/apps/casbin_route_registration_contract_test.go 的挂载清单，否则启动审计 fail-fast。
//
// 重构建议：若后续增加"组成员自助查看所属组"等普通用户端点，新增独立只读子路由并
//
//	按 101.sql 口径补 TENANT_USER 授权行，不在本管理面组内混挂。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// UserGroupRouter 用户组与组权限路由组。
type UserGroupRouter struct{}

// InitUserGroup 挂载用户组路由（/api/v1/user_group[...]）。
func (*UserGroupRouter) InitUserGroup(Router *gin.RouterGroup) {
	r := Router.Group("user_group")
	{
		r.POST("", api.Controllers.UserGroupApi.CreateUserGroup)
		r.PUT("", api.Controllers.UserGroupApi.UpdateUserGroup)
		r.GET(":id", api.Controllers.UserGroupApi.GetUserGroup)
		r.DELETE(":id", api.Controllers.UserGroupApi.DeleteUserGroup)
		r.GET(":id/users", api.Controllers.UserGroupApi.GetUserGroupMembers)
		r.POST(":id/users", api.Controllers.UserGroupApi.AssignUserGroupMembers)
		r.GET(":id/permissions", api.Controllers.UserGroupApi.GetUserGroupPermissions)
		r.POST(":id/permissions", api.Controllers.UserGroupApi.AssignUserGroupPermissions)
	}
	// 列表独立挂载（GET /api/v1/user_groups），与角色 role/roles 双路径形态一致。
	groups := Router.Group("user_groups")
	{
		groups.GET("", api.Controllers.UserGroupApi.GetUserGroupList)
	}
}
