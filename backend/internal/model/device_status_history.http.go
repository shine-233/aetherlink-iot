// 文件用途：定义 device status history 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// GetDeviceStatusHistoryReq 获取设备状态历史请求
type GetDeviceStatusHistoryReq struct {
	PageReq
	DeviceID  string `json:"device_id" form:"device_id" validate:"required,max=36"` // 设备ID
	StartTime *int64 `json:"start_time" form:"start_time" validate:"omitempty"`     // 开始时间戳（秒）
	EndTime   *int64 `json:"end_time" form:"end_time" validate:"omitempty"`         // 结束时间戳（秒）
	Status    *int16 `json:"status" form:"status" validate:"omitempty,oneof=0 1"`   // 状态筛选：0-离线，1-在线
}
