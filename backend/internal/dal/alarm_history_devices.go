// 文件用途：集中处理告警历史结果中的设备 ID 展开与当前活动设备过滤。
//
// alarm.go 保留告警查询、事务更新和备注合并主流程；本文件只承载与设备摘要
// 装配直接相关的 DAL helper。拆分不改变租户/owner 过滤、SQL 条件或返回字段契约。
package dal

import (
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

func expandAlarmHistoryListDeviceFields(list []map[string]interface{}, ownerUserID *string) {
	recordDeviceIDs := make([][]string, len(list))
	allDeviceIDs := make([]string, 0)
	seenDeviceIDs := make(map[string]struct{})

	for i, record := range list {
		deviceIDs := alarmHistoryDeviceIDsFromValue(record["alarm_device_list"])
		recordDeviceIDs[i] = deviceIDs
		for _, deviceID := range deviceIDs {
			deviceID = strings.TrimSpace(deviceID)
			if deviceID == "" {
				continue
			}
			if _, ok := seenDeviceIDs[deviceID]; ok {
				continue
			}
			seenDeviceIDs[deviceID] = struct{}{}
			allDeviceIDs = append(allDeviceIDs, deviceID)
		}
	}

	devicesByID := loadAlarmHistoryDevicesByID(allDeviceIDs, ownerUserID)
	for i, deviceIDs := range recordDeviceIDs {
		list[i]["alarm_device_list"] = alarmHistoryDeviceRows(deviceIDs, devicesByID)
	}
}

type currentActiveAlarmDeviceLink struct {
	AlarmHistoryID string `gorm:"column:alarm_history_id"`
	DeviceID       string `gorm:"column:device_id"`
}

func expandCurrentActiveAlarmHistoryDeviceFields(list []map[string]interface{}, ownerUserID *string) error {
	historyIDs := make([]string, 0, len(list))
	seenHistoryIDs := make(map[string]struct{}, len(list))
	for _, record := range list {
		historyID := alarmHistoryDeviceRowID(record)
		if historyID == "" {
			continue
		}
		if _, ok := seenHistoryIDs[historyID]; ok {
			continue
		}
		seenHistoryIDs[historyID] = struct{}{}
		historyIDs = append(historyIDs, historyID)
	}
	if len(historyIDs) == 0 {
		for i := range list {
			list[i]["alarm_device_list"] = []map[string]interface{}{}
		}
		return nil
	}

	links := make([]currentActiveAlarmDeviceLink, 0)
	builder := global.DB.Table("current_device_alarm_streams AS current_alarm").
		Select("current_alarm.id AS alarm_history_id, current_alarm.device_id").
		Joins("INNER JOIN devices current_device ON current_device.id = current_alarm.device_id AND current_device.tenant_id = current_alarm.tenant_id AND current_device.activate_flag = ?", "active").
		Where("current_alarm.id IN ?", historyIDs).
		Where("current_alarm.alarm_status IN ?", []string{"H", "M", "L"})
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		builder = builder.Where("current_device.owner_user_id = ?", strings.TrimSpace(*ownerUserID))
	}
	if err := builder.Scan(&links).Error; err != nil {
		return err
	}

	activeDeviceIDsByHistory := make(map[string]map[string]struct{}, len(historyIDs))
	for _, link := range links {
		historyID := strings.TrimSpace(link.AlarmHistoryID)
		deviceID := strings.TrimSpace(link.DeviceID)
		if historyID == "" || deviceID == "" {
			continue
		}
		if activeDeviceIDsByHistory[historyID] == nil {
			activeDeviceIDsByHistory[historyID] = make(map[string]struct{})
		}
		activeDeviceIDsByHistory[historyID][deviceID] = struct{}{}
	}

	recordDeviceIDs := make([][]string, len(list))
	allDeviceIDs := make([]string, 0)
	seenDeviceIDs := make(map[string]struct{})
	for i, record := range list {
		historyID := alarmHistoryDeviceRowID(record)
		allowed := activeDeviceIDsByHistory[historyID]
		for _, deviceID := range alarmHistoryDeviceIDsFromValue(record["alarm_device_list"]) {
			deviceID = strings.TrimSpace(deviceID)
			if _, ok := allowed[deviceID]; !ok {
				continue
			}
			recordDeviceIDs[i] = append(recordDeviceIDs[i], deviceID)
			if _, ok := seenDeviceIDs[deviceID]; ok {
				continue
			}
			seenDeviceIDs[deviceID] = struct{}{}
			allDeviceIDs = append(allDeviceIDs, deviceID)
		}
	}

	devicesByID := loadAlarmHistoryDevicesByID(allDeviceIDs, ownerUserID)
	for i, deviceIDs := range recordDeviceIDs {
		list[i]["alarm_device_list"] = alarmHistoryDeviceRows(deviceIDs, devicesByID)
	}
	return nil
}

func loadAlarmHistoryDevicesByID(deviceIDs []string, ownerUserID *string) map[string]map[string]interface{} {
	devicesByID := make(map[string]map[string]interface{}, len(deviceIDs))
	if len(deviceIDs) == 0 {
		return devicesByID
	}

	deviceList := make([]map[string]interface{}, 0, len(deviceIDs))
	deviceQuery := query.Device.Where(query.Device.ID.In(deviceIDs...))
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		deviceQuery = deviceQuery.Where(query.Device.OwnerUserID.Eq(strings.TrimSpace(*ownerUserID)))
	}
	if err := deviceQuery.Select(query.Device.ID, query.Device.Name).
		Scan(&deviceList); err != nil {
		logrus.WithError(err).Warn("load alarm history devices failed")
		return devicesByID
	}

	for _, device := range deviceList {
		deviceID := alarmHistoryDeviceRowID(device)
		if deviceID == "" {
			continue
		}
		devicesByID[deviceID] = device
	}
	return devicesByID
}

func alarmHistoryDeviceRows(deviceIDs []string, devicesByID map[string]map[string]interface{}) []map[string]interface{} {
	deviceRows := make([]map[string]interface{}, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		device, ok := devicesByID[deviceID]
		if !ok {
			continue
		}
		deviceRows = append(deviceRows, device)
	}
	return deviceRows
}

func alarmHistoryDeviceRowID(device map[string]interface{}) string {
	for _, key := range []string{"id", "ID"} {
		value, ok := device[key]
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(value))
		if id != "" {
			return id
		}
	}
	return ""
}
