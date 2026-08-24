// 文件用途：验证心跳监控过期事件订阅协程的幂等启动与确定性停止。
// 核心逻辑：注入假订阅器，断言重复 Start 不叠加消费协程、Stop 后监听协程真正退出且订阅只关闭一次。
// 关键注意事项：测试不连接真实 Redis；Stop 内部自带有界等待，返回即代表退出信号已收到。
package service

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// fakeExpirySubscriber 提供永不投递消息的过期事件通道，记录 Close 次数。
type fakeExpirySubscriber struct {
	closeCount atomic.Int32
}

func (f *fakeExpirySubscriber) Channel(...redis.ChannelOption) <-chan *redis.Message {
	return make(chan *redis.Message)
}

func (f *fakeExpirySubscriber) Close() error {
	f.closeCount.Add(1)
	return nil
}

func TestHeartbeatMonitorStartIsIdempotentAndStopWaitsForListenerExit(t *testing.T) {
	monitor := NewHeartbeatMonitor(nil, nil, logrus.New())

	subscriber := &fakeExpirySubscriber{}
	var subscribeCount atomic.Int32
	monitor.subscribeExpiry = func(pattern string) (heartbeatExpirySubscriber, error) {
		if pattern == "" {
			t.Errorf("subscribe pattern is empty")
		}
		subscribeCount.Add(1)
		return subscriber, nil
	}

	// 重复 Start 必须被幂等守卫拦截：只允许创建一个订阅与一个消费协程，
	// 否则同一过期事件会被处理两次，导致重复发布离线状态。
	if err := monitor.Start(); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	if err := monitor.Start(); err != nil {
		t.Fatalf("duplicate Start failed: %v", err)
	}
	if got := subscribeCount.Load(); got != 1 {
		t.Fatalf("subscribe count after duplicate Start = %d, want 1", got)
	}

	// Stop 返回即代表监听协程已确认退出（内部等待 done 关闭），订阅只关闭一次。
	if err := monitor.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if got := subscriber.closeCount.Load(); got != 1 {
		t.Fatalf("subscriber close count = %d, want 1", got)
	}

	// 停止后重新启动（模拟服务重启场景），再次停止仍需干净退出。
	if err := monitor.Start(); err != nil {
		t.Fatalf("restart Start failed: %v", err)
	}
	if got := subscribeCount.Load(); got != 2 {
		t.Fatalf("subscribe count after restart = %d, want 2", got)
	}
	if err := monitor.Stop(); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
	if got := subscriber.closeCount.Load(); got != 2 {
		t.Fatalf("subscriber close count after restart = %d, want 2", got)
	}
}

func TestHeartbeatMonitorStopWithoutStartCancelsContextAndIsSafe(t *testing.T) {
	monitor := NewHeartbeatMonitor(nil, nil, logrus.New())

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := monitor.Stop(); err != nil {
			t.Errorf("Stop without Start failed: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(heartbeatMonitorStopTimeout):
		t.Fatal("Stop without Start blocked")
	}

	if monitor.ctx.Err() == nil {
		t.Fatal("expected constructor ctx to be cancelled by Stop without Start")
	}
}
