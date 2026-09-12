// 文件用途：SCADA / Widget 基础层的持久化模型与集中校验（ROADMAP P1.3）。
// 核心逻辑：多项目容器、画布文档（草稿/发布/归档三态）、发布版本快照、控制命令审计。
// 关键注意事项：
//  1. published_version 可空：NULL 表示"从未发布过"，与"发布了第 0 版"是两种不同事实。
//     不要为了省事默认成 0，那会把"没发布过"伪装成"发布了一个版本"。
//  2. 版本号是乐观并发凭证。保存必须带 expectedVersion，不匹配即拒绝——
//     静默覆盖会让人辛苦画好的画布被别人的保存无声冲掉。
//  3. 画布 JSON 超限一律拒绝，绝不静默截断：截断后的 JSON 根本无法解析，
//     等于存了一份永久损坏的画布，且加载时才发现。
//  4. 回滚语义与 fleet 批次回滚保持一致：**不改历史**，而是把目标版本的内容
//     写成一个新的草稿版本。直接把 published_version 指回旧值会让"当前草稿"
//     与"已发布内容"脱节，且丢掉这次回滚动作本身的可追溯性。
package model

import (
	"errors"
	"strings"
	"time"
)

// 表名常量。
const (
	TableNameScadaProject         = "scada_projects"
	TableNameScadaDocument        = "scada_documents"
	TableNameScadaDocumentVersion = "scada_document_versions"
	TableNameScadaControlAudit    = "scada_control_audits"
)

// 文档状态。
const (
	ScadaStatusDraft     = "DRAFT"
	ScadaStatusPublished = "PUBLISHED"
	ScadaStatusArchived  = "ARCHIVED"
)

// 控制命令审计结果。
const (
	// ControlOutcomePending 已通过全部前置闸、准备下发的中间态。
	// 审计必须先于执行落库，才能实现"审计写不进去就拒绝执行"。
	ControlOutcomePending = "pending"
	ControlOutcomeSuccess = "success"
	ControlOutcomeDenied  = "denied"
	ControlOutcomeFailed  = "failed"
)

// 大小上限。
const (
	ScadaMaxNameLength      = 128
	ScadaMaxDescriptionSize = 4096
	// ScadaMaxCanvasBytes 画布 JSON 体积上限。超限拒绝而非截断。
	ScadaMaxCanvasBytes = 2 * 1024 * 1024
	ScadaMaxWidgetIDLen = 64
	ScadaMaxCommandLen  = 64
	ScadaMaxDetailSize  = 2048
)

var (
	ErrScadaNilDocument       = errors.New("scada document is nil")
	ErrScadaMissingTenant     = errors.New("scada document requires tenant id")
	ErrScadaMissingProject    = errors.New("scada document requires project id")
	ErrScadaMissingDocument   = errors.New("scada version snapshot requires document id")
	ErrScadaMissingName       = errors.New("scada entity requires a name")
	ErrScadaNameTooLong       = errors.New("scada entity name is too long")
	ErrScadaDescriptionTooBig = errors.New("scada description is too large")
	ErrScadaCanvasTooLarge    = errors.New("scada canvas payload is too large")
	ErrScadaInvalidVersion    = errors.New("scada document version must be positive")
	ErrScadaInvalidStatus     = errors.New("scada document status is not allowed")
	ErrScadaPublishedAhead    = errors.New("published version cannot be ahead of the draft version")
	ErrScadaCanvasNotObject   = errors.New("scada canvas payload must be a JSON object")
	ErrScadaMissingWidgetID   = errors.New("control audit requires widget id")
	ErrScadaMissingCommand    = errors.New("control audit requires command name")
	ErrScadaMissingActor      = errors.New("control audit requires actor user id")
	ErrScadaMissingOutcome    = errors.New("control audit outcome is not allowed")
)

