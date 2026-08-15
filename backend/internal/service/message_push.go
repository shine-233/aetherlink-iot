// message_push.go 负责消息推送配置、外部推送发送、结果归类与审计日志落库，
// 重点保证配置权限边界、返回码判定和失败次数统计的一致性。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type MessagePush struct{}

const (
	messagePushStatusSuccess int16 = 1
	messagePushStatusFailed  int16 = 2

	messagePushMaxResponseBytes int64 = 1 << 20

	messagePushDeliveryFailedReason      = "MESSAGE_PUSH_DELIVERY_FAILED"
	messagePushExternalUnavailableReason = "MESSAGE_PUSH_EXTERNAL_UNAVAILABLE"
)

var (
	ErrMessagePushDisabled            = errors.New("message push is disabled")
	ErrMessagePushExternalUnavailable = errors.New("message push external service unavailable")

	// net/http clients and transports retain connection pools and are safe for
	// concurrent use. Reuse one bounded client instead of creating a pool per push.
	messagePushHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

type messagePushDeliveryResult struct {
	status     int16
	errMessage string
	skipped    bool
}

// requireMessagePushConfigAdmin 约束推送配置只能由系统管理员维护，
// 避免租户用户误改全局推送出口。
func requireMessagePushConfigAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage message push config")
	}
	return nil
}

func validateMessagePushURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push url is required")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push url must use http or https")
	}
	return nil
}

func (receiver *MessagePush) CreateMessagePush(req *model.CreateMessagePushReq, userId string) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push registration is required")
	}
	req.PushId = strings.TrimSpace(req.PushId)
	req.DeviceType = strings.TrimSpace(req.DeviceType)
	if req.PushId == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "push id is required")
	}
	if req.DeviceType == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device type is required")
	}
	exists, err := dal.GetMessagePushMangeExists(userId, req.PushId)
	if err != nil {
		return err
	}
	if exists {
		return dal.ActiveMessagePushMange(userId, req.PushId, req.DeviceType)
	}
	return dal.CreateMessagePushMange(&model.MessagePushManage{
		ID:         uuid.New(),
		UserID:     userId,
		PushID:     req.PushId,
		DeviceType: req.DeviceType,
		Status:     messagePushStatusSuccess,
		CreateTime: time.Now(),
	})
}

func (receiver *MessagePush) MessagePushMangeLogout(req *model.MessagePushMangeLogoutReq, userId string) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push logout is required")
	}
	req.PushId = strings.TrimSpace(req.PushId)
	if req.PushId == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "push id is required")
	}
	exists, err := dal.GetMessagePushMangeExists(userId, req.PushId)
	if err != nil {
		return err
	}
	if exists {
		return dal.LogoutMessagePushMange(userId, req.PushId)
	}
	return errors.New("message push registration does not exist for current user")
}

func (receiver *MessagePush) GetMessagePushConfig(claims *utils.UserClaims) (*model.MessagePushConfigRes, error) {
	if err := requireMessagePushConfigAdmin(claims); err != nil {
		return nil, err
	}
	return dal.GetMessagePushConfig()
}

func (receiver *MessagePush) SetMessagePushConfig(req *model.MessagePushConfigReq, claims *utils.UserClaims) error {
	if err := requireMessagePushConfigAdmin(claims); err != nil {
		return err
	}
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "message push config is required")
	}
	if err := validateMessagePushURL(req.Url); err != nil {
		return err
	}
	req.Url = strings.TrimSpace(req.Url)
	return dal.SetMessagePushConfig(req)
}

func (receiver *MessagePush) MessagePushSend(message model.MessagePushSend) (string, error) {
	config, err := dal.GetMessagePushConfig()
	if err != nil {
		return "", err
	}
	if config == nil || strings.TrimSpace(config.Url) == "" {
		return "", ErrMessagePushDisabled
	}
	if err := validateMessagePushURL(config.Url); err != nil {
		return "", fmt.Errorf("%w: stored endpoint is invalid", ErrMessagePushDisabled)
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).Error("message push payload marshal failed")
		return "", err
	}
	logrus.WithFields(logrus.Fields{
		"payload_bytes": len(jsonData),
		"title_bytes":   len(message.Title),
		"content_bytes": len(message.Content),
	}).Debug("message push request prepared")

	return deliverMessagePushHTTP(strings.TrimSpace(config.Url), jsonData)
}

