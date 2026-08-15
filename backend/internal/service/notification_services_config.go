// 文件用途：维护通知服务 provider 配置和告警发送参数。
// 核心逻辑：保存邮件、webhook 等 provider 设置，并为告警、消息和租户通知流程提供配置。
// 关键注意事项：配置可能包含密钥或 webhook，错误日志和响应中不能泄露敏感信息。
// 重构建议：抽出 provider 配置 value object，补齐权限、密钥脱敏、事务和 provider 兼容测试。
// notification_services_config.go owns notification-service configuration.
//
// It manages email/webhook/provider settings used by alarms, messages, and
// tenant notification flows. Keep secrets out of logs and validate provider
// payload changes with focused tests.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"aetherlink-iot/backend/third_party/others/http_client"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type NotificationServicesConfig struct{}

var (
	// ErrEmailProviderUnavailable marks the optional email capability as disabled or incomplete.
	// The application can still start; only flows that require email are externally blocked.
	ErrEmailProviderUnavailable = errors.New("email provider unavailable")
	// ErrEmailExternalUnavailable marks a configured provider whose SMTP delivery failed.
	ErrEmailExternalUnavailable = errors.New("email external service unavailable")
	// ErrWebhookProviderUnavailable marks a missing or invalid optional webhook endpoint.
	ErrWebhookProviderUnavailable = errors.New("webhook provider unavailable")
	// ErrWebhookExternalUnavailable marks a configured endpoint whose external delivery failed.
	ErrWebhookExternalUnavailable = errors.New("webhook external service unavailable")
)

type tenantEmailFailureReason string

const (
	tenantEmailFailureRecipientsEmpty       tenantEmailFailureReason = "RECIPIENTS_EMPTY"
	tenantEmailFailureGroupConfigInvalid    tenantEmailFailureReason = "GROUP_CONFIG_INVALID"
	tenantEmailFailureProviderNotConfigured tenantEmailFailureReason = "PROVIDER_NOT_CONFIGURED"
	tenantEmailFailureProviderLookupFailed  tenantEmailFailureReason = "PROVIDER_LOOKUP_FAILED"
	tenantEmailFailureProviderDisabled      tenantEmailFailureReason = "PROVIDER_DISABLED"
	tenantEmailFailureProviderConfigInvalid tenantEmailFailureReason = "PROVIDER_CONFIG_INVALID"
	tenantEmailFailureSMTPDeliveryFailed    tenantEmailFailureReason = "SMTP_DELIVERY_FAILED"

	tenantEmailUnresolvedTarget = "[unresolved]"
)

// requireNotificationServicesAdmin 仅允许系统管理员维护通知服务配置。
func requireNotificationServicesAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage notification service config")
	}
	return nil
}

func resolveEmailProviderConfig(config *model.NotificationServicesConfig) (model.EmailConfig, error) {
	if config == nil {
		return model.EmailConfig{}, fmt.Errorf("%w: config not found", ErrEmailProviderUnavailable)
	}

	switch config.Status {
	case "OPEN":
		// Continue below. Only an explicitly enabled provider may send email.
	case "CLOSE":
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service is disabled", ErrEmailProviderUnavailable)
	default:
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service status %q is not enabled", ErrEmailProviderUnavailable, config.Status)
	}

	if config.Config == nil {
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service config is empty", ErrEmailProviderUnavailable)
	}

	var emailConfig model.EmailConfig
	if err := json.Unmarshal([]byte(*config.Config), &emailConfig); err != nil {
		return model.EmailConfig{}, fmt.Errorf("%w: invalid email notification service config: %v", ErrEmailProviderUnavailable, err)
	}
	if strings.TrimSpace(emailConfig.Host) == "" {
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service host is required", ErrEmailProviderUnavailable)
	}
	if emailConfig.Port < 1 || emailConfig.Port > 65535 {
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service port must be between 1 and 65535", ErrEmailProviderUnavailable)
	}
	if strings.TrimSpace(emailConfig.FromEmail) == "" {
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service from email is required", ErrEmailProviderUnavailable)
	}
	if strings.TrimSpace(emailConfig.FromPassword) == "" {
		return model.EmailConfig{}, fmt.Errorf("%w: email notification service from password is required", ErrEmailProviderUnavailable)
	}
	return emailConfig, nil
}

