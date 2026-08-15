package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

type executeNotificationTemplateVars struct {
	alertData map[string]interface{}
	subject   string
	content   string
	deviceIDs []string
}

type executeWebhookConfig struct {
	PayloadURL string
	Secret     string
}

// Send notification
func (n *NotificationServicesConfig) ExecuteNotification(notificationGroupId, alertJson, expectedTenantID string) {
	logrus.Info("execute notification group:", notificationGroupId)

	notificationGroup, ok := loadExecutableNotificationGroup(notificationGroupId, expectedTenantID)
	if !ok {
		return
	}

	templateVars, ok := parseExecuteNotificationTemplateVars(alertJson)
	if !ok {
		return
	}

	n.dispatchNotificationChannels(notificationGroup, alertJson, templateVars)
}

func loadExecutableNotificationGroup(notificationGroupId, expectedTenantID string) (*model.NotificationGroup, bool) {
	notificationGroup, err := dal.GetNotificationGroupById(notificationGroupId)
	if err != nil {
		logrus.Error("get notification group failed:", err)
		return nil, false
	}
	if expectedTenantID != "" && notificationGroup.TenantID != expectedTenantID {
		logrus.Error("notification group tenant mismatch:", notificationGroupId)
		return nil, false
	}

	logrus.Info("notification group type/status:", notificationGroup.NotificationType, notificationGroup.Status)

	if notificationGroup.Status != "OPEN" {
		logrus.Info("notification group is not open:", notificationGroupId)
		return nil, false
	}

	return notificationGroup, true
}

func parseExecuteNotificationTemplateVars(alertJson string) (*executeNotificationTemplateVars, bool) {
	var alertData map[string]interface{}
	if err := json.Unmarshal([]byte(alertJson), &alertData); err != nil {
		logrus.Error("parse alert json failed:", err)
		return nil, false
	}

	subject, _ := alertData["subject"].(string)
	content, _ := alertData["content"].(string)
	return &executeNotificationTemplateVars{
		alertData: alertData,
		subject:   subject,
		content:   content,
		deviceIDs: notificationDeviceIDsFromAlertData(alertData),
	}, true
}

func notificationDeviceIDsFromAlertData(alertData map[string]interface{}) []string {
	if alertData == nil {
		return nil
	}
	rawDeviceIDs, exists := alertData["device_ids"]
	if !exists {
		return nil
	}

	deviceIDs := make([]string, 0)
	switch values := rawDeviceIDs.(type) {
	case []string:
		deviceIDs = append(deviceIDs, values...)
	case []interface{}:
		for _, value := range values {
			if deviceID, ok := value.(string); ok {
				deviceIDs = append(deviceIDs, deviceID)
			}
		}
	case string:
		deviceIDs = append(deviceIDs, values)
	}
	return deviceIDs
}

func (n *NotificationServicesConfig) dispatchNotificationChannels(notificationGroup *model.NotificationGroup, alertJson string, templateVars *executeNotificationTemplateVars) {
	for _, notifyType := range splitExecuteNotificationTypes(notificationGroup.NotificationType) {
		n.dispatchNotificationChannel(notificationGroup, alertJson, templateVars, notifyType)
	}
}

func splitExecuteNotificationTypes(rawTypes string) []string {
	notificationTypes := strings.Split(rawTypes, ",")
	for i := range notificationTypes {
		notificationTypes[i] = strings.TrimSpace(notificationTypes[i])
	}
	return notificationTypes
}

func (n *NotificationServicesConfig) dispatchNotificationChannel(notificationGroup *model.NotificationGroup, alertJson string, templateVars *executeNotificationTemplateVars, notifyType string) {
	switch notifyType {
	case model.NoticeType_Member:
		n.sendMemberNotificationChannel(notificationGroup, alertJson, templateVars)
	case model.NoticeType_Email:
		n.sendEmailNotification(notificationGroup, alertJson, templateVars)
	case model.NoticeType_SME_CODE:
		n.sendSMSNotification(notifyType)
	case model.NoticeType_Webhook:
		n.sendWebhookNotification(notificationGroup, alertJson, templateVars.deviceIDs)
	case model.NoticeType_APP:
		n.sendAppNotification()
	default:
		logUnsupportedNotificationType(notifyType)
	}
}

func (n *NotificationServicesConfig) sendMemberNotificationChannel(notificationGroup *model.NotificationGroup, alertJson string, templateVars *executeNotificationTemplateVars) {
	if err := n.handleMemberNotification(notificationGroup, alertJson, templateVars.subject, templateVars.content, notificationGroup.TenantID, templateVars.deviceIDs...); err != nil {
		logrus.Error("member notification failed:", err)
	}
}

