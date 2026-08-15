package service

import (
	"encoding/json"
	"fmt"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

func (n *NotificationServicesConfig) handleMemberNotification(notificationGroup *model.NotificationGroup, alertJson, subject, content, tenantID string, deviceIDs ...string) error {
	logrus.Info("start member notification, group:", notificationGroup.ID)

	if notificationGroup.NotificationConfig == nil {
		return fmt.Errorf("notification config is nil")
	}

	alertData, err := parseMemberNotificationAlertData(alertJson)
	if err != nil {
		return err
	}
	config, err := parseMemberNotificationConfig(*notificationGroup.NotificationConfig)
	if err != nil {
		return err
	}
	members, err := parseNotificationMemberConfig(config)
	if err != nil {
		return err
	}

	displayNames := notificationMemberDisplayNames(members)
	for _, member := range members {
		n.handleNotificationMember(member, displayNames, alertData, notificationGroup, subject, content, tenantID, deviceIDs)
	}
	return nil
}

func parseMemberNotificationAlertData(alertJson string) (map[string]interface{}, error) {
	var alertData map[string]interface{}
	if err := json.Unmarshal([]byte(alertJson), &alertData); err != nil {
		return nil, fmt.Errorf("parse alert json failed: %v", err)
	}
	return alertData, nil
}

func parseMemberNotificationConfig(rawConfig string) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return nil, fmt.Errorf("parse notification config failed: %v", err)
	}
	return config, nil
}

func (n *NotificationServicesConfig) handleNotificationMember(member map[string]interface{}, displayNames map[string]string, alertData map[string]interface{}, notificationGroup *model.NotificationGroup, subject, content, tenantID string, deviceIDs []string) {
	userID, _ := member["name"].(string)
	logrus.Info("processing notification member:", userID)

	userName := displayNames[userID]
	if userName == "" {
		userName = userID
	}
	notifyTypes, ok := memberNotificationTypes(member, userName)
	if !ok {
		return
	}

	for _, notifyType := range notifyTypes {
		n.handleMemberNotificationType(notifyType, userID, userName, alertData, notificationGroup, subject, content, tenantID, deviceIDs)
	}
}

func notificationMemberDisplayNames(members []map[string]interface{}) map[string]string {
	names := make(map[string]string, len(members))
	userIDs := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))

	for _, member := range members {
		userID, _ := member["name"].(string)
		if userID == "" {
			continue
		}
		names[userID] = userID
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}

	if len(userIDs) == 0 {
		return names
	}

	type userNameRow struct {
		ID   string  `gorm:"column:id"`
		Name *string `gorm:"column:name"`
	}
	var rows []userNameRow
	err := query.User.Where(query.User.ID.In(userIDs...)).Select(query.User.ID, query.User.Name).Scan(&rows)
	if err != nil {
		logrus.Warn("batch query notification member names failed:", err)
		return names
	}

	for _, row := range rows {
		if row.Name != nil && *row.Name != "" {
			names[row.ID] = *row.Name
		}
	}

	return names
}

func memberNotificationTypes(member map[string]interface{}, userName string) ([]string, bool) {
	notificationTypes, ok := member["notificationType"]
	if !ok {
		logrus.Info("member", userName, "has no notificationType config")
		return nil, false
	}
	return notificationTypesFromValue(notificationTypes), true
}

func notificationTypesFromValue(notificationTypes interface{}) []string {
	var notifyTypes []string
	switch nt := notificationTypes.(type) {
	case []interface{}:
		for _, item := range nt {
			if typeStr, ok := item.(string); ok {
				notifyTypes = append(notifyTypes, typeStr)
			}
		}
	case string:
		notifyTypes = append(notifyTypes, nt)
	}
	return notifyTypes
}

func (n *NotificationServicesConfig) handleMemberNotificationType(notifyType, userID, userName string, alertData map[string]interface{}, notificationGroup *model.NotificationGroup, subject, content, tenantID string, deviceIDs []string) {
	switch notifyType {
	case "APP":
		logrus.Info("send member APP notification:", notificationGroup.ID, "member:", userName)
		n.sendMemberAppNotification(userID, userName, alertData, subject, content, tenantID, deviceIDs)
	default:
		logrus.Warn("unsupported member notification type:", notifyType)
	}
}

func (n *NotificationServicesConfig) sendMemberAppNotification(userID, userName string, alertData map[string]interface{}, subject, content, tenantID string, deviceIDs []string) {
	pushManages, err := dal.GetUserMessagePushManages(userID)
	if err != nil {
		logrus.Warn("query user push records failed:", userName, err)
		n.saveMemberAppFailure(tenantID, userName, subject, content, fmt.Sprintf("query push records failed: %v | detail content: %s", err, content), deviceIDs)
		return
	}

	if len(pushManages) == 0 {
		logrus.Warn("user has no push id:", userName, "skip APP push")
		n.saveMemberAppFailure(tenantID, userName, subject, content, fmt.Sprintf("user has no push id | detail content: %s", content), deviceIDs)
		return
	}

	for _, pushManage := range pushManages {
		sendMemberAppPush(pushManage, userName, alertData, subject, content)
	}

	pushTarget := fmt.Sprintf("user:%s", userName)
	n.saveNotificationHistory("APP", tenantID, pushTarget, subject, "SUCCESS", nil, deviceIDs...)
}

func (n *NotificationServicesConfig) saveMemberAppFailure(tenantID, userName, subject, content, remark string, deviceIDs []string) {
	pushTarget := fmt.Sprintf("user:%s", userName)
	n.saveNotificationHistory("APP", tenantID, pushTarget, subject, "FAILURE", &remark, deviceIDs...)
}

func sendMemberAppPush(pushManage *model.MessagePushManage, userName string, alertData map[string]interface{}, subject, content string) {
	if pushManage.PushID == "" {
		logrus.Warn("user push id is empty:", userName, "device type:", pushManage.DeviceType, "skip")
		return
	}

	message := model.MessagePushSend{
		Title:        subject,
		Content:      content,
		PushClientId: pushManage.PushID,
	}
	if alarmID, ok := alertData["id"].(string); ok && alarmID != "" {
		message.AlarmId = &alarmID
	}
	GroupApp.MessagePush.MessagePushSendAndLog(message, *pushManage, 2)
}

func parseNotificationMemberConfig(config map[string]interface{}) ([]map[string]interface{}, error) {
	memberConfig, ok := config["MEMBER"]
	if !ok {
		return nil, fmt.Errorf("MEMBER config not found")
	}

	var members []map[string]interface{}
	switch memberData := memberConfig.(type) {
	case []interface{}:
		for _, item := range memberData {
			if memberMap, ok := item.(map[string]interface{}); ok {
				members = append(members, memberMap)
			}
		}
	case map[string]interface{}:
		members = append(members, memberData)
	default:
		return nil, fmt.Errorf("invalid MEMBER config format")
	}

	return members, nil
}