func deliverMessagePushHTTP(endpoint string, payload []byte) (string, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("%w: create request", ErrMessagePushExternalUnavailable)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := messagePushHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: request failed", ErrMessagePushExternalUnavailable)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, messagePushMaxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: read response", ErrMessagePushExternalUnavailable)
	}
	if int64(len(body)) > messagePushMaxResponseBytes {
		return "", fmt.Errorf("%w: response exceeds %d bytes", ErrMessagePushExternalUnavailable, messagePushMaxResponseBytes)
	}
	logrus.WithFields(logrus.Fields{
		"status_code":    resp.StatusCode,
		"response_bytes": len(body),
	}).Debug("message push response received")
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: HTTP status %d", ErrMessagePushExternalUnavailable, resp.StatusCode)
	}
	return string(body), nil
}

func (receiver *MessagePush) AlarmMessagePushSend(triggered, alarmConfigId string, deviceInfo *model.Device) {
	if deviceInfo == nil {
		logrus.Error("alarm message push skipped: device info is nil")
		return
	}
	pushManges, err := dal.GetUserMessagePushId(deviceInfo.TenantID)
	if err != nil {
		logrus.Error("query message push users failed:", err)
		return
	}
	if len(pushManges) == 0 {
		return
	}
	logrus.Debug(fmt.Sprintf("pushManges:%#v", len(pushManges)))

	message := model.MessagePushSend{
		Title:        fmt.Sprintf("alarm:%v", triggered),
		Content:      deviceInfo.DeviceNumber,
		AlarmId:      &alarmConfigId,
		PushClientId: "",
	}
	for _, v := range pushManges {
		if v.PushID == "" {
			continue
		}
		message.PushClientId = v.PushID
		receiver.MessagePushSendAndLog(message, v, 1)
	}
}

func (receiver *MessagePush) MessagePushSendAndLog(message model.MessagePushSend, mange model.MessagePushManage, messageType int64) {
	// 只有真正尝试过发送才记录结果并更新 err_count；未配置外部推送时属于禁用，而非投递失败。
	result := receiver.deliverMessagePushForLog(message)
	if result.skipped {
		return
	}
	log := buildMessagePushLog(message, mange, messageType, result)
	saveMessagePushLog(&log)
	updateMessagePushManageAfterSend(mange.ID, log.Status)
}

func (receiver *MessagePush) deliverMessagePushForLog(message model.MessagePushSend) messagePushDeliveryResult {
	res, err := receiver.MessagePushSend(message)
	if err != nil {
		return classifyMessagePushDeliveryError(err)
	}
	status, errMessage := classifyMessagePushResponse(res)
	return messagePushDeliveryResult{
		status:     status,
		errMessage: errMessage,
	}
}

func classifyMessagePushDeliveryError(err error) messagePushDeliveryResult {
	if errors.Is(err, ErrMessagePushDisabled) {
		return messagePushDeliveryResult{skipped: true}
	}

	reason := messagePushDeliveryFailedReason
	if errors.Is(err, ErrMessagePushExternalUnavailable) {
		reason = messagePushExternalUnavailableReason
	}
	return messagePushDeliveryResult{
		status:     messagePushStatusFailed,
		errMessage: reason,
	}
}

func buildMessagePushLog(message model.MessagePushSend, mange model.MessagePushManage, messageType int64, result messagePushDeliveryResult) model.MessagePushLog {
	contents, _ := json.Marshal(message)
	return model.MessagePushLog{
		ID:          uuid.New(),
		UserID:      mange.UserID,
		MessageType: messageType,
		Content:     string(contents),
		Status:      result.status,
		ErrMessage:  result.errMessage,
		CreateTime:  time.Now(),
	}
}

func saveMessagePushLog(log *model.MessagePushLog) {
	err := dal.MessagePushSendLogSave(log)
	if err != nil {
		logrus.Error("save message push log failed", err)
	}
}

