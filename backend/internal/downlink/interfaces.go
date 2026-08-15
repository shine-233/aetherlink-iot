// 文件用途：承载下行消息模块的 interfaces 逻辑。
// 核心逻辑：定义下行消息类型、发布订阅总线、处理器接口和 MQTT 发布处理流程，主要围绕 type MessagePublisher 等声明展开。
// 关键注意事项：下行链路需保持消息类型、topic 构造和发布错误语义兼容。
// 重构建议：后续可把总线、处理器和发布器边界继续接口化，便于独立压测与替换。

package downlink

// MessagePublisher 消息发布接口（协议无关）
// 实现可以是：MQTT、Kafka、AMQP 等
type MessagePublisher interface {
	// PublishMessage 发布消息到设备
	// deviceNumber: 目标设备编号
	// msgType: 消息类型（用于选择Topic路径）
	// deviceType: 设备类型（用于区分devices/*还是gateway/*）
	// topicPrefix: Topic前缀（协议插件使用，MQTT为空）
	// messageID: 消息唯一标识（命令/属性设置需要拼接到Topic）
	// qos: 消息质量等级
	// payload: 消息内容（字节流）
	PublishMessage(deviceNumber string, msgType MessageType, deviceType string, topicPrefix string, messageID string, qos byte, payload []byte) error
}
