// 文件用途：定义 tenants 的持久化模型和租户管理与自助开通的数据传输对象（DTO）。
package model

import (
	"time"
)

const TableNameTenant = "tenants"

// Tenant mapped from table <tenants>
type Tenant struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	Name           string    `gorm:"column:name;not null" json:"name"`
	ParentTenantID string    `gorm:"column:parent_tenant_id;not null" json:"parent_tenant_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName Tenant's table name
func (*Tenant) TableName() string {
	return TableNameTenant
}

// CreateTenantReq 创建租户请求体（管理员接口）
type CreateTenantReq struct {
	Name           string `json:"name" binding:"required"`
	ParentTenantID string `json:"parent_tenant_id"`
}

// UpdateTenantReq 更新租户请求体
type UpdateTenantReq struct {
	Name           string  `json:"name"`
	ParentTenantID *string `json:"parent_tenant_id"`
}

// TenantDetailVO 租户详情及统计视图
type TenantDetailVO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ParentTenantID string    `json:"parent_tenant_id"`
	DeviceCount    int64     `json:"device_count"`
	UserCount      int64     `json:"user_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SelfProvisionTenantReq 客户自助开通租户请求体（开箱入驻，原子创建租户、管理员和初始看板）
type SelfProvisionTenantReq struct {
	TenantName    string `json:"tenant_name" binding:"required"`
	AdminEmail    string `json:"admin_email" binding:"required"`
	AdminPhone    string `json:"admin_phone" binding:"required"`
	AdminPassword string `json:"admin_password" binding:"required"`
	AdminName     string `json:"admin_name"`
}

// SelfProvisionTenantRsp 客户自助开通返回
type SelfProvisionTenantRsp struct {
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	AdminID    string `json:"admin_id"`
	AdminEmail string `json:"admin_email"`
	AdminName  string `json:"admin_name"`
}