func newEmailProviderDialer(config model.EmailConfig) *gomail.Dialer {
	dialer := gomail.NewDialer(config.Host, config.Port, config.FromEmail, config.FromPassword)
	// The saved checkbox value is authoritative. This also prevents gomail from
	// silently enabling implicit TLS solely because the configured port is 465.
	dialer.SSL = config.SSL != nil && *config.SSL
	return dialer
}

// 告警邮件的重试口径与 Webhook 保持一致：瞬时 SMTP 故障（连接超时、对端临时拒收）
// 不应该让一封告警邮件直接丢掉，而此前 DialAndSend 只调用一次，失败即放弃。
const tenantAlarmEmailMaxAttempts = 2

// tenantAlarmEmailRetryDelay 在两次投递之间留出间隔，避免对端瞬时限流时立刻重撞。
var tenantAlarmEmailRetryDelay = 2 * time.Second

// sendMailWithDialer 是可替换的投递缝，便于在没有真实 SMTP 的环境下测试重试语义。
var sendMailWithDialer = func(d *gomail.Dialer, m *gomail.Message) error {
	return d.DialAndSend(m)
}

// deliverTenantAlarmEmail 按有界次数重试投递，返回最后一次错误。
// 只重试投递本身；配置解析、收件人校验等确定性失败不在这里重试。
func deliverTenantAlarmEmail(d *gomail.Dialer, m *gomail.Message) error {
	var lastErr error
	for attempt := 1; attempt <= tenantAlarmEmailMaxAttempts; attempt++ {
		if attempt > 1 {
			logrus.Info(fmt.Sprintf("Alarm email send retry, attempt %d", attempt))
			time.Sleep(tenantAlarmEmailRetryDelay)
		}
		if err := sendMailWithDialer(d, m); err != nil {
			lastErr = err
			logrus.Error(fmt.Sprintf("Email send failed, attempt %d", attempt), err)
			continue
		}
		return nil
	}
	return lastErr
}

func (n *NotificationServicesConfig) saveNotificationHistory(notificationType, tenantID, target, content, status string, remark *string, deviceIDs ...string) error {
	history := &model.NotificationHistory{
		ID:               uuid.New(),
		SendTime:         time.Now().UTC(),
		SendContent:      &content,
		SendTarget:       target,
		SendResult:       &status,
		NotificationType: notificationType,
		TenantID:         tenantID,
		Remark:           remark,
	}

	err := GroupApp.NotificationHisory.SaveNotificationHistory(history, deviceIDs...)
	if err != nil {
		logrus.Error("保存通知历史失败", err)
		return err
	}
	return nil
}

func normalizeTenantEmailFailureTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return tenantEmailUnresolvedTarget
	}
	return target
}

func classifyTenantEmailProviderFailure(config *model.NotificationServicesConfig, lookupErr error) tenantEmailFailureReason {
	if lookupErr != nil {
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return tenantEmailFailureProviderNotConfigured
		}
		return tenantEmailFailureProviderLookupFailed
	}
	if config == nil {
		return tenantEmailFailureProviderNotConfigured
	}
	if config.Status != "OPEN" {
		return tenantEmailFailureProviderDisabled
	}
	return tenantEmailFailureProviderConfigInvalid
}

func buildTenantEmailFailureHistory(
	tenantID, target, content string,
	reason tenantEmailFailureReason,
	now time.Time,
) *model.NotificationHistory {
	status := "FAILURE"
	remark := string(reason)
	return &model.NotificationHistory{
		ID:               uuid.New(),
		SendTime:         now.UTC(),
		SendContent:      &content,
		SendTarget:       normalizeTenantEmailFailureTarget(target),
		SendResult:       &status,
		NotificationType: model.NoticeType_Email,
		TenantID:         tenantID,
		Remark:           &remark,
	}
}

