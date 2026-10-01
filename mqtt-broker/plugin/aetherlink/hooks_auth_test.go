// 文件用途：验证已认证客户端绑定表的两阶段生命周期与 pending 兜底回收，防止无界增长。
// 核心逻辑：注入 mqttClientBindingNow 推进时间，覆盖 pending 写入、OnConnected 转正、
// TTL 扫描只回收未建连条目、扫描节流、CAS 不复活已删除条目，以及 OnClosed panic 时仍回收。

package aetherlink

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/golang/mock/gomock"
)

func withMQTTBindingClock(t *testing.T, start time.Time) *time.Time {
	t.Helper()
	now := start
	prevNow := mqttClientBindingNow
	prevSweep := mqttPendingBindingLastSweepNanos.Load()
	mqttClientBindingNow = func() time.Time { return now }
	mqttPendingBindingLastSweepNanos.Store(0)
	t.Cleanup(func() {
		mqttClientBindingNow = prevNow
		mqttPendingBindingLastSweepNanos.Store(prevSweep)
	})
	return &now
}

func trackMQTTBindingClients(t *testing.T, clients ...server.Client) {
	t.Helper()
	t.Cleanup(func() {
		for _, c := range clients {
			mqttAuthenticatedClientBindings.Delete(c)
		}
	})
}

func rawMQTTBinding(t *testing.T, client server.Client) (mqttAuthenticatedClientBinding, bool) {
	t.Helper()
	value, ok := mqttAuthenticatedClientBindings.Load(client)
	if !ok {
		return mqttAuthenticatedClientBinding{}, false
	}
	binding, isBinding := value.(mqttAuthenticatedClientBinding)
	if !isBinding {
		t.Fatalf("unexpected binding type %T", value)
	}
	return binding, true
}

func TestMQTTAuthBindingStoredAsPendingThenPromoted(t *testing.T) {
	ctrl := gomock.NewController(t)
	now := withMQTTBindingClock(t, time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC))
	client := server.NewMockClient(ctrl)
	trackMQTTBindingClients(t, client)

	storePendingMQTTAuthenticatedClientBinding(client, mqttAuthenticatedClientBinding{deviceID: "dev-1"})
	binding, ok := rawMQTTBinding(t, client)
	if !ok || !binding.pendingSince.Equal(*now) {
		t.Fatalf("binding after auth = %#v ok=%v, want pending since %v", binding, ok, *now)
	}
	// pending 阶段查询依然可见，保持认证通过后到 OnConnected 之间的行为不变。
	if id, ok := mqttAuthenticatedDeviceForClient(client); !ok || id != "dev-1" {
		t.Fatalf("pending binding lookup = %q ok=%v", id, ok)
	}

	promoteMQTTAuthenticatedClientBinding(client)
	binding, ok = rawMQTTBinding(t, client)
	if !ok || !binding.pendingSince.IsZero() || binding.deviceID != "dev-1" {
		t.Fatalf("binding after promote = %#v ok=%v, want connected dev-1", binding, ok)
	}
}

func TestMQTTAuthPendingSweepReclaimsOnlyStaleUnconnectedBindings(t *testing.T) {
	ctrl := gomock.NewController(t)
	now := withMQTTBindingClock(t, time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC))
	abandoned := server.NewMockClient(ctrl) // 认证通过但建连失败：永远不会触发 OnClosed
	connected := server.NewMockClient(ctrl) // 正常在线的长连接
	fresh := server.NewMockClient(ctrl)     // 刚认证、仍在建连窗口内
	legacy := server.NewMockClient(ctrl)    // 旧实例写入的字符串绑定
	trackMQTTBindingClients(t, abandoned, connected, fresh, legacy)

	storePendingMQTTAuthenticatedClientBinding(abandoned, mqttAuthenticatedClientBinding{deviceID: "dev-a"})
	storePendingMQTTAuthenticatedClientBinding(connected, mqttAuthenticatedClientBinding{deviceID: "dev-c"})
	promoteMQTTAuthenticatedClientBinding(connected)
	mqttAuthenticatedClientBindings.Store(legacy, "dev-l")

	*now = now.Add(mqttPendingClientBindingTTL + time.Second)
	storePendingMQTTAuthenticatedClientBinding(fresh, mqttAuthenticatedClientBinding{deviceID: "dev-f"})

	if _, ok := mqttAuthenticatedBindingForClient(abandoned); ok {
		t.Fatal("stale pending binding of a never-connected client should be swept")
	}
	for name, c := range map[string]server.Client{"connected": connected, "fresh": fresh, "legacy": legacy} {
		if _, ok := mqttAuthenticatedBindingForClient(c); !ok {
			t.Fatalf("%s binding must survive the pending sweep", name)
		}
	}
}

