// 文件用途：提供 Casbin 角色与用户授权关系的 HTTP Handler，负责把请求参数转换成 service 层可消费的输入，并将执行结果写回统一响应上下文。
// 核心逻辑：本文件只处理参数绑定、基础空值检查、路径参数读取和错误透传；实际的策略写入、查询与删除均委托给 service.GroupApp.Casbin。
// 权限边界：本文件中的 Handler 未直接读取 claims，也不自行判定调用者身份；访问控制依赖路由层、中间件或上游网关先完成鉴权，避免未授权调用绕过授权关系管理。
// 静态审查建议：重点检查 BindAndValidate 覆盖了 body/query 的必填字段、更新接口的空值判断是否符合预期、Casbin service 返回 false 时是否需要区分“无变更”和“真正失败”，以及删除/覆盖写入场景是否需要审计日志。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// CasbinApi 负责处理角色-功能、用户-角色两类授权关系的接口入口。
type CasbinApi struct{}

var casbinService = service.GroupApp.Casbin

// AddFunctionToRole 为指定角色批量追加功能权限。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.FunctionsRoleValidate，要求提供 role_id 与 functions_ids。
// Claims：当前实现不直接读取 c 中的 claims；若接口仅允许管理员调用，需要依赖上游鉴权中间件保证请求不会越过权限边界。
// 权限边界：Handler 只负责校验入参与转发，不验证角色是否属于当前租户，也不验证功能是否可被当前操作者授予，这些约束若有需要应在 service 或更上层统一收口。
// Service 调用链：api.AddFunctionToRole -> service.GroupApp.Casbin.AddFunctionToRole -> CasbinEnforcer.AddNamedPolicies。
// 静态审查建议：关注 functions_ids 为空切片、重复功能 ID、同一角色重复授权时返回 false 的语义，以及错误码是否需要细分为参数错误与状态冲突。
// @Router   /api/v1/casbin/function [post]
func (*CasbinApi) AddFunctionToRole(c *gin.Context) {
	var req model.FunctionsRoleValidate
	if !BindAndValidate(c, &req) {
		return
	}
	if len(req.FunctionsIDs) == 0 {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id":      req.RoleID,
			"function_ids": req.FunctionsIDs,
			"error":        "AddFunctionToRole failed",
		}))
		return
	}

	ok := casbinService.AddFunctionToRole(req.RoleID, req.FunctionsIDs)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id":      req.RoleID,
			"function_ids": req.FunctionsIDs,
			"error":        "AddFunctionToRole failed",
		}))
		return
	}

	c.Set("data", nil)
}

// HandleFunctionFromRole 查询指定角色当前已绑定的功能权限。
// 参数绑定：通过 BindAndValidate 读取 query/form 中的 role_id 到 model.RoleValidate。
// Claims：当前实现不消费 claims，默认信任上游已经完成操作者身份校验。
// 权限边界：只暴露角色对应的功能 ID 列表，不在本层附加租户隔离或角色可见性判断。
// Service 调用链：api.HandleFunctionFromRole -> service.GroupApp.Casbin.GetFunctionFromRole -> CasbinEnforcer.GetFilteredPolicy。
// 静态审查建议：确认查询型接口是否需要按租户或操作者范围裁剪结果，以及 GetFunctionFromRole 恒返回 true 的实现是否会掩盖底层异常。
// @Router   /api/v1/casbin/function [get]
func (*CasbinApi) HandleFunctionFromRole(c *gin.Context) {
	var req model.RoleValidate
	if !BindAndValidate(c, &req) {
		return
	}

	roles, ok := casbinService.GetFunctionFromRole(req.RoleID)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id": req.RoleID,
			"error":   "GetFunctionFromRole failed",
		}))
		return
	}

	c.Set("data", roles)
}

