package service

import (
	"aetherlink-iot/backend/internal/dal"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// AddAlarmInfo is the legacy device-less alarm_info writer.
// Deprecated: new execution paths must use AlarmExecute, whose deviceIDs are
// persisted in alarm_history. Calling this method cannot create an owner-safe
// TENANT_USER record and intentionally remains an administrator-only legacy
// path until alarm_info gains a complete stream/device/recovery model.
func (*Alarm) AddAlarmInfo(alarmConfigID, content string) (bool, string) {
	alarmConfig, err := dal.GetAlarmByID(alarmConfigID)
	if err != nil {
		logrus.Error(err)
		return false, ""
	}
	if alarmConfig.Enabled != "Y" {
		return false, ""
	}
	notifyAlarmInfo(alarmConfig, content)
	id, err := createAlarmInfoRecord(alarmConfig, alarmConfigID, content)
	if err != nil {
		logrus.Error(err)
		return false, ""
	}
	return true, id
}

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
	return true, alarmName, ""
}
