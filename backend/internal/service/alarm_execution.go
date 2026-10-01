package service

import (
	"context"

	"aetherlink-iot/backend/internal/dal"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func (*Alarm) AlarmRecovery(alarmConfigID, content, sceneAutomationID, groupID string, deviceIDs []string) (string, error) {
	alarmConfig, err := dal.GetAlarmByID(alarmConfigID)
	if err != nil {
		return "", err
	}
	id := uuid.New()
	err = saveAlarmHistoryRecord(alarmConfig, id, alarmConfigID, content, sceneAutomationID, groupID, "N", deviceIDs)
	if err != nil {
		return "", err
	}
	PublishAlarmEvent(context.Background(), alarmConfig.TenantID, map[string]interface{}{
		"type":                "recovery",
		"alarm_id":            id,
		"alarm_config_id":     alarmConfigID,
		"name":                alarmConfig.Name,
		"level":               "N",
		"content":             content,
		"scene_automation_id": sceneAutomationID,
		"group_id":            groupID,
		"device_ids":          deviceIDs,
	})
	return id, nil
}

func (*Alarm) AlarmExecute(alarmConfigID, content, sceneAutomationID, groupID string, deviceIDs []string) (bool, string, string) {
	var alarmName string
	alarmConfig, err := dal.GetAlarmByID(alarmConfigID)
	if err != nil {
		logrus.Error(err)
		return false, alarmName, err.Error()
	}
	if alarmConfig.Enabled != "Y" {
		return false, alarmName, "\u544a\u8b66\u914d\u7f6e\u672a\u542f\u7528"
	}
	alarmName = alarmConfig.Name
	id := uuid.New()
	notifyAlarmExecution(alarmConfig, id, alarmConfigID, content, deviceIDs)
	err = saveAlarmHistoryRecord(alarmConfig, id, alarmConfigID, content, sceneAutomationID, groupID, alarmConfig.AlarmLevel, deviceIDs)
	if err != nil {
		logrus.Error(err)
		return false, alarmName, err.Error()
	}
	PublishAlarmEvent(context.Background(), alarmConfig.TenantID, map[string]interface{}{
		"type":                "trigger",
		"alarm_id":            id,
		"alarm_config_id":     alarmConfigID,
		"name":                alarmName,
		"level":               alarmConfig.AlarmLevel,
		"content":             content,
		"scene_automation_id": sceneAutomationID,
		"group_id":            groupID,
		"device_ids":          deviceIDs,
	})
	return true, alarmName, ""
}
