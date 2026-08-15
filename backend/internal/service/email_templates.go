// 文件用途：实现告警邮件模板的权限、CRUD、预览和安全渲染。
// 核心逻辑：租户模板优先于系统默认模板；模板只能读取固定数据字段，查找或渲染失败时告警发送回退原文。
// 关键注意事项：模板是纯文本包装层，不决定收件人，也不能访问 SMTP 凭据、用户对象或任意函数。
package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type alarmEmailTemplateData struct {
	Subject     string
	Message     string
	TenantID    string
	DeviceIDs   string
	DeviceCount int
	SentAt      string
}

var alarmEmailTemplateTokenPattern = regexp.MustCompile(`{{\s*\.([A-Za-z][A-Za-z0-9]*)\s*}}`)

func emailTemplateScopeForClaims(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage email templates")
	}
	switch claims.Authority {
	case constant.SYS_ADMIN:
		return "", nil
	case constant.TENANT_ADMIN:
		tenantID := strings.TrimSpace(claims.TenantID)
		if tenantID == "" {
			return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required to manage email templates")
		}
		return tenantID, nil
	default:
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage email templates")
	}
}

func buildAlarmEmailTemplateData(subject, message, tenantID string, deviceIDs []string, now time.Time) alarmEmailTemplateData {
	cleanDeviceIDs := normalizeAlarmEmailTemplateDeviceIDs(deviceIDs)
	return alarmEmailTemplateData{
		Subject:     subject,
		Message:     message,
		TenantID:    tenantID,
		DeviceIDs:   strings.Join(cleanDeviceIDs, ", "),
		DeviceCount: len(cleanDeviceIDs),
		SentAt:      now.UTC().Format(time.RFC3339),
	}
}

func normalizeAlarmEmailTemplateDeviceIDs(deviceIDs []string) []string {
	result := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, rawDeviceID := range deviceIDs {
		deviceID := strings.TrimSpace(rawDeviceID)
		if deviceID == "" {
			continue
		}
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		result = append(result, deviceID)
	}
	return result
}

func renderAlarmEmailTemplateText(name, source string, data alarmEmailTemplateData, maxBytes int) (string, error) {
	values := map[string]string{
		"Subject":     data.Subject,
		"Message":     data.Message,
		"TenantID":    data.TenantID,
		"DeviceIDs":   data.DeviceIDs,
		"DeviceCount": fmt.Sprintf("%d", data.DeviceCount),
		"SentAt":      data.SentAt,
	}
	var tokenErr error
	rendered := alarmEmailTemplateTokenPattern.ReplaceAllStringFunc(source, func(token string) string {
		match := alarmEmailTemplateTokenPattern.FindStringSubmatch(token)
		if len(match) != 2 {
			tokenErr = fmt.Errorf("invalid %s template token", name)
			return ""
		}
		value, ok := values[match[1]]
		if !ok {
			tokenErr = fmt.Errorf("unsupported %s template variable %s", name, match[1])
			return ""
		}
		return value
	})
	if tokenErr != nil {
		return "", tokenErr
	}
	remainingActions := alarmEmailTemplateTokenPattern.ReplaceAllString(source, "")
	if strings.Contains(remainingActions, "{{") || strings.Contains(remainingActions, "}}") {
		return "", fmt.Errorf("%s template contains an unsupported action", name)
	}
	if len(rendered) > maxBytes {
		return "", fmt.Errorf("rendered %s template exceeds %d bytes", name, maxBytes)
	}
	return rendered, nil
}

func renderAlarmEmailTemplatePair(subjectTemplate, bodyTemplate string, data alarmEmailTemplateData) (string, string, error) {
	subject, err := renderAlarmEmailTemplateText("alarm-email-subject", subjectTemplate, data, 2000)
	if err != nil {
		return "", "", err
	}
	body, err := renderAlarmEmailTemplateText("alarm-email-body", bodyTemplate, data, 256*1024)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(body) == "" {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "rendered email subject and body must not be empty")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "rendered email subject must be a single line")
	}
	return subject, body, nil
}

func validateAlarmEmailTemplate(req *model.EmailTemplateUpsertReq) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "email template is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "email template name is required")
	}
	if req.IsDefault && !req.Enabled {
		return errcode.NewWithMessage(errcode.CodeParamError, "default email template must be enabled")
	}
	_, _, err := renderAlarmEmailTemplatePair(
		req.SubjectTemplate,
		req.BodyTemplate,
		buildAlarmEmailTemplateData("Alarm subject", "Alarm message", "tenant-preview", []string{"device-1"}, time.Unix(0, 0)),
	)
	return err
}