// persistTenantEmailFailureHistory 是可替换的持久化缝，便于在没有真实数据库的
// 环境下断言"哪些调用点写了受控的审计历史"，而不是只检查源码里出现过某个标识符。
var persistTenantEmailFailureHistory = func(history *model.NotificationHistory, deviceIDs ...string) error {
	return GroupApp.NotificationHisory.SaveNotificationHistory(history, deviceIDs...)
}

// saveTenantEmailFailure records only a controlled reason code. Raw provider
// configuration and transport errors must never be copied into audit history.
// System/account-flow mail has no tenant scope and intentionally remains outside
// this tenant notification history.
func (n *NotificationServicesConfig) saveTenantEmailFailure(
	tenantID, target, content string,
	reason tenantEmailFailureReason,
	deviceIDs ...string,
) error {
	if strings.TrimSpace(tenantID) == "" {
		return nil
	}
	history := buildTenantEmailFailureHistory(tenantID, target, content, reason, time.Now())
	if err := persistTenantEmailFailureHistory(history, deviceIDs...); err != nil {
		logrus.Error("保存邮件失败通知历史失败", err)
		return err
	}
	return nil
}

func (n *NotificationServicesConfig) joinTenantEmailFailure(
	cause error,
	tenantID, target, content string,
	reason tenantEmailFailureReason,
	deviceIDs ...string,
) error {
	return errors.Join(cause, n.saveTenantEmailFailure(tenantID, target, content, reason, deviceIDs...))
}

const webhookExternalUnavailableReason = "WEBHOOK_EXTERNAL_UNAVAILABLE"

func resolveWebhookEndpoint(rawURL string) (*url.URL, string, error) {
	endpoint, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, "", fmt.Errorf("%w: endpoint must be an absolute HTTP(S) URL", ErrWebhookProviderUnavailable)
	}

	auditEndpoint := *endpoint
	auditEndpoint.User = nil
	auditEndpoint.RawQuery = ""
	auditEndpoint.ForceQuery = false
	auditEndpoint.Fragment = ""
	return endpoint, auditEndpoint.String(), nil
}

func (n *NotificationServicesConfig) sendWebhookMessage(payloadURL, secret, alertJson, tenantID string, deviceIDs ...string) error {
	endpoint, auditTarget, err := resolveWebhookEndpoint(payloadURL)
	if err != nil {
		return err
	}

	cleanJson, err := cleanWebhookAlertJSON(alertJson)
	if err != nil {
		logrus.Error("清理 Webhook 告警 JSON 失败", err)
		return err
	}

	// 先记录 PENDING 状态，后续发送成功或失败时再回写结果。审计目标不得包含 URL 凭据或查询参数。
	historyID := uuid.New()
	pendingStatus := "PENDING"
	history := &model.NotificationHistory{
		ID:               historyID,
		SendTime:         time.Now().UTC(),
		SendContent:      &cleanJson,
		SendTarget:       auditTarget,
		SendResult:       &pendingStatus,
		NotificationType: model.NoticeType_Webhook,
		TenantID:         tenantID,
		Remark:           nil,
	}

	err = GroupApp.NotificationHisory.SaveNotificationHistory(history, deviceIDs...)
	if err != nil {
		logrus.Error("保存 Webhook 通知历史失败", err)
		return err
	}

	// 发送 Webhook；失败会按配置的重试次数重试，并在最终错误中保留最后一次外部失败原因。
	var lastErr error
	maxRetries := 2 // 默认最多尝试 2 次，后续可迁移到配置项。

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logrus.Info(fmt.Sprintf("Webhook send retry, attempt %d", i))
		}

		// 每次发送使用独立超时上下文，避免外部 Webhook 阻塞工作线程。
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		err = http_client.SendSignedRequestWithTimeout(ctx, endpoint.String(), cleanJson, secret)
		cancel()
		if err == nil {
			// 发送成功后回写历史状态。
			successStatus := "SUCCESS"
			_, updateErr := dal.UpdateNotificationHistory(historyID, &successStatus, nil)
			if updateErr != nil {
				logrus.Error("更新 Webhook 通知历史成功状态失败", updateErr)
				return fmt.Errorf("webhook sent but notification history update failed: %w", updateErr)
			}
			logrus.Info("Webhook send succeeded: ", auditTarget)
			return nil
		}
		lastErr = err
		logrus.Warnf("Webhook external delivery failed, attempt %d", i+1)
	}

	failureStatus := "FAILURE"
	remarkText := webhookExternalUnavailableReason

	_, updateErr := dal.UpdateNotificationHistoryWithContent(historyID, &failureStatus, &remarkText, &cleanJson)
	if updateErr != nil {
		logrus.Error("更新 Webhook 通知历史失败状态失败", updateErr)
	}

	externalErr := fmt.Errorf("%w: delivery to %s failed", ErrWebhookExternalUnavailable, auditTarget)
	return errors.Join(externalErr, lastErr, updateErr)
}

