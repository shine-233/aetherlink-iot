// 文件用途：定义 alarm info 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

// AlarmHistoryQueryStatusActive is a read-only list-filter alias for alarm
// streams whose current persisted state is H/M/L. It is not an alarm_history
// value and must never be stored.
const AlarmHistoryQueryStatusActive = "ACTIVE"

type GetAlarmInfoListByPageReq struct {
	PageReq
	StartTime        *time.Time `json:"start_time" form:"start_time" validate:"omitempty"`               // Alarm start time
	EndTime          *time.Time `json:"end_time" form:"end_time" validate:"omitempty"`                   // Alarm end time
	AlarmLevel       *string    `json:"alarm_level" form:"alarm_level" validate:"omitempty"`             // Alarm level
	ProcessingResult *string    `json:"processing_result" form:"processing_result" validate:"omitempty"` // Processing result
	TenantID         string     `json:"tenant_id" form:"tenant_id" validate:"omitempty"`
}

type UpdateAlarmInfoReq struct {
	Id               string  `json:"id" validate:"required,max=36"`
	ProcessingResult *string `json:"processing_result" validate:"required"` // Processing result
}

type UpdateAlarmInfoBatchReq struct {
	Id                     []string `json:"id" validate:"required"`
	ProcessingResult       *string  `json:"processing_result" validate:"required"`       // Processing result
	ProcessingInstructions *string  `json:"processing_instructions" validate:"required"` // Processing instructions
}

type GetAlarmHisttoryListByPage struct {
	PageReq
	StartTime   *time.Time `json:"start_time" form:"start_time" validate:"omitempty"`     // Alarm start time
	EndTime     *time.Time `json:"end_time" form:"end_time" validate:"omitempty"`         // Alarm end time
	AlarmStatus *string    `json:"alarm_status" form:"alarm_status" validate:"omitempty"` // H/M/L/N, ACTIVE for H+M+L, empty for all
	AlarmType   *string    `json:"alarm_type" form:"alarm_type" validate:"omitempty"`     // RDI alarm event type
	DeviceId    *string    `json:"device_id" form:"device_id" validate:"omitempty"`       // Device ID
	AllTenants  bool       `json:"all_tenants" form:"all_tenants"`                        // Explicit SYS_ADMIN-only cross-tenant scope
}

type AlarmHistoryMonthlyTrendReq struct {
	Year       int    `json:"year" form:"year" validate:"required,min=2000,max=2100"`
	Timezone   string `json:"timezone" form:"timezone" validate:"omitempty,max=64"`
	AllTenants bool   `json:"all_tenants" form:"all_tenants"`
}

type AlarmHistoryMonthlyTrendPoint struct {
	Month int   `json:"month"`
	Count int64 `json:"count"`
}

type AlarmHistoryMonthlyTrendResp struct {
	Year   int                             `json:"year"`
	Months []AlarmHistoryMonthlyTrendPoint `json:"months"`
}

type AlarmHistoryDescUpdateReq struct {
	AlarmHistoryId string `json:"id" validate:"required"`          // Alarm history ID
	Description    string `json:"description" validate:"required"` // Alarm description
}

type AlarmHistoryActionResp struct {
	ID             string  `json:"id"`
	AlarmStatus    string  `json:"alarm_status"`
	Remark         *string `json:"remark"`
	AcknowledgedBy *string `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *string `json:"acknowledged_at,omitempty"`
	ResetBy        *string `json:"reset_by,omitempty"`
	ResetAt        *string `json:"reset_at,omitempty"`
	ActionNote     *string `json:"action_note,omitempty"`
}

type AlarmHistoryBatchActionReq struct {
	IDs    []string `json:"ids" validate:"required"`
	Action string   `json:"action" validate:"required"`
	Note   *string  `json:"note" validate:"omitempty,max=500"`
}

type AlarmHistoryBatchActionItemResp struct {
	ID      string                  `json:"id"`
	OK      bool                    `json:"ok"`
	Error   string                  `json:"error,omitempty"`
	History *AlarmHistoryActionResp `json:"history,omitempty"`
}

type AlarmHistoryBatchActionResp struct {
	Action       string                            `json:"action"`
	SuccessCount int                               `json:"success_count"`
	FailureCount int                               `json:"failure_count"`
	Results      []AlarmHistoryBatchActionItemResp `json:"results"`
}

type GetDeviceAlarmStatusReq struct {
	DeviceId string `json:"device_id" form:"device_id" validate:"required"` // Device ID
}
