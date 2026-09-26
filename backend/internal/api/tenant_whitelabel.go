// 文件用途：白标（TB-47）租户翻译覆盖与自定义 CSS 的 HTTP 处理器。
// 核心逻辑：请求绑定与校验、claims 租户边界解析（写操作 SYS_ADMIN/TENANT_ADMIN，
//
//	覆盖获取任何登录用户）、Service 分发与统一 API 响应包装。
//
// 关键注意事项：
//  1. 请求上下文一律透传 c.Request.Context()，不引入 context.Background()
//     （internal/api 的 context.Background() 有数量预算守卫，预算 4）。
//  2. 身份边界 fail-closed：claims 缺失按无权限处理；租户作用域与角色矩阵集中在
//     service.whitelabelWriteScope/whitelabelReadScope，API 层不重复实现授权判断。
//  3. 本组路由全部位于 CasbinRBAC 之后（134.sql 登记 g2/p；GET overrides 对
//     TENANT_USER 放行），路由路径调整必须同步迁移里的 casbin 种子。
//
// 重构建议：branding-setting.vue 接入（下一阶段前端面）若需要"预览合并结果"，
//
//	优先复用 GET overrides 返回值在前端合并，不要新增服务端合并端点。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TenantWhitelabelApi 白标（翻译覆盖 + 自定义 CSS）控制器。
type TenantWhitelabelApi struct{}

// whitelabelClaims 从上下文解析登录态 claims（JWT 中间件保证存在，缺失误为无权限）。
func whitelabelClaims(c *gin.Context) *utils.UserClaims {
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*utils.UserClaims)
	return claims
}

// UpsertTenantTranslations 批量写入翻译覆盖（UPSERT，同键覆盖）。
// @Summary Upsert tenant translation overrides
// @Tags Whitelabel
// @Accept json
// @Produce json
// @Param body body model.UpsertTenantTranslationsReq true "Translation items"
// @Router /api/v1/whitelabel/translations [put]
func (*TenantWhitelabelApi) UpsertTenantTranslations(c *gin.Context) {
	var req model.UpsertTenantTranslationsReq
	if !BindAndValidate(c, &req) {
		return
	}
	count, err := service.GroupApp.Whitelabel.UpsertTranslations(c.Request.Context(), &req, whitelabelClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"count": count})
}

// ListTenantTranslations 管理面列表翻译覆盖（lang 可选过滤）。
// @Summary List tenant translation overrides
// @Tags Whitelabel
// @Produce json
// @Param lang query string false "Language filter (zh-cn/en-us/es-es/fr-fr)"
// @Router /api/v1/whitelabel/translations [get]
func (*TenantWhitelabelApi) ListTenantTranslations(c *gin.Context) {
	var req model.ListTenantTranslationsReq
	if !BindAndValidate(c, &req) {
		return
	}
	resp, err := service.GroupApp.Whitelabel.ListTranslations(c.Request.Context(), req.Lang, whitelabelClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// DeleteTenantTranslations 批量删除翻译覆盖。
// @Summary Delete tenant translation overrides
// @Tags Whitelabel
// @Accept json
// @Produce json
// @Param body body model.DeleteTenantTranslationsReq true "Translation keys"
// @Router /api/v1/whitelabel/translations [delete]
func (*TenantWhitelabelApi) DeleteTenantTranslations(c *gin.Context) {
	var req model.DeleteTenantTranslationsReq
	if !BindAndValidate(c, &req) {
		return
	}
	deleted, err := service.GroupApp.Whitelabel.DeleteTranslations(c.Request.Context(), &req, whitelabelClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"deleted": deleted})
}

// GetTenantCustomCSS 管理面读取本作用域自定义 CSS（回显编辑框）。
// @Summary Get tenant custom CSS
// @Tags Whitelabel
// @Produce json
// @Router /api/v1/whitelabel/custom-css [get]
func (*TenantWhitelabelApi) GetTenantCustomCSS(c *gin.Context) {
	resp, err := service.GroupApp.Whitelabel.GetCustomCSS(c.Request.Context(), whitelabelClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// UpsertTenantCustomCSS 写入自定义 CSS（空串=清除；服务端拒绝 </style 序列与超长）。
// @Summary Upsert tenant custom CSS
// @Tags Whitelabel
// @Accept json
// @Produce json
// @Param body body model.UpsertTenantCustomCSSReq true "Custom CSS (empty clears)"
// @Router /api/v1/whitelabel/custom-css [put]
func (*TenantWhitelabelApi) UpsertTenantCustomCSS(c *gin.Context) {
	var req model.UpsertTenantCustomCSSReq
	if !BindAndValidate(c, &req) {
		return
	}
	if err := service.GroupApp.Whitelabel.UpsertCustomCSS(c.Request.Context(), &req, whitelabelClaims(c)); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetWhitelabelOverrides 登录后可读的覆盖获取：本作用域全部翻译覆盖（按语言分组）
// 与自定义 CSS，供前端 i18n 初始化后合并与 style 注入。
// @Summary Get whitelabel overrides for the caller tenant
// @Tags Whitelabel
// @Produce json
// @Router /api/v1/whitelabel/overrides [get]
func (*TenantWhitelabelApi) GetWhitelabelOverrides(c *gin.Context) {
	resp, err := service.GroupApp.Whitelabel.GetWhitelabelOverrides(c.Request.Context(), whitelabelClaims(c))
	if err != nil {
		logrus.Error(err)
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
