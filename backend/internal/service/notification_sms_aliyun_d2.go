package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// PHASE-D-D2 BEGIN 阿里云短信渠道（NoticeType_SME_CODE 桩位补实）
//
// 现状：dispatchNotificationChannel 对 SME_CODE 仅打"不支持"日志。
// 本文件按阿里云短信 RPC 签名协议（HMAC-SHA1）实现 SendSms 最小集：
//   - 配置来源仍为 notification_groups.notification_config（map[string]string）：
//     ACCESS_KEY_ID / ACCESS_KEY_SECRET / SIGN_NAME / TEMPLATE_CODE / PHONE（逗号分隔）
//   - 未配置时 fail-closed：写 FAILURE 历史（SMS_GROUP_CONFIG_INVALID），不发请求；
//   - 短信内容经模板变量引擎渲染后放入 TemplateParam.content，
//     模板本体在阿里云控制台配置（占位符 ${content}）。
// 审计红线与邮件/IM 渠道一致：历史 remark 只落受控原因码，密钥与外部错误细节只进日志。

const (
	smsGroupConfigInvalidReason  = "SMS_GROUP_CONFIG_INVALID"
	smsExternalUnavailableReason = "SMS_EXTERNAL_UNAVAILABLE"
	aliyunSMSMaxContentRunes     = 500
	aliyunSMSSignatureMethod     = "HMAC-SHA1"
	aliyunSMSSignatureVersion    = "1.0"
	aliyunSMSAPIVersion          = "2017-05-25"
	aliyunSMSRegionID            = "cn-hangzhou"
	aliyunSMSAction              = "SendSms"
)

// aliyunSMSEndpoint 独立成变量，测试替换为 httptest 地址。
var aliyunSMSEndpoint = "https://dysmsapi.aliyuncs.com/"

// aliyunSMSGetJSON 传输缝：测试替换为桩。
var aliyunSMSGetJSON = func(ctx context.Context, targetURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buffer := new(strings.Builder)
	tmp := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buffer.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return []byte(buffer.String()), fmt.Errorf("%w: unexpected status %d", ErrIMExternalUnavailable, resp.StatusCode)
	}
	return []byte(buffer.String()), nil
}

// percentEncode 按阿里云 RPC 签名规范编码（RFC3986 子集）。
func percentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// buildAliyunSMSRequestURL 构造带签名的 SendSms GET URL。
// 入参 onlyPublicParams=false 时携带业务参数（手机号/签名/模板/内容）。
func buildAliyunSMSRequestURL(accessKeyID, accessKeySecret, signName, templateCode, phones, templateParam string, now time.Time) string {
	params := map[string]string{
		"AccessKeyId":      accessKeyID,
		"Action":           aliyunSMSAction,
		"Format":           "JSON",
		"PhoneNumbers":     phones,
		"RegionId":         aliyunSMSRegionID,
		"SignName":         signName,
		"SignatureMethod":  aliyunSMSSignatureMethod,
		"SignatureNonce":   uuid.New(),
		"SignatureVersion": aliyunSMSSignatureVersion,
		"TemplateCode":     templateCode,
		"TemplateParam":    templateParam,
		"Timestamp":        now.UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          aliyunSMSAPIVersion,
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, percentEncode(key)+"="+percentEncode(params[key]))
	}
	canonicalQuery := strings.Join(pairs, "&")

	// 待签名串 = GET & / & 编码后的规范查询串；密钥 = AccessKeySecret + "&"。
	stringToSign := "GET&%2F&" + percentEncode(canonicalQuery)
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return aliyunSMSEndpoint + "?" + canonicalQuery + "&Signature=" + percentEncode(signature)
}

type aliyunSMSResponse struct {
	Code       string `json:"Code"`
	Message    string `json:"Message"`
	BizID      string `json:"BizId"`
	RequestsID string `json:"RequestId"`
}

