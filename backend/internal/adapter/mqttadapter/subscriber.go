// 文件用途：封装 MQTT adapter 的订阅和上行消息分发逻辑。
// 核心逻辑：订阅设备上行 topic，解析消息并投递到后端 uplink 处理流程。
// 关键注意事项：订阅通配符和回调错误处理会影响所有设备上行链路，修改需避免吞错或重复投递。
// 重构建议：可将 topic 匹配、消息反序列化和 uplink 分发拆成小函数，提升解析失败覆盖率。

package mqttadapter

import (
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/uplink"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// SubscribeResponseTopics 订阅响应 Topic（供 MQTT 服务初始化时调用）。
// 在 MQTT 客户端连接成功后调用，主要承接设备对平台下行命令/属性设置的响应。
func (a *Adapter) SubscribeResponseTopics(client mqtt.Client) error {
	topics := map[string]byte{
		TopicPatternCommandResponse:             1, // 设备命令响应
		TopicPatternAttributeSetResponse:        1, // 设备属性设置响应
		TopicPatternGatewayCommandResponse:      1, // 网关命令响应
		TopicPatternGatewayAttributeSetResponse: 1, // 网关属性设置响应
	}

	for topic, qos := range topics {
		// 使用共享订阅（VerneMQ 支持）
		topic := genSharedTopic(topic)
		token := client.Subscribe(topic, qos, a.handleResponseMessage)
		if err := waitMQTTToken(token, mqttAdapterOperationTimeout); err != nil {
			a.logger.WithFields(logrus.Fields{
				"topic": topic,
				"error": err,
			}).Error("Failed to subscribe response topic")
			return err
		}
		a.logger.WithField("topic", topic).Info("Subscribed to response topic")
	}

	return nil
}

// handleResponseMessage 处理响应消息（MQTT 回调函数）。
// 关键链路：topic 解析 -> payload 验证 -> 缓存查设备 -> 构造 UplinkMessage -> 投递到 Bus。
func (a *Adapter) handleResponseMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	a.logger.WithFields(logrus.Fields{
		"topic":   topic,
		"payload": string(payload),
	}).Debug("Received response message")

	// 1. 从 Topic 解析 message_id
	// Topic 格式: devices/command/response/{message_id}
	//           gateway/attributes/set/response/{message_id}
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		a.logger.WithField("topic", topic).Error("Invalid response topic format")
		return
	}

	messageID := parts[len(parts)-1]
	msgType := a.detectResponseType(topic)

	// 2. 验证 payload 格式
	responsePayload, err := a.verifyPayload(payload)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Invalid response payload")
		return
	}

	// 3. 获取设备信息
	device, err := initialize.GetDeviceCacheById(responsePayload.DeviceId)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"device_id": responsePayload.DeviceId,
			"error":     err,
		}).Error("Device not found in cache")
		return
	}

	// 4. 构造 UplinkMessage
	flowMsg := &UplinkMessage{
		Type:      msgType,
		DeviceID:  device.ID,
		TenantID:  device.TenantID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   responsePayload.Values,
		Metadata: map[string]interface{}{
			"device_id":       device.ID,
			"topic":           topic,
			"source_protocol": "mqtt",
			"message_id":      messageID, // ✨ 关键：传递 message_id
		},
	}

	// 5. 发送到 Bus
	if err := a.bus.Publish(flowMsg); err != nil {
		a.logger.WithFields(logrus.Fields{
			"device_id":  device.ID,
			"message_id": messageID,
			"error":      err,
		}).Error("Failed to publish response message to bus")
		return
	}

	a.logger.WithFields(logrus.Fields{
		"device_id":  device.ID,
		"message_id": messageID,
		"msg_type":   msgType,
	}).Info("Response message published to bus")
}

// detectResponseType 检测响应类型。
// 这里依赖 topic 字符串包含关系做分类，后续若协议路径扩展，建议用更显式的模板映射表。
func (a *Adapter) detectResponseType(topic string) string {
	// Topic 格式:
	// - devices/command/response/{message_id} → "command_response"
	// - devices/attributes/set/response/{message_id} → "attribute_set_response"
	// - gateway/command/response/{message_id} → "gateway_command_response"
	// - gateway/attributes/set/response/{message_id} → "gateway_attribute_set_response"

	if strings.Contains(topic, "command/response") {
		if strings.HasPrefix(topic, "gateway/") {
			return uplink.MessageTypeGatewayCommandResponse
		}
		return uplink.MessageTypeCommandResponse
	}

	if strings.Contains(topic, "attributes/set/response") {
		if strings.HasPrefix(topic, "gateway/") {
			return uplink.MessageTypeGatewayAttributeSetResponse
		}
		return uplink.MessageTypeAttributeSetResponse
	}

	return "unknown_response"
}

// genSharedTopic 生成共享订阅 Topic（用于负载均衡）。
// 共享订阅是否启用完全取决于配置；broker 不支持时应回退到普通订阅语义。
func genSharedTopic(topic string) string {
	// 如果mqtt_server为vernemq，则使用$share/mygroup/topic
	group := ""
	isTrue := viper.GetBool("mqtt.enable_shared_subscription")
	if isTrue {
		groupName := viper.GetString("mqtt.shared_subscription_group")
		if groupName == "" {
			group = "$share/mygroup/"
			logrus.Debugf("使用默认共享组: %s", groupName)
		} else {
			group = "$share/" + groupName + "/"
		}
	}
	return group + topic
}

