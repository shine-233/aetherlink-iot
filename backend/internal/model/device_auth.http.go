// 文件用途：定义 device auth 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// DeviceAuthReq 设备动态认证请求结构体
type DeviceAuthReq struct {
	TemplateSecret     string  `json:"template_secret" validate:"required,max=255"`       // 设备配置密钥
	DeviceNumber       string  `json:"device_number" validate:"required,max=255"`         // 设备唯一标识
	DeviceName         *string `json:"device_name" validate:"omitempty,max=255"`          // 设备名称(可选)
	ProductKey         *string `json:"product_key" validate:"omitempty,max=255"`          // 产品密钥(可选，用于产品关联)
	SubDeviceAddr      *string `json:"sub_device_addr" validate:"omitempty,max=255"`      // 子设备地址(可选，用于子设备关联)
	ParentDeviceNumber *string `json:"parent_device_number" validate:"omitempty,max=255"` // 父设备编号(可选，用于子设备关联)
}

// DeviceAuthRes 设备动态认证响应结构体
type DeviceAuthRes struct {
	DeviceID string `json:"device_id"` // 设备ID
	Voucher  string `json:"voucher"`   // 设备凭证
}
