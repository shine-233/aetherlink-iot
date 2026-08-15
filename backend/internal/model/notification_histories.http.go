// 文件用途：定义 notification histories 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

// NotificationHistory table definition:
// type NotificationHistory struct {
// 	ID               string    `gorm:"column:id;primaryKey" json:"id"`
// 	SendTime         time.Time `gorm:"column:send_time;not null" json:"send_time"`
// 	SendContent      *string   `gorm:"column:send_content" json:"send_content"`
// 	SendTarget       string    `gorm:"column:send_target;not null" json:"send_target"`
// 	SendResult       *string   `gorm:"column:send_result" json:"send_result"`
// 	NotificationType string    `gorm:"column:notification_type;not null" json:"notification_type"`
// 	TenantID         string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
// 	Remark           *string   `gorm:"column:remark" json:"remark"`
// }

type GetNotificationHistoryListByPageReq struct {
	PageReq
	SendTarget       *string    `json:"send_target" form:"send_target" validate:"omitempty"`                                            // 发送目标
	NotificationType *string    `json:"notification_type" form:"notification_type" validate:"omitempty" example:"MEMBER"`               // 通知类型
	SendTimeStart    *time.Time `json:"send_time_start" form:"send_time_start" validate:"omitempty" example:"2024-04-12T00:00:00.000Z"` // 发送时间起始
	SendTimeStop     *time.Time `json:"send_time_stop" form:"send_time_stop" validate:"omitempty" example:"2024-04-12T00:00:00.000Z"`   // 发送时间终止
	TenantID         string     `json:"tenant_id"  validate:"omitempty"`                                                                // 租户ID
}