// ScadaProject 多项目容器。此前看板只有扁平的 boards / vis_dashboard，
// 没有项目层，多项目/多看板无法表达，项目 CRUD 只能返回 unsupported。
type ScadaProject struct {
	ID       string `gorm:"column:id;primaryKey" json:"id"`
	// 复合唯一索引与迁移保持一致：只写在迁移里的话，AutoMigrate 建出的库
	// （测试库、以及任何不跑迁移的环境）就没有这层保护，代码与真实 schema 会分叉。
	TenantID string `gorm:"column:tenant_id;not null;uniqueIndex:idx_scada_projects_tenant_name,priority:1" json:"tenant_id"`
	Name     string `gorm:"column:name;not null;uniqueIndex:idx_scada_projects_tenant_name,priority:2" json:"name"`
	Description *string    `gorm:"column:description" json:"description,omitempty"`
	CreatedBy   *string    `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName ScadaProject's table name
func (*ScadaProject) TableName() string { return TableNameScadaProject }

// ScadaDocument 画布文档。
type ScadaDocument struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID         string     `gorm:"column:tenant_id;not null;uniqueIndex:idx_scada_documents_project_name,priority:1" json:"tenant_id"`
	ProjectID        string     `gorm:"column:project_id;not null;uniqueIndex:idx_scada_documents_project_name,priority:2" json:"project_id"`
	Name             string     `gorm:"column:name;not null;uniqueIndex:idx_scada_documents_project_name,priority:3" json:"name"`
	Status           string     `gorm:"column:status;not null" json:"status"`
	CurrentVersion   int32      `gorm:"column:current_version;not null" json:"current_version"`
	PublishedVersion *int32     `gorm:"column:published_version" json:"published_version"`
	JSONData         *string    `gorm:"column:json_data;type:jsonb;not null" json:"json_data"`
	CreatedBy        *string    `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy        *string    `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName ScadaDocument's table name
func (*ScadaDocument) TableName() string { return TableNameScadaDocument }

// ScadaDocumentVersion 发布版本快照（不可变，供回滚与对比）。
// 只记录"发布过"的版本：草稿保存不产生快照，否则版本列表会被保存噪声淹没，
// 回滚下拉里出现一堆从没发布过的版本。
type ScadaDocumentVersion struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;not null;uniqueIndex:idx_scada_doc_versions_unique,priority:1" json:"tenant_id"`
	DocumentID  string    `gorm:"column:document_id;not null;uniqueIndex:idx_scada_doc_versions_unique,priority:2" json:"document_id"`
	Version     int32     `gorm:"column:version;not null;uniqueIndex:idx_scada_doc_versions_unique,priority:3" json:"version"`
	JSONData    *string   `gorm:"column:json_data;type:jsonb;not null" json:"json_data"`
	PublishedBy *string   `gorm:"column:published_by" json:"published_by,omitempty"`
	PublishedAt time.Time `gorm:"column:published_at;not null" json:"published_at"`
}

// TableName ScadaDocumentVersion's table name
func (*ScadaDocumentVersion) TableName() string { return TableNameScadaDocumentVersion }

// ScadaControlAudit 控制命令审计。被拒绝的命令同样落审计（outcome=denied）——
// 只审计成功命令，等于把"谁在反复尝试越权控制"从记录里抹掉。
type ScadaControlAudit struct {
	ID                 string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID           string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	DocumentID         string    `gorm:"column:document_id;not null" json:"document_id"`
	WidgetID           string    `gorm:"column:widget_id;not null" json:"widget_id"`
	Command            string    `gorm:"column:command;not null" json:"command"`
	Params             *string   `gorm:"column:params;type:jsonb;not null" json:"params"`
	ActorUserID        string    `gorm:"column:actor_user_id;not null" json:"actor_user_id"`
	ConfirmationToken  string    `gorm:"column:confirmation_token;not null" json:"confirmation_token"`
	Outcome            string    `gorm:"column:outcome;not null" json:"outcome"`
	Detail             *string   `gorm:"column:detail" json:"detail,omitempty"`
	CreatedAt          time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// TableName ScadaControlAudit's table name
func (*ScadaControlAudit) TableName() string { return TableNameScadaControlAudit }

// IsAllowedScadaStatus 判断文档状态是否合法。
func IsAllowedScadaStatus(status string) bool {
	switch status {
	case ScadaStatusDraft, ScadaStatusPublished, ScadaStatusArchived:
		return true
	default:
		return false
	}
}

// IsAllowedControlOutcome 判断审计结果是否合法。
func IsAllowedControlOutcome(outcome string) bool {
	switch outcome {
	case ControlOutcomePending, ControlOutcomeSuccess, ControlOutcomeDenied, ControlOutcomeFailed:
		return true
	default:
		return false
	}
}

// IsScadaTerminalStatus 归档为终态：归档文档不再接受保存/发布/控制。
// 草稿与已发布都不是终态，它们之间可以来回转换。
func IsScadaTerminalStatus(status string) bool {
	return status == ScadaStatusArchived
}

func validateScadaName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrScadaMissingName
	}
	if len([]rune(trimmed)) > ScadaMaxNameLength {
		return ErrScadaNameTooLong
	}
	return nil
}

