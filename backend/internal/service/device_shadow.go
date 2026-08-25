// 文件用途：设备影子消息服务层，处理离线命令缓存的设置、查询、取消和上线投递。
// 核心逻辑：设备在线时命令直接走下发链路；离线时写入影子队列，设备重新上线后自动投递。
// 关键注意事项：影子消息是核心差异化功能（ROADMAP A3）——解决"设备离线时下发命令失败/静默丢失"痛点；
//   上线投递挂靠 uplink 在线钩子与 status_flow 状态切换；TTL 过期由 cron 定时清理。
package service

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const deviceShadowDefaultTTLSeconds = 86400

// DeviceShadow 设备影子服务入口。
type DeviceShadow struct{}

// SetDeviceShadowMessageReq 设置设备影子消息请求。
type SetDeviceShadowMessageReq struct {
	MessageType string          `json:"message_type" validate:"required,oneof=command property notification"`
	Payload     json.RawMessage `json:"payload" validate:"required"`
	TTLSeconds  int             `json:"ttl_seconds" validate:"omitempty,min=60,max=604800"`
}

// SetShadowMessageResp 设置影子消息响应：direct=true 表示设备在线已直接下发。
type SetShadowMessageResp struct {
	Message *model.DeviceShadowMessage `json:"message"`
	Direct  bool                       `json:"direct"`
}

// GetDeviceShadowMessagesResp 影子消息列表响应。
type GetDeviceShadowMessagesResp struct {
	List   []*model.DeviceShadowMessage `json:"list"`
	Counts map[string]int64             `json:"counts"`
	Total  int64                        `json:"total"`
}

