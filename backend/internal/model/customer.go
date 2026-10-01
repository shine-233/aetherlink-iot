package model

import (
	"time"
)

const TableNameCustomers = "customers"
const TableNameCustomerDevices = "customer_devices"

// Customer 映射 customers 表
type Customer struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	Name           string    `gorm:"column:name;not null" json:"name"`
	TenantID       string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Country        string    `gorm:"column:country" json:"country"`
	State          string    `gorm:"column:state" json:"state"`
	City           string    `gorm:"column:city" json:"city"`
	Address        string    `gorm:"column:address" json:"address"`
	Address2       string    `gorm:"column:address2" json:"address2"`
	Zip            string    `gorm:"column:zip" json:"zip"`
	Phone          string    `gorm:"column:phone" json:"phone"`
	Email          string    `gorm:"column:email" json:"email"`
	AdditionalInfo string    `gorm:"column:additional_info;type:jsonb;default:'{}'" json:"additional_info"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (*Customer) TableName() string {
	return TableNameCustomers
}

// CustomerDevice 映射 customer_devices 表
type CustomerDevice struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	CustomerID string    `gorm:"column:customer_id;not null" json:"customer_id"`
	DeviceID   string    `gorm:"column:device_id;not null" json:"device_id"`
	TenantID   string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	AssignedAt time.Time `gorm:"column:assigned_at;autoCreateTime" json:"assigned_at"`
}

func (*CustomerDevice) TableName() string {
	return TableNameCustomerDevices
}

// CustomerReq 创建/更新客户入参
type CustomerReq struct {
	ID             string `json:"id"`
	Name           string `json:"name" binding:"required"`
	Country        string `json:"country"`
	State          string `json:"state"`
	City           string `json:"city"`
	Address        string `json:"address"`
	Address2       string `json:"address2"`
	Zip            string `json:"zip"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	AdditionalInfo string `json:"additional_info"`
}

// CustomerAssignDevicesReq 分配设备入参
type CustomerAssignDevicesReq struct {
	DeviceIDs []string `json:"device_ids" binding:"required"`
}
