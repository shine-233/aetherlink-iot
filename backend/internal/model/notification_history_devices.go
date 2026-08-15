package model

const TableNameNotificationHistoryDevice = "notification_history_devices"

// NotificationHistoryDevice records the devices whose data is present in one
// notification history entry. A history entry may cover multiple devices.
type NotificationHistoryDevice struct {
	NotificationHistoryID string `gorm:"column:notification_history_id;primaryKey" json:"notification_history_id"`
	DeviceID              string `gorm:"column:device_id;primaryKey" json:"device_id"`
	TenantID              string `gorm:"column:tenant_id;not null" json:"tenant_id"`
}

func (*NotificationHistoryDevice) TableName() string {
	return TableNameNotificationHistoryDevice
}