// SubscribeDeviceTopics 订阅设备上行 Topic（供 MQTT 服务初始化时调用）。
// 这是直连设备的主上行入口，topic 通配符调整会直接影响接入覆盖面。
func (a *Adapter) SubscribeDeviceTopics(client mqtt.Client) error {
	topics := map[string]struct {
		qos      byte
		handler  mqtt.MessageHandler
		describe string
	}{
		TopicPatternTelemetry: {
			qos:      1,
			handler:  a.handleTelemetryMessage,
			describe: "设备遥测上报",
		},
		TopicPatternAttribute: {
			qos:      1,
			handler:  a.handleAttributeMessage,
			describe: "设备属性上报",
		},
		TopicPatternEvent: {
			qos:      1,
			handler:  a.handleEventMessage,
			describe: "设备事件上报",
		},
		TopicPatternStatus: {
			qos:      1,
			handler:  a.handleStatusMessage,
			describe: "设备状态上报",
		},
		TopicPatternOTAProgress: {
			qos:      1,
			handler:  a.handleEventMessage,
			describe: "OTA设备进度上报",
		},
	}

	for topic, config := range topics {
		// 使用共享订阅（VerneMQ 支持）
		sharedTopic := genSharedTopic(topic)
		token := client.Subscribe(sharedTopic, config.qos, config.handler)
		if err := waitMQTTToken(token, mqttAdapterOperationTimeout); err != nil {
			a.logger.WithFields(logrus.Fields{
				"topic": sharedTopic,
				"error": err,
			}).Error("Failed to subscribe device topic")
			return err
		}
		a.logger.WithFields(logrus.Fields{
			"topic":    sharedTopic,
			"describe": config.describe,
		}).Info("Subscribed to device topic")
	}

	return nil
}

// SubscribeGatewayTopics 订阅网关上行 Topic（供 MQTT 服务初始化时调用）。
// 网关 topic 与直连 topic 共用部分处理器，因此改动时要同步审查 detectMessageType。
func (a *Adapter) SubscribeGatewayTopics(client mqtt.Client) error {
	topics := map[string]struct {
		qos      byte
		handler  mqtt.MessageHandler
		describe string
	}{
		TopicPatternGatewayTelemetry: {
			qos:      0,
			handler:  a.handleTelemetryMessage,
			describe: "网关遥测上报",
		},
		TopicPatternGatewayAttribute: {
			qos:      1,
			handler:  a.handleAttributeMessage,
			describe: "网关属性上报",
		},
		TopicPatternGatewayEvent: {
			qos:      1,
			handler:  a.handleEventMessage,
			describe: "网关事件上报",
		},
	}

	for topic, config := range topics {
		// 使用共享订阅（VerneMQ 支持）
		sharedTopic := genSharedTopic(topic)
		token := client.Subscribe(sharedTopic, config.qos, config.handler)
		if err := waitMQTTToken(token, mqttAdapterOperationTimeout); err != nil {
			a.logger.WithFields(logrus.Fields{
				"topic": sharedTopic,
				"error": err,
			}).Error("Failed to subscribe gateway topic")
			return err
		}
		a.logger.WithFields(logrus.Fields{
			"topic":    sharedTopic,
			"describe": config.describe,
		}).Info("Subscribed to gateway topic")
	}

	return nil
}

// handleTelemetryMessage 处理遥测消息（MQTT 回调函数）。
// MQTT 回调层只做日志和错误记录，真实协议处理继续下沉到 HandleTelemetryMessage。
func (a *Adapter) handleTelemetryMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	a.logger.WithFields(logrus.Fields{
		"topic":        topic,
		"payload_size": len(payload),
	}).Debug("【设备遥测】Received telemetry message")

	// 直接调用 Adapter 的处理方法（会发送到 Bus）
	if err := a.HandleTelemetryMessage(payload, topic); err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Failed to handle telemetry message")
	}
}

// handleAttributeMessage 处理属性消息（MQTT 回调函数）。
func (a *Adapter) handleAttributeMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	a.logger.WithFields(logrus.Fields{
		"topic":        topic,
		"payload_size": len(payload),
	}).Debug("Received attribute message")

	// 调用处理方法并立即发送 ACK
	if err := a.HandleAttributeMessage(payload, topic); err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Failed to handle attribute message")
	}
}

// handleEventMessage 处理事件消息（MQTT 回调函数）。
func (a *Adapter) handleEventMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	a.logger.WithFields(logrus.Fields{
		"topic":        topic,
		"payload_size": len(payload),
	}).Debug("Received event message")

	// 调用处理方法并立即发送 ACK
	if err := a.HandleEventMessage(payload, topic); err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Failed to handle event message")
	}
}

// handleStatusMessage 处理状态消息（MQTT 回调函数）。
func (a *Adapter) handleStatusMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload, err := a.decodeStatusPayload(msg.Payload())

	a.logger.WithFields(logrus.Fields{
		"topic":   topic,
		"payload": string(msg.Payload()),
	}).Debug("【设备上下线】Received status message")
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Invalid status payload")
		return
	}

	// source = "status_message" 表示来自设备主动上报
	if err := a.HandleStatusMessage(payload, topic, "status_message"); err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Failed to handle status message")
	}
}

// decodeStatusPayload unwraps the broker's standard device uplink envelope.
// The broker rewrites every device publish into {device_id, values}, with the
// original bytes represented as base64 by encoding/json. StatusUplink parses
// only the inner 0/1 value, so passing the JSON envelope through would make a
// valid online heartbeat look malformed and leave the device offline.
func (a *Adapter) decodeStatusPayload(payload []byte) ([]byte, error) {
	statusPayload, err := a.verifyPayload(payload)
	if err != nil {
		return nil, err
	}
	return statusPayload.Values, nil
}
