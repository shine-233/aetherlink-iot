package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// PHASE-D-D2 BEGIN IM 通知渠道（D2 通知中心 2.0）
//
// 新增四个纯文本 IM 渠道：钉钉 / 企业微信 / 飞书 / Telegram。
// 契约与既有 WEBHOOK 渠道完全对齐：
//   - 配置来源为 notification_groups.notification_config（map[string]string JSON）；
//   - 发送历史先记 PENDING，再回写 SUCCESS / FAILURE（remark 只落受控原因码，
//     不落原始 URL 凭证与外部错误细节——沿用 saveTenantEmailFailure 的审计红线）；
//   - 审计目标（SendTarget）必须脱敏：钉钉/飞书/企微的 webhook 凭证在 query 中，
//     由 resolveWebhookEndpoint 剥离；Telegram 的 token 在路径中，单独构造脱敏目标；
//   - 每次发送独立超时上下文，最多重试 2 次（与 sendWebhookMessage 一致）。

const (
	noticeTypeDingTalk = "DINGTALK"
	noticeTypeWeCom    = "WECOM"
	noticeTypeFeishu   = "FEISHU"
	noticeTypeTelegram = "TELEGRAM"
)

const (
	dingtalkExternalUnavailableReason = "DINGTALK_EXTERNAL_UNAVAILABLE"
	wecomExternalUnavailableReason    = "WECOM_EXTERNAL_UNAVAILABLE"
	feishuExternalUnavailableReason   = "FEISHU_EXTERNAL_UNAVAILABLE"
	telegramExternalUnavailableReason = "TELEGRAM_EXTERNAL_UNAVAILABLE"
)

const (
	dingtalkGroupConfigInvalidReason = "DINGTALK_GROUP_CONFIG_INVALID"
	wecomGroupConfigInvalidReason    = "WECOM_GROUP_CONFIG_INVALID"
	feishuGroupConfigInvalidReason   = "FEISHU_GROUP_CONFIG_INVALID"
	telegramGroupConfigInvalidReason = "TELEGRAM_GROUP_CONFIG_INVALID"
)

// ErrIMExternalUnavailable 与 ErrWebhookExternalUnavailable 同语义的 IM 侧错误根。
var ErrIMExternalUnavailable = errors.New("im external service unavailable")

// d2Now 可注入时钟：钉钉/飞书签名含时间戳，测试需要确定性。
var d2Now = time.Now

// imPostJSON 传输缝：测试中替换为桩以捕获 URL 与载荷。
var imPostJSON = postJSONWithTimeout

// postJSONWithTimeout 平台 http_client 现有助手只提供带 HMAC 头的签名 POST 与
// 无超时的 PostJson；IM 渠道需要"纯 JSON POST + 独立超时"，故用标准库实现，
// 超时语义与 sendWebhookMessage 的 10s 上下文一致。
func postJSONWithTimeout(ctx context.Context, targetURL string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("%w: unexpected status %d", ErrIMExternalUnavailable, resp.StatusCode)
	}
	return body, nil
}

