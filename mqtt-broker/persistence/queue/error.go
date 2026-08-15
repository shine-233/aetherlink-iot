// 文件用途：维护 persistence\queue\error.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package queue

import (
	"errors"
)

var (
	ErrClosed                   = errors.New("queue has been closed")
	ErrDropExceedsMaxPacketSize = errors.New("maximum packet size exceeded")
	ErrDropQueueFull            = errors.New("the message queue is full")
	ErrDropExpired              = errors.New("the message is expired")
	ErrDropExpiredInflight      = errors.New("the inflight message is expired")
)

// InternalError wraps the error of the backend storage.
type InternalError struct {
	// Err is the error return by the backend storage.
	Err error
}

func (i *InternalError) Error() string {
	return i.Err.Error()
}