func cleanWebhookAlertJSON(alertJson string) (string, error) {
	var alertData map[string]interface{}
	if err := json.Unmarshal([]byte(alertJson), &alertData); err != nil {
		return "", err
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false) // Keep webhook alert content readable for downstream receivers.
	if err := encoder.Encode(alertData); err != nil {
		return "", err
	}
	return strings.TrimSpace(buffer.String()), nil
}

func (*NotificationServicesConfig) SaveNotificationServicesConfig(req *model.SaveNotificationServicesConfigReq, claims *utils.UserClaims) (*model.NotificationServicesConfig, error) {
	if err := requireNotificationServicesAdmin(claims); err != nil {
		return nil, err
	}

	// 读取已有配置；存在则更新，不存在则创建。
	c, err := dal.GetNotificationServicesConfigByType(req.NoticeType)
	if err != nil {
		return nil, err
	}

	config := model.NotificationServicesConfig{}

	var strconf []byte
	switch req.NoticeType {
	case model.NoticeType_Email:
		strconf, err = json.Marshal(req.EMailConfig)
		if err != nil {
			return nil, err
		}
	case model.NoticeType_SME_CODE:
		strconf, err = json.Marshal(req.SMEConfig)
		if err != nil {
			return nil, err
		}
	}

	if c == nil {
		config.ID = uuid.New()
	} else {
		config.ID = c.ID
	}

	configStr := string(strconf)
	config.NoticeType = req.NoticeType
	config.Remark = req.Remark
	config.Status = req.Status
	config.Config = &configStr

	data, err := dal.SaveNotificationServicesConfig(&config)
	if err != nil {
		return nil, err
	}

	return data, err
}

func (*NotificationServicesConfig) GetNotificationServicesConfig(noticeType string, claims *utils.UserClaims) (*model.NotificationServicesConfig, error) {
	if err := requireNotificationServicesAdmin(claims); err != nil {
		return nil, err
	}

	c, err := dal.GetNotificationServicesConfigByType(noticeType)
	return c, err
}

func (n *NotificationServicesConfig) SendTestEmailByAdmin(req *model.SendTestEmailReq, claims *utils.UserClaims) error {
	if err := requireNotificationServicesAdmin(claims); err != nil {
		return err
	}
	return n.SendTestEmail(req)
}

