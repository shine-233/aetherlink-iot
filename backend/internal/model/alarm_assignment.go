// 文件用途：告警指派流水的持久化模型与请求/响应契约（ROADMAP TB-1 第二片）。
// 核心逻辑：告警生命周期里的责任归属——谁在什么时候把这条告警交给谁，又在哪一次被收回。
// 关键注意事项：
//  1. 指派挂 **alarm_history**（现代告警记录），不挂 alarm_info —— 后者是已废弃的
//     device-less 旧表，给它加字段等于给死代码续命；而且单列只能保存当前值，
//     改派会覆盖上一次，审计就没了。
//  2. 流水 append-only：不 UPDATE、不 DELETE。取消指派 = 新写一行 assignee_user_id
//     为 NULL，不是删掉上一行。**当前处理人 = 最新一行的 assignee_user_id**。
//  3. assignee_user_id 用 *string 而不是 string：NULL 与空串在语义上必须可分
//     （NULL=取消指派，空串=非法值，被 CHECK 拒绝）。写成 string 会把二者都变成 ""。
//  4. 租户与被指派人的归属校验在 service 层做，本文件只描述形状。
// 重构建议：若后续要支持"指派给团队/角色"或 SLA 计时，另开表或另加列，
// 不要复用 remark 塞结构化内容。
package model

import "time"

const TableNameAlarmAssignment = "alarm_assignment"

// AlarmAssignment 一条告警指派流水（只增不改）。
type AlarmAssignment struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	AlarmHistoryID string     `gorm:"column:alarm_history_id;not null" json:"alarm_history_id"`
	// nil 表示取消指派；非 nil 时必须非空（空串由 CHECK 与服务层双重拒绝）。
	AssigneeUserID *string    `gorm:"column:assignee_user_id" json:"assignee_user_id"`
	OperatorUserID string     `gorm:"column:operator_user_id;not null" json:"operator_user_id"`
	Remark         *string    `gorm:"column:remark;type:text" json:"remark"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
}

func (*AlarmAssignment) TableName() string { return TableNameAlarmAssignment }

// CreateAlarmAssignmentReq 指派或取消指派。
// assignee_user_id 传 null（或省略）表示取消指派；传用户 ID 表示指派给该用户。
type CreateAlarmAssignmentReq struct {
	AlarmHistoryID string  `json:"-" uri:"id" validate:"required,max=36"`
	AssigneeUserID *string `json:"assignee_user_id" validate:"omitempty,max=36"`
	Remark         *string `json:"remark" validate:"omitempty,max=2000"`
}

// ListAlarmAssignmentsReq 按告警列出指派流水（时间倒序，首行即当前处理人）。
type ListAlarmAssignmentsReq struct {
	AlarmHistoryID string `json:"-" uri:"id" validate:"required,max=36"`
}
