// 文件用途：提供 dashboard menu 相关模型补充类型、常量或转换 helper，支撑 backend/internal/model 内的共享数据契约。
// 核心逻辑：围绕模型层的通用结构、枚举和轻量转换函数组织代码，供 API、DAL 与 service 层调用。
// 关键注意事项：模型文件应保持无副作用和轻业务逻辑，复杂校验、权限判断或事务编排应留在 service/DAL 层。
// 重构建议：随着模型职责增多，可按领域拆分文件并为关键转换补充单元测试，避免通用文件继续膨胀。

package model

import "time"

const TableNameTenantDashboardMenu = "tenant_dashboard_menus"

type TenantDashboardMenu struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;not null;index:idx_tenant_dashboard_menu_unique,unique" json:"tenant_id"`
	DashboardID   string    `gorm:"column:dashboard_id;not null;index:idx_tenant_dashboard_menu_unique,unique" json:"dashboard_id"`
	DashboardName string    `gorm:"column:dashboard_name;not null" json:"dashboard_name"`
	MenuName      string    `gorm:"column:menu_name;not null" json:"menu_name"`
	ParentCode    string    `gorm:"column:parent_code;not null;default:home" json:"parent_code"`
	Sort          int16     `gorm:"column:sort;not null;default:1" json:"sort"`
	Enabled       bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*TenantDashboardMenu) TableName() string {
	return TableNameTenantDashboardMenu
}

type UpsertTenantDashboardMenuReq struct {
	MenuName      string  `json:"menu_name" validate:"required,max=99"`
	DashboardName *string `json:"dashboard_name" validate:"omitempty,max=99"`
	Sort          *int16  `json:"sort" validate:"omitempty,max=10000"`
	Enabled       *bool   `json:"enabled"`
}

type BatchTenantDashboardMenuReq struct {
	DashboardIDs []string `json:"dashboard_ids" validate:"required,min=1,max=100,dive,required,max=64"`
}

type TenantDashboardMenuRsp struct {
	DashboardID   string `json:"dashboard_id"`
	DashboardName string `json:"dashboard_name"`
	MenuName      string `json:"menu_name"`
	ParentCode    string `json:"parent_code"`
	Sort          int16  `json:"sort"`
	Enabled       bool   `json:"enabled"`
}

func (m *TenantDashboardMenu) ToRsp() *TenantDashboardMenuRsp {
	return &TenantDashboardMenuRsp{
		DashboardID:   m.DashboardID,
		DashboardName: m.DashboardName,
		MenuName:      m.MenuName,
		ParentCode:    m.ParentCode,
		Sort:          m.Sort,
		Enabled:       m.Enabled,
	}
}
