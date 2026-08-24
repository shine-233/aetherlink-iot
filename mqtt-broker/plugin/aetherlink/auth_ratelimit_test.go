// 文件用途：验证认证失败滑动窗口限速器的窗口累计、成功豁免、时间恢复、来源隔离与惰性清理。
// 核心逻辑：通过注入 clock 函数推进时间，覆盖 allow/record/cleanup 与配置读取兜底。
// 关键注意事项：不依赖 viper 全局状态以外的外部资源；wrapper 级测试用桩连接模拟来源 IP。

package aetherlink

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// fakeAddr 是仅实现 RemoteAddr 所需行为的桩地址。
type fakeAddr struct {
	addr string
}

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return a.addr }

// fakeConn 是只提供 RemoteAddr 的桩 net.Conn；限速路径不会触及其他方法。
type fakeConn struct {
	remote net.Addr
}

func (c *fakeConn) Read(b []byte) (int, error)         { return 0, errors.New("not implemented") }
func (c *fakeConn) Write(b []byte) (int, error)        { return 0, errors.New("not implemented") }
func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return fakeAddr{addr: "127.0.0.1:1883"} }
func (c *fakeConn) RemoteAddr() net.Addr               { return c.remote }
func (c *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// newFakeAuthClient 构造带指定来源地址的 mock client。
func newFakeAuthClient(t *testing.T, remote string) server.Client {
	t.Helper()
	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	client.EXPECT().Connection().Return(&fakeConn{remote: fakeAddr{addr: remote}}).AnyTimes()
	return client
}

// mutableClock 提供可手动推进的时钟，供时间推进类用例使用。
type mutableClock struct {
	current time.Time
}

func (c *mutableClock) now() time.Time          { return c.current }
func (c *mutableClock) advance(d time.Duration) { c.current = c.current.Add(d) }

func TestAuthRateLimiterBlocksAfterWindowFailures(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(3, time.Minute, time.Minute, clock.now)

	ip := "203.0.113.10"
	for i := 0; i < 3; i++ {
		if !limiter.allow(ip) {
			t.Fatalf("attempt %d should be allowed before reaching cap", i+1)
		}
		limiter.record(ip)
	}
	if limiter.allow(ip) {
		t.Fatal("ip should be blocked after reaching failure cap")
	}
}

func TestAuthRateLimiterDoesNotCountSuccessesOrRejectedAttempts(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(2, time.Minute, time.Minute, clock.now)

	ip := "203.0.113.20"
	// 成功认证不计数：任意多次 allow 都不应触发拒绝。
	for i := 0; i < 50; i++ {
		if !limiter.allow(ip) {
			t.Fatalf("success-only attempts must never be limited (iteration %d)", i)
		}
	}
	// 达到上限后被拒绝的 CONNECT 不再计数：封禁随窗口滑动自然解除，而不是被无限延长。
	limiter.record(ip)
	limiter.record(ip)
	if limiter.allow(ip) {
		t.Fatal("ip should be blocked at cap")
	}
	for i := 0; i < 100; i++ {
		if limiter.allow(ip) {
			t.Fatal("rejected attempts must not extend the block")
		}
	}
	clock.advance(time.Minute)
	if !limiter.allow(ip) {
		t.Fatal("ip should recover once failures age out of the window")
	}
}

func TestAuthRateLimiterRecoversWithSlidingWindow(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(3, time.Minute, time.Hour, clock.now)

	ip := "203.0.113.30"
	limiter.record(ip) // t=0s
	clock.advance(10 * time.Second)
	limiter.record(ip) // t=10s
	clock.advance(10 * time.Second)
	limiter.record(ip) // t=20s
	if limiter.allow(ip) {
		t.Fatal("ip should be blocked with three in-window failures")
	}

	// 部分滑出：t=65s 时最早一次失败已过期，剩余两次未达上限。
	clock.advance(45 * time.Second)
	if !limiter.allow(ip) {
		t.Fatal("ip should be allowed after oldest failures slide out of the window")
	}
	// 完全滑出后再失败一次，仍应放行（窗口内只有 1 次）。
	clock.advance(20 * time.Second)
	limiter.record(ip)
	if !limiter.allow(ip) {
		t.Fatal("single fresh failure must not block the ip")
	}
}

func TestAuthRateLimiterIsolatesSourceIPs(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(2, time.Minute, time.Minute, clock.now)

	blocked := "203.0.113.40"
	other := "198.51.100.50"
	limiter.record(blocked)
	limiter.record(blocked)
	if limiter.allow(blocked) {
		t.Fatal("attacking ip should be blocked")
	}
	if !limiter.allow(other) {
		t.Fatal("other ips must not be affected by a different source's failures")
	}
}

func TestAuthRateLimiterUnknownSourceBypassesLimiting(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(1, time.Minute, time.Minute, clock.now)

	unknown := ""
	if !limiter.allow(unknown) {
		t.Fatal("unattributable sources must always be allowed")
	}
	limiter.record(unknown)
	if !limiter.allow(unknown) {
		t.Fatal("unattributable sources must stay allowed even after record calls")
	}
	if got := limiter.cleanup(); got != 0 {
		t.Fatalf("unknown source must never enter the map, remaining keys = %d", got)
	}
}

func TestAuthRateLimiterLazyCleanupRemovesExpiredKeys(t *testing.T) {
	clock := &mutableClock{current: time.Unix(0, 0)}
	limiter := newAuthFailureLimiter(1, time.Minute, time.Minute, clock.now)

	limiter.record("203.0.113.60")
	limiter.record("203.0.113.61")
	if len(limiter.failures) != 2 {
		t.Fatalf("expected two tracked keys, got %d", len(limiter.failures))
	}

	// 未到清扫间隔时，即使条目已过期也不主动清理（惰性策略）。
	clock.advance(90 * time.Second)
	if got := len(limiter.failures); got != 2 {
		t.Fatalf("lazy policy must keep keys until gc interval elapses, got %d", got)
	}

	// 触发新一轮记录/清扫后，过期 key 被移除。
	clock.advance(1 * time.Second)
	limiter.record("203.0.113.62")
	if got := len(limiter.failures); got != 1 {
		t.Fatalf("expired keys should be swept on gc, remaining keys = %d", got)
	}
	if !limiter.allow("203.0.113.60") {
		t.Fatal("swept ip should be allowed again")
	}
	if limiter.cleanup() != 1 {
		t.Fatal("cleanup should leave only the fresh key")
	}
}

func TestMQTTAuthRateLimitMaxFailuresFallsBackToDefault(t *testing.T) {
	prev := viper.Get(authRateLimitConfigKey)
	t.Cleanup(func() { viper.Set(authRateLimitConfigKey, prev) })

	viper.Set(authRateLimitConfigKey, nil)
	if got := mqttAuthRateLimitMaxFailures(); got != defaultAuthRateLimitMaxFailuresPerMinute {
		t.Fatalf("default max failures = %d, want %d", got, defaultAuthRateLimitMaxFailuresPerMinute)
	}

	viper.Set(authRateLimitConfigKey, 7)
	if got := mqttAuthRateLimitMaxFailures(); got != 7 {
		t.Fatalf("configured max failures = %d, want 7", got)
	}

	viper.Set(authRateLimitConfigKey, -3)
	if got := mqttAuthRateLimitMaxFailures(); got != defaultAuthRateLimitMaxFailuresPerMinute {
		t.Fatalf("invalid config must fall back to default, got %d", got)
	}
}

func TestMQTTAuthRemoteIPIgnoresUnparseableConnections(t *testing.T) {
	ctrl := gomock.NewController(t)
	nilConnClient := server.NewMockClient(ctrl)
	nilConnClient.EXPECT().Connection().Return(nil).AnyTimes()
	if got := mqttAuthRemoteIP(nilConnClient); got != "" {
		t.Fatalf("nil connection remote ip = %q, want empty", got)
	}
	if got := mqttAuthRemoteIP(nil); got != "" {
		t.Fatalf("nil client remote ip = %q, want empty", got)
	}
	if got := mqttAuthRemoteIP(newFakeAuthClient(t, "[2001:db8::1]:52411")); got != "2001:db8::1" {
		t.Fatalf("ipv6 remote ip = %q, want 2001:db8::1", got)
	}
}

func TestOnBasicAuthWrapperAuthLimitRejectsAndCountsPerIP(t *testing.T) {
	prevLog := Log
	Log = zap.NewNop()
	prevLimiter := mqttAuthRateLimiter
	t.Cleanup(func() {
		Log = prevLog
		mqttAuthRateLimiter = prevLimiter
	})

	clock := &mutableClock{current: time.Unix(0, 0)}
	mqttAuthRateLimiter = newAuthFailureLimiter(2, time.Minute, time.Minute, clock.now)

	ctx := context.Background()
	plugin := &AetherLinkPlugin{}
	preErr := errors.New("password error")
	preCalls := 0
	hook := plugin.OnBasicAuthWrapper(func(context.Context, server.Client, *server.ConnectRequest) error {
		preCalls++
		return preErr
	})

	client := newFakeAuthClient(t, "203.0.113.70:40000")
	req := &server.ConnectRequest{Connect: &packets.Connect{
		Username: []byte("root"),
		Password: []byte("wrong-password"),
	}}

	// 前置钩子失败计入来源 IP：两次后达到上限。
	for i := 0; i < 2; i++ {
		if err := hook(ctx, client, req); !errors.Is(err, preErr) {
			t.Fatalf("attempt %d error = %v, want pre hook sentinel", i+1, err)
		}
	}
	if preCalls != 2 {
		t.Fatalf("pre hook calls = %d, want 2", preCalls)
	}

	// 超限后的 CONNECT 在进入认证链路（含前置钩子）之前直接拒绝。
	if err := hook(ctx, client, req); !errors.Is(err, errMQTTAuthRateLimited) {
		t.Fatalf("blocked attempt error = %v, want errMQTTAuthRateLimited", err)
	}
	if preCalls != 2 {
		t.Fatalf("pre hook must not run for rejected connects, calls = %d", preCalls)
	}

	// 其他来源 IP 不受影响。
	other := newFakeAuthClient(t, "198.51.100.80:40001")
	if err := hook(ctx, other, req); !errors.Is(err, preErr) {
		t.Fatalf("other ip error = %v, want pre hook sentinel", err)
	}

	// 时间滑出窗口后原 IP 恢复并可再次进入认证链路。
	clock.advance(time.Minute)
	if err := hook(ctx, client, req); !errors.Is(err, preErr) {
		t.Fatalf("recovered ip error = %v, want pre hook sentinel", err)
	}
}

func TestOnBasicAuthWrapperAuthLimitCountsSystemUserFailures(t *testing.T) {
	prevLog := Log
	Log = zap.NewNop()
	prevLimiter := mqttAuthRateLimiter
	prevRootPassword := viper.GetString("mqtt.password")
	t.Cleanup(func() {
		Log = prevLog
		mqttAuthRateLimiter = prevLimiter
		viper.Set("mqtt.password", prevRootPassword)
	})
	viper.Set("mqtt.password", "correct-horse")

	clock := &mutableClock{current: time.Unix(0, 0)}
	mqttAuthRateLimiter = newAuthFailureLimiter(2, time.Minute, time.Minute, clock.now)

	ctx := context.Background()
	plugin := &AetherLinkPlugin{}
	preCalls := 0
	hook := plugin.OnBasicAuthWrapper(func(context.Context, server.Client, *server.ConnectRequest) error {
		preCalls++
		return nil
	})

	client := newFakeAuthClient(t, "203.0.113.90:41000")
	wrongReq := &server.ConnectRequest{Connect: &packets.Connect{
		Username: []byte("root"),
		Password: []byte("bad"),
	}}

	// root 密码错误同样计数，达到上限后入口直接拒绝且不再调用前置钩子。
	for i := 0; i < 2; i++ {
		err := hook(ctx, client, wrongReq)
		if err == nil || errors.Is(err, errMQTTAuthRateLimited) {
			t.Fatalf("system user wrong password attempt %d must fail with auth error, got %v", i+1, err)
		}
	}
	if preCalls != 2 {
		t.Fatalf("pre hook calls = %d, want 2 before the cap is hit", preCalls)
	}
	if err := hook(ctx, client, wrongReq); !errors.Is(err, errMQTTAuthRateLimited) {
		t.Fatalf("system user source should be rate limited, got %v", err)
	}

	// 同一 IP 换成正确密码也仍被拒绝：限速在认证前生效。
	correctReq := &server.ConnectRequest{Connect: &packets.Connect{
		Username: []byte("root"),
		Password: []byte("correct-horse"),
	}}
	if err := hook(ctx, client, correctReq); !errors.Is(err, errMQTTAuthRateLimited) {
		t.Fatalf("rate limit must apply before credential check, got %v", err)
	}
	if preCalls != 2 {
		t.Fatal("previous hook must not run while the source is blocked")
	}
}
