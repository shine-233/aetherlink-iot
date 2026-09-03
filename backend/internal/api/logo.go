// logo.go 提供品牌 Logo 与站点品牌配置相关的 HTTP 入口。
// 核心链路：
// 1. 更新接口负责绑定品牌配置表单，并把当前操作者 claims 传给 Logo service。
// 2. 查询接口读取当前品牌资源列表，供前端系统设置页回显标题、Logo、登录页图片等视觉资产。
// 3. API 层只做参数与身份边界收口，不在这里处理资源可访问性校验、文件上传或缓存刷新细节。
// 静态审查建议：
//  1. 品牌资源通常会影响全局标题、页签图标和登录页展示，后续若拆多品牌或环境隔离，要先补清读取优先级。
//  2. `HandleLogoList` 当前不读取 claims，若未来品牌配置改成租户隔离模型，需要同步补上可见范围控制。
//  3. 若更新逻辑继续扩展，适合补结构化审计信息，记录谁在何时修改了哪些品牌资源字段。
//  4. `HandleLogoList` 已改为租户隔离模型：经 OptionalJWTAuth 注入 claims 后按 tenant_id 下传，
//     匿名请求（登录页等）在 tenant_id 为空串时返回系统全局兜底品牌，已登录租户仅见本租户品牌。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type LogoApi struct{}

// UpdateLogo 更新品牌 Logo 与视觉配置。
// 该入口负责请求体绑定与 claims 注入，真正的字段更新、资源地址处理和缓存副作用由 service 层统一编排。
// @Router   /api/v1/logo [put]
func (LogoApi) UpdateLogo(c *gin.Context) {
	var req model.UpdateLogoReq
	if !BindAndValidate(c, &req) {
		return
	}

	// claims 用于确认当前操作者是否有权修改系统品牌资源，避免把权限判断散落在前端。
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Logo.UpdateLogo(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleLogoList 获取当前品牌配置列表。
// 已改为租户隔离模型：从 claims 读取 tenant_id 并下传 service，仅返回调用者所属租户（或系统全局）的品牌资源。
// 未携带 claims 的公开请求（如登录页）在 tenant_id 为空串时返回系统全局兜底品牌。
// @Router   /api/v1/logo [get]
func (LogoApi) HandleLogoList(c *gin.Context) {
	tenantID := ""
	if v, ok := c.Get("claims"); ok {
		if uc, ok2 := v.(*utils.UserClaims); ok2 {
			tenantID = uc.TenantID
		}
	}
	logoList, err := service.GroupApp.Logo.GetLogoList(tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", logoList)
}