// UpdateFunctionFromRole 以“先删后加”的方式覆盖角色的功能权限集合。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.FunctionsRoleValidate。
// Claims：当前实现不直接读取 claims，因此是否允许覆盖某个角色的权限完全依赖上游路由保护。
// 权限边界：Handler 只做最小的空值校验，不校验 role_id 是否存在、functions_ids 是否属于合法功能，也不处理并发更新冲突。
// Service 调用链：api.UpdateFunctionFromRole -> service.GroupApp.Casbin.GetFunctionFromRole -> service.GroupApp.Casbin.RemoveRoleAndFunction -> service.GroupApp.Casbin.AddFunctionToRole。
// 静态审查建议：重点审查 `req.RoleID == "" && req.FunctionsIDs == nil` 的条件是否过宽，当前逻辑仅在两者同时为空时才报错；若 Remove 成功但 Add 失败，角色会处于已清空状态，后续可考虑事务化或补偿策略。
// @Router   /api/v1/casbin/function [put]
func (*CasbinApi) UpdateFunctionFromRole(c *gin.Context) {
	var req model.FunctionsRoleValidate
	if !BindAndValidate(c, &req) {
		return
	}
	if len(req.FunctionsIDs) == 0 {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id":      req.RoleID,
			"function_ids": req.FunctionsIDs,
			"error":        "AddFunctionToRole failed",
		}))
		return
	}

	if req.RoleID == "" && req.FunctionsIDs == nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id":      req.RoleID,
			"function_ids": req.FunctionsIDs,
			"error":        "UpdateFunctionFromRole failed",
		}))
		return
	}

	f, _ := casbinService.GetFunctionFromRole(req.RoleID)
	if len(f) > 0 {
		// 没有记录时删除会返回 false，这里仅在存在绑定关系时执行删除。
		ok := casbinService.RemoveRoleAndFunction(req.RoleID)
		if !ok {
			c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"role_id": req.RoleID,
				"error":   "RemoveRoleAndFunction failed",
			}))
			return
		}
	}
	ok := casbinService.AddFunctionToRole(req.RoleID, req.FunctionsIDs)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id":      req.RoleID,
			"function_ids": req.FunctionsIDs,
			"error":        "AddFunctionToRole failed",
		}))
	}
	c.Set("data", nil)
}

// DeleteFunctionFromRole 删除某个角色与全部功能之间的绑定关系。
// 参数绑定：直接通过 c.Param("id") 读取路径参数中的角色 ID，不经过结构体验证。
// Claims：当前实现不读取 claims，默认由上游保证只有有权操作者可删除授权关系。
// 权限边界：只删除角色到功能的 Casbin policy，不删除角色实体本身，也不处理角色是否仍被其他业务引用。
// Service 调用链：api.DeleteFunctionFromRole -> service.GroupApp.Casbin.RemoveRoleAndFunction -> CasbinEnforcer.RemoveFilteredPolicy。
// 静态审查建议：建议人工检查路径参数是否需要统一长度/格式校验，以及“无记录可删”被当作失败是否符合前端期望。
// @Router   /api/v1/casbin/function/{id} [delete]
func (*CasbinApi) DeleteFunctionFromRole(c *gin.Context) {
	id := c.Param("id")
	ok := casbinService.RemoveRoleAndFunction(id)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id": id,
			"error":   "RemoveRoleAndFunction failed",
		}))
		return
	}
	c.Set("data", nil)
}

// AddRoleToUser 为指定用户批量追加角色绑定。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.RolesUserValidate，要求提供 user_id 与 roles_ids。
// Claims：Handler 本身不消费 claims；若产品要求仅管理员可改用户角色，需依赖外层中间件或 service 层做二次收口。
// 权限边界：本层不校验用户是否属于当前租户，也不校验角色是否跨租户或是否允许被授予该用户。
// Service 调用链：api.AddRoleToUser -> service.GroupApp.Casbin.AddRolesToUser -> CasbinEnforcer.AddNamedGroupingPolicies。
// 静态审查建议：关注重复 role ID、用户已拥有角色时的返回值语义，以及失败场景是否需要输出更明确的审计上下文。
// @Router   /api/v1/casbin/user [post]
func (*CasbinApi) AddRoleToUser(c *gin.Context) {
	var req model.RolesUserValidate
	if !BindAndValidate(c, &req) {
		return
	}
	if len(req.RolesIDs) == 0 {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"role_id": req.RolesIDs,
			"error":   "AddRolesToUser failed",
		}))
		return
	}

	ok := casbinService.AddRolesToUser(req.UserID, req.RolesIDs)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"role_id": req.RolesIDs,
			"error":   "AddRolesToUser failed",
		}))
		return
	}

	c.Set("data", nil)

}

