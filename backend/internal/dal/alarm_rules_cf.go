package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// GetActiveAlarmHistoryForDeviceAndName 查询指定设备与告警名称处于活动态（H/M/L）的最新告警历史。
//
// 设备命中条件由 alarm_device_list::text LIKE '%id%' 改为 142.sql 的关联表 EXISTS：
// LIKE 既强制逐行顺序扫描，又存在子串误命中（某设备 id 是另一 id 的前缀时会错配到
// 别人的告警）；关联表上的等值匹配命中主键索引且语义精确。
func GetActiveAlarmHistoryForDeviceAndName(tenantID, deviceID, alarmName string) (*model.AlarmHistory, error) {
	var row model.AlarmHistory
	err := global.DB.Where("tenant_id = ? AND name = ? AND alarm_status IN ('H', 'M', 'L') AND "+alarmHistoryDeviceExistsByIDUnqualified,
		tenantID, alarmName, deviceID).
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
