// 文件用途：定义 expected datas 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

// 创建预期数据请求
// device_id send_type payload expiry_time label
type CreateExpectedDataReq struct {
	DeviceID string     `json:"device_id" form:"device_id" validate:"required,max=36"`                                   // 设备ID
	SendType string     `json:"send_type" form:"send_type" validate:"required,max=50,oneof=telemetry attribute command"` // 发送类型
	Payload  *string    `json:"payload" form:"payload" validate:"omitempty,max=9999"`                                    // 数据内容
	Expiry   *time.Time `json:"expiry" form:"expiry" validate:"omitempty"`                                               // 过期时间
	Label    *string    `json:"label" form:"label" validate:"omitempty,max=100"`                                         // 标签
	Identify *string    `json:"identify" form:"identify" validate:"omitempty,max=100"`                                   // 标识
}

// 删除预期数据请求
type DeleteExpectedDataReq struct {
	ID string `json:"id" form:"id" validate:"required,max=36"` // 预期数据ID
}

// 分页查询预期数据请求
type GetExpectedDataPageReq struct {
	PageReq
	DeviceID string  `json:"device_id" form:"device_id" validate:"required,max=36"`  // 设备ID
	SendType *string `json:"send_type" form:"send_type" validate:"omitempty,max=50"` // 发送类型
	Label    *string `json:"label" form:"label" validate:"omitempty,max=100"`        // 标签
	Status   *string `json:"status" form:"status" validate:"omitempty,max=50"`       // 状态
}
