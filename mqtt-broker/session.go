// 文件用途：维护 session.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package gmqtt

import (
	"time"
)

// Session represents a MQTT session.
type Session struct {
	// ClientID represents the client id.
	ClientID string
	// Will is the will message of the client, can be nil if there is no will message.
	Will *Message
	// WillDelayInterval represents the Will Delay Interval in seconds
	WillDelayInterval uint32
	// ConnectedAt is the session create time.
	ConnectedAt time.Time
	// ExpiryInterval represents the Session Expiry Interval in seconds
	ExpiryInterval uint32
}

// IsExpired return whether the session is expired
func (s *Session) IsExpired(now time.Time) bool {
	return s.ConnectedAt.Add(time.Duration(s.ExpiryInterval) * time.Second).Before(now)
}
