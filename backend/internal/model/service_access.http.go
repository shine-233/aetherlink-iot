// 文件用途：定义 service access 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateAccessReq struct {
	Name                string `json:"name" binding:"required"`
	ServicePluginID     string `json:"service_plugin_id" binding:"required"`
	Voucher             string `json:"voucher" binding:"required"`
	Description         string `json:"description"`
	ServiceAccessConfig string `json:"service_access_config"`
	Remark              string `json:"remark" `
}

type UpdateAccessReq struct {
	ID                  string  `json:"id" binding:"required"`
	ServiceAccessConfig *string `json:"service_access_config"`
	Name                *string `json:"name"`
	Voucher             *string `json:"voucher"`
}

type DeleteAccessReq struct {
	ID string `json:"id" form:"id" binding:"required"`
}

type GetServiceAccessByPageReq struct {
	PageReq
	ServicePluginID string `json:"service_plugin_id" form:"service_plugin_id"`
}

type GetServiceAccessVoucherFormReq struct {
	ServicePluginID string `json:"service_plugin_id" form:"service_plugin_id"  binding:"required"`
}

// 服务接入点设备列表 voucher page_size page
type ServiceAccessDeviceListReq struct {
	PageReq
	Voucher string `json:"voucher" form:"voucher" binding:"required"`
}

type GetPluginServiceAccessListReq struct {
	ServiceIdentifier string `json:"service_identifier" form:"service_identifier" binding:"required"`
}

type GetPluginServiceAccessReq struct {
	ServiceAccessID string `json:"service_access_id" form:"service_access_id" binding:"required"`
}