// ValidateScadaProject 校验项目写入。
func ValidateScadaProject(p *ScadaProject) error {
	if p == nil {
		return errors.New("scada project is nil")
	}
	if strings.TrimSpace(p.TenantID) == "" {
		return ErrScadaMissingTenant
	}
	if err := validateScadaName(p.Name); err != nil {
		return err
	}
	if p.Description != nil && len(*p.Description) > ScadaMaxDescriptionSize {
		return ErrScadaDescriptionTooBig
	}
	return nil
}

// ValidateScadaCanvas 校验画布载荷：必须是 JSON 对象且不超过体积上限。
// 空字符串视为"空画布"，归一化为 {}。
func ValidateScadaCanvas(canvas *string) error {
	if canvas == nil || strings.TrimSpace(*canvas) == "" {
		return nil
	}
	if len(*canvas) > ScadaMaxCanvasBytes {
		return ErrScadaCanvasTooLarge
	}
	trimmed := strings.TrimSpace(*canvas)
	// 顶层必须是对象：数组/标量会让画布加载出的布局结构无从谈起。
	if trimmed[0] != '{' {
		return ErrScadaCanvasNotObject
	}
	return nil
}

// ValidateScadaDocument 校验文档写入。
func ValidateScadaDocument(d *ScadaDocument) error {
	if d == nil {
		return ErrScadaNilDocument
	}
	if strings.TrimSpace(d.TenantID) == "" {
		return ErrScadaMissingTenant
	}
	if strings.TrimSpace(d.ProjectID) == "" {
		return ErrScadaMissingProject
	}
	if err := validateScadaName(d.Name); err != nil {
		return err
	}
	if !IsAllowedScadaStatus(d.Status) {
		return ErrScadaInvalidStatus
	}
	if d.CurrentVersion <= 0 {
		return ErrScadaInvalidVersion
	}
	if d.PublishedVersion != nil {
		if *d.PublishedVersion <= 0 {
			return ErrScadaInvalidVersion
		}
		if *d.PublishedVersion > d.CurrentVersion {
			return ErrScadaPublishedAhead
		}
	}
	return ValidateScadaCanvas(d.JSONData)
}

// ValidateScadaVersionSnapshot 校验发布快照。
func ValidateScadaVersionSnapshot(v *ScadaDocumentVersion) error {
	if v == nil {
		return errors.New("scada document version is nil")
	}
	if strings.TrimSpace(v.TenantID) == "" {
		return ErrScadaMissingTenant
	}
	if strings.TrimSpace(v.DocumentID) == "" {
		return ErrScadaMissingDocument
	}
	if v.Version <= 0 {
		return ErrScadaInvalidVersion
	}
	return ValidateScadaCanvas(v.JSONData)
}

// ValidateScadaControlAudit 校验控制审计写入。拒绝记录同样必须通过校验，
// 否则越权尝试会因为没有合法字段而被静默丢弃。
func ValidateScadaControlAudit(a *ScadaControlAudit) error {
	if a == nil {
		return errors.New("scada control audit is nil")
	}
	if strings.TrimSpace(a.TenantID) == "" {
		return ErrScadaMissingTenant
	}
	if strings.TrimSpace(a.DocumentID) == "" {
		return ErrScadaMissingProject
	}
	widgetID := strings.TrimSpace(a.WidgetID)
	if widgetID == "" || len(widgetID) > ScadaMaxWidgetIDLen {
		return ErrScadaMissingWidgetID
	}
	command := strings.TrimSpace(a.Command)
	if command == "" || len(command) > ScadaMaxCommandLen {
		return ErrScadaMissingCommand
	}
	if strings.TrimSpace(a.ActorUserID) == "" {
		return ErrScadaMissingActor
	}
	// 确认令牌刻意允许为空：空令牌本身就是"这次操作没走确认流程"的证据，
	// 而这类操作正是审计要抓的目标。若在此拒绝空令牌，越权尝试会因为
	// 字段不合法而落不了库，审计反而记录了不到它——与审计目的完全相反。
	if !IsAllowedControlOutcome(a.Outcome) {
		return ErrScadaMissingOutcome
	}
	if a.Detail != nil && len(*a.Detail) > ScadaMaxDetailSize {
		return errors.New("control audit detail is too large")
	}
	return nil
}
