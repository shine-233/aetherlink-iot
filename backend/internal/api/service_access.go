// service_access.go 提供服务接入点相关的 HTTP 入口。
// 核心链路：
// 1. 后台管理侧负责服务接入点的创建、分页列表、更新、删除和凭证表单查询。
// 2. 业务侧提供三方服务设备列表查询。
// 3. 插件侧接口通过 OpenAPIKey 鉴权暴露服务接入点清单与详情。
// 静态审查建议：
// 1. 当前文件同时服务后台、业务查询和插件接入三类场景，后续继续扩展时建议按调用方拆分。
// 2. `plugin/service/access` 与后台 `/service/access` 的路径契约很相近，改动路由时要注意不要误伤插件侧。
// 3. 凭证表单与设备列表查询都会影响前端服务接入流程，接口字段漂移时要同步设备管理页二级筛选与接入页表单。
// 迁移说明：标准“绑定 -> 校验 -> 取 claims -> 调 service -> respond”样板已收敛到 handler_adapter 骨架；
// 插件侧两个入口因 OpenAPI 门禁必须先于 claims 读取（OpenAPIKeyAuth 成功时自行写入等效 claims），
// 保持手工编排，仅复用 RequireClaims / respond 出口。
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
	Handle(c, func(req *model.CreateAccessReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.ServiceAccess.CreateAccess(req, claims)
	})
}

// HandleList 服务接入点分页列表。
// 主要服务于前端接入点管理页与设备管理页的二级服务筛选前置数据。
// /api/v1/service/access/list
func (*ServiceAccessApi) HandleList(c *gin.Context) {
	Handle(c, func(req *model.GetServiceAccessByPageReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.ServiceAccess.List(req, claims)
	})
}

// Update 更新服务接入点。
// 该入口保持薄控制器模式，具体配置差异判断和下游同步交给 service。
// /api/v1/service/access [put]
func (*ServiceAccessApi) Update(c *gin.Context) {
	HandleAction(c, func(req *model.UpdateAccessReq, claims *utils.UserClaims) error {
		return service.GroupApp.ServiceAccess.Update(req, claims)
	})
}

// Delete 删除服务接入点。
// 删除后可能影响设备接入筛选与插件对接，因此仅做 ID 和 claims 透传，由 service 处理副作用。
// /api/v1/service/access/:id [delete]
func (*ServiceAccessApi) Delete(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, claims *utils.UserClaims) error {
		return service.GroupApp.ServiceAccess.Delete(id, claims)
	})
}

// HandleVoucherForm 返回服务接入点凭证表单定义。
// 前端服务接入创建/编辑页会依赖这个接口动态装配凭证字段。
// /api/v1/service/access/voucher/form [get]
func (*ServiceAccessApi) HandleVoucherForm(c *gin.Context) {
	HandlePublic(c, func(req *model.GetServiceAccessVoucherFormReq) (interface{}, error) {
		return service.GroupApp.ServiceAccess.GetVoucherForm(req)
	})
}

// HandleDeviceList 返回三方服务侧的设备列表。
// 该接口常用于把外部服务接入设备映射到本平台设备管理视图。
// /api/v1/service/access/device/list
func (*ServiceAccessApi) HandleDeviceList(c *gin.Context) {
	Handle(c, func(req *model.ServiceAccessDeviceListReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.ServiceAccess.GetServiceAccessDeviceList(req, claims)
	})
}

// HandlePluginServiceAccessList 为插件侧返回服务接入点列表。
// 这类接口必须先经过 OpenAPIKey 鉴权，避免插件侧绕过后台权限边界。
// 编排约束：OpenAPIKeyAuth 成功时会自行写入等效 claims，门禁必须先于 claims 读取，
// 因此不能改用 Handle（其会在门禁之前取 claims，导致插件请求被误判为未授权）。
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
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	resp, err := service.GroupApp.ServiceAccess.GetPluginServiceAccessList(&req, claims)
	respond(c, resp, err)
}

// HandlePluginServiceAccess 为插件侧返回单个服务接入点详情或匹配结果。
// 编排约束同 HandlePluginServiceAccessList：OpenAPI 门禁必须先于 claims 读取。
// /api/v1/plugin/service/access
func (*ServiceAccessApi) HandlePluginServiceAccess(c *gin.Context) {
	var req model.GetPluginServiceAccessReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	resp, err := service.GroupApp.ServiceAccess.GetPluginServiceAccess(&req, claims)
	respond(c, resp, err)
}
