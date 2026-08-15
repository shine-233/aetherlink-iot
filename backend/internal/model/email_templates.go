// 文件用途：定义告警邮件模板的持久化模型与 HTTP 请求/响应契约。
// 核心逻辑：模板按系统或租户作用域隔离，并只暴露受控的主题、正文、启用和默认状态。
// 关键注意事项：模板变量由 service 层白名单渲染；本模型不保存 SMTP 凭据或收件人地址。
package model

import "time"

const (
	TableNameEmailTemplate    = "email_templates"
	EmailTemplatePurposeAlarm = "ALARM"
)

type EmailTemplate struct {
	ID              string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID        string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name            string    `gorm:"column:name;not null" json:"name"`
	Purpose         string    `gorm:"column:purpose;not null" json:"purpose"`
	SubjectTemplate string    `gorm:"column:subject_template;not null" json:"subject_template"`
	BodyTemplate    string    `gorm:"column:body_template;not null" json:"body_template"`
	Enabled         bool      `gorm:"column:enabled;not null" json:"enabled"`
	IsDefault       bool      `gorm:"column:is_default;not null" json:"is_default"`
	CreatedBy       string    `gorm:"column:created_by;not null" json:"created_by"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*EmailTemplate) TableName() string {
	return TableNameEmailTemplate
}

type EmailTemplateUpsertReq struct {
	Name            string `json:"name" validate:"required,max=120"`
	SubjectTemplate string `json:"subject_template" validate:"required,max=500"`
	BodyTemplate    string `json:"body_template" validate:"required,max=20000"`
	Enabled         bool   `json:"enabled"`
	IsDefault       bool   `json:"is_default"`
}

type EmailTemplatePreviewReq struct {
	SubjectTemplate string   `json:"subject_template" validate:"required,max=500"`
	BodyTemplate    string   `json:"body_template" validate:"required,max=20000"`
	Subject         string   `json:"subject" validate:"omitempty,max=500"`
	Message         string   `json:"message" validate:"omitempty,max=20000"`
	DeviceIDs       []string `json:"device_ids" validate:"omitempty,max=100"`
}

type EmailTemplatePreviewRsp struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type EmailTemplateListRsp struct {
	List  []*EmailTemplate `json:"list"`
	Total int64            `json:"total"`
}
