// 文件用途：维护 persistence\unack\unack.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package unack

import (
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// Store represents a unack store for one client.
// Unack store is used to persist the unacknowledged qos2 messages.
type Store interface {
	// Init will be called when the client connect.
	// If cleanStart set to true, the implementation should remove any associated data in backend store.
	// If it set to false, the implementation should retrieve the associated data from backend store.
	Init(cleanStart bool) error
	// Set sets the given id into store.
	// The return boolean indicates whether the id exist.
	Set(id packets.PacketID) (bool, error)
	// Remove removes the given id from store.
	Remove(id packets.PacketID) error
}
