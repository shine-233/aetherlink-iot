// 文件用途：承载下行消息模块的 types 逻辑。
// 核心逻辑：定义下行消息类型、发布订阅总线、处理器接口和 MQTT 发布处理流程，主要围绕 type MessageType、type Message 等声明展开。
// 关键注意事项：下行链路需保持消息类型、topic 构造和发布错误语义兼容。
// 重构建议：后续可把总线、处理器和发布器边界继续接口化，便于独立压测与替换。

package downlink

import "encoding/json"

// MessageType 下行消息类型
type MessageType string

const (
	MessageTypeCommand      MessageType = "command"       // 命令下发
	MessageTypeAttributeSet MessageType = "attribute_set" // 属性设置
	MessageTypeAttributeGet MessageType = "attribute_get" // 属性获取
	MessageTypeTelemetry    MessageType = "telemetry"     // 遥测数据下发
)

// Message 下行消息（Service 传递给 downlink 的数据）
type Message struct {
	DeviceID       string          // 设备 ID（Service 层已处理网关子设备 ID）
	DeviceNumber   string          // 目标设备编号（网关/子设备时为顶层网关编号）
	DeviceType     string          // 设备类型："1"=直连,"2"=网关,"3"=子设备
	DeviceConfigID string          // 设备配置 ID（用于查找脚本）
	Type           MessageType     // 消息类型
	Data           json.RawMessage // 标准化数据（JSON 格式）
	TopicPrefix    string          // 协议插件Topic前缀（MQTT为空）
	MessageID      string          // 消息 ID（用于日志关联）
}