func parseExecuteEmailConfig(notificationGroup *model.NotificationGroup) (map[string]string, error) {
	nConfig := make(map[string]string)
	if notificationGroup.NotificationConfig == nil || strings.TrimSpace(*notificationGroup.NotificationConfig) == "" {
		return nConfig, nil
	}
	if err := json.Unmarshal([]byte(*notificationGroup.NotificationConfig), &nConfig); err != nil {
		return nil, fmt.Errorf("parse email notification config failed: %w", err)
	}
	return nConfig, nil
}

func (n *NotificationServicesConfig) sendEmailNotification(notificationGroup *model.NotificationGroup, alertJson string, templateVars *executeNotificationTemplateVars) {
	emailBody := buildExecuteEmailBody(templateVars.content)
	nConfig, err := parseExecuteEmailConfig(notificationGroup)
	if err != nil {
		historyErr := n.saveTenantEmailFailure(
			notificationGroup.TenantID,
			"",
			emailBody,
			tenantEmailFailureGroupConfigInvalid,
			templateVars.deviceIDs...,
		)
		logrus.Error("email notification config rejected:", errors.Join(err, historyErr))
		return
	}

	recipients := resolveExecuteEmailRecipients(
		nConfig["EMAIL"],
		warningEmailsForOwnedDevices(notificationGroup.TenantID, templateVars.deviceIDs...),
	)
	if len(recipients) == 0 {
		logrus.Warn("email notification recipients are empty for tenant:", notificationGroup.TenantID)
		if err := n.saveTenantEmailFailure(
			notificationGroup.TenantID,
			"",
			emailBody,
			tenantEmailFailureRecipientsEmpty,
			templateVars.deviceIDs...,
		); err != nil {
			logrus.Error("save empty-recipient email failure history failed:", err)
		}
		return
	}
	for _, emailAddr := range recipients {
		n.sendEmailNotificationRecipient(notificationGroup, alertJson, emailBody, templateVars.subject, emailAddr, templateVars.deviceIDs)
	}
}

func buildExecuteEmailBody(content string) string {
	return content + "\n\n---\nThis email was sent by AetherLink IoT"
}

// splitExecuteEmailRecipients 复用 parseRDIEmailRecipients 的校验/归一/去重口径。
// 这里原先只做 trim，非法或重复地址会直接进入 gomail 的 To 头，导致整封通知在
// SMTP 层失败；两条邮件路径必须共用同一套收件人契约。
func splitExecuteEmailRecipients(rawRecipients string) []string {
	return parseRDIEmailRecipients(rawRecipients)
}

func resolveExecuteEmailRecipients(rawRecipients string, defaultRecipients []string) []string {
	recipients := splitExecuteEmailRecipients(rawRecipients)
	if len(recipients) > 0 {
		return recipients
	}
	return defaultRecipients
}

func (n *NotificationServicesConfig) sendEmailNotificationRecipient(notificationGroup *model.NotificationGroup, alertJson, emailBody, subject, emailAddr string, deviceIDs []string) {
	if err := sendEmailMessageForDevices(emailBody, subject, notificationGroup.TenantID, deviceIDs, emailAddr); err != nil {
		logrus.Error("email notification failed for alert payload:", err, " payload_size=", len(alertJson))
	}
}

func (n *NotificationServicesConfig) sendSMSNotification(notifyType string) {
	logUnsupportedNotificationType(notifyType)
}

func parseExecuteWebhookConfig(notificationGroup *model.NotificationGroup) (executeWebhookConfig, bool) {
	var nConfig executeWebhookConfig
	if err := json.Unmarshal([]byte(*notificationGroup.NotificationConfig), &nConfig); err != nil {
		logrus.Error("parse webhook notification config failed:", err)
		return executeWebhookConfig{}, false
	}
	return nConfig, true
}

func (n *NotificationServicesConfig) sendWebhookNotification(notificationGroup *model.NotificationGroup, alertJson string, deviceIDs []string) {
	nConfig, ok := parseExecuteWebhookConfig(notificationGroup)
	if !ok {
		return
	}

	if err := n.sendWebhookMessage(nConfig.PayloadURL, nConfig.Secret, alertJson, notificationGroup.TenantID, deviceIDs...); err != nil {
		logrus.Error("webhook notification failed:", err)
	}
}

func (n *NotificationServicesConfig) sendAppNotification() {
	logrus.Warn("direct APP notification type is not supported; use MEMBER notification config with notificationType APP")
}

func logUnsupportedNotificationType(notifyType string) {
	logrus.Warn("unsupported notification type:", notifyType)
}
