// 文件用途：定义 service plugins 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// 服务接入配置
type ServiceAccessConfig struct {
	HttpAddress    string `json:"http_address"`
	SubTopicPrefix string `json:"sub_topic_prefix"`
}

// 协议接入配置
type ProtocolAccessConfig struct {
	DeviceType     int    `json:"device_type"`
	AccessAddress  string `json:"access_address"`
	HttpAddress    string `json:"http_address"`
	SubTopicPrefix string `json:"sub_topic_prefix"`
}

type CreateServicePluginReq struct {
	Name              string `json:"name" binding:"required,max=255"`
	ServiceIdentifier string `json:"service_identifier" binding:"required,max=100"`
	ServiceType       int32  `json:"service_type" binding:"required,oneof=1 2"`
	Version           string `json:"version" binding:"omitempty,max=100"`
	Description       string `json:"description" binding:"omitempty,max=255"`
	ServiceConfig     string `json:"service_config" binding:"omitempty"`
	Remark            string `json:"remark" binding:"omitempty,max=255"`
}

type GetServicePluginByPageReq struct {
	PageReq
	ServiceType int32 `json:"service_type" form:"service_type"`
}

type UpdateServicePluginReq struct {
	ID string `json:"id" form:"id" binding:"required"`

	Name              string `json:"name" binding:"max=255"`
	ServiceIdentifier string `json:"service_identifier" binding:"max=100"`
	ServiceType       int32  `json:"service_type" binding:"omitempty,oneof=1 2"`
	Version           string `json:"version" binding:"max=100"`
	Description       string `json:"description" binding:"max=255"`
	Remark            string `json:"remark" binding:"max=255"`

	ServiceConfig string `json:"service_config" binding:"omitempty"`
}

type DeleteServicePluginReq struct {
	ID string `json:"id" form:"id" binding:"required"`
}

// HeartbeatReq
type HeartbeatReq struct {
	ServiceIdentifier string `json:"service_identifier" binding:"required,max=100"`
}

// GetServiceSelectReq
type GetServiceSelectReq struct {
	DeviceType *int `json:"device_type" form:"device_type"`
}

type GetServicePluginByServiceIdentifierReq struct {
	ServiceIdentifier string `json:"service_identifier" form:"service_identifier" binding:"required,max=100"`
}
