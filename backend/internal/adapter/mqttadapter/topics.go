// 文件用途：集中定义 MQTT adapter 使用的设备协议 topic。
// 核心逻辑：生成遥测、属性、事件、状态和下行响应相关的 topic 模板。
// 关键注意事项：topic 字符串是设备接入协议边界，修改需同步设备文档、broker 订阅和自动化测试。
// 重构建议：可为每个 topic 模板增加正反向解析表驱动测试，减少路径参数漂移。

package mqttadapter

import "fmt"

// Topic templates are the device wire protocol. Keep these values synchronized
// with device documentation and broker subscriptions before changing them.

// 上行 Topic 模板（设备 → 平台）
const (
	// TopicPatternTelemetry 遥测数据上报 Topic 模式
	// 格式: devices/telemetry (不使用通配符，订阅所有遥测数据)
	TopicPatternTelemetry = "devices/telemetry"

	// TopicPatternAttribute 属性上报 Topic 模式
	// 格式: devices/attributes/{message_id}
	TopicPatternAttribute = "devices/attributes/+"

	// TopicPatternEvent 事件上报 Topic 模式
	// 格式: devices/event/{message_id}
	TopicPatternEvent = "devices/event/+"

	// TopicPatternStatus 状态上报 Topic 模式
	// 格式: devices/status/{device_id}
	TopicPatternStatus = "devices/status/+"

	// TopicPatternOTAProgress OTA升级进度上报 Topic 模式
	// 格式: ota/devices/progress
	TopicPatternOTAProgress = "ota/devices/progress"

	// 网关 Topic 模式（前缀为 gateway/）
	TopicPatternGatewayTelemetry = "gateway/telemetry"
	TopicPatternGatewayAttribute = "gateway/attributes/+"
	TopicPatternGatewayEvent     = "gateway/event/+"
)

// 下行 Topic 模板（平台 → 设备）
const (
	// TopicTemplateAttributeSet 属性设置 Topic 模板
	// 第一个 %s = device_number，第二个 %s = message_id
	TopicTemplateAttributeSet = "devices/attributes/set/%s/%s"

	// TopicTemplateAttributeGet 属性查询 Topic 模板
	// %s = device_number（无 message_id）
	TopicTemplateAttributeGet = "devices/attributes/get/%s"

	// TopicTemplateCommand 命令下发 Topic 模板
	// 第一个 %s = device_number，第二个 %s = message_id
	TopicTemplateCommand = "devices/command/%s/%s"

	// TopicTemplateTelemetryControl 遥测控制 Topic 模板
	// %s = device_number（无 message_id）
	TopicTemplateTelemetryControl = "devices/telemetry/control/%s"

	// 网关下行 Topic 模板
	TopicTemplateGatewayAttributeSet     = "gateway/attributes/set/%s/%s" // gateway_number + message_id
	TopicTemplateGatewayAttributeGet     = "gateway/attributes/get/%s"    // gateway_number
	TopicTemplateGatewayCommand          = "gateway/command/%s/%s"        // gateway_number + message_id
	TopicTemplateGatewayTelemetryControl = "gateway/telemetry/control/%s" // gateway_number
)

// 响应 Topic 模板（设备 → 平台的 ACK）
const (
	// TopicTemplateAttributeResponse 属性上报响应 Topic 模板
	// 参数: device_number, message_id
	TopicTemplateAttributeResponse = "devices/attributes/response/%s/%s"

	// TopicTemplateEventResponse 事件上报响应 Topic 模板
	// 参数: device_number, message_id
	TopicTemplateEventResponse = "devices/event/response/%s/%s"

	// 命令/属性设置响应订阅模式
	TopicPatternCommandResponse             = "devices/command/response/+"
	TopicPatternAttributeSetResponse        = "devices/attributes/set/response/+"
	TopicPatternGatewayCommandResponse      = "gateway/command/response/+"
	TopicPatternGatewayAttributeSetResponse = "gateway/attributes/set/response/+"
)

// BuildAttributeResponseTopic 构造属性上报响应 Topic
func BuildAttributeResponseTopic(deviceNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateAttributeResponse, deviceNumber, messageID)
}

// BuildEventResponseTopic 构造事件上报响应 Topic
func BuildEventResponseTopic(deviceNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateEventResponse, deviceNumber, messageID)
}

// BuildAttributeSetTopic 构造属性设置 Topic
func BuildAttributeSetTopic(deviceNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateAttributeSet, deviceNumber, messageID)
}

// BuildAttributeGetTopic 构造属性查询 Topic
func BuildAttributeGetTopic(deviceNumber string) string {
	return fmt.Sprintf(TopicTemplateAttributeGet, deviceNumber)
}

// BuildCommandTopic 构造命令下发 Topic
func BuildCommandTopic(deviceNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateCommand, deviceNumber, messageID)
}

// BuildTelemetryControlTopic 构造遥测控制 Topic
func BuildTelemetryControlTopic(deviceNumber string) string {
	return fmt.Sprintf(TopicTemplateTelemetryControl, deviceNumber)
}

// BuildGatewayAttributeSetTopic 构造网关属性设置 Topic
func BuildGatewayAttributeSetTopic(gatewayNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateGatewayAttributeSet, gatewayNumber, messageID)
}

// BuildGatewayAttributeGetTopic 构造网关属性查询 Topic
func BuildGatewayAttributeGetTopic(gatewayNumber string) string {
	return fmt.Sprintf(TopicTemplateGatewayAttributeGet, gatewayNumber)
}

// BuildGatewayCommandTopic 构造网关命令下发 Topic
func BuildGatewayCommandTopic(gatewayNumber, messageID string) string {
	return fmt.Sprintf(TopicTemplateGatewayCommand, gatewayNumber, messageID)
}

// BuildGatewayTelemetryControlTopic 构造网关遥测控制 Topic
func BuildGatewayTelemetryControlTopic(gatewayNumber string) string {
	return fmt.Sprintf(TopicTemplateGatewayTelemetryControl, gatewayNumber)
}
