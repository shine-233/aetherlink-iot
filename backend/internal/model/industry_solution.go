// File purpose: TB-19 解决方案模板引擎模型与请求/响应契约。
// Core logic: 方案 = 有序资源引用清单（复用资源中心类型白名单）+ 安装流水。
// Key notes: 本表只存「引用」不存内容——打包/签名/冲突闸门全部复用 TP-5/P1.6 链路。
package model

import (
	"encoding/json"
	"time"
)

// IndustrySolution 行业方案定义行（表 industry_solutions，见 113.sql）。
type IndustrySolution struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description *string   `gorm:"column:description" json:"description,omitempty"`
	// Resources 有序资源引用，json.RawMessage + type:jsonb（仓库手写模型惯例，
	// 见 calculated_field.go：切片类型没有 driver.Valuer，直接落 jsonb 会失败）。
	Resources json.RawMessage `gorm:"column:resources;type:jsonb;not null" json:"resources"`
	Status    string                        `gorm:"column:status;not null;default:active" json:"status"`
	CreatedAt time.Time                     `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time                     `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

// TableName 指向 113.sql 建的表。
func (IndustrySolution) TableName() string { return "industry_solutions" }

// IndustrySolutionInstall 方案安装流水行（append-only）。
type IndustrySolutionInstall struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID     string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	SolutionID   string    `gorm:"column:solution_id;not null" json:"solution_id"`
	SolutionName string    `gorm:"column:solution_name;not null" json:"solution_name"`
	ItemIndex    int       `gorm:"column:item_index;not null" json:"item_index"`
	ResourceType string    `gorm:"column:resource_type;not null" json:"resource_type"`
	ResourceID   string    `gorm:"column:resource_id;not null" json:"resource_id"`
	TargetID     *string   `gorm:"column:target_id" json:"target_id,omitempty"`
	Status       string    `gorm:"column:status;not null" json:"status"`
	Error        *string   `gorm:"column:error" json:"error,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

// TableName 指向 113.sql 建的表。
func (IndustrySolutionInstall) TableName() string { return "industry_solution_installs" }

// 方案/安装状态（113.sql CHECK 同源）。
const (
	IndustrySolutionStatusActive   = "active"
	IndustrySolutionStatusDisabled = "disabled"
	IndustryInstallStatusApplied   = "applied"
	IndustryInstallStatusFailed    = "failed"
)

// IndustrySolutionResourceRef 方案内的单项资源引用。
type IndustrySolutionResourceRef struct {
	// ResourceType 复用资源中心白名单：device_template | board_template。
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	// TargetName 可选：安装时覆盖实例名称。
	TargetName string `json:"target_name,omitempty"`
}

// CreateIndustrySolutionReq 创建方案。
type CreateIndustrySolutionReq struct {
	Name        string                        `json:"name" validate:"required,max=128"`
	Description string                        `json:"description" validate:"omitempty,max=512"`
	Resources   []IndustrySolutionResourceRef `json:"resources" validate:"required,min=1,max=20,dive"`
}

// InstallIndustrySolutionReq 一键安装方案。
type InstallIndustrySolutionReq struct {
	// ContinueOnError 逐项失败是否继续（缺省 true，结果逐项汇报）。
	ContinueOnError *bool `json:"continue_on_error,omitempty"`
}

// IndustrySolutionInstallItemResult 安装逐项结果。
type IndustrySolutionInstallItemResult struct {
	ItemIndex    int    `json:"item_index"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Status       string `json:"status"`
	TargetID     string `json:"target_id,omitempty"`
	TargetName   string `json:"target_name,omitempty"`
	Error        string `json:"error,omitempty"`
}

// IndustrySolutionInstallRsp 一键安装响应。
type IndustrySolutionInstallRsp struct {
	SolutionID   string                             `json:"solution_id"`
	SolutionName string                             `json:"solution_name"`
	Total        int                                `json:"total"`
	Applied      int                                `json:"applied"`
	Failed       int                                `json:"failed"`
	Items        []IndustrySolutionInstallItemResult `json:"items"`
}