// truncateIMContent IM 渠道对文本长度都有限制（企微 2046 字节最紧），
// 统一按 rune 截断到 2000 并追加省略号，避免整条消息被远端整体拒绝。
func truncateIMContent(text string) string {
	const maxRunes = 2000
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

// dingTalkSignedURL 按钉钉自定义机器人加签规范构造发送 URL：
// timestamp(毫秒) + "\n" + secret 作为待签名串，HMAC-SHA256(密钥=secret) 后 base64。
func dingTalkSignedURL(webhook, secret string, now time.Time) string {
	ts := now.UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", ts, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	sep := "?"
	if strings.Contains(webhook, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhook, sep, ts, sign)
}

// feishuSignedFields 按飞书自定义机器人加签规范构造 timestamp/sign 字段：
// 待签名串 = timestamp(秒) + "\n" + secret，HMAC-SHA256(密钥=待签名串，数据=空) 后 base64。
func feishuSignedFields(secret string, now time.Time) (int64, string) {
	ts := now.Unix()
	stringToSign := fmt.Sprintf("%d\n%s", ts, secret)
	mac := hmac.New(sha256.New, []byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return ts, sign
}

func mustMarshalIMPayload(payload map[string]interface{}) []byte {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		// payload 全部由本文件构造，编码失败属编程错误而非运行态分支。
		logrus.Error("编码 IM 通知载荷失败", err)
		return []byte("{}")
	}
	return bytes.TrimSpace(buffer.Bytes())
}

// sendIMNotification 是 dispatchNotificationChannel 的 IM 渠道统一入口。
func (n *NotificationServicesConfig) sendIMNotification(notificationGroup *model.NotificationGroup, templateVars *executeNotificationTemplateVars, notifyType string) {
	nConfig, err := parseExecuteEmailConfig(notificationGroup)
	if err != nil {
		logrus.Error("解析 IM 通知配置失败:", err)
		n.saveTenantChannelFailure(notificationGroup.TenantID, "", notifyType, imConfigInvalidReason(notifyType), buildIMTextContent(templateVars), templateVars.deviceIDs...)
		return
	}

	text := truncateIMContent(buildIMTextContent(templateVars))

	var sendURL, auditTarget, invalidReason string
	var payload []byte
	switch notifyType {
	case noticeTypeDingTalk:
		invalidReason = dingtalkGroupConfigInvalidReason
		webhook := strings.TrimSpace(nConfig["WEBHOOK"])
		if webhook == "" {
			break
		}
		target := webhook
		if secret := strings.TrimSpace(nConfig["SECRET"]); secret != "" {
			target = dingTalkSignedURL(webhook, secret, d2Now())
		}
		endpoint, audit, epErr := resolveWebhookEndpoint(target)
		if epErr != nil {
			break
		}
		sendURL, auditTarget = endpoint.String(), audit
		payload = mustMarshalIMPayload(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
	case noticeTypeWeCom:
		invalidReason = wecomGroupConfigInvalidReason
		webhook := strings.TrimSpace(nConfig["WEBHOOK"])
		if webhook == "" {
			break
		}
		endpoint, audit, epErr := resolveWebhookEndpoint(webhook)
		if epErr != nil {
			break
		}
		sendURL, auditTarget = endpoint.String(), audit
		payload = mustMarshalIMPayload(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
	case noticeTypeFeishu:
		invalidReason = feishuGroupConfigInvalidReason
		webhook := strings.TrimSpace(nConfig["WEBHOOK"])
		if webhook == "" {
			break
		}
		endpoint, audit, epErr := resolveWebhookEndpoint(webhook)
		if epErr != nil {
			break
		}
		sendURL, auditTarget = endpoint.String(), audit
		imPayload := map[string]interface{}{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		}
		if secret := strings.TrimSpace(nConfig["SECRET"]); secret != "" {
			ts, sign := feishuSignedFields(secret, d2Now())
			imPayload["timestamp"] = ts
			imPayload["sign"] = sign
		}
		payload = mustMarshalIMPayload(imPayload)
	case noticeTypeTelegram:
		invalidReason = telegramGroupConfigInvalidReason
		botToken := strings.TrimSpace(nConfig["BOT_TOKEN"])
		chatID := strings.TrimSpace(nConfig["CHAT_ID"])
		if botToken == "" || chatID == "" {
			break
		}
		// token 位于路径中，审计目标必须显式脱敏，绝不能落原始 sendURL。
		sendURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		auditTarget = "https://api.telegram.org/botREDACTED/sendMessage"
		payload = mustMarshalIMPayload(map[string]interface{}{
			"chat_id": chatID,
			"text":    text,
		})
	default:
		logUnsupportedNotificationType(notifyType)
		return
	}

	if sendURL == "" || payload == nil {
		logrus.Warn("IM 通知配置不完整:", notifyType)
		n.saveTenantChannelFailure(notificationGroup.TenantID, "", notifyType, invalidReason, text, templateVars.deviceIDs...)
		return
	}

	n.sendIMNotificationMessage(notifyType, sendURL, auditTarget, payload, notificationGroup.TenantID, templateVars.deviceIDs...)
}

// sendIMNotificationMessage 与 sendWebhookMessage 同构：PENDING → 重试 → SUCCESS/FAILURE 回写。
func (n *NotificationServicesConfig) sendIMNotificationMessage(notifyType, sendURL, auditTarget string, payload []byte, tenantID string, deviceIDs ...string) error {
	historyID := uuid.New()
	pendingStatus := "PENDING"
	content := string(payload)
	history := &model.NotificationHistory{
		ID:               historyID,
		SendTime:         d2Now().UTC(),
		SendContent:      &content,
		SendTarget:       auditTarget,
		SendResult:       &pendingStatus,
		NotificationType: notifyType,
		TenantID:         tenantID,
		Remark:           nil,
	}
	if err := persistTenantIMHistory(history, deviceIDs...); err != nil {
		logrus.Error("保存 IM 通知历史失败", err)
		return err
	}

	var lastErr error
	maxRetries := 2
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logrus.Info(fmt.Sprintf("IM(%s) send retry, attempt %d", notifyType, i))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := imPostJSON(ctx, sendURL, payload)
		cancel()
		if err == nil {
			successStatus := "SUCCESS"
			if _, updateErr := updateNotificationHistoryStatusD2(historyID, &successStatus, nil, nil); updateErr != nil {
				logrus.Error("更新 IM 通知历史成功状态失败", updateErr)
				return fmt.Errorf("im sent but notification history update failed: %w", updateErr)
			}
			return nil
		}
		lastErr = err
		logrus.Warnf("IM(%s) external delivery failed, attempt %d", notifyType, i+1)
	}

	failureStatus := "FAILURE"
	remarkText := imExternalUnavailableReason(notifyType)
	_, updateErr := updateNotificationHistoryStatusD2(historyID, &failureStatus, &remarkText, &content)
	if updateErr != nil {
		logrus.Error("更新 IM 通知历史失败状态失败", updateErr)
	}
	externalErr := fmt.Errorf("%w: delivery to %s failed", ErrIMExternalUnavailable, auditTarget)
	return errors.Join(externalErr, lastErr, updateErr)
}

func imExternalUnavailableReason(notifyType string) string {
	switch notifyType {
	case noticeTypeDingTalk:
		return dingtalkExternalUnavailableReason
	case noticeTypeWeCom:
		return wecomExternalUnavailableReason
	case noticeTypeFeishu:
		return feishuExternalUnavailableReason
	case noticeTypeTelegram:
		return telegramExternalUnavailableReason
	default:
		return "IM_EXTERNAL_UNAVAILABLE"
	}
}

func imConfigInvalidReason(notifyType string) string {
	switch notifyType {
	case noticeTypeDingTalk:
		return dingtalkGroupConfigInvalidReason
	case noticeTypeWeCom:
		return wecomGroupConfigInvalidReason
	case noticeTypeFeishu:
		return feishuGroupConfigInvalidReason
	case noticeTypeTelegram:
		return telegramGroupConfigInvalidReason
	default:
		return "IM_GROUP_CONFIG_INVALID"
	}
}

// persistTenantIMHistory 持久化缝：与 persistTenantEmailFailureHistory 同风格，
// 测试替换以断言"哪些调用点写了受控审计历史"。
var persistTenantIMHistory = func(history *model.NotificationHistory, deviceIDs ...string) error {
	return GroupApp.NotificationHisory.SaveNotificationHistory(history, deviceIDs...)
}

var updateNotificationHistoryStatusD2 = func(historyID string, status, remark, content *string) (int64, error) {
	if remark == nil {
		return dal.UpdateNotificationHistory(historyID, status, nil)
	}
	return dal.UpdateNotificationHistoryWithContent(historyID, status, remark, content)
}

// saveTenantChannelFailure 是 saveTenantEmailFailure 的多渠道泛化：
// 只落受控原因码，不落原始配置与外部错误细节；IM/SMS 渠道共用。
func (n *NotificationServicesConfig) saveTenantChannelFailure(tenantID, target, notifyType, reason, content string, deviceIDs ...string) error {
	if strings.TrimSpace(tenantID) == "" {
		return nil
	}
	status := "FAILURE"
	remark := reason
	now := d2Now().UTC()
	history := &model.NotificationHistory{
		ID:               uuid.New(),
		SendTime:         now,
		SendContent:      &content,
		SendTarget:       target,
		SendResult:       &status,
		NotificationType: notifyType,
		TenantID:         tenantID,
		Remark:           &remark,
	}
	if err := persistTenantIMHistory(history, deviceIDs...); err != nil {
		logrus.Error("保存渠道失败通知历史失败", err)
		return err
	}
	return nil
}

// PHASE-D-D2 END
