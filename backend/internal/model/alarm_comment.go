// 文件用途：告警评论的持久化模型与请求/响应契约（ROADMAP TB-1 第一片）。
// 核心逻辑：告警生命周期里的协作面——谁在什么时间对某次告警留了什么话。
// 关键注意事项：
//  1. 评论挂 **alarm_history**（现代告警记录，带 stream 身份与设备关联），
//     不挂 alarm_info —— 后者是已废弃的 device-less 旧表，给它加功能等于给死代码续命。
//  2. 评论是只增不改的协作记录（允许作者删除），不要把它当成告警的可变字段。
//  3. 租户与告警归属校验在 service 层做，本文件只描述形状。
// 重构建议：若后续要支持 @提及、附件或富文本，另开表而不是往 content 里塞结构化内容。
package model

import "time"

const TableNameAlarmComment = "alarm_comment"

// AlarmComment 一条告警评论。
type AlarmComment struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID       string    `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	AlarmHistoryID string    `gorm:"column:alarm_history_id;not null" json:"alarm_history_id"`
	Content        string    `gorm:"column:content;type:text;not null" json:"content"`
	AuthorUserID   string    `gorm:"column:author_user_id;not null" json:"author_user_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*AlarmComment) TableName() string { return TableNameAlarmComment }

// CreateAlarmCommentReq 新增评论。content 上限与产品可读性对齐，不做无上限文本。
type CreateAlarmCommentReq struct {
	AlarmHistoryID string `json:"-" uri:"id" validate:"required,max=36"`
	Content        string `json:"content" validate:"required,max=2000"`
}

// ListAlarmCommentsReq 按告警列出评论。
type ListAlarmCommentsReq struct {
	AlarmHistoryID string `json:"-" uri:"id" validate:"required,max=36"`
}

// DeleteAlarmCommentReq 删除评论（仅作者本人或租户管理员，见 service 层）。
type DeleteAlarmCommentReq struct {
	AlarmHistoryID string `json:"-" uri:"id" validate:"required,max=36"`
	CommentID      string `json:"-" uri:"comment_id" validate:"required,max=36"`
}
