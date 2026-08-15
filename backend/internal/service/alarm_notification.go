package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func alarmNotificationDescription(alarmConfig *model.AlarmConfig) string {
	if alarmConfig.Description == nil {
		return ""
	}
	return *alarmConfig.Description
}

func alarmNotificationSubject(alarmConfig *model.AlarmConfig) string {
	return fmt.Sprintf("[ALERT] %s [%s]", alarmConfig.Name, alarmConfig.AlarmLevel)
}

func alarmNotificationBody(alarmConfig *model.AlarmConfig, content string) string {
	return fmt.Sprintf(`Alert: %s
Level: %s
Time: %s
Description: %s
Details: %s`,
		alarmConfig.Name,
		alarmConfig.AlarmLevel,
		time.Now().Format("2006-01-02 15:04:05"),
		alarmNotificationDescription(alarmConfig),
		content)
}

func loadAlarmTenantAdminID(tenantID string) string {
	if tenantAdmin, err := dal.GetTenantAdmin(tenantID); err == nil && tenantAdmin != nil {
		return tenantAdmin.ID
	}
	return ""
}

func buildAlarmNotificationBasePayload(alarmConfig *model.AlarmConfig, content string) map[string]interface{} {
	return map[string]interface{}{
		"subject":         alarmNotificationSubject(alarmConfig),
		"content":         alarmNotificationBody(alarmConfig, content),
		"timestamp":       time.Now().Format(time.RFC3339),
		"alarm_level":     alarmConfig.AlarmLevel,
		"tenant_id":       alarmConfig.TenantID,
		"tenant_admin_id": loadAlarmTenantAdminID(alarmConfig.TenantID),
	}
}

func alarmNotificationDeviceMap(deviceInfo *model.Device) map[string]interface{} {
	return map[string]interface{}{
		"id":              deviceInfo.ID,
		"device_number":   deviceInfo.DeviceNumber,
		"name":            deviceInfo.Name,
		"current_version": deviceInfo.CurrentVersion,
		"created_at":      deviceInfo.CreatedAt,
		"label":           deviceInfo.Label,
		"product_id":      deviceInfo.ProductID,
		"is_online":       deviceInfo.IsOnline,
		"access_way":      deviceInfo.AccessWay,
		"description":     deviceInfo.Description,
		"tenant_id":       deviceInfo.TenantID,
	}
}

func loadAlarmNotificationDevices(deviceIDs []string) []map[string]interface{} {
	devices := make([]map[string]interface{}, 0, len(deviceIDs))
	devicesByID, err := dal.GetDevicesByIDs(deviceIDs)
	if err != nil {
		return devices
	}

	for _, rawDeviceID := range deviceIDs {
		deviceID := strings.TrimSpace(rawDeviceID)
		if deviceID == "" {
			continue
		}
		deviceInfo := devicesByID[deviceID]
		if deviceInfo == nil {
			continue
		}
		devices = append(devices, alarmNotificationDeviceMap(deviceInfo))
	}
	return devices
}

func notifyAlarmInfo(alarmConfig *model.AlarmConfig, content string) {
	alertData := buildAlarmNotificationBasePayload(alarmConfig, content)
	alertData["alarm_config_id"] = alarmConfig.ID
	alertData["device_ids"] = []string{}
	alertData["devices"] = []map[string]interface{}{}
	dispatchAlarmNotification(alarmConfig, alertData)
}

func notifyAlarmExecution(alarmConfig *model.AlarmConfig, historyID, alarmConfigID, content string, deviceIDs []string) {
	alertData := buildAlarmNotificationBasePayload(alarmConfig, content)
	alertData["id"] = historyID
	alertData["alarm_config_id"] = alarmConfigID
	alertData["alarm_config_name"] = alarmConfig.Name
	alertData["device_ids"] = deviceIDs
	alertData["devices"] = loadAlarmNotificationDevices(deviceIDs)
	dispatchAlarmNotification(alarmConfig, alertData)
}

func dispatchAlarmNotification(alarmConfig *model.AlarmConfig, alertData map[string]interface{}) {
	if strings.TrimSpace(alarmConfig.NotificationGroupID) != "" {
		executeAlarmNotificationPayload(alarmConfig, alertData)
		return
	}
	sendDefaultAlarmEmailNotification(alarmConfig, alertData)
}

func sendDefaultAlarmEmailNotification(alarmConfig *model.AlarmConfig, alertData map[string]interface{}) {
	deviceIDs := notificationDeviceIDsFromAlertData(alertData)
	subject, _ := alertData["subject"].(string)
	content, _ := alertData["content"].(string)
	body := buildExecuteEmailBody(content)
	recipients := warningEmailsForOwnedDevices(alarmConfig.TenantID, deviceIDs...)
	if len(recipients) == 0 {
		logrus.Warn("alarm default email recipients are empty for tenant:", alarmConfig.TenantID)
		if err := (&NotificationServicesConfig{}).saveTenantEmailFailure(
			alarmConfig.TenantID,
			"",
			body,
			tenantEmailFailureRecipientsEmpty,
			deviceIDs...,
		); err != nil {
			logrus.Error("save default alarm empty-recipient history failed:", err)
		}
		return
	}
	for _, recipient := range recipients {
		if err := sendEmailMessageForDevices(body, subject, alarmConfig.TenantID, deviceIDs, recipient); err != nil {
			logrus.Error("alarm default email notification failed:", err)
		}
	}
}

// createAlarmInfoRecord supports only the deprecated device-less AddAlarmInfo
// path. Do not call it from AlarmExecute: doing so without stream identity,
// device links and a matching recovery transition would create stale active
// rows and make owner authorization impossible.
func createAlarmInfoRecord(alarmConfig *model.AlarmConfig, alarmConfigID, content string) (string, error) {
	id := uuid.New()
	err := dal.CreateAlarmInfo(&model.AlarmInfo{
		ID:               id,
		Name:             alarmConfig.Name,
		AlarmConfigID:    alarmConfigID,
		AlarmLevel:       &alarmConfig.AlarmLevel,
		Content:          &content,
		AlarmTime:        time.Now().UTC(),
		Description:      alarmConfig.Description,
		ProcessingResult: "UND",
		TenantID:         alarmConfig.TenantID,
	})
	return id, err
}

func alarmDeviceListJSON(deviceIDs []string) string {
	deviceIDsJSON, _ := json.Marshal(deviceIDs)
	return string(deviceIDsJSON)
}

func saveAlarmHistoryRecord(
	alarmConfig *model.AlarmConfig,
	historyID, alarmConfigID, content, sceneAutomationID, groupID, alarmStatus string,
	deviceIDs []string,
) error {
	return dal.AlarmHistorySave(&model.AlarmHistory{
		ID:                historyID,
		Name:              alarmConfig.Name,
		AlarmConfigID:     alarmConfigID,
		Content:           &content,
		Description:       alarmConfig.Description,
		TenantID:          alarmConfig.TenantID,
		SceneAutomationID: sceneAutomationID,
		GroupID:           groupID,
		AlarmDeviceList:   alarmDeviceListJSON(deviceIDs),
		AlarmStatus:       alarmStatus,
		CreateAt:          time.Now().UTC(),
	})
}