func TestMQTTAuthPendingSweepIsThrottled(t *testing.T) {
	ctrl := gomock.NewController(t)
	start := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	withMQTTBindingClock(t, start)
	stale := server.NewMockClient(ctrl)
	trackMQTTBindingClients(t, stale)

	if got := maybeSweepStalePendingMQTTClientBindings(start); got != 0 {
		t.Fatalf("first sweep on empty state removed %d", got)
	}
	mqttAuthenticatedClientBindings.Store(stale, mqttAuthenticatedClientBinding{
		deviceID:     "dev-s",
		pendingSince: start.Add(-2 * mqttPendingClientBindingTTL),
	})
	if got := maybeSweepStalePendingMQTTClientBindings(start.Add(mqttPendingClientBindingSweepInterval / 2)); got != 0 {
		t.Fatalf("sweep inside throttle interval removed %d, want 0", got)
	}
	if _, ok := mqttAuthenticatedBindingForClient(stale); !ok {
		t.Fatal("throttled sweep must not touch the table")
	}
	if got := maybeSweepStalePendingMQTTClientBindings(start.Add(mqttPendingClientBindingSweepInterval)); got != 1 {
		t.Fatalf("sweep after interval removed %d, want 1", got)
	}
}

func TestMQTTAuthPromoteDoesNotResurrectForgottenBinding(t *testing.T) {
	ctrl := gomock.NewController(t)
	withMQTTBindingClock(t, time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC))
	client := server.NewMockClient(ctrl)
	trackMQTTBindingClients(t, client)

	storePendingMQTTAuthenticatedClientBinding(client, mqttAuthenticatedClientBinding{deviceID: "dev-1"})
	forgetMQTTAuthenticatedClientBinding(client) // 例如会话吊销抢先删除
	promoteMQTTAuthenticatedClientBinding(client)
	if _, ok := mqttAuthenticatedClientBindings.Load(client); ok {
		t.Fatal("promote must not re-create a binding that was already forgotten")
	}
}

func TestMQTTAuthReauthKeepsConnectedBindingOutOfSweep(t *testing.T) {
	ctrl := gomock.NewController(t)
	now := withMQTTBindingClock(t, time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC))
	client := server.NewMockClient(ctrl)
	trackMQTTBindingClients(t, client)

	storePendingMQTTAuthenticatedClientBinding(client, mqttAuthenticatedClientBinding{deviceID: "dev-1"})
	promoteMQTTAuthenticatedClientBinding(client)
	// 同一连接对象再次走认证（如 v5 重认证），不能退回 pending 后被 TTL 回收。
	storePendingMQTTAuthenticatedClientBinding(client, mqttAuthenticatedClientBinding{deviceID: "dev-1"})

	*now = now.Add(mqttPendingClientBindingTTL + mqttPendingClientBindingSweepInterval)
	if got := sweepStalePendingMQTTClientBindings(*now); got != 0 {
		t.Fatalf("sweep removed %d connected bindings, want 0", got)
	}
	if _, ok := mqttAuthenticatedBindingForClient(client); !ok {
		t.Fatal("connected binding must survive re-authentication and sweep")
	}
}

func TestMQTTAuthOnClosedForgetsBindingEvenWhenInnerHookPanics(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	trackMQTTBindingClients(t, client)
	mqttAuthenticatedClientBindings.Store(client, mqttAuthenticatedClientBinding{deviceID: "dev-1"})

	plugin := &AetherLinkPlugin{}
	closed := plugin.OnClosedWrapper(func(context.Context, server.Client, error) {
		panic("inner OnClosed hook failed")
	})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected inner hook panic to propagate")
			}
		}()
		closed(context.Background(), client, errors.New("boom"))
	}()
	if _, ok := mqttAuthenticatedClientBindings.Load(client); ok {
		t.Fatal("binding must be forgotten even when an inner OnClosed hook panics")
	}
}
