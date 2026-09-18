package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// GetActiveAlarmHistoryForDeviceAndName 查询指定设备与告警名称处于活动态（H/M/L）的最新告警历史。
func GetActiveAlarmHistoryForDeviceAndName(tenantID, deviceID, alarmName string) (*model.AlarmHistory, error) {
	var row model.AlarmHistory
	err := global.DB.Where("tenant_id = ? AND name = ? AND alarm_status IN ('H', 'M', 'L') AND alarm_device_list::text LIKE ?",
		tenantID, alarmName, "%"+deviceID+"%").
		Order("create_at DESC").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// EscalateAlarmHistorySeverity 严重度平滑升级：就地升级活动告警的 alarm_status 与描述内容。
func EscalateAlarmHistorySeverity(id, tenantID, newSeverity, newContent string) error {
	return global.DB.Model(&model.AlarmHistory{}).
		Where("id = ? AND tenant_id = ? AND alarm_status IN ('H', 'M', 'L')", id, tenantID).
		Updates(map[string]interface{}{
			"alarm_status": newSeverity,
			"content":      newContent,
		}).Error
}

// AutoClearAlarmHistoryRecord 自动清除活动告警（转为 CLEARED 态 N）。
func AutoClearAlarmHistoryRecord(id, tenantID, note string) error {
	_, err := ClearAlarmHistory(id, tenantID, "system", note)
	return err
}
