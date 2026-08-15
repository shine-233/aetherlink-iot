// 文件用途: 定义 command set logs 相关 HTTP 入参、出参和列表查询结构。
// 核心逻辑: 使用 json/form/validate 标签描述分页筛选和请求校验契约。
// 关键注意事项: 这里只维护传输结构和校验标签，不放入权限、事务或数据库访问逻辑。
package model

type GetCommandSetLogsListByPageReq struct {
	PageReq
	DeviceId      string  `json:"device_id" form:"device_id" validate:"required,max=36"`               // 设备ID
	Identify      *string `json:"identify" form:"identify" validate:"omitempty,max=36"`                // 数据标识符
	Status        *string `json:"status" form:"status" validate:"omitempty,oneof=0 1 2 3 4"`           // 状态: 0-待发送 1-发送成功 2-失败 3-返回成功 4-返回失败
	OperationType *string `json:"operation_type" form:"operation_type" validate:"omitempty,oneof=1 2"` // 操作类型: 1-手动操作 2-自动触发
	IdentifyName  *string `json:"identify_name" form:"identify_name" validate:"omitempty,max=100"`     // 数据标识符名称
}

// DirectMethodCommandReq requests one auditable command and waits briefly for
// the correlated device response. A zero timeout uses the service default.
type DirectMethodCommandReq struct {
	DeviceID       string  `json:"device_id" form:"device_id" validate:"required,max=36"`
	Value          *string `json:"value" form:"value" validate:"omitempty,max=9999"`
	Identify       string  `json:"identify" form:"identify" validate:"required,max=255"`
	TimeoutSeconds int     `json:"timeout_seconds" form:"timeout_seconds" validate:"omitempty,min=1,max=30"`
}
