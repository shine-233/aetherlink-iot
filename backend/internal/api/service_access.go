// service_access.go 提供服务接入点相关的 HTTP 入口。
// 核心链路：
// 1. 后台管理侧负责服务接入点的创建、分页列表、更新、删除和凭证表单查询。
// 2. 业务侧提供三方服务设备列表查询。
// 3. 插件侧接口通过 OpenAPIKey 鉴权暴露服务接入点清单与详情。
// 静态审查建议：
// 1. 当前文件同时服务后台、业务查询和插件接入三类场景，后续继续扩展时建议按调用方拆分。
// 2. `plugin/service/access` 与后台 `/service/access` 的路径契约很相近，改动路由时要注意不要误伤插件侧。
// 3. 凭证表单与设备列表查询都会影响前端服务接入流程，接口字段漂移时要同步设备管理页二级筛选与接入页表单。
package api

import (
	"aetherlink-iot/backend/internal/middleware"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ServiceAccessApi struct{}

// Create 创建服务接入点。
// 请求体通常承载接入点配置、凭证模式和插件关联信息；claims 决定租户与权限边界。
// /api/v1/service/access [post]
func (*ServiceAccessApi) Create(c *gin.Context) {
	var req model.CreateAccessReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServiceAccess.CreateAccess(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleList 服务接入点分页列表。
// 主要服务于前端接入点管理页与设备管理页的二级服务筛选前置数据。
// /api/v1/service/access/list
func (*ServiceAccessApi) HandleList(c *gin.Context) {
	var req model.GetServiceAccessByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServiceAccess.List(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Update 更新服务接入点。
// 该入口保持薄控制器模式，具体配置差异判断和下游同步交给 service。
// /api/v1/service/access [put]
func (*ServiceAccessApi) Update(c *gin.Context) {
	var req model.UpdateAccessReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.ServiceAccess.Update(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// Delete 删除服务接入点。
// 删除后可能影响设备接入筛选与插件对接，因此仅做 ID 和 claims 透传，由 service 处理副作用。
// /api/v1/service/access/:id [delete]
func (*ServiceAccessApi) Delete(c *gin.Context) {
	id := c.Param("id")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.ServiceAccess.Delete(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleVoucherForm 返回服务接入点凭证表单定义。
// 前端服务接入创建/编辑页会依赖这个接口动态装配凭证字段。
// /api/v1/service/access/voucher/form [get]
func (*ServiceAccessApi) HandleVoucherForm(c *gin.Context) {
	var req model.GetServiceAccessVoucherFormReq
	if !BindAndValidate(c, &req) {
		return
	}
	resp, err := service.GroupApp.ServiceAccess.GetVoucherForm(&req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleDeviceList 返回三方服务侧的设备列表。
// 该接口常用于把外部服务接入设备映射到本平台设备管理视图。
// /api/v1/service/access/device/list
func (*ServiceAccessApi) HandleDeviceList(c *gin.Context) {
	var req model.ServiceAccessDeviceListReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServiceAccess.GetServiceAccessDeviceList(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandlePluginServiceAccessList 为插件侧返回服务接入点列表。
// 这类接口必须先经过 OpenAPIKey 鉴权，避免插件侧绕过后台权限边界。
// /api/v1/plugin/service/access/list
func (*ServiceAccessApi) HandlePluginServiceAccessList(c *gin.Context) {
	logrus.Info("get plugin list")
	var req model.GetPluginServiceAccessListReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServiceAccess.GetPluginServiceAccessList(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandlePluginServiceAccess 为插件侧返回单个服务接入点详情或匹配结果。
// /api/v1/plugin/service/access
func (*ServiceAccessApi) HandlePluginServiceAccess(c *gin.Context) {
	var req model.GetPluginServiceAccessReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ServiceAccess.GetPluginServiceAccess(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
