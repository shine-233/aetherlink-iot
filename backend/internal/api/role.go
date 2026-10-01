// 文件用途：提供角色管理相关 HTTP Handler，负责接收前端请求、解析登录用户 claims，并把角色 CRUD 与角色权限接口分派到对应服务。
// 核心逻辑：“绑定 -> 校验 -> 取 claims -> 调 service -> respond”样板统一收敛到 handler_adapter.go 骨架
// （Handle / HandleAction / HandleNoBody / HandlePath / respond 出口），迁移前后的 JSON 包络逐字节一致，
// 由 handler_adapter_golden_2_test.go 的 golden 对比测试固化。
// 权限边界：接口层只读取 claims 并把它传给 service 作为权限判定依据；真正的角色管理权限、租户隔离与角色可写范围由 service.GroupApp.Role 收口。
// 静态审查建议：claims 缺失或类型不符时 RequireClaims 统一返回 CodeUnauthorized，替代了迁移前手写的
// CodeNoPermission/"unauthorized" 分支与 MustGet panic（详见 handler_adapter.go 头注释的有意差异说明）；
// AssignRolePermissions / AssignRoleUsers 的参数错误经共享助手 bindBodyLegacyParamError 保留
// errcode.WithData 包络（响应带 data 字段），与 BindAndValidate -> reportParamError 的消息形态不同，
// 改写时注意不要混用。
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
// 参数绑定：HandleAction 从 JSON body 绑定 model.CreateRoleReq，包含角色名称和可选描述。
// Claims：RequireClaims 从上下文读取 *utils.UserClaims，至少包含 Authority 与 TenantID，用于 service 层判断是否具备创建权限及新角色归属的租户。
// 权限边界：Handler 只负责把 claims 透传给 service，不在本层重复实现 SYS_ADMIN/TENANT_ADMIN 判定。
// Service 调用链：api.CreateRole -> service.GroupApp.Role.CreateRole -> requireRoleManager -> dal.CreateRole。
// 静态审查建议：关注角色名唯一性、租户归属和审计日志是否在更下层有保障。
// @Router   /api/v1/role [post]
func (*RoleApi) CreateRole(c *gin.Context) {
	HandleAction(c, func(req *model.CreateRoleReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.Role.CreateRole(req, userClaims)
	})
}

// UpdateRole 更新角色基础信息。
// 参数绑定：HandleAction 按 JSON body 绑定 model.UpdateRoleReq，要求 id 与 name，description 为可选指针字段。
// Claims：RequireClaims 从上下文提取 *utils.UserClaims，供 service 判断调用者是否可修改该角色以及是否跨租户越权。
// 权限边界：本层仅额外拦截“名称与描述都未提供”的空更新请求；角色是否存在、是否属于当前租户、是否允许写入由 service 保证。
// 注意：空更新分支沿用迁移前的手写响应 {"code":400,"message":"修改内容不能为空"}（HTTP 200，非统一错误码包络），
// 直接 c.JSON 写出后返回零值；响应中间件对已写出的响应短路（middleware/response/response.go:62），
// respond 的 c.Set("data", ...) 不会改变已写出的响应字节。该分支因 UpdateRoleReq.Name 带 required
// 校验而经 HTTP 不可达，仅为保留迁移前行为而存在。
// Service 调用链：api.UpdateRole -> service.GroupApp.Role.UpdateRole -> ensureRoleWriteAccess -> dal.UpdateRole -> dal.GetRoleByID。
// @Router   /api/v1/role [put]
func (*RoleApi) UpdateRole(c *gin.Context) {
	Handle(c, func(req *model.UpdateRoleReq, userClaims *utils.UserClaims) (model.Role, error) {
		if req.Description == nil && req.Name == "" {
			// 迁移前手写响应：HTTP 200 + {"code":400,...}，不走统一错误包络；响应已写出，返回零值即可。
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
			return model.Role{}, nil
		}
		return service.GroupApp.Role.UpdateRole(req, userClaims)
	})
}

// DeleteRole 删除角色。
// 参数绑定：HandlePath 读取路径参数中的角色 ID，不经过结构体验证。
// Claims：RequireClaims 从上下文提取 *utils.UserClaims，供 service 层做最终权限与租户边界校验。
// 权限边界：删除前在 API 层先调用 Casbin 检查角色是否仍被用户占用，避免直接删除被引用角色；真正的角色删除权限和租户范围仍由 service 控制。
// Service 调用链：api.DeleteRole -> service.GroupApp.Casbin.HasRole -> service.GroupApp.Role.DeleteRole -> ensureRoleWriteAccess -> dal.DeleteRole。
// 静态审查建议：路径参数建议统一走格式校验；“角色仍被引用”只检查了 Casbin 用户-角色绑定，未覆盖其他业务外键依赖，审查时可补充这一风险说明。
// @Router   /api/v1/role/{id} [delete]
func (*RoleApi) DeleteRole(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		// 删除前先确认该角色没有被用户绑定，避免遗留悬空授权关系。
		if service.GroupApp.Casbin.HasRole(id) {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"role_id": id,
				"error":   "Role in use",
			})
		}
		return service.GroupApp.Role.DeleteRole(id, userClaims)
	})
}

