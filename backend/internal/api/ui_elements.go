// 文件用途：提供 UI 元素权限配置相关的 HTTP handler，
// 负责把前端提交的页面元素、菜单权限和查询条件转换成 service 层调用。
// 核心职责：在 API 边界完成请求绑定、claims 提取、错误透传和统一响应数据挂载，不承载权限业务实现细节。
// claims 约定：依赖鉴权中间件预先通过 c.Set("claims") 注入 *utils.UserClaims，
// 这里的 c.MustGet("claims") 默认要求当前请求已经通过认证且具备可判定的 authority 信息。
// 调用链概览：handler -> service.GroupApp.UiElements -> dal.*；写操作最终落到 sys_ui_elements 相关 DAL。
// 静态审查建议：关注 UI 可见性配置是否被误当作后端鉴权、路径 ID 是否需要统一校验、
// 以及局部必填校验与 Update 语义之间是否存在不一致。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type UiElementsApi struct{}

// CreateUiElements 创建 UI 元素配置。
// 参数绑定：通过 BindAndValidate 绑定 JSON body 到 model.CreateUiElementsReq，
// 重点校验 parent_id、element_code、authority 等字段，其他扩展字段按标签做长度约束。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service 层据此限制仅系统管理员可写入。
// 调用链：UiElementsApi.CreateUiElements -> service.GroupApp.UiElements.CreateUiElements -> dal.CreateUiElements。
// 静态审查建议：关注 authority、element_type 的枚举值是否需要在 API 层更早失败，以及 route_path/多语言字段是否需要格式校验。
// @Router   /api/v1/ui_elements [post]
func (*UiElementsApi) CreateUiElements(c *gin.Context) {
	var req model.CreateUiElementsReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.UiElements.CreateUiElements(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// UpdateUiElements 更新 UI 元素配置。
// 参数绑定：通过 BindAndValidate 绑定 JSON body 到 model.UpdateUiElementsReq，
// 随后在 handler 内补充要求 element_type 或 authority 至少提供一个，以避免无效更新请求。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service 层据此限制仅系统管理员可更新。
// 调用链：UiElementsApi.UpdateUiElements -> service.GroupApp.UiElements.UpdateUiElements -> dal.UpdateUiElements。
// 静态审查建议：关注指针字段的“未传/显式置空”语义是否清晰，以及局部校验条件是否应该沉淀为复用校验器。
// @Router   /api/v1/ui_elements [put]
func (*UiElementsApi) UpdateUiElements(c *gin.Context) {
	var req model.UpdateUiElementsReq
	if !BindAndValidate(c, &req) {
		return
	}

	if req.ElementType == nil && req.Authority == nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"element_type": "element_type or authority is required",
		}))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.UiElements.UpdateUiElements(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// DeleteUiElements 删除 UI 元素配置。
// 参数绑定：通过 c.Param("id") 读取路径参数，不经过 BindAndValidate。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service 层据此限制仅系统管理员可删除。
// 调用链：UiElementsApi.DeleteUiElements -> service.GroupApp.UiElements.DeleteUiElements -> dal.DeleteUiElements。
// 静态审查建议：关注路径 id 的格式校验、级联删除/子节点残留风险，以及删除操作是否需要审计记录。
// @Router   /api/v1/ui_elements/{id} [delete]
func (*UiElementsApi) DeleteUiElements(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.UiElements.DeleteUiElements(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ServeUiElementsListByPage UI 元素分页查询。
// 参数绑定：通过 BindAndValidate 绑定 Query/Form 到 model.ServeUiElementsListByPageReq，
// 当前主要承接分页参数，筛选逻辑由 service/DAL 层决定。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service 层据此限制仅系统管理员可查询完整列表。
// 调用链：UiElementsApi.ServeUiElementsListByPage -> service.GroupApp.UiElements.ServeUiElementsListByPage -> dal.ServeUiElementsListByPage。
// 静态审查建议：关注分页上限、防止全量拉取，以及返回树形/平铺结构契约是否与前端保持一致。
// @Router   /api/v1/ui_elements [get]
func (*UiElementsApi) ServeUiElementsListByPage(c *gin.Context) {
	var req model.ServeUiElementsListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	UiElementsList, err := service.GroupApp.UiElements.ServeUiElementsListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", UiElementsList)
}

// ServeUiElementsListByAuthority 根据当前用户权限查询 UI 元素。
// 参数绑定：不绑定请求体，直接依赖上下文中的 claims 决定返回范围。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service/DAL 使用其中的用户与权限信息裁剪可见元素。
// 调用链：UiElementsApi.ServeUiElementsListByAuthority -> service.GroupApp.UiElements.ServeUiElementsListByAuthority -> dal.ServeUiElementsListByAuthority。
// 静态审查建议：持续确认“前端可见”与“后端可调用”是两条独立安全边界，避免把 UI 权限当作接口鉴权替代。
// @Router   /api/v1/ui_elements/menu [get]
func (*UiElementsApi) ServeUiElementsListByAuthority(c *gin.Context) {
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	uiElementsList, err := service.GroupApp.UiElements.ServeUiElementsListByAuthority(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", uiElementsList)
}

// ServeUiElementsListByTenant 获取租户侧菜单权限配置表单。
// 参数绑定：不绑定请求体，直接依赖上下文中的 claims 判定是否允许查询租户配置树。
// claims：从 c.MustGet("claims") 读取 *utils.UserClaims，service 层允许 SYS_ADMIN/TENANT_ADMIN 查看租户配置表单。
// 调用链：UiElementsApi.ServeUiElementsListByTenant -> service.GroupApp.UiElements.GetTenantUiElementsList -> dal.GetTenantUiElementsList。
// 静态审查建议：关注返回树结构的稳定性、租户管理员边界是否和产品预期一致，并补齐 Swagger 路由注释的一致性。
// @Router   /api/v1/ui_elements/select/form [get]
func (*UiElementsApi) ServeUiElementsListByTenant(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	uiElementsList, err := service.GroupApp.UiElements.GetTenantUiElementsList(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", uiElementsList)
}
