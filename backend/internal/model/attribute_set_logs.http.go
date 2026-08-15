// 文件用途：定义 attribute set logs 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type GetAttributeSetLogsListByPageReq struct {
	PageReq
	DeviceId      string  `json:"device_id" form:"device_id" validate:"required,max=36"`               // 设备ID
	Status        *string `json:"status" form:"status" validate:"omitempty,oneof=1 2 3 4"`             //状态 1-发送成功 2- 发送失败3-返回成功 4-返回失败
	OperationType *string `json:"operation_type" form:"operation_type" validate:"omitempty,oneof=1 2"` //操作类型 1-手动操作 2-自动触发
}

type AttributePutMessage struct {
	DeviceID string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Value    string `json:"value" form:"value" validate:"required"`
}

// 发送
type AttributeGetMessageReq struct {
	DeviceID string   `json:"device_id" form:"device_id" validate:"required,max=36"`
	Keys     []string `json:"keys" form:"keys" validate:"max=9999"`
}

// 根据key查询设备属性
type GetDataListByKeyReq struct {
	DeviceId string `json:"device_id" form:"device_id" validate:"required,max=36"`
	Key      string `json:"key" form:"key" validate:"required,max=255"`
}
