// 文件用途：定义设备地理空间定位（TB-13 Geospatial Map Tracking）相关入参、出参和轨迹数据模型。
// 核心逻辑：对标 ThingsBoard 4.0 地图部件数据规约，支持批量最新位置标绘与单设备历史轨迹回放。
package model

// DeviceLocationLatestReq 请求最新设备位置参数
type DeviceLocationLatestReq struct {
	GroupID   *string `form:"group_id" json:"group_id" validate:"omitempty,max=36"`
	SearchKey *string `form:"search_key" json:"search_key" validate:"omitempty,max=100"`
	IsOnline  *int16  `form:"is_online" json:"is_online" validate:"omitempty,oneof=0 1"`
	Limit     int     `form:"limit" json:"limit" validate:"omitempty,min=1,max=1000"`
}

// DeviceLocationLatestItem 单个设备最新位置及状态
type DeviceLocationLatestItem struct {
	DeviceID     string                 `json:"device_id"`
	DeviceName   string                 `json:"device_name"`
	DeviceNumber string                 `json:"device_number"`
	TenantID     string                 `json:"tenant_id"`
	IsOnline     int16                  `json:"is_online"`
	Latitude     *float64               `json:"latitude"`
	Longitude    *float64               `json:"longitude"`
	Speed        *float64               `json:"speed,omitempty"`
	Altitude     *float64               `json:"altitude,omitempty"`
	Address      *string                `json:"address,omitempty"`
	LastTime     *int64                 `json:"last_time,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// DeviceLocationLatestResp 租户设备最新位置列表响应
type DeviceLocationLatestResp struct {
	List  []DeviceLocationLatestItem `json:"list"`
	Total int64                      `json:"total"`
}

// DeviceLocationHistoryReq 历史轨迹查询入参
type DeviceLocationHistoryReq struct {
	From  int64 `form:"from" json:"from"`
	To    int64 `form:"to" json:"to"`
	Limit int   `form:"limit" json:"limit" validate:"omitempty,min=1,max=2000"`
}

// DeviceLocationHistoryPoint 单个历史轨迹点
type DeviceLocationHistoryPoint struct {
	Timestamp int64    `json:"ts"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Speed     *float64 `json:"speed,omitempty"`
	Altitude  *float64 `json:"altitude,omitempty"`
}

// DeviceLocationHistoryResp 设备历史轨迹回放响应
type DeviceLocationHistoryResp struct {
	DeviceID   string                       `json:"device_id"`
	DeviceName string                       `json:"device_name"`
	Points     []DeviceLocationHistoryPoint `json:"points"`
	Total      int                          `json:"total"`
}
