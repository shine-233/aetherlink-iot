// 文件用途：维护 persistence\session\session.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package session

import (
	"github.com/DrmagicE/gmqtt"
)

// IterateFn is the callback function used by Iterate()
// Return false means to stop the iteration.
type IterateFn func(session *gmqtt.Session) bool

type Store interface {
	Set(session *gmqtt.Session) error
	Remove(clientID string) error
	Get(clientID string) (*gmqtt.Session, error)
	Iterate(fn IterateFn) error
	SetSessionExpiry(clientID string, expiry uint32) error
}