func (*NotificationServicesConfig) ListEmailTemplates(page, pageSize int, claims *utils.UserClaims) (*model.EmailTemplateListRsp, error) {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	total, list, err := dal.ListEmailTemplates(tenantID, page, pageSize)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return &model.EmailTemplateListRsp{List: list, Total: total}, nil
}

func (*NotificationServicesConfig) CreateEmailTemplate(req *model.EmailTemplateUpsertReq, claims *utils.UserClaims) (*model.EmailTemplate, error) {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return nil, err
	}
	if err := validateAlarmEmailTemplate(req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	template := &model.EmailTemplate{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Name:            strings.TrimSpace(req.Name),
		Purpose:         model.EmailTemplatePurposeAlarm,
		SubjectTemplate: req.SubjectTemplate,
		BodyTemplate:    req.BodyTemplate,
		Enabled:         req.Enabled,
		IsDefault:       req.IsDefault,
		CreatedBy:       claims.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := dal.SaveEmailTemplate(template); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return template, nil
}

func (*NotificationServicesConfig) UpdateEmailTemplate(id string, req *model.EmailTemplateUpsertReq, claims *utils.UserClaims) (*model.EmailTemplate, error) {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return nil, err
	}
	if err := validateAlarmEmailTemplate(req); err != nil {
		return nil, err
	}
	template, err := dal.GetEmailTemplateByIDForScope(strings.TrimSpace(id), tenantID)
	if err != nil {
		return nil, emailTemplatePersistenceError(err)
	}
	template.Name = strings.TrimSpace(req.Name)
	template.SubjectTemplate = req.SubjectTemplate
	template.BodyTemplate = req.BodyTemplate
	template.Enabled = req.Enabled
	template.IsDefault = req.IsDefault
	template.UpdatedAt = time.Now().UTC()
	if err := dal.SaveEmailTemplate(template); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return template, nil
}

func (*NotificationServicesConfig) DeleteEmailTemplate(id string, claims *utils.UserClaims) error {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return err
	}
	if err := dal.DeleteEmailTemplateForScope(strings.TrimSpace(id), tenantID); err != nil {
		return emailTemplatePersistenceError(err)
	}
	return nil
}

func (*NotificationServicesConfig) SetDefaultEmailTemplate(id string, claims *utils.UserClaims) error {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return err
	}
	if err := dal.SetDefaultEmailTemplateForScope(strings.TrimSpace(id), tenantID, time.Now().UTC()); err != nil {
		return emailTemplatePersistenceError(err)
	}
	return nil
}

func (*NotificationServicesConfig) PreviewEmailTemplate(req *model.EmailTemplatePreviewReq, claims *utils.UserClaims) (*model.EmailTemplatePreviewRsp, error) {
	tenantID, err := emailTemplateScopeForClaims(claims)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "email template preview is required")
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = "Alarm subject"
	}
	message := req.Message
	if strings.TrimSpace(message) == "" {
		message = "Alarm message"
	}
	renderedSubject, renderedBody, err := renderAlarmEmailTemplatePair(
		req.SubjectTemplate,
		req.BodyTemplate,
		buildAlarmEmailTemplateData(subject, message, tenantID, req.DeviceIDs, time.Now().UTC()),
	)
	if err != nil {
		return nil, err
	}
	return &model.EmailTemplatePreviewRsp{Subject: renderedSubject, Body: renderedBody}, nil
}

func emailTemplatePersistenceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.NewWithMessage(errcode.CodeParamError, "email template not found")
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
}

func applyAlarmEmailTemplate(message, subject, tenantID string, deviceIDs []string) (string, string) {
	template, err := dal.GetEffectiveAlarmEmailTemplate(strings.TrimSpace(tenantID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return message, subject
	}
	if err != nil {
		logrus.WithError(err).Warn("load alarm email template failed; using original content")
		return message, subject
	}
	renderedSubject, renderedBody, err := renderAlarmEmailTemplatePair(
		template.SubjectTemplate,
		template.BodyTemplate,
		buildAlarmEmailTemplateData(subject, message, tenantID, deviceIDs, time.Now().UTC()),
	)
	if err != nil {
		logrus.WithError(err).Warn("render alarm email template failed; using original content")
		return message, subject
	}
	return renderedBody, renderedSubject
}
