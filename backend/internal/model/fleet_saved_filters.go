package model

import "time"

const TableNameFleetSavedFilter = "fleet_saved_filters"

type FleetSavedFilter struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	TenantID     string `gorm:"column:tenant_id;not null" json:"tenant_id"`
	UserID       string `gorm:"column:user_id;not null" json:"user_id"`
	Name         string `gorm:"column:name;not null" json:"name"`
	DeviceFilter string `gorm:"column:device_filter;type:jsonb;not null" json:"device_filter"`
	PreviewTotal *int64 `gorm:"column:preview_total" json:"preview_total"`
	// Shared 为 true 时同租户其他成员可以读取该筛选器，但仍然只有 UserID 本人可以修改或删除。
	Shared    bool      `gorm:"column:shared;not null;default:false" json:"shared"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*FleetSavedFilter) TableName() string {
	return TableNameFleetSavedFilter
}
