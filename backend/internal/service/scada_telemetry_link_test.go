package service

import (
	"testing"
	"time"
)

func TestTelemetryLinkRejectsIllegalTransitions(t *testing.T) {
	base := time.Date(2026, 9, 11, 22, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		state string
		event string
		want  bool
	}{
		{name: "idle can start", state: LinkStateIdle, event: LinkEventConnectStart, want: true},
		{name: "idle cannot receive message", state: LinkStateIdle, event: LinkEventMessage, want: false},
		{name: "connecting can succeed", state: LinkStateConnecting, event: LinkEventConnectOK, want: true},
		{name: "connecting can fail", state: LinkStateConnecting, event: LinkEventConnectFail, want: true},
		{name: "connecting cannot go idle", state: LinkStateConnecting, event: LinkEventClose, want: true},
		{name: "connected can receive message", state: LinkStateConnected, event: LinkEventMessage, want: true},
		{name: "connected can timeout", state: LinkStateConnected, event: LinkEventTimeout, want: true},
		{name: "connected cannot start again", state: LinkStateConnected, event: LinkEventConnectStart, want: false},
		{name: "disconnected can reconnect", state: LinkStateDisconnected, event: LinkEventConnectStart, want: true},
		{name: "disconnected cannot receive message", state: LinkStateDisconnected, event: LinkEventMessage, want: false},
		{name: "reconnecting can succeed", state: LinkStateReconnecting, event: LinkEventConnectOK, want: true},
		{name: "error can restart", state: LinkStateError, event: LinkEventConnectStart, want: true},
		{name: "error cannot receive message", state: LinkStateError, event: LinkEventMessage, want: false},
		{name: "unknown state rejected", state: "bogus", event: LinkEventMessage, want: false},
		{name: "unknown event rejected", state: LinkStateConnected, event: "bogus", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanApply(tc.state, tc.event); got != tc.want {
				t.Fatalf("CanApply(%q, %q) = %v, want %v", tc.state, tc.event, got, tc.want)
			}
		})
	}

	// 非法迁移不得改变状态。
	link := &TelemetryLink{State: LinkStateConnected, Since: base}
	if err := link.Apply(LinkEventConnectStart, base); err != ErrIllegalLinkTransition {
		t.Fatalf("illegal transition error = %v, want %v", err, ErrIllegalLinkTransition)
	}
	if link.State != LinkStateConnected {
		t.Fatalf("illegal transition mutated state to %q", link.State)
	}
}

// TestTelemetryLinkStaleAfterDisconnect 锁定本状态机的核心：断线后数据一律判为陈旧。
// 断线瞬间即使最后一帧"刚刚到达"，也不能当作实时值——那正是本项目要消灭的假成功。
func TestTelemetryLinkStaleAfterDisconnect(t *testing.T) {
	base := time.Date(2026, 9, 11, 22, 0, 0, 0, time.UTC)
	link := NewTelemetryLink(base)

	if !link.IsStale(base, 30*time.Second) {
		t.Fatal("idle link must report stale")
	}

	steps := []struct {
		event string
		at    time.Time
	}{
		{LinkEventConnectStart, base},
		{LinkEventConnectOK, base.Add(time.Second)},
		{LinkEventMessage, base.Add(2 * time.Second)},
	}
	for _, s := range steps {
		if err := link.Apply(s.event, s.at); err != nil {
			t.Fatalf("apply %s: %v", s.event, err)
		}
	}
	if !link.IsLive() {
		t.Fatal("link should be live after connect_ok + message")
	}
	if link.IsStale(base.Add(3*time.Second), 30*time.Second) {
		t.Fatal("fresh message within silence window must not be stale")
	}
	if !link.IsStale(base.Add(40*time.Second), 30*time.Second) {
		t.Fatal("silence beyond threshold must be stale even while connected")
	}

	// 断开后：无论最后一帧多新，都是陈旧。
	if err := link.Apply(LinkEventTimeout, base.Add(4*time.Second)); err != nil {
		t.Fatalf("timeout: %v", err)
	}
	if link.State != LinkStateDisconnected {
		t.Fatalf("state after timeout = %q, want %q", link.State, LinkStateDisconnected)
	}
	if link.IsLive() {
		t.Fatal("disconnected link must not be live")
	}
	if !link.IsStale(base.Add(4*time.Second), 30*time.Second) {
		t.Fatal("disconnected link must report stale even with a very recent last message")
	}
	if link.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1", link.ConsecutiveFailures)
	}
}

func TestTelemetryLinkConnectedWithoutMessageIsStale(t *testing.T) {
	base := time.Date(2026, 9, 11, 22, 0, 0, 0, time.UTC)
	link := NewTelemetryLink(base)
	_ = link.Apply(LinkEventConnectStart, base)
	_ = link.Apply(LinkEventConnectOK, base)

	// 连上了但一帧都没到：没有数据可展示，不能当作"已有实时数据"。
	if link.LastMessageAt != nil {
		t.Fatal("no message should have been recorded")
	}
	if !link.IsStale(base, 30*time.Second) {
		t.Fatal("connected but never received a message must be stale")
	}
}

func TestTelemetryLinkReconnectFromDisconnected(t *testing.T) {
	base := time.Date(2026, 9, 11, 22, 0, 0, 0, time.UTC)
	link := &TelemetryLink{State: LinkStateDisconnected, Since: base}
	if err := link.Apply(LinkEventConnectStart, base); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if link.State != LinkStateReconnecting {
		t.Fatalf("state = %q, want %q (reconnect must be distinguishable from first connect)", link.State, LinkStateReconnecting)
	}
	if err := link.Apply(LinkEventConnectOK, base.Add(time.Second)); err != nil {
		t.Fatalf("connect ok: %v", err)
	}
	if link.State != LinkStateConnected || link.ConsecutiveFailures != 0 {
		t.Fatalf("after reconnect success: state=%q failures=%d, want connected/0", link.State, link.ConsecutiveFailures)
	}
}