// sendSMSNotification 分发入口：把 SME_CODE 桩位替换为真实阿里云短信发送。
func (n *NotificationServicesConfig) sendSMSAliyunNotification(notificationGroup *model.NotificationGroup, templateVars *executeNotificationTemplateVars) {
	nConfig, err := parseExecuteEmailConfig(notificationGroup)
	if err != nil {
		logrus.Error("解析短信通知配置失败:", err)
		n.saveTenantChannelFailure(notificationGroup.TenantID, "", model.NoticeType_SME_CODE, smsGroupConfigInvalidReason, buildIMTextContent(templateVars), templateVars.deviceIDs...)
		return
	}

	accessKeyID := strings.TrimSpace(nConfig["ACCESS_KEY_ID"])
	accessKeySecret := strings.TrimSpace(nConfig["ACCESS_KEY_SECRET"])
	signName := strings.TrimSpace(nConfig["SIGN_NAME"])
	templateCode := strings.TrimSpace(nConfig["TEMPLATE_CODE"])
	phones := splitSMSPhones(nConfig["PHONE"])
	if accessKeyID == "" || accessKeySecret == "" || signName == "" || templateCode == "" || len(phones) == 0 {
		logrus.Warn("短信通知配置不完整")
		n.saveTenantChannelFailure(notificationGroup.TenantID, "", model.NoticeType_SME_CODE, smsGroupConfigInvalidReason, buildIMTextContent(templateVars), templateVars.deviceIDs...)
		return
	}

	// TemplateParam 以 JSON 对象传给阿里云模板占位符 ${content}。
	content := truncateRunesForSMS(buildIMTextContent(templateVars))
	templateParam, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		logrus.Error("编码短信模板参数失败", err)
		return
	}

	requestURL := buildAliyunSMSRequestURL(accessKeyID, accessKeySecret, signName, templateCode, strings.Join(phones, ","), string(templateParam), d2Now())
	// 审计目标：端点本身无凭证，但参数含 AccessKeyId/手机号——按红线不落参数，只落端点。
	auditTarget := aliyunSMSEndpoint
	// 发送历史沿用 IM 的 PENDING → SUCCESS/FAILURE 契约（复用其持久化缝）。
	err = n.sendSMSWithHistory(requestURL, auditTarget, content, notificationGroup.TenantID, templateVars.deviceIDs...)
	if err != nil {
		logrus.Error("短信通知发送失败:", err)
	}
}

func (n *NotificationServicesConfig) sendSMSWithHistory(requestURL, auditTarget, content, tenantID string, deviceIDs ...string) error {
	historyID := uuid.New()
	pendingStatus := "PENDING"
	history := &model.NotificationHistory{
		ID:               historyID,
		SendTime:         d2Now().UTC(),
		SendContent:      &content,
		SendTarget:       auditTarget,
		SendResult:       &pendingStatus,
		NotificationType: model.NoticeType_SME_CODE,
		TenantID:         tenantID,
		Remark:           nil,
	}
	if err := persistTenantIMHistory(history, deviceIDs...); err != nil {
		logrus.Error("保存短信通知历史失败", err)
		return err
	}

	var lastErr error
	maxRetries := 2
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		body, reqErr := aliyunSMSGetJSON(ctx, requestURL)
		cancel()
		if reqErr == nil {
			var parsed aliyunSMSResponse
			if parseErr := json.Unmarshal(body, &parsed); parseErr == nil && parsed.Code == "OK" {
				successStatus := "SUCCESS"
				if _, updateErr := updateNotificationHistoryStatusD2(historyID, &successStatus, nil, nil); updateErr != nil {
					logrus.Error("更新短信通知历史成功状态失败", updateErr)
					return fmt.Errorf("sms sent but notification history update failed: %w", updateErr)
				}
				return nil
			}
			// 业务拒绝（Code != OK）也算发送失败，进入重试；细节只进日志。
			lastErr = fmt.Errorf("%w: sms rejected", ErrIMExternalUnavailable)
			logrus.Warnf("SMS external rejected, attempt %d", i+1)
			continue
		}
		lastErr = reqErr
		logrus.Warnf("SMS external delivery failed, attempt %d", i+1)
	}

	failureStatus := "FAILURE"
	remarkText := smsExternalUnavailableReason
	_, updateErr := updateNotificationHistoryStatusD2(historyID, &failureStatus, &remarkText, &content)
	if updateErr != nil {
		logrus.Error("更新短信通知历史失败状态失败", updateErr)
	}
	externalErr := fmt.Errorf("%w: sms delivery to %s failed", ErrIMExternalUnavailable, auditTarget)
	return errors.Join(externalErr, lastErr, updateErr)
}

// splitSMSPhones 解析/归一/去重手机号（逗号分隔）。
func splitSMSPhones(raw string) []string {
	seen := map[string]struct{}{}
	phones := make([]string, 0)
	for _, phone := range strings.Split(raw, ",") {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}
		if _, exists := seen[phone]; exists {
			continue
		}
		seen[phone] = struct{}{}
		phones = append(phones, phone)
	}
	return phones
}

// truncateRunesForSMS 短信内容按 rune 截断（长内容会被运营商拆分计费，超长直接拒收）。
func truncateRunesForSMS(text string) string {
	runes := []rune(text)
	if len(runes) <= aliyunSMSMaxContentRunes {
		return text
	}
	return string(runes[:aliyunSMSMaxContentRunes]) + "…"
}

// PHASE-D-D2 END
