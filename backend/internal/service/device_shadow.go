// 文件用途：设备影子消息服务层，处理离线命令缓存的设置、查询、取消和投递。
// 核心逻辑：接收 API 请求写入影子消息；设备上线时批量查询 pending 并投递；TTL 过期自动清理。
// 关键注意事项：影子消息是核心差异化功能——解决"设备离线时下发命令失败"痛点；
//   上线投递依赖 telemetry uplink 首条消息钩子或 MQTT broker OnConnected 钩子触发。
package service

import (
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// DeviceShadow 提供设备影子消息的服务方法集。
type DeviceShadow struct{}

// SetDeviceShadowMessageReq 设置设备影子消息请求。
type SetDeviceShadowMessageReq struct {
	MessageType string          `json:"message_type" validate:"required,oneof=command property notification"`
	Payload     json.RawMessage `json:"payload" validate:"required"`
	TTLSeconds  int             `json:"ttl_seconds" validate:"omitempty,min=60,max=604800"`
}

// GetDeviceShadowMessagesResp 影子消息列表响应。
type GetDeviceShadowMessagesResp struct {
	List  []*model.DeviceShadowMessage `json:"list"`
	Total int64                        `json:"total"`
}

// SetShadowMessage 设置设备影子消息。如果设备在线则直接下发，否则缓存。
func (*DeviceShadow) SetShadowMessage(deviceId string, req *SetDeviceShadowMessageReq, claims *utils.UserClaims) (*model.DeviceShadowMessage, error) {
	if claims == nil || claims.ID == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if deviceId == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}

	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = 86400 // 默认 24h
	}
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)
	payloadBytes, _ := json.Marshal(req.Payload)

	msg := &model.DeviceShadowMessage{
		DeviceID:    deviceId,
		MessageType: req.MessageType,
		Payload:     strPtr(string(payloadBytes)),
		TTLSeconds:  ttl,
		Status:      "pending",
		CreatedBy:   strPtr(claims.ID),
		ExpiresAt:   expiresAt,
	}
	if err := dal.CreateShadowMessage(msg); err != nil {
		logrus.Errorf("SetShadowMessage create failed: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return msg, nil
}

// GetShadowMessages 查询指定设备的影子消息列表（含所有状态）。
func (*DeviceShadow) GetShadowMessages(deviceId string, claims *utils.UserClaims) (*GetDeviceShadowMessagesResp, error) {
	if claims == nil || claims.ID == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	msgs, err := dal.GetPendingShadowMessages(deviceId)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return &GetDeviceShadowMessagesResp{List: msgs, Total: int64(len(msgs))}, nil
}

// CancelShadowMessage 取消指定的 pending 影子消息。
func (*DeviceShadow) CancelShadowMessage(msgId string, claims *utils.UserClaims) error {
	if claims == nil || claims.ID == "" {
		return errcode.New(errcode.CodeNoPermission)
	}
	err := dal.CancelShadowMessage(msgId)
	if err != nil {
		logrus.Warnf("CancelShadowMessage id=%s err=%v", msgId, err)
	}
	return err
}

func strPtr(s string) *string { return &s }