func updateMessagePushManageAfterSend(manageID string, status int16) {
	updates := buildMessagePushManageUpdates(status)
	err := dal.MessagePushMangeSendUpdate(manageID, updates)
	if err != nil {
		logrus.Error("update message push manage state failed", err)
	}
}

func (receiver *MessagePush) NotificationMessagePushSend(tenantId string, title string, content string, payload map[string]interface{}) {
	pushManges, err := dal.GetUserMessagePushId(tenantId)
	if err != nil {
		logrus.Error("query message push users failed:", err)
		return
	}
	if len(pushManges) == 0 {
		logrus.Debug("tenant has no bound message push users", tenantId)
		return
	}
	logrus.Debug(fmt.Sprintf("message push user count: %d", len(pushManges)))

	message := model.MessagePushSend{
		Title:        title,
		Content:      content,
		PushClientId: "",
	}

	if payload != nil {
		if alarmConfigId, ok := payload["alarm_config_id"].(string); ok && alarmConfigId != "" {
			message.AlarmId = &alarmConfigId
		}
	}

	for _, mange := range pushManges {
		if mange.PushID == "" {
			continue
		}
		message.PushClientId = mange.PushID
		receiver.MessagePushSendAndLog(message, mange, 2)
	}
}

func (receiver *MessagePush) MessagePushMangeClear() {
	err := dal.GetMessagePushMangeInactiveWithSeven()
	if err != nil {
		logrus.Error("mark stale offline message push users failed", err)
		return
	}

	err = dal.GetMessagePushMangeInactive()
	if err != nil {
		logrus.Error("mark inactive message push users failed", err)
		return
	}
}

func messagePushResponseLogMessage(responseBytes int) string {
	return fmt.Sprintf("message push response classified, response_bytes:%d", responseBytes)
}

func classifyMessagePushResponse(res string) (int16, string) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(res), &result)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"response_bytes": len(res),
		}).Error("message push response parse failed")
		return messagePushStatusFailed, fmt.Sprintf("message push response parse failed: %v, response_bytes:%d", err, len(res))
	}

	logrus.WithField("keys", len(result)).Debug("message push response parsed")

	// 历史推送服务既出现过 errCode，也出现过 code，这里两种协议都兼容。
	if errCode, ok := result["errCode"]; ok {
		logrus.Debug("found errCode:", messagePushDebugFieldValue(errCode))
		return classifyMessagePushNumericField("errCode", errCode, 0, len(res))
	}
	if code, ok := result["code"]; ok {
		logrus.Debug("errCode absent, checking code:", messagePushDebugFieldValue(code))
		return classifyMessagePushNumericField("code", code, 200, len(res))
	}

	logrus.Debug("message push response missing errCode/code")
	return messagePushStatusFailed, messagePushResponseLogMessage(len(res))
}

func classifyMessagePushNumericField(fieldName string, raw interface{}, successValue float64, responseBytes int) (int16, string) {
	value, ok := raw.(float64)
	if !ok {
		logrus.Debugf("message push failed: %s type mismatch %T", fieldName, raw)
		return messagePushStatusFailed, messagePushResponseLogMessage(responseBytes)
	}
	if value == successValue {
		logrus.Debugf("message push succeeded: %s = %v", fieldName, value)
		return messagePushStatusSuccess, messagePushResponseLogMessage(responseBytes)
	}
	logrus.Debugf("message push failed: %s = %v", fieldName, value)
	return messagePushStatusFailed, messagePushResponseLogMessage(responseBytes)
}

func buildMessagePushManageUpdates(status int16) map[string]interface{} {
	updates := map[string]interface{}{
		"last_push_time": time.Now(),
	}
	if status == messagePushStatusSuccess {
		updates["err_count"] = 0
	} else {
		updates["err_count"] = gorm.Expr("err_count + ?", 1)
	}
	return updates
}

func messagePushDebugFieldValue(raw interface{}) interface{} {
	if _, ok := raw.(float64); ok {
		return raw
	}
	return fmt.Sprintf("%T", raw)
}
