// 文件说明：
// 1. 本文件承载服务插件相关 HTTP Handler，只负责协议层编排，不直接实现业务规则。
// 2. 参数入口分为两类：`BindAndValidate` 负责绑定 query/json/form 并执行基础校验，`c.Param("id")` 负责读取路由参数。
// 3. 绝大多数接口依赖上游鉴权中间件写入 `claims`，本层只做 `c.MustGet("claims")` 断言读取，再将身份上下文继续传给 service 层。
// 4. 调用链统一为 `api -> service.GroupApp.ServicePlugin -> 领域/存储层`，返回值通过 `c.Set("data", ...)` 交给统一响应中间件封装。
// 5. 权限边界应保持在中间件与 service 层：Handler 不应绕过 claims、租户隔离、角色校验，也不应在本层吞掉错误。
// 6. 静态审查建议：重点检查未鉴权路由是否误读 `claims`、路径 ID 是否允许空值透传、匿名心跳接口是否存在额外签名或来源校验。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ServicePluginApi struct{}

// Create 创建服务插件。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.CreateServicePluginReq`，失败时直接返回，由统一错误流程处理。
// Claims：从 Gin 上下文读取 `claims`，默认要求路由已完成鉴权并注入用户、租户等身份信息。
// 调用链：`Create -> service.GroupApp.ServicePlugin.Create`，由 service 层执行创建规则、权限校验和持久化。
// 权限边界：Handler 只负责传递身份上下文，不在本层决定是否允许跨租户或越权创建。
// 静态审查建议：确认创建请求的关键字段校验已在绑定或 service 层覆盖，避免仅依赖前端约束。
// 路由：`POST /api/v1/service`
func (*ServicePluginApi) Create(c *gin.Context) {
	var req model.CreateServicePluginReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServicePlugin.Create(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleList 分页查询服务插件列表。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.GetServicePluginByPageReq`，通常承接分页、筛选、排序等查询参数。
// Claims：读取 `claims` 并向下传递，供 service 层做租户范围、角色可见性和数据裁剪。
// 调用链：`HandleList -> service.GroupApp.ServicePlugin.List`。
// 权限边界：列表可见范围不应在 Handler 组装，必须由 service 层基于 claims 统一收口。
// 静态审查建议：关注分页参数默认值与上限是否在下游兜底，避免出现大页查询或无界扫描。
// 路由：`GET /api/v1/service/list`
func (*ServicePluginApi) HandleList(c *gin.Context) {
	var req model.GetServicePluginByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServicePlugin.List(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Handle 查询单个服务插件详情。
// 参数绑定：通过 `c.Param("id")` 读取路由中的服务插件 ID，不在本层额外做格式转换。
// Claims：读取 `claims` 并传递给 service 层，以便在详情查询时执行资源归属和访问权限判断。
// 调用链：`Handle -> service.GroupApp.ServicePlugin.Get`。
// 权限边界：资源是否属于当前租户、当前用户是否可读，均应由 service 层裁定。
// 静态审查建议：检查下游是否对空 ID、非法 ID 和越权读取返回稳定错误，避免泄漏资源存在性。
// 路由：`GET /api/v1/service/detail/{id}`
func (*ServicePluginApi) Handle(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServicePlugin.Get(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Update 更新服务插件。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.UpdateServicePluginReq`，请求体中应包含待更新目标及变更字段。
// Claims：读取 `claims`，将操作者身份传给 service 层用于更新授权、审计字段或租户隔离判断。
// 调用链：`Update -> service.GroupApp.ServicePlugin.Update`。
// 权限边界：字段级可修改性、状态机约束与跨租户保护必须在 service 层统一校验，Handler 不做业务分支。
// 静态审查建议：确认更新接口不会因零值覆盖产生误写，必要时在下游显式区分“未传值”和“传入零值”。
// 路由：`PUT /api/v1/service`
func (*ServicePluginApi) Update(c *gin.Context) {
	var req model.UpdateServicePluginReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.ServicePlugin.Update(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// Delete 删除服务插件。
// 参数绑定：通过 `c.Param("id")` 获取待删除资源 ID。
// Claims：读取 `claims` 并传入 service 层，以便执行删除权限、租户归属和关联资源检查。
// 调用链：`Delete -> service.GroupApp.ServicePlugin.Delete`。
// 权限边界：是否允许删除、是否需要阻止删除被引用资源，均应由 service 层负责，不在 Handler 侧短路。
// 静态审查建议：检查删除语义是硬删除还是软删除，并确认越权删除不会仅通过枚举 ID 命中他人资源。
// 路由：`DELETE /api/v1/service/{id}`
func (*ServicePluginApi) Delete(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.ServicePlugin.Delete(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// Heartbeat 处理插件心跳上报。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.HeartbeatReq`，用于承接插件标识、状态或保活信息。
// Claims：本接口未读取 `claims`，说明它依赖匿名访问或使用非用户态认证方式，真实身份校验应在网关、中间件或 service 层完成。
// 调用链：`Heartbeat -> service.GroupApp.ServicePlugin.Heartbeat`。
// 权限边界：Handler 不负责判定插件来源真伪；若需要签名、令牌、白名单或租户映射，应由更下游统一执行。
// 静态审查建议：重点确认匿名心跳不会被伪造刷写状态，且请求频率、来源校验和幂等策略已有落点。
// 路由：`POST /api/v1/plugin/heartbeat`
func (*ServicePluginApi) Heartbeat(c *gin.Context) {
	var req model.HeartbeatReq
	if !BindAndValidate(c, &req) {
		return
	}
	err := service.GroupApp.ServicePlugin.Heartbeat(&req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

//func (*ServicePluginApi) NoticeTest(c *gin.Context) {
//	service.GroupApp.NotificationServicesConfig.ExecuteNotification("3eb7b6aa-d1ca-8c7d-9b62-3ab54bf4b9ab", "消息体: 2025-08-01", "tenant_id")
//
//	c.Set("data", map[string]interface{}{})
//}

// HandleServiceSelect 返回服务插件下拉选项。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.GetServiceSelectReq`，通常包含筛选条件或上游业务上下文。
// Claims：读取 `claims`，供 service 层基于租户和角色裁剪可选项。
// 调用链：`HandleServiceSelect -> service.GroupApp.ServicePlugin.GetServiceSelect`。
// 权限边界：哪些插件可见、是否允许跨项目或跨租户选择，应由 service 层统一裁定。
// 静态审查建议：关注选择器接口是否被误当作公开元数据接口，避免返回过多内部字段。
// 路由：`GET /api/v1/service/plugin/select`
func (*ServicePluginApi) HandleServiceSelect(c *gin.Context) {
	var req model.GetServiceSelectReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServicePlugin.GetServiceSelect(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleServicePluginByServiceIdentifier 按服务标识查询插件信息。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.GetServicePluginByServiceIdentifierReq`，核心输入为 `ServiceIdentifier`。
// Claims：读取 `claims` 并传给 service 层，避免仅凭公开标识绕过租户或角色范围。
// 调用链：`HandleServicePluginByServiceIdentifier -> service.GroupApp.ServicePlugin.GetServicePluginByServiceIdentifier`。
// 权限边界：标识符到真实插件资源的映射和访问授权均应下沉到 service 层，本层不缓存也不拼装额外规则。
// 静态审查建议：检查 `ServiceIdentifier` 是否具备唯一性和输入规范化，防止大小写、前后缀或历史别名造成歧义读取。
// 路由：`GET /api/v1/service/plugin/info`
func (*ServicePluginApi) HandleServicePluginByServiceIdentifier(c *gin.Context) {
	var req model.GetServicePluginByServiceIdentifierReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.ServicePlugin.GetServicePluginByServiceIdentifier(req.ServiceIdentifier, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
