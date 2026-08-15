// 文件用途：定义 protocol plugins 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// 协议插件获取设备配置请求
type GetDeviceConfigReq struct {
	DeviceId     string `json:"device_id"  form:"device_id" validate:"omitempty,max=36"`
	Voucher      string `json:"voucher"  form:"voucher" validate:"omitempty,max=255"`
	DeviceNumber string `json:"device_number"  form:"device_number" validate:"omitempty,max=255"`
}

type GetProtocolPluginFormByProtocolType struct {
	ProtocolType string `json:"protocol_type"  form:"protocol_type" validate:"required,max=255"`
	DeviceType   string `json:"device_type"  form:"device_type" validate:"required,max=10"`
}

// 协议插件获取设备配置
type DeviceConfigForProtocolPlugin struct {
	ID                     string                             `json:"id"`
	Voucher                string                             `json:"voucher"`
	DeviceType             string                             `json:"device_type"`
	ProtocolType           string                             `json:"protocol_type"`
	DeviceNumber           string                             `json:"device_number"`
	Config                 map[string]interface{}             `json:"config"`
	ProtocolConfigTemplate map[string]interface{}             `json:"protocol_config_template"`
	SubDivices             []SubDeviceConfigForProtocolPlugin `json:"sub_devices"`
}

// 协议插件获取设备配置的子设备配置
type SubDeviceConfigForProtocolPlugin struct {
	DeviceID               string                 `json:"device_id"`
	DeviceNumber           string                 `json:"device_number"`
	Voucher                string                 `json:"voucher"`
	SubDeviceAddr          string                 `json:"sub_device_addr"`
	Config                 map[string]interface{} `json:"config"`
	ProtocolConfigTemplate map[string]interface{} `json:"protocol_config_template"`
}

type GetDevicesByProtocolPluginRsp struct {
	List  []DeviceConfigForProtocolPlugin `json:"list"`
	Total int64                           `json:"total"`
}

type GetDevicesByProtocolPluginReq struct {
	ServiceIdentifier string `json:"service_identifier"  form:"service_identifier" validate:"required,max=255"`
	DeviceType        string `json:"device_type"  form:"device_type" validate:"required,max=10"`
	PageReq
}
