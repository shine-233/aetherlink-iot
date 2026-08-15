// 文件用途：封装平台向设备发布 MQTT 下行消息的能力。
// 核心逻辑：根据设备 topic 和 payload 调用 MQTT client 发布，并记录发布结果。
// 关键注意事项：下行 topic 与 QoS 是外部协议契约，修改需同步设备端和 broker 路由验证。
// 重构建议：可把 topic 构造、payload 序列化和发布错误处理拆开，便于独立测试。

package mqttadapter

import (
	"aetherlink-iot/backend/pkg/common"

	"github.com/sirupsen/logrus"
)

// publishAttributeResponse 发送属性上报 ACK 响应。
// 协议层行为：告诉设备“我收到了你的属性上报”；messageID 或 deviceNumber 缺失时直接跳过。
func (a *Adapter) publishAttributeResponse(deviceNumber, messageID string, err error) {
	if deviceNumber == "" || messageID == "" {
		a.logger.Debug("Skip attribute response: empty deviceNumber or messageID")
		return
	}

	// 构造响应 Topic
	topic := BuildAttributeResponseTopic(deviceNumber, messageID)

	// 构造响应 Payload
	payload := common.GetResponsePayload("", err)

	// 发布消息
	token := a.mqttClient.Publish(topic, 1, false, payload)
	if publishErr := waitMQTTToken(token, mqttAdapterOperationTimeout); publishErr != nil {
		a.logger.WithFields(logrus.Fields{
			"device_number": deviceNumber,
			"message_id":    messageID,
			"topic":         topic,
			"error":         publishErr,
		}).Error("Failed to publish attribute response")
	} else {
		a.logger.WithFields(logrus.Fields{
			"device_number": deviceNumber,
			"message_id":    messageID,
			"topic":         topic,
		}).Debug("Attribute response sent successfully")
	}
}

// publishEventResponse 发送事件上报 ACK 响应。
// 与属性 ACK 类似，这里主要承担协议确认职责，不做业务层成功与否的二次编排。
func (a *Adapter) publishEventResponse(deviceNumber, messageID, method string, err error) {
	if deviceNumber == "" || messageID == "" {
		a.logger.Debug("Skip event response: empty deviceNumber or messageID")
		return
	}

	// 构造响应 Topic
	topic := BuildEventResponseTopic(deviceNumber, messageID)

	// 构造响应 Payload（包含 method）
	payload := common.GetResponsePayload(method, err)

	// 发布消息
	token := a.mqttClient.Publish(topic, 1, false, payload)
	if publishErr := waitMQTTToken(token, mqttAdapterOperationTimeout); publishErr != nil {
		a.logger.WithFields(logrus.Fields{
			"device_number": deviceNumber,
			"message_id":    messageID,
			"method":        method,
			"topic":         topic,
			"error":         publishErr,
		}).Error("Failed to publish event response")
	} else {
		a.logger.WithFields(logrus.Fields{
			"device_number": deviceNumber,
			"message_id":    messageID,
			"method":        method,
			"topic":         topic,
		}).Debug("Event response sent successfully")
	}
}
