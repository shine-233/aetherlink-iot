// 文件用途：定义设备自动测试工具的统一设备接口与接收消息模型。
// 核心逻辑：抽象直连设备和网关设备共同具备的 MQTT 连接、上报、响应、订阅和消息读取能力。
// 关键注意事项：该接口是测试用例与具体设备实现之间的边界，新增设备类型应优先保持调用方依赖接口。
// 重构建议：可把命令响应、属性响应和消息缓存能力拆成更小接口，便于按测试场景注入最小依赖。

/*
Purpose: 定义设备自动测试工具的统一设备接口与接收消息模型。
Core logic: 抽象直连设备和网关设备共同具备的 MQTT 连接、上报、响应、订阅和消息读取能力。
Important notes: 该接口是测试用例与具体设备实现之间的边界，新增设备类型时应优先保持调用方依赖接口而不是具体结构体。
Refactor suggestion: 后续可把命令响应、属性响应、消息缓存能力拆成更小接口，便于按测试场景注入最小依赖。
*/
package device

import (
	"time"
)

// Device 设备接口，定义所有设备类型的统一行为
type Device interface {
	// Connect 连接到MQTT Broker
	Connect() error

	// Disconnect 断开连接
	Disconnect()

	// IsConnected 检查连接状态
	IsConnected() bool

	// PublishTelemetry 上报遥测数据
	PublishTelemetry(data interface{}) error

	// PublishAttribute 上报属性数据
	PublishAttribute(data interface{}, messageID string) error

	// PublishEvent 上报事件数据
	PublishEvent(method string, params interface{}, messageID string) error

	// PublishCommandResponse 发送命令响应
	PublishCommandResponse(messageID string, success bool, method string) error

	// PublishAttributeSetResponse 发送属性设置响应
	PublishAttributeSetResponse(messageID string, success bool) error

	// SubscribeAll 统一订阅平台可能下发的控制/响应主题，避免测试前逐项拼装订阅逻辑。
	SubscribeAll() error

	// GetReceivedMessages 按 topic 模板轮询消息缓存，供集成测试等待平台下行或响应回流。
	GetReceivedMessages(topicPattern string, timeout time.Duration) []ReceivedMessage

	// ClearReceivedMessages 清理指定模式或全部缓存，避免历史消息污染后续断言。
	ClearReceivedMessages(topicPattern string)
}

// ReceivedMessage 接收到的消息
type ReceivedMessage struct {
	Topic     string    // MQTT 原始主题
	Payload   []byte    // MQTT 原始报文
	Timestamp time.Time // 本地接收时间，用于调试时序问题
}
