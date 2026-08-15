package model

import "time"

type FleetSavedFilterReq struct {
	ID           string                 `json:"id" validate:"omitempty,max=36"`
	Name         string                 `json:"name" validate:"required,max=80"`
	DeviceFilter map[string]interface{} `json:"device_filter" validate:"required"`
	PreviewTotal *int64                 `json:"preview_total,omitempty"`
	// Shared 省略时按 false 处理，即保持私有；只有所有者可以改变共享状态。
	Shared *bool `json:"shared,omitempty"`
}

type FleetSavedFilterRsp struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	DeviceFilter map[string]interface{} `json:"device_filter"`
	PreviewTotal *int64                 `json:"preview_total,omitempty"`
	// Shared 表示该筛选器是否对同租户其他成员可见。
	Shared bool `json:"shared"`
	// Owned 为 false 时表示这是别人共享出来的筛选器，前端应禁用编辑和删除。
	Owned bool `json:"owned"`
	// OwnerUserID 让前端可以展示共享者，便于区分同名筛选器。
	OwnerUserID string     `json:"owner_user_id,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type FleetSavedFilterListRsp struct {
	List []FleetSavedFilterRsp `json:"list"`
}