func (n *NotificationServicesConfig) SendTestEmail(req *model.SendTestEmailReq) error {
	if !utils.ValidateEmail(req.Email) {
		return errcode.New(200014)
	}
	if err := n.deliverTestEmail(req); err != nil {
		code := errcode.CodeSystemError
		if errors.Is(err, ErrEmailProviderUnavailable) {
			code = errcode.CodeParamError
		}
		return errcode.WithData(code, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return nil
}

// deliverTestEmail keeps the optional-provider boundary visible to internal callers.
// Public API callers continue to receive the established errcode response above.
func (*NotificationServicesConfig) deliverTestEmail(req *model.SendTestEmailReq) error {
	c, err := dal.GetNotificationServicesConfigByType(model.NoticeType_Email)
	if err != nil {
		return fmt.Errorf("%w: provider lookup failed: %w", ErrEmailProviderUnavailable, err)
	}
	emailConf, err := resolveEmailProviderConfig(c)
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", emailConf.FromEmail)
	m.SetHeader("To", req.Email)
	m.SetHeader("Subject", "IoT test email")
	// 使用 HTML 正文测试邮箱配置，保持与正式告警邮件一致。
	m.SetBody("text/html", req.Body)

	if err := newEmailProviderDialer(emailConf).DialAndSend(m); err != nil {
		return fmt.Errorf("%w: %w", ErrEmailExternalUnavailable, err)
	}
	return nil
}

// sendEmailMessage 发送邮件通知并写入通知历史。
func sendEmailMessage(message string, subject string, tenantId string, to ...string) error {
	return sendEmailMessageForDevices(message, subject, tenantId, nil, to...)
}

func sendEmailMessageForDevices(message string, subject string, tenantId string, deviceIDs []string, to ...string) (err error) {
	if len(to) == 0 || strings.TrimSpace(to[0]) == "" {
		return (&NotificationServicesConfig{}).joinTenantEmailFailure(
			fmt.Errorf("email recipient is required"),
			tenantId,
			"",
			message,
			tenantEmailFailureRecipientsEmpty,
			deviceIDs...,
		)
	}
	if strings.TrimSpace(tenantId) == "" {
		return sendSystemEmailMessage(message, subject, to...)
	}
	message, subject = applyAlarmEmailTemplate(message, subject, tenantId, deviceIDs)

	c, err := dal.GetNotificationServicesConfigByType(model.NoticeType_Email)
	if err != nil {
		return (&NotificationServicesConfig{}).joinTenantEmailFailure(
			err,
			tenantId,
			to[0],
			message,
			classifyTenantEmailProviderFailure(nil, err),
			deviceIDs...,
		)
	}
	emailConf, err := resolveEmailProviderConfig(c)
	if err != nil {
		return (&NotificationServicesConfig{}).joinTenantEmailFailure(
			err,
			tenantId,
			to[0],
			message,
			classifyTenantEmailProviderFailure(c, nil),
			deviceIDs...,
		)
	}

	d := newEmailProviderDialer(emailConf)

	m := gomail.NewMessage()
	m.SetHeader("From", emailConf.FromEmail)
	m.SetHeader("To", to...)
	m.SetBody("text/plain", message)
	m.SetHeader("Subject", subject)

	// 邮件发送结果需要同步写入通知历史，便于运营侧排查。
	nsc := &NotificationServicesConfig{}

	if sendErr := deliverTenantAlarmEmail(d, m); sendErr != nil {
		return nsc.joinTenantEmailFailure(
			fmt.Errorf("%w: %w", ErrEmailExternalUnavailable, sendErr),
			tenantId,
			to[0],
			message,
			tenantEmailFailureSMTPDeliveryFailed,
			deviceIDs...,
		)
	} else {
		logrus.Info("Email send succeeded: ", to[0])
		if historyErr := nsc.saveNotificationHistory(model.NoticeType_Email, tenantId, to[0], message, "SUCCESS", nil, deviceIDs...); historyErr != nil {
			return fmt.Errorf("email sent but notification history save failed: %w", historyErr)
		}
	}
	return nil
}

// sendSystemEmailMessage sends public account-flow emails that do not have a tenant context.
func sendSystemEmailMessage(message string, subject string, to ...string) error {
	c, err := dal.GetNotificationServicesConfigByType(model.NoticeType_Email)
	if err != nil {
		return fmt.Errorf("%w: provider lookup failed: %w", ErrEmailProviderUnavailable, err)
	}
	emailConf, err := resolveEmailProviderConfig(c)
	if err != nil {
		return err
	}

	d := newEmailProviderDialer(emailConf)
	m := gomail.NewMessage()
	m.SetHeader("From", emailConf.FromEmail)
	m.SetHeader("To", to...)
	m.SetBody("text/plain", message)
	m.SetHeader("Subject", subject)
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("%w: %w", ErrEmailExternalUnavailable, err)
	}
	return nil
}
