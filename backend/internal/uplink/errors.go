// 文件用途：承载上行消息模块的 errors 处理逻辑。
// 核心逻辑：从 MQTT 或内部总线接收遥测、属性、事件、状态和响应消息并分发到处理、存储或通知链路。
// 关键注意事项：上行链路包含 goroutine、缓存和外部服务调用，修改需关注并发关闭和消息类型兼容。
// 重构建议：后续可拆分通用解析、自动化触发和副作用发送逻辑，提升可测试性。

package uplink

import "errors"

var (
	// ErrBusClosed Bus 已开始关闭或已经关闭，因此消息未被接受。
	ErrBusClosed = errors.New("bus is closed")

	// ErrUnknownMessageType 未知的消息类型
	ErrUnknownMessageType = errors.New("unknown message type")

	ErrChannelFull = errors.New("channel is full") // ✨ 新增

	// ErrUplinkStopped uplink 已停止
	ErrUplinkStopped = errors.New("uplink is stopped")

	// ErrProcessorFailed 数据处理失败
	ErrProcessorFailed = errors.New("processor failed")

	// ErrInvalidPayload 无效的 payload
	ErrInvalidPayload = errors.New("invalid payload")
)
