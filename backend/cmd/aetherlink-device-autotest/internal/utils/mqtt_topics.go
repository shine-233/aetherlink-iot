// 文件用途：集中生成设备自动测试使用的 MQTT 发布与订阅主题。
// 核心逻辑：根据直连或网关模式返回遥测、属性、事件、命令、OTA 和平台下行响应相关 topic。
// 关键注意事项：topic 字符串是外部协议契约，改动会同时影响设备实现、API 下行测试和 broker/平台兼容性。
// 重构建议：可将 topic 模板沉淀为表驱动契约测试，并为直连和网关模式增加正反向解析能力。

package utils

import "fmt"

// MQTTTopics MQTT主题构建器
type MQTTTopics struct {
	deviceNumber string
	isGateway    bool // 是否为网关设备
}

// NewMQTTTopics 创建主题构建器（直连设备）
func NewMQTTTopics(deviceNumber string) *MQTTTopics {
	return &MQTTTopics{
		deviceNumber: deviceNumber,
		isGateway:    false,
	}
}

// NewGatewayMQTTTopics 创建网关主题构建器
func NewGatewayMQTTTopics(deviceNumber string) *MQTTTopics {
	return &MQTTTopics{
		deviceNumber: deviceNumber,
		isGateway:    true,
	}
}

// 设备上报主题。直连和网关除了前缀不同，其余 message_id 组织方式保持平行结构。
func (t *MQTTTopics) Telemetry() string {
	if t.isGateway {
		return "gateway/telemetry"
	}
	return "devices/telemetry"
}

func (t *MQTTTopics) Attributes(messageID string) string {
	if t.isGateway {
		return fmt.Sprintf("gateway/attributes/%s", messageID)
	}
	return fmt.Sprintf("devices/attributes/%s", messageID)
}

func (t *MQTTTopics) Event(messageID string) string {
	if t.isGateway {
		return fmt.Sprintf("gateway/event/%s", messageID)
	}
	return fmt.Sprintf("devices/event/%s", messageID)
}

// Status returns the direct-device status topic consumed by the backend
// status uplink. The status topic carries the stable device ID (not the
// device number used by downlink topics), so callers must pass the ID that
// the platform uses for cache and persistence lookups.
func (t *MQTTTopics) Status(deviceID string) string {
	if t.isGateway {
		return fmt.Sprintf("gateway/status/%s", deviceID)
	}
	return fmt.Sprintf("devices/status/%s", deviceID)
}

func (t *MQTTTopics) CommandResponse(messageID string) string {
	if t.isGateway {
		return fmt.Sprintf("gateway/command/response/%s", messageID)
	}
	return fmt.Sprintf("devices/command/response/%s", messageID)
}

func (t *MQTTTopics) AttributeSetResponse(messageID string) string {
	if t.isGateway {
		return fmt.Sprintf("gateway/attributes/set/response/%s", messageID)
	}
	return fmt.Sprintf("devices/attributes/set/response/%s", messageID)
}

func (t *MQTTTopics) OTAProgress() string {
	return "ota/devices/progress"
}

// 设备订阅主题。`+` 通配符意味着平台会把 message_id 放在最后一层供测试捕获。
func (t *MQTTTopics) TelemetryControl() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/telemetry/control/%s", t.deviceNumber)
	}
	return fmt.Sprintf("devices/telemetry/control/%s", t.deviceNumber)
}

func (t *MQTTTopics) AttributeSet() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/attributes/set/%s/+", t.deviceNumber)
	}
	return fmt.Sprintf("devices/attributes/set/%s/+", t.deviceNumber)
}

func (t *MQTTTopics) AttributeGet() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/attributes/get/%s", t.deviceNumber)
	}
	return fmt.Sprintf("devices/attributes/get/%s", t.deviceNumber)
}

func (t *MQTTTopics) Command() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/command/%s/+", t.deviceNumber)
	}
	return fmt.Sprintf("devices/command/%s/+", t.deviceNumber)
}

func (t *MQTTTopics) AttributeResponse() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/attributes/response/%s/+", t.deviceNumber)
	}
	return fmt.Sprintf("devices/attributes/response/%s/+", t.deviceNumber)
}

func (t *MQTTTopics) EventResponse() string {
	if t.isGateway {
		return fmt.Sprintf("gateway/event/response/%s/+", t.deviceNumber)
	}
	return fmt.Sprintf("devices/event/response/%s/+", t.deviceNumber)
}

func (t *MQTTTopics) OTAInform() string {
	return fmt.Sprintf("ota/devices/inform/%s", t.deviceNumber)
}