// SetShadowMessage 设置设备影子消息：设备在线则直接下发（不落库），离线则缓存待投递。
func (*DeviceShadow) SetShadowMessage(deviceId string, req *SetDeviceShadowMessageReq, claims *utils.UserClaims) (*SetShadowMessageResp, error) {
	if claims == nil || claims.ID == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if deviceId == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(deviceId, claims)
	if err != nil {
		return nil, err
	}
	payload := []byte(req.Payload)
	if !json.Valid(payload) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "payload must be valid json")
	}

	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = deviceShadowDefaultTTLSeconds
	}

	// 在线设备直接走命令下发链路，语义与 ROADMAP A3 流程图一致。
	if deviceInfo.IsOnline == 1 && req.MessageType == "command" {
		method, params := decodeShadowCommandPayload(payload)
		putMessage := &model.PutMessageForCommand{
			DeviceID: deviceId,
			Identify: method,
			Value:    &params,
		}
		if sendErr := GroupApp.CommandData.CommandPutMessage(context.Background(), "", putMessage, "2"); sendErr == nil {
			return &SetShadowMessageResp{Direct: true}, nil
		} else {
			logrus.Warnf("shadow direct send failed, falling back to shadow queue: device=%s err=%v", deviceId, sendErr)
		}
	}

	now := time.Now().UTC()
	msg := &model.DeviceShadowMessage{
		ID:          uuid.New(),
		DeviceID:    deviceId,
		MessageType: req.MessageType,
		Payload:     StringPtr(string(payload)),
		TTLSeconds:  ttl,
		Status:      "pending",
		CreatedBy:   StringPtr(claims.ID),
		ExpiresAt:   now.Add(time.Duration(ttl) * time.Second),
	}
	if err := dal.CreateShadowMessage(msg); err != nil {
		logrus.Errorf("SetShadowMessage create failed: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return &SetShadowMessageResp{Message: msg, Direct: false}, nil
}

// GetShadowMessages 查询指定设备的影子消息列表（可按状态过滤）及各状态计数。
func (*DeviceShadow) GetShadowMessages(deviceId string, status string, claims *utils.UserClaims) (*GetDeviceShadowMessagesResp, error) {
	if _, err := ensureTelemetryDeviceReadAccess(deviceId, claims); err != nil {
		return nil, err
	}
	msgs, err := dal.GetAllShadowMessages(deviceId, status)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	counts, err := dal.CountShadowMessagesByDevice(deviceId)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	total := int64(0)
	for _, n := range counts {
		total += n
	}
	return &GetDeviceShadowMessagesResp{List: msgs, Counts: counts, Total: total}, nil
}

// CancelShadowMessage 取消指定的 pending 影子消息。
func (*DeviceShadow) CancelShadowMessage(deviceId, msgId string, claims *utils.UserClaims) error {
	if _, err := ensureTelemetryDeviceWriteAccess(deviceId, claims); err != nil {
		return err
	}
	err := dal.CancelShadowMessage(msgId)
	if err != nil {
		logrus.Warnf("CancelShadowMessage id=%s err=%v", msgId, err)
		return errcode.NewWithMessage(errcode.CodeParamError, "pending shadow message not found")
	}
	return nil
}

// DeliverPendingShadowMessages 设备上线后调用：先把到期消息置为 expired，再逐条投递剩余 pending。
// 单条失败不影响后续投递；失败的保持 pending，等待下次上线重试。返回成功投递条数。
func (*DeviceShadow) DeliverPendingShadowMessages(deviceId string) (int, error) {
	if _, err := dal.ExpireDueShadowMessages(); err != nil {
		logrus.Warnf("shadow expire sweep failed before delivery: device=%s err=%v", deviceId, err)
	}
	pending, err := dal.GetPendingShadowMessages(deviceId)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, msg := range pending {
		if sendErr := dispatchShadowMessage(deviceId, msg); sendErr != nil {
			logrus.Warnf("shadow deliver failed (kept pending): device=%s msg=%s err=%v", deviceId, msg.ID, sendErr)
			continue
		}
		if markErr := dal.MarkShadowMessageDelivered(msg.ID); markErr != nil {
			logrus.Errorf("shadow mark delivered failed: msg=%s err=%v", msg.ID, markErr)
			continue
		}
		delivered++
	}
	return delivered, nil
}

// CleanupExpiredShadowMessages cron 入口：到期标记 + 过期历史清理。
func (*DeviceShadow) CleanupExpiredShadowMessages() (expired int64, deleted int64) {
	expired, err := dal.ExpireDueShadowMessages()
	if err != nil {
		logrus.Warnf("shadow expire sweep failed: %v", err)
	}
	deleted, err = dal.DeleteStaleShadowMessages()
	if err != nil {
		logrus.Warnf("shadow stale cleanup failed: %v", err)
	}
	return expired, deleted
}

// dispatchShadowMessage 把单条影子消息送入现有命令下发链路。
func dispatchShadowMessage(deviceId string, msg *model.DeviceShadowMessage) error {
	switch msg.MessageType {
	case "command":
		params := derefString(msg.Payload)
		putMessage := &model.PutMessageForCommand{
			DeviceID: deviceId,
			Identify: "shadow_dispatch",
			Value:    &params,
		}
		return GroupApp.CommandData.CommandPutMessage(context.Background(), "", putMessage, "2")
	default:
		// property/notification 类型当前仅支持 command 链路；其余类型标记后由订阅方消费。
		logrus.Warnf("shadow message type %q has no dispatch channel; treating as delivered", msg.MessageType)
		return nil
	}
}

// decodeShadowCommandPayload 解析影子命令负载中的 method 与 params 字符串。
func decodeShadowCommandPayload(payload []byte) (method, params string) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return "shadow_dispatch", string(payload)
	}
	if m, ok := raw["method"].(string); ok && m != "" {
		method = m
	} else {
		method = "shadow_dispatch"
	}
	if p, ok := raw["params"]; ok {
		if pBytes, pErr := json.Marshal(p); pErr == nil {
			params = string(pBytes)
		}
	} else if raw2, ok := raw["identify"].(string); ok {
		method = raw2
	}
	return method, params
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