// HandleRolesFromUser 查询指定用户当前拥有的角色列表。
// 参数绑定：通过 BindAndValidate 从 query/form 绑定 model.UserValidate 中的 user_id。
// Claims：不直接读取 claims，默认上游已保证查询方具备查看该用户授权关系的权限。
// 权限边界：只负责返回 Casbin 分组策略中的角色 ID 列表，不追加用户详情、角色详情或租户级过滤。
// Service 调用链：api.HandleRolesFromUser -> service.GroupApp.Casbin.GetRoleFromUser -> CasbinEnforcer.GetFilteredNamedGroupingPolicy。
// 静态审查建议：建议确认该接口是否会泄露敏感角色分配信息，以及 GetRoleFromUser 恒返回 true 是否需要补充异常分支。
// @Router   /api/v1/casbin/user [get]
func (*CasbinApi) HandleRolesFromUser(c *gin.Context) {
	var req model.UserValidate
	if !BindAndValidate(c, &req) {
		return
	}

	roles, ok := casbinService.GetRoleFromUser(req.UserID)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"error":   "GetRoleFromUser failed",
		}))
		return
	}

	c.Set("data", roles)

}

// UpdateRolesFromUser 以覆盖写入方式更新用户角色集合。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.RolesUserValidate。
// Claims：当前实现不读取 claims，因此“谁能改谁的角色”不是本层关心的约束。
// 权限边界：本层不会检查用户、角色与租户的归属关系，也不会阻止将用户现有角色全部清空。
// Service 调用链：api.UpdateRolesFromUser -> service.GroupApp.Casbin.RemoveUserAndRole -> service.GroupApp.Casbin.AddRolesToUser。
// 静态审查建议：重点关注 `req.UserID == "" && req.RolesIDs == nil` 仅拦截双空输入的问题；当前先删后加没有事务保护，新增失败后用户将失去原角色，适合在静态审查中标记为一致性风险。
// @Router   /api/v1/casbin/user [put]
func (*CasbinApi) UpdateRolesFromUser(c *gin.Context) {
	var req model.RolesUserValidate
	if !BindAndValidate(c, &req) {
		return
	}
	if len(req.RolesIDs) == 0 {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"role_id": req.RolesIDs,
			"error":   "AddRolesToUser failed",
		}))
		return
	}

	if req.UserID == "" && req.RolesIDs == nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"role_id": req.RolesIDs,
			"error":   "UpdateRolesFromUser failed",
		}))
		return
	}

	casbinService.RemoveUserAndRole(req.UserID)
	ok := casbinService.AddRolesToUser(req.UserID, req.RolesIDs)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": req.UserID,
			"role_id": req.RolesIDs,
			"error":   "AddRolesToUser failed",
		}))
	}
	c.Set("data", nil)
}

// DeleteRolesFromUser 删除指定用户与全部角色之间的绑定关系。
// 参数绑定：直接通过 c.Param("id") 读取路径参数中的用户 ID。
// Claims：不直接使用 claims，调用资格依赖路由鉴权和上游中间件。
// 权限边界：仅清空用户到角色的 Casbin 分组策略，不影响用户实体，也不校验是否会使系统失去最后一个管理员。
// Service 调用链：api.DeleteRolesFromUser -> service.GroupApp.Casbin.RemoveUserAndRole -> CasbinEnforcer.RemoveFilteredNamedGroupingPolicy。
// 静态审查建议：建议在代码审查中确认是否需要保护关键账号、是否需要记录操作审计，以及“用户无角色可删”是否应视为幂等成功。
// @Router   /api/v1/casbin/user/{id} [delete]
func (*CasbinApi) DeleteRolesFromUser(c *gin.Context) {
	id := c.Param("id")
	ok := casbinService.RemoveUserAndRole(id)
	if !ok {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"user_id": id,
			"error":   "RemoveUserAndRole failed",
		}))
		return
	}
	c.Set("data", nil)
}