// HandleRoleListByPage 分页查询角色列表。
// 参数绑定：Handle 对 GET 请求按 query/form 绑定 model.GetRoleListByPageReq（经 BindAndValidate 分发），包含分页参数和可选名称筛选。
// Claims：RequireClaims 从上下文提取 *utils.UserClaims，service 依赖其中的 TenantID 对结果集做租户范围过滤。
// 权限边界：API 层不自行裁剪返回字段，也不判断调用者是否具备角色管理权限；若列表可见性需要更严格约束，应由 service 统一收口。
// Service 调用链：api.HandleRoleListByPage -> service.GroupApp.Role.GetRoleListByPage -> dal.GetRoleListByPage。
// 静态审查建议：建议检查分页参数默认值和上限是否由公共结构体兜底，避免大页查询；同时确认非管理员能否查询到本租户全部角色是否符合预期。
// @Router   /api/v1/role [get]
func (*RoleApi) HandleRoleListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetRoleListByPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
		return service.GroupApp.Role.GetRoleListByPage(req, userClaims)
	})
}

// ListPermissions 查询权限字典列表
// 参数绑定：无请求体；module 从 query 读取（与迁移前一致）。
// Claims：RequireClaims 提取 *utils.UserClaims；缺失或类型不符时返回统一 CodeUnauthorized。
// @Router /api/v1/permissions [get]
func (*RoleApi) ListPermissions(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) ([]model.SysPermission, error) {
		return service.RolePermission.ListPermissions(c.Request.Context(), c.Query("module"), userClaims)
	})
}

// GetRolePermissions 获取角色的权限配置
// 参数绑定：HandlePath 读取路径参数 id；无请求体。
// Claims：RequireClaims 提取 *utils.UserClaims；缺失或类型不符时返回统一 CodeUnauthorized。
// @Router /api/v1/roles/:id/permissions [get]
func (*RoleApi) GetRolePermissions(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (*model.RolePermissionsResp, error) {
		return service.RolePermission.GetRolePermissions(c.Request.Context(), roleID, userClaims)
	})
}

// AssignRolePermissions 为角色分配权限点
// 参数绑定：请求体在闭包内经 bindBodyLegacyParamError 绑定，参数错误沿用迁移前的
// errcode.WithData(CodeParamError, {"error": ...}) 包络（响应带 data 字段、消息取错误码默认文案），
// 与 BindAndValidate -> reportParamError 的 NewWithMessage 形态不同，以保证迁移前后 JSON 逐字节一致。
// Claims：RequireClaims 提取 *utils.UserClaims；缺失或类型不符时返回统一 CodeUnauthorized。
// @Router /api/v1/roles/:id/permissions [post]
func (*RoleApi) AssignRolePermissions(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (interface{}, error) {
		var req model.AssignRolePermissionsReq
		if err := bindBodyLegacyParamError(c, &req); err != nil {
			return nil, err
		}
		if err := service.RolePermission.AssignRolePermissions(c.Request.Context(), roleID, &req, userClaims); err != nil {
			return nil, err
		}
		return gin.H{"status": "ok"}, nil
	})
}

// GetRoleUsers 获取角色关联的用户列表
// 参数绑定：HandlePath 读取路径参数 id；无请求体。
// Claims：RequireClaims 提取 *utils.UserClaims；缺失或类型不符时返回统一 CodeUnauthorized。
// @Router /api/v1/roles/:id/users [get]
func (*RoleApi) GetRoleUsers(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (*model.RoleUsersResp, error) {
		return service.RolePermission.GetRoleUsers(c.Request.Context(), roleID, userClaims)
	})
}

// AssignRoleUsers 为角色批量分配用户
// 参数绑定：请求体在闭包内经 bindBodyLegacyParamError 绑定，参数错误沿用迁移前的
// errcode.WithData(CodeParamError, {"error": ...}) 包络（响应带 data 字段），以保证迁移前后 JSON 逐字节一致。
// Claims：RequireClaims 提取 *utils.UserClaims；缺失或类型不符时返回统一 CodeUnauthorized。
// @Router /api/v1/roles/:id/users [post]
func (*RoleApi) AssignRoleUsers(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (interface{}, error) {
		var req model.AssignRoleUsersReq
		if err := bindBodyLegacyParamError(c, &req); err != nil {
			return nil, err
		}
		if err := service.RolePermission.AssignRoleUsers(c.Request.Context(), roleID, &req, userClaims); err != nil {
			return nil, err
		}
		return gin.H{"status": "ok"}, nil
	})
}
