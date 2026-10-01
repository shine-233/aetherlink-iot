package aetherlink

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DrmagicE/gmqtt/plugin/aetherlink/util"
	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

var errMQTTMessageDiscarded = errors.New("message is discarded")

func (t *AetherLinkPlugin) OnMsgArrivedWrapper(pre server.OnMsgArrived) server.OnMsgArrived {
	return func(ctx context.Context, client server.Client, req *server.MsgArrivedRequest) (err error) {
		if err := pre(ctx, client, req); err != nil {
			return err
		}
		username := client.ClientOptions().Username
		msg := parseMQTTArrivedPayload(req)
		logMQTTMessageArrived(client, msg, username)

		if isMQTTSystemUser(username) {
			routeMQTTSystemMessage(ctx, client, msg, username)
			return nil
		}

		return routeMQTTDeviceMessage(ctx, client, req, msg, username)
	}
}

type mqttArrivedPayload struct {
	topic      string
	rawPayload []byte
	qos        byte
	retained   bool
}

type mqttDeviceRoute struct {
	deviceID       string
	deviceNumber   string
	deviceConfigID string
}

func parseMQTTArrivedPayload(req *server.MsgArrivedRequest) mqttArrivedPayload {
	return mqttArrivedPayload{
		topic:      req.Message.Topic,
		rawPayload: req.Message.Payload,
		qos:        req.Message.QoS,
		retained:   req.Message.Retained,
	}
}

func logMQTTMessageArrived(client server.Client, msg mqttArrivedPayload, username string) {
	Log.Debug(
		"mqtt message arrived",
		zap.String("topic", msg.topic),
		zap.String("client_id", client.ClientOptions().ClientID),
		zap.String("username", username),
		zap.Int("payload_size", len(msg.rawPayload)),
	)
}

func routeMQTTSystemMessage(ctx context.Context, client server.Client, msg mqttArrivedPayload, username string) {
	src, outPayload, deviceID, ok := resolveMQTTDownlinkRoute(ctx, msg)
	if !ok {
		return
	}

	forwardMQTTDownlink(client, username, msg.topic, src, msg.rawPayload, outPayload, deviceID)
}

func resolveMQTTDownlinkRoute(ctx context.Context, msg mqttArrivedPayload) (string, []byte, string, bool) {
	deviceNumber, ok := TryExtractDeviceNumberFromNormalized(msg.topic)
	if !ok || deviceNumber == "" {
		return "", nil, "", false
	}

	dev, derr := GetDeviceByNumber(deviceNumber)
	if derr != nil || ensureMQTTDeviceActive(dev) != nil || dev.DeviceConfigID == nil {
		return "", nil, "", false
	}

	svc := NewTopicMapService()
	src, outPayload, matched := svc.ResolveDownSource(ctx, *dev.DeviceConfigID, msg.topic, deviceNumber, msg.rawPayload)
	if !matched || src == "" {
		return "", nil, "", false
	}

	return src, outPayload, dev.ID, true
}

func forwardMQTTDownlink(client server.Client, username string, topic string, src string, originalPayload []byte, outPayload []byte, deviceID string) {
	if err := DefaultMqttClient.SendData(src, outPayload); err != nil {
		Log.Warn("custom downlink forward failed", zap.String("topic", topic), zap.String("client_id", client.ClientOptions().ClientID), zap.Error(err))
		writeMQTTForwardDebugLog(client, username, deviceID, topic, src, "error", err.Error(), originalPayload)
		return
	}
	Log.Info("custom downlink forward succeeded", zap.String("topic", topic), zap.String("client_id", client.ClientOptions().ClientID), zap.String("target", src))
	writeMQTTForwardDebugLog(client, username, deviceID, topic, src, "ok", "", originalPayload)
}

func routeMQTTDeviceMessage(ctx context.Context, client server.Client, req *server.MsgArrivedRequest, msg mqttArrivedPayload, username string) error {
	route, err := resolveMQTTDeviceRoute(client)
	if err != nil {
		return err
	}

	return route.dispatchMQTTUplink(ctx, client, req, msg, username)
}

