// 文件用途：维护 retained\interface.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package retained

import (
	"github.com/DrmagicE/gmqtt"
)

// IterateFn is the callback function used by iterate()
// Return false means to stop the iteration.
type IterateFn func(message *gmqtt.Message) bool

// Store is the interface used by gmqtt.server and external logic to handler the operations of retained messages.
// User can get the implementation from gmqtt.Server interface.
// This interface provides the ability for extensions to interact with the retained message store.
// Notice:
// This methods will not trigger any gmqtt hooks.
type Store interface {
	// GetRetainedMessage returns the message that equals the passed topic.
	GetRetainedMessage(topicName string) *gmqtt.Message
	// ClearAll clears all retained messages.
	ClearAll()
	// AddOrReplace adds or replaces a retained message.
	AddOrReplace(message *gmqtt.Message)
	// remove removes a retained message.
	Remove(topicName string)
	// GetMatchedMessages returns the retained messages that match the passed topic filter.
	GetMatchedMessages(topicFilter string) []*gmqtt.Message
	// Iterate iterate all retained messages. The callback is called once for each message.
	// If callback return false, the iteration will be stopped.
	// Notice:
	// The results are not sorted in any way, no ordering of any kind is guaranteed.
	// This method will walk through all retained messages,
	// so this will be a expensive operation if there are a large number of retained messages.
	Iterate(fn IterateFn)
}
