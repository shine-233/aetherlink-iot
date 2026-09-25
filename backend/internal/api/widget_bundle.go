// 文件用途：部件库（widget_bundles，ROADMAP TB-04）HTTP 处理器。
// 核心逻辑：入参绑定校验、鉴权边界注入（claims）、Service 分发与统一 API 响应包装；
//
//	含内置四部件定义导出与一键种子落库两个入口。
//
// 关键注意事项：请求上下文一律透传 c.Request.Context()，不引入 context.Background()
//
//	（internal/api 的 context.Background() 有数量预算守卫）。
//
// 重构建议：后续若部件库加封面/缩略图，新增独立 handler，不在 CRUD 里塞文件上传。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type WidgetBundleApi struct{}

// CreateWidgetBundle 创建部件库
// @Router   /api/v1/widget-bundles [post]
func (*WidgetBundleApi) CreateWidgetBundle(c *gin.Context) {
	var req model.CreateWidgetBundleReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.WidgetBundle.CreateWidgetBundle(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateWidgetBundle 更新部件库
// @Router   /api/v1/widget-bundles [put]
func (*WidgetBundleApi) UpdateWidgetBundle(c *gin.Context) {
	var req model.UpdateWidgetBundleReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.WidgetBundle.UpdateWidgetBundle(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetWidgetBundleByID 查询单个部件库
// @Router   /api/v1/widget-bundles/:id [get]
func (*WidgetBundleApi) GetWidgetBundleByID(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.WidgetBundle.GetWidgetBundleByID(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteWidgetBundle 删除部件库
// @Router   /api/v1/widget-bundles/:id [delete]
func (*WidgetBundleApi) DeleteWidgetBundle(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.WidgetBundle.DeleteWidgetBundle(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"id": id})
}

// ListWidgetBundles 分页查询列表
// @Router   /api/v1/widget-bundles [get]
func (*WidgetBundleApi) ListWidgetBundles(c *gin.Context) {
	var req model.GetWidgetBundleListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.WidgetBundle.ListWidgetBundles(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetBuiltinWidgetBundle 内置四部件定义导出描述（种子 bundle 内容源）
// @Router   /api/v1/widget-bundles/builtin [get]
func (*WidgetBundleApi) GetBuiltinWidgetBundle(c *gin.Context) {
	data, err := service.GroupApp.WidgetBundle.ExportBuiltinWidgetBundle()
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SeedBuiltinWidgetBundle 一键把内置四部件落为租户可管理种子 bundle
// @Router   /api/v1/widget-bundles/seed [post]
func (*WidgetBundleApi) SeedBuiltinWidgetBundle(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.WidgetBundle.SeedBuiltinWidgetBundle(c.Request.Context(), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
