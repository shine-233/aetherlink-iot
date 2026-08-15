// 文件说明：
// 1. 本文件提供场景管理相关 HTTP Handler，职责是把请求协议转换为场景领域服务调用。
// 2. 参数主要来自 `BindAndValidate` 绑定的 query/json/form，以及 `c.Param("id")` 读取的路由参数。
// 3. 受保护接口默认依赖上游中间件写入 `claims`，本层只做断言读取并向 service 传递，不自行派生权限结论。
// 4. 调用链统一为 `api -> service.GroupApp.Scene -> 领域/存储层`，成功结果通过 `c.Set("data", ...)` 交给统一响应包装。
// 5. 权限边界应落在鉴权中间件与 scene service：租户隔离、资源归属、激活条件和日志可见性不应散落在 Handler。
// 6. 静态审查建议：关注路径 ID 空值透传、列表接口分页上限、激活动作幂等性，以及日志查询是否可能越权读取。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type SceneApi struct{}

// CreateScene 创建场景。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.CreateSceneReq`，由绑定层完成基础格式校验。
// Claims：读取 `claims` 作为操作者身份与租户上下文，下传给 service 层执行创建授权和归属写入。
// 调用链：`CreateScene -> service.GroupApp.Scene.CreateScene`。
// 权限边界：是否允许创建、允许落入哪个项目或租户，由 service 层根据 claims 决定。
// 静态审查建议：确认请求中的关联资源 ID 在下游均有归属校验，避免通过合法场景创建挂接他人资源。
// 路由：`POST /api/v1/scene`
func (*SceneApi) CreateScene(c *gin.Context) {
	var req model.CreateSceneReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	id, err := service.GroupApp.Scene.CreateScene(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"scene_id": id})
}

// DeleteScene 删除场景。
// 参数绑定：通过 `c.Param("id")` 读取待删除场景 ID。
// Claims：读取 `claims` 并传给 service 层，供其校验操作者身份、租户归属和删除前置条件。
// 调用链：`DeleteScene -> service.GroupApp.Scene.DeleteScene`。
// 权限边界：删除是否允许、是否存在启用中或被引用的场景，必须由 service 层统一判定。
// 静态审查建议：检查对不存在场景和越权场景的错误返回是否一致，避免通过删除接口探测资源存在性。
// 路由：`DELETE /api/v1/scene/{id}`
func (*SceneApi) DeleteScene(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Scene.DeleteScene(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateScene 更新场景。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.UpdateSceneReq`，请求体承载目标场景及修改内容。
// Claims：读取 `claims`，用于下游执行修改授权、审计和租户隔离。
// 调用链：`UpdateScene -> service.GroupApp.Scene.UpdateScene`。
// 权限边界：可改字段、状态迁移与关联资源合法性应由 service 层统一收口，Handler 不做规则判断。
// 静态审查建议：确认更新接口对部分更新与全量覆盖的语义清晰，避免零值或空集合误伤已有配置。
// 路由：`PUT /api/v1/scene`
func (*SceneApi) UpdateScene(c *gin.Context) {
	var req model.UpdateSceneReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	id, err := service.GroupApp.Scene.UpdateScene(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"scene_id": id})
}

// DryRunScene previews ordinary scene action references without saving or executing actions.
// @Summary Dry-run ordinary scene actions
// @Tags Scene
// @Accept json
// @Produce json
// @Param request body model.DryRunSceneReq true "Scene dry-run payload"
// @Success 200 {object} model.SceneAutomationDryRunResult "Dry-run validation result"
// @Router /api/v1/scene/dry-run [post]
// 路由：`POST /api/v1/scene/dry-run`
func (*SceneApi) DryRunScene(c *gin.Context) {
	var req model.DryRunSceneReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Scene.DryRunScene(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleScene 查询场景详情。
// 参数绑定：通过 `c.Param("id")` 获取场景 ID。
// Claims：读取 `claims` 并传给 service 层，以便根据操作者范围控制详情可见性。
// 调用链：`HandleScene -> service.GroupApp.Scene.GetScene`。
// 权限边界：资源归属判断与敏感字段裁剪应在 service 层完成，避免 Handler 根据 claims 自行拼装返回。
// 静态审查建议：检查空 ID、非法 ID 与越权读取是否都走统一错误通道，减少信息侧漏。
// 路由：`GET /api/v1/scene/detail/{id}`
func (*SceneApi) HandleScene(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Scene.GetScene(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleSceneByPage 分页查询场景列表。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.GetSceneListByPageReq`，通常承接分页和筛选条件。
// Claims：读取 `claims`，供 service 层限制可见租户、项目和场景集合。
// 调用链：`HandleSceneByPage -> service.GroupApp.Scene.GetSceneListByPage`。
// 权限边界：列表结果和统计口径应由 service 层统一控制，本层不额外拼接过滤条件。
// 静态审查建议：确认分页参数存在上限且排序字段受控，避免慢查询或通过排序字段注入异常行为。
// 路由：`GET /api/v1/scene`
func (*SceneApi) HandleSceneByPage(c *gin.Context) {
	var req model.GetSceneListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Scene.GetSceneListByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ActiveScene 激活场景。
// 参数绑定：通过 `c.Param("id")` 读取待激活场景 ID。
// Claims：读取 `claims` 并传递给 service 层，用于校验操作者是否具备激活权限。
// 调用链：`ActiveScene -> service.GroupApp.Scene.ActiveScene`。
// 权限边界：激活前置条件、互斥规则、幂等语义和跨租户保护必须在 service 层统一处理。
// 静态审查建议：重点检查重复激活、并发激活和激活其他租户场景时的行为是否稳定且可审计。
// 路由：`POST /api/v1/scene/active/{id}`
func (*SceneApi) ActiveScene(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Scene.ActiveScene(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleSceneLog 分页查询场景日志。
// 参数绑定：通过 `BindAndValidate` 绑定 `model.GetSceneLogListByPageReq`，承接分页、筛选和时间范围等条件。
// Claims：读取 `claims` 并传给 service 层，确保日志查询遵循租户与角色可见性。
// 调用链：`HandleSceneLog -> service.GroupApp.Scene.GetSceneLog`。
// 权限边界：日志可见范围、敏感字段脱敏和审计约束应在 service 层统一处理，Handler 不做二次裁剪。
// 静态审查建议：检查时间范围和分页是否有限流或上限控制，避免高成本日志扫描与越权批量导出。
// 路由：`GET /api/v1/scene/log`
func (*SceneApi) HandleSceneLog(c *gin.Context) {
	var req model.GetSceneLogListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Scene.GetSceneLog(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
