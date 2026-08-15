// 文件用途：提供角色管理相关 HTTP Handler，负责接收前端请求、解析登录用户 claims，并把角色 CRUD 请求分派到角色服务。
// 核心逻辑：本文件围绕角色创建、更新、删除和分页查询四类场景进行参数绑定、claims 提取、最小前置校验与错误透传。
// 权限边界：接口层只读取 claims 并把它传给 service 作为权限判定依据；真正的角色管理权限、租户隔离与角色可写范围由 service.GroupApp.Role 收口。
// 静态审查建议：重点关注 claims 的存在性假设、`MustGet("claims")` 的 panic 风险、自定义 JSON 错误响应是否破坏统一错误协议，以及删除前 Casbin 占用检查与 service 层权限判断是否覆盖了所有边界条件。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RoleApi 负责承接角色管理接口的请求入口。
type RoleApi struct{}

// CreateRole 创建角色。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.CreateRoleReq，包含角色名称和可选描述。
// Claims：从 c.MustGet("claims") 读取 *utils.UserClaims，至少包含 Authority 与 TenantID，用于 service 层判断是否具备创建权限及新角色归属的租户。
// 权限边界：Handler 只负责把 claims 透传给 service，不在本层重复实现 SYS_ADMIN/TENANT_ADMIN 判定。
// Service 调用链：api.CreateRole -> service.GroupApp.Role.CreateRole -> requireRoleManager -> dal.CreateRole。
// 静态审查建议：确认中间件始终注入 claims，避免 MustGet 触发 panic；同时关注角色名唯一性、租户归属和审计日志是否在更下层有保障。
// @Router   /api/v1/role [post]
func (*RoleApi) CreateRole(c *gin.Context) {
	var req model.CreateRoleReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.Role.CreateRole(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// UpdateRole 更新角色基础信息。
// 参数绑定：通过 BindAndValidate 从 JSON body 绑定 model.UpdateRoleReq，要求 id 与 name，description 为可选指针字段。
// Claims：从上下文提取 *utils.UserClaims，供 service 判断调用者是否可修改该角色以及是否跨租户越权。
// 权限边界：本层仅额外拦截“名称与描述都未提供”的空更新请求；角色是否存在、是否属于当前租户、是否允许写入由 service 保证。
// Service 调用链：api.UpdateRole -> service.GroupApp.Role.UpdateRole -> ensureRoleWriteAccess -> dal.UpdateRole -> dal.GetRoleByID。
// 静态审查建议：当前空更新分支直接返回手写 JSON，建议在审查中确认这是否与全局错误格式一致；另外，BindAndValidate 已要求 name 必填，这里的空更新判定与请求模型约束存在重叠，值得人工复核。
// @Router   /api/v1/role [put]
func (*RoleApi) UpdateRole(c *gin.Context) {
	var req model.UpdateRoleReq
	if !BindAndValidate(c, &req) {
		return
	}

	if req.Description == nil && req.Name == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Role.UpdateRole(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// DeleteRole 删除角色。
// 参数绑定：通过 c.Param("id") 读取路径参数中的角色 ID，不经过结构体验证。
// Claims：从上下文提取 *utils.UserClaims，供 service 层做最终权限与租户边界校验。
// 权限边界：删除前在 API 层先调用 Casbin 检查角色是否仍被用户占用，避免直接删除被引用角色；真正的角色删除权限和租户范围仍由 service 控制。
// Service 调用链：api.DeleteRole -> service.GroupApp.Casbin.HasRole -> service.GroupApp.Role.DeleteRole -> ensureRoleWriteAccess -> dal.DeleteRole。
// 静态审查建议：路径参数建议统一走格式校验；同时关注“角色仍被引用”只检查了 Casbin 用户-角色绑定，未覆盖其他业务外键依赖，审查时可补充这一风险说明。
// @Router   /api/v1/role/{id} [delete]
func (*RoleApi) DeleteRole(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	// 删除前先确认该角色没有被用户绑定，避免遗留悬空授权关系。
	if service.GroupApp.Casbin.HasRole(id) {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id": id,
			"error":   "Role in use",
		}))
		return
	}

	err := service.GroupApp.Role.DeleteRole(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleRoleListByPage 分页查询角色列表。
// 参数绑定：通过 BindAndValidate 从 query/form 绑定 model.GetRoleListByPageReq，包含分页参数和可选名称筛选。
// Claims：从上下文提取 *utils.UserClaims，service 依赖其中的 TenantID 对结果集做租户范围过滤。
// 权限边界：API 层不自行裁剪返回字段，也不判断调用者是否具备角色管理权限；若列表可见性需要更严格约束，应由 service 统一收口。
// Service 调用链：api.HandleRoleListByPage -> service.GroupApp.Role.GetRoleListByPage -> dal.GetRoleListByPage。
// 静态审查建议：建议检查分页参数默认值和上限是否由公共结构体兜底，避免大页查询；同时确认非管理员能否查询到本租户全部角色是否符合预期。
// @Router   /api/v1/role [get]
func (*RoleApi) HandleRoleListByPage(c *gin.Context) {
	var req model.GetRoleListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	roleList, err := service.GroupApp.Role.GetRoleListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", roleList)
}