func resolveMQTTDeviceRoute(client server.Client) (mqttDeviceRoute, error) {
	clientID := client.ClientOptions().ClientID
	deviceID, ok := mqttAuthenticatedDeviceForClient(client)
	if !ok {
		return mqttDeviceRoute{}, errors.New("mqtt client device binding is missing")
	}
	device, err := loadActiveMQTTDevice(deviceID)
	if err != nil {
		forgetMQTTAuthenticatedClientBinding(client)
		forgetMQTTAuthenticatedDevice(clientID)
		return mqttDeviceRoute{}, err
	}

	return mqttDeviceRoute{
		deviceID:       device.ID,
		deviceNumber:   device.DeviceNumber,
		deviceConfigID: mqttDeviceConfigID(device),
	}, nil
}

func mqttDeviceConfigID(device *Device) string {
	if device == nil || device.DeviceConfigID == nil {
		return ""
	}
	return *device.DeviceConfigID
}

func (route mqttDeviceRoute) dispatchMQTTUplink(ctx context.Context, client server.Client, req *server.MsgArrivedRequest, msg mqttArrivedPayload, username string) error {
	// 所有上行都先经过同一 schema 门禁，避免自定义 topic mapping 绕过设备配置约束。
	if enforcePayloadSchemaOnUplink(route.deviceID, route.deviceConfigID, msg.rawPayload) {
		writeMQTTPublishDebugLog(client, username, route.deviceID, msg.topic, false, "", "drop", "payload schema enforcement rejected", msg.rawPayload)
		req.Drop()
		return errMQTTMessageDiscarded
	}

	if route.deviceConfigID != "" {
		handled, err := route.tryMappedMQTTUplink(ctx, client, msg, username)
		if handled {
			req.MarkHandled()
			req.Drop()
			return err
		}
		Log.Debug("mqtt uplink did not match custom mapping", zap.String("topic", msg.topic), zap.String("client_id", client.ClientOptions().ClientID))
	}

	return handleStandardMQTTUplink(client, req, msg, username, route.deviceID, route.deviceNumber)
}

func (route mqttDeviceRoute) tryMappedMQTTUplink(ctx context.Context, client server.Client, msg mqttArrivedPayload, username string) (bool, error) {
	if route.deviceConfigID == "" {
		return false, nil
	}

	target, ok := resolveUpTarget(ctx, route.deviceConfigID, msg.topic)
	if !ok || target == "" {
		return false, nil
	}

	if err := mappedMQTTPublisherReady(ctx); err != nil {
		writeMQTTPublishDebugLog(client, username, route.deviceID, msg.topic, true, target, "error", err.Error(), msg.rawPayload)
		return true, fmt.Errorf("custom uplink forward unavailable: %w", err)
	}
	payload := buildMQTTUplinkPayload(route.deviceID, msg.rawPayload)
	if msg.retained && len(msg.rawPayload) == 0 {
		payload = nil
	}
	if err := mappedMQTTPublisher.SendMessage(target, msg.qos, msg.retained, payload); err != nil {
		writeMQTTPublishDebugLog(client, username, route.deviceID, msg.topic, true, target, "error", err.Error(), msg.rawPayload)
		Log.Warn("custom uplink forward failed", zap.String("topic", msg.topic), zap.String("client_id", client.ClientOptions().ClientID), zap.Error(err))
		return true, fmt.Errorf("custom uplink forward failed: %w", err)
	}

	Log.Info("custom uplink forward succeeded", zap.String("topic", msg.topic), zap.String("client_id", client.ClientOptions().ClientID), zap.String("target", target))
	writeMQTTPublishDebugLog(client, username, route.deviceID, msg.topic, true, target, "ok", "", msg.rawPayload)
	return true, nil
}

