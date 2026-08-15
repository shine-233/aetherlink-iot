// 文件用途：承载下行消息模块的 errors 逻辑。
// 核心逻辑：定义下行消息类型、发布订阅总线、处理器接口和 MQTT 发布处理流程。
// 关键注意事项：下行链路需保持消息类型、topic 构造和发布错误语义兼容。
// 重构建议：后续可把总线、处理器和发布器边界继续接口化，便于独立压测与替换。

package downlink

import "errors"

var (
	ErrInvalidMessage = errors.New("invalid message")
	ErrEncodeFailed   = errors.New("script encode failed")
	ErrPublishFailed  = errors.New("mqtt publish failed")
)
