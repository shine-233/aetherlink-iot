// 文件用途：验证 SSE 管理器 Redis Pub/Sub 监听循环的启停语义。
// 核心逻辑：注入假 Pub/Sub 接收器，用 channel 同步信号断言监听协程进入、幂等守卫和 Stop 后确定性退出。
// 关键注意事项：测试不连接真实 Redis 或 SSE 客户端，全部等待均有超时上限，避免 flaky 的 sleep 断言。
package global

import (
	"context"
	"testing"
)

func TestSSEManagerListenForEventsStopsOnStopListen(t *testing.T) {
	receiver := &fakePubSubReceiver{entered: make(chan struct{}), pattern: "sse:tenant:*"}
	manager := NewSSEManager()
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

func TestSSEManagerListenForEventsSecondCallWhileRunningIsNoOp(t *testing.T) {
	receiver := &fakePubSubReceiver{entered: make(chan struct{})}
	manager := NewSSEManager()
	manager.pubSubFactory = func(context.Context, string) wsPubSubReceiver {
		return receiver
	}

	listenExited := make(chan struct{})
	go func() {
		defer close(listenExited)
		manager.ListenForEvents()
	}()

	waitForChannel(t, receiver.entered, "listener entering receive loop")

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
