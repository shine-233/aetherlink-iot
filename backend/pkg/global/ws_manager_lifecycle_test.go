// 文件用途：验证 WebSocket 管理器 Redis Pub/Sub 监听循环的启停语义。
// 核心逻辑：注入假 Pub/Sub 接收器，用 channel 同步信号断言监听协程进入、幂等守卫和 Stop 后确定性退出。
// 关键注意事项：测试不连接真实 Redis 或 WebSocket，全部等待均有超时上限，避免 flaky 的 sleep 断言。
package global

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const lifecycleTestTimeout = 2 * time.Second

// fakePubSubReceiver 模拟一个已建立订阅的 Pub/Sub 接收器：
// 首次 ReceiveMessage 时广播“已进入接收循环”，随后阻塞直到 ctx 取消。
type fakePubSubReceiver struct {
	entered    chan struct{}
	enterOnce  atomic.Bool
	closeCount atomic.Int32
	pattern    string
}

func (f *fakePubSubReceiver) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	if f.enterOnce.CompareAndSwap(false, true) {
		close(f.entered)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakePubSubReceiver) Close() error {
	f.closeCount.Add(1)
	return nil
}

func waitForChannel[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(lifecycleTestTimeout):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

func TestWSManagerListenForEventsStopsOnStopListen(t *testing.T) {
	receiver := &fakePubSubReceiver{entered: make(chan struct{}), pattern: "ws:device:*"}
	manager := NewWSManager()
	manager.pubSubFactory = func(_ context.Context, pattern string) wsPubSubReceiver {
		if pattern != receiver.pattern {
			t.Errorf("subscribe pattern = %q, want %q", pattern, receiver.pattern)
		}
		return receiver
	}

	listenExited := make(chan struct{})
	go func() {
		defer close(listenExited)
		manager.ListenForEvents()
	}()

	waitForChannel(t, receiver.entered, "listener entering receive loop")
	manager.StopListen()
	waitForChannel(t, listenExited, "ListenForEvents exiting after StopListen")

	if got := receiver.closeCount.Load(); got != 1 {
		t.Fatalf("pubsub close count = %d, want 1", got)
	}
}

func TestWSManagerListenForEventsSecondCallWhileRunningIsNoOp(t *testing.T) {
	receiver := &fakePubSubReceiver{entered: make(chan struct{})}
	manager := NewWSManager()
	manager.pubSubFactory = func(context.Context, string) wsPubSubReceiver {
		return receiver
	}

	listenExited := make(chan struct{})
	go func() {
		defer close(listenExited)
		manager.ListenForEvents()
	}()

	waitForChannel(t, receiver.entered, "listener entering receive loop")

	// 运行中重复调用必须被幂等守卫拦截：立即返回且不再创建第二个接收器，
	// 否则会叠加第二个监听协程导致同一事件被投递两次。
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.ListenForEvents()
	}()
	waitForChannel(t, done, "duplicate ListenForEvents returning immediately")

	manager.StopListen()
	waitForChannel(t, listenExited, "ListenForEvents exiting after StopListen")

	if got := receiver.closeCount.Load(); got != 1 {
		t.Fatalf("pubsub close count = %d, want exactly one receiver created and closed", got)
	}
}

func TestWSManagerStopListenWithoutStartIsNoOp(t *testing.T) {
	manager := NewWSManager()

	// 未启动时 Stop 必须是安全的空操作，不允许 panic 或阻塞。
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.StopListen()
	}()
	waitForChannel(t, done, "StopListen returning without start")

	manager.StopListen() // 重复调用同样必须安全。
}