func handleStandardMQTTUplink(client server.Client, req *server.MsgArrivedRequest, msg mqttArrivedPayload, username string, deviceID string, deviceNumber string) error {
	// 发布校验与订阅侧 ValidateSubTopicForDevice 对称：
	// 形状匹配之外，topic 中携带设备身份的槽位（devices/status 的设备 ID、
	// '+/up' 首层的设备编号）必须等于发布者自身身份，防止跨设备注入。
	if !util.ValidatePubTopicForDevice(msg.topic, deviceID, deviceNumber) {
		return handleMQTTPublishPermissionDenied(client, username, deviceID, msg)
	}

	req.Message.Payload = buildMQTTUplinkPayload(deviceID, msg.rawPayload)
	writeMQTTPublishDebugLog(client, username, deviceID, msg.topic, false, "", "ok", "", msg.rawPayload)
	return nil
}

func handleMQTTPublishPermissionDenied(client server.Client, username string, deviceID string, msg mqttArrivedPayload) error {
	writeMQTTPublishDebugLog(client, username, deviceID, msg.topic, false, "", "deny", "permission denied", msg.rawPayload)
	Log.Warn("mqtt publish permission denied", zap.String("topic", msg.topic), zap.String("client_id", client.ClientOptions().ClientID))
	return errors.New("permission denied")
}

// buildMQTTUplinkPayload 产出 {"device_id":"<id>","values":"<base64(payload)>"}，
// 与 json.Marshal(map[string]interface{}{...}) 逐字节一致（键序、base64、nil→null）。
// 每条上行都会调用：device_id 为纯安全 ASCII（UUID 常态）时一次性分配直接拼装，
// 否则回退 json.Marshal 以复用其转义规则（HTML 字符、控制符、非法 UTF-8）。
func buildMQTTUplinkPayload(deviceID string, values []byte) []byte {
	if !jsonSafeASCII(deviceID) {
		newMsgJSON, _ := json.Marshal(map[string]interface{}{
			"device_id": deviceID,
			"values":    values,
		})
		return newMsgJSON
	}
	const prefix, middle = `{"device_id":"`, `","values":`
	n := len(prefix) + len(deviceID) + len(middle) + 1
	if values == nil {
		n += len("null")
	} else {
		n += base64.StdEncoding.EncodedLen(len(values)) + 2
	}
	out := make([]byte, 0, n)
	out = append(out, prefix...)
	out = append(out, deviceID...)
	out = append(out, middle...)
	if values == nil {
		out = append(out, "null"...)
	} else {
		out = append(out, '"')
		start := len(out)
		out = out[:start+base64.StdEncoding.EncodedLen(len(values))]
		base64.StdEncoding.Encode(out[start:], values)
		out = append(out, '"')
	}
	return append(out, '}')
}

// jsonSafeASCII 报告 s 能否原样放进 JSON 字符串（encoding/json 默认 HTML 转义口径）。
func jsonSafeASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c >= 0x7f || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			return false
		}
	}
	return true
}

func writeMQTTForwardDebugLog(client server.Client, username string, deviceID string, topic string, sourceTopic string, outcome string, errMsg string, payload []byte) {
	recordMQTTDiagnosticForClient(client, mqttDiagnosticEvent{
		deviceID:    deviceID,
		username:    username,
		action:      "forward",
		direction:   "down",
		outcome:     outcome,
		error:       errMsg,
		code:        "downlink_forward_" + outcome,
		topic:       topic,
		mapped:      true,
		targetTopic: topic,
		sourceTopic: sourceTopic,
		payload:     payload,
	})
}

func writeMQTTPublishDebugLog(client server.Client, username string, deviceID string, topic string, mapped bool, targetTopic string, outcome string, errMsg string, payload []byte) {
	recordMQTTDiagnosticForClient(client, mqttDiagnosticEvent{
		deviceID:    deviceID,
		username:    username,
		action:      "publish",
		direction:   "up",
		outcome:     outcome,
		error:       errMsg,
		code:        "publish_" + outcome,
		topic:       topic,
		mapped:      mapped,
		targetTopic: targetTopic,
		payload:     payload,
	})
}
