// 文件用途：SCADA 遥测链路状态机（ROADMAP P1.3 门禁"遥测断线有状态"）。
// 核心逻辑：把链路的连接生命周期显式建模，并据此判定画布上的数据是否可信。
//
// 关键注意事项（这条状态机存在的唯一理由）：
//  1. **未连接时数据一律判为陈旧**。断线后继续把最后一帧当实时值显示，
//     是最典型的假成功——界面看着"有数据且在跳动"，实际上读数可能已经停了十分钟。
//     因此 IsStale 在 disconnected/error/reconnecting 下恒为 true，
//     不看 LastMessageAt 有多近：刚断 1 秒的数据同样不是实时数据。
//  2. 只有 connected 且静默时长未超阈值，才算新鲜。
//  3. 非法状态迁移直接拒绝，不做"宽容处理"——静默忽略异常事件会把
//     重复连接/重复断开这类真实故障掩盖掉。
package service

import (
	"errors"
	"time"
)

// 链路状态。
const (
	LinkStateIdle         = "idle"
	LinkStateConnecting   = "connecting"
	LinkStateConnected    = "connected"
	LinkStateDisconnected = "disconnected"
	LinkStateReconnecting = "reconnecting"
	LinkStateError        = "error"
)

// 链路事件。
const (
	LinkEventConnectStart = "connect_start"
	LinkEventConnectOK    = "connect_ok"
	LinkEventConnectFail  = "connect_fail"
	LinkEventMessage      = "message"
	LinkEventTimeout      = "timeout"
	LinkEventClose        = "close"
)

// ErrIllegalLinkTransition 非法状态迁移。
var ErrIllegalLinkTransition = errors.New("illegal telemetry link transition")

// telemetryLinkTransitions 合法迁移表。
var telemetryLinkTransitions = map[string]map[string]bool{
	LinkStateIdle:         {LinkEventConnectStart: true},
	LinkStateConnecting:   {LinkEventConnectOK: true, LinkEventConnectFail: true, LinkEventClose: true},
	LinkStateConnected:    {LinkEventMessage: true, LinkEventTimeout: true, LinkEventClose: true, LinkEventConnectFail: true},
	LinkStateDisconnected: {LinkEventConnectStart: true, LinkEventClose: true},
	LinkStateReconnecting: {LinkEventConnectStart: true, LinkEventConnectOK: true, LinkEventConnectFail: true, LinkEventClose: true},
	LinkStateError:        {LinkEventConnectStart: true, LinkEventClose: true},
}

// TelemetryLink 一条遥测链路的运行时状态。
type TelemetryLink struct {
	State               string
	Since               time.Time
	LastMessageAt       *time.Time
	ConsecutiveFailures int
}

// NewTelemetryLink 创建初始（idle）链路。
func NewTelemetryLink(now time.Time) *TelemetryLink {
	return &TelemetryLink{State: LinkStateIdle, Since: now}
}

// CanApply 判断该事件在当前状态下是否合法。
func CanApply(state, event string) bool {
	allowed, ok := telemetryLinkTransitions[state]
	if !ok {
		return false
	}
	return allowed[event]
}

// Apply 施加一次事件，非法迁移返回错误且不改变状态。
func (l *TelemetryLink) Apply(event string, at time.Time) error {
	if l == nil {
		return ErrIllegalLinkTransition
	}
	if !CanApply(l.State, event) {
		return ErrIllegalLinkTransition
	}
	switch event {
	case LinkEventConnectStart:
		if l.State == LinkStateDisconnected || l.State == LinkStateError {
			l.State = LinkStateReconnecting
		} else {
			l.State = LinkStateConnecting
		}
		l.Since = at
	case LinkEventConnectOK:
		l.State = LinkStateConnected
		l.Since = at
		l.ConsecutiveFailures = 0
	case LinkEventConnectFail:
		l.ConsecutiveFailures++
		l.State = LinkStateDisconnected
		l.Since = at
	case LinkEventMessage:
		l.State = LinkStateConnected
		l.LastMessageAt = &at
	case LinkEventTimeout:
		// 超时视为一次失败：不自动重连，重连必须由显式 connect_start 触发，
		// 否则"超时即重连"会让故障恢复动作在审计里无迹可寻。
		l.ConsecutiveFailures++
		l.State = LinkStateDisconnected
		l.Since = at
	case LinkEventClose:
		l.State = LinkStateDisconnected
		l.Since = at
	}
	return nil
}

// IsLive 判断链路当前是否处于已连接状态。
func (l *TelemetryLink) IsLive() bool {
	return l != nil && l.State == LinkStateConnected
}

// IsStale 判断画布上的数据是否陈旧（不可当作实时值展示）。
//
// 未连接时恒为 true，且**刻意不参考 LastMessageAt**：
// 断开状态下最后一帧有多新都不改变"链路已断"这一事实，
// 把它当实时值显示就是本项目要消灭的假成功。
func (l *TelemetryLink) IsStale(now time.Time, maxSilence time.Duration) bool {
	if l == nil || !l.IsLive() {
		return true
	}
	if maxSilence <= 0 {
		maxSilence = 30 * time.Second
	}
	if l.LastMessageAt == nil {
		// 连接着但一帧都还没到：尚无数据可展示，不能当作已有数据。
		return true
	}
	return now.Sub(*l.LastMessageAt) > maxSilence
}
