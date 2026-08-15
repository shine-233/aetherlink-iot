// 文件用途：定义 alarm config 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateAlarmConfigReq struct {
	Name                string  `json:"name" validate:"required"`
	Description         *string `json:"description" validate:"omitempty"`
	AlarmLevel          string  `json:"alarm_level" validate:"required"`
	NotificationGroupID string  `json:"notification_group_id" validate:"omitempty"`
	CreatedAt           *string `json:"created_at" validate:"omitempty"`
	UpdatedAt           *string `json:"updated_at" validate:"omitempty"`
	TenantID            string  `json:"tenant_id" validate:"omitempty"`
	Remark              *string `json:"remark" validate:"omitempty"`
	Enabled             string  `json:"enabled" validate:"omitempty"`
	// TriggerDuration 为告警条件需连续满足的秒数，0 表示立即触发。
	TriggerDuration *int32 `json:"trigger_duration" validate:"omitempty"`
}

type UpdateAlarmConfigReq struct {
	ID                  string  `json:"id" validate:"required,max=36"`
	Name                *string `json:"name" validate:"omitempty"`
	Description         *string `json:"description" validate:"omitempty"`
	AlarmLevel          *string `json:"alarm_level" validate:"omitempty"`
	NotificationGroupID *string `json:"notification_group_id" validate:"omitempty"`
	CreatedAt           *string `json:"created_at" validate:"omitempty"`
	UpdatedAt           *string `json:"updated_at" validate:"omitempty"`
	TenantID            *string `json:"tenant_id" validate:"omitempty"`
	Remark              *string `json:"remark" validate:"omitempty"`
	Enabled             *string `json:"enabled" validate:"omitempty"`
	// TriggerDuration 为告警条件需连续满足的秒数，0 表示立即触发。
	TriggerDuration *int32 `json:"trigger_duration" validate:"omitempty"`
}

type GetAlarmConfigListByPageReq struct {
	PageReq
	Name       *string `json:"name" form:"name" validate:"omitempty"`
	AlarmLevel *string `json:"alarm_level" form:"alarm_level" validate:"omitempty"`
	Enabled    string  `json:"enabled" form:"enabled" validate:"omitempty"`
	TenantID   string  `json:"tenant_id" form:"tenant_id" validate:"omitempty"`
}
