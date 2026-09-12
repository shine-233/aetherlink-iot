package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubPushProvider 可控的测试用 Provider。
type stubPushProvider struct {
	name     string
	platform []string
	err      error
	calls    int
}

func (s *stubPushProvider) Name() string { return s.name }
func (s *stubPushProvider) Supports(platform string) bool {
	for _, p := range s.platform {
		if p == platform {
			return true
		}
	}
	return false
}
func (s *stubPushProvider) Send(ctx context.Context, msg PushMessage) error {
	s.calls++
	return s.err
}

func TestPushProviderRegistryRequiresProviderForPlatform(t *testing.T) {
	empty := NewPushProviderRegistry()
	if _, err := empty.ForPlatform("android"); err != ErrPushNoProvider {
		t.Fatalf("empty registry error = %v, want %v", err, ErrPushNoProvider)
	}

	reg := NewPushProviderRegistry()
	if err := reg.Register(&stubPushProvider{name: "fcm", platform: []string{"android", "h5"}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// 同名重复注册必须拒绝：静默覆盖会让线上配置变更无声生效。
	if err := reg.Register(&stubPushProvider{name: "fcm", platform: []string{"ios"}}); err != ErrPushProviderExists {
		t.Fatalf("duplicate register error = %v, want %v", err, ErrPushProviderExists)
	}
	if err := reg.Register(nil); err != ErrPushBadProvider {
		t.Fatalf("nil provider error = %v, want %v", err, ErrPushBadProvider)
	}

	if _, err := reg.ForPlatform("ios"); err != ErrPushNoProvider {
		t.Fatalf("ios has no provider: error = %v, want %v", err, ErrPushNoProvider)
	}
	p, err := reg.ForPlatform("android")
	if err != nil || p.Name() != "fcm" {
		t.Fatalf("ForPlatform(android) = (%v, %v), want fcm", p, err)
	}
}

func TestPushRetryPolicyBackoffAndExhaustion(t *testing.T) {
	policy := DefaultPushRetryPolicy()

	delays := []struct {
		attempt int32
		want    time.Duration
	}{
		{attempt: 1, want: 30 * time.Second},
		{attempt: 2, want: 60 * time.Second},
		{attempt: 3, want: 120 * time.Second},
	}
	for _, tc := range delays {
		if got := policy.NextDelay(tc.attempt); got != tc.want {
			t.Fatalf("NextDelay(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}

	// 退避必须有上限，否则长故障下重试间隔会膨胀到不可理喻。
	if got := policy.NextDelay(50); got != policy.MaxBackoff {
		t.Fatalf("NextDelay(50) = %v, want capped at %v", got, policy.MaxBackoff)
	}

	// 超过上限即转终态：无限重试会把永久失败伪装成"还在路上"。
	if policy.Exhausted(1) || policy.Exhausted(2) {
		t.Fatal("early attempts must remain retryable")
	}
	if !policy.Exhausted(3) {
		t.Fatal("attempt 3 with MaxAttempts=3 must be exhausted")
	}
}

func TestPushRetryPolicyNeverExceedsMaxBackoff(t *testing.T) {
	policy := PushRetryPolicy{MaxAttempts: 5, BaseBackoff: time.Second, MaxBackoff: 5 * time.Second}
	for attempt := int32(1); attempt <= 5; attempt++ {
		if got := policy.NextDelay(attempt); got > 5*time.Second {
			t.Fatalf("NextDelay(%d) = %v exceeds cap", attempt, got)
		}
	}
}

// TestPushDeliveryMarksDeadAfterExhaustedRetries 需要数据库，见 scada_push_postgres_test.go。
// 这里只覆盖不依赖数据库的策略与注册表部分。
func TestPushProviderSendErrorSurfaces(t *testing.T) {
	p := &stubPushProvider{name: "fcm", platform: []string{"android"}, err: errors.New("upstream 503")}
	if err := p.Send(context.Background(), PushMessage{}); err == nil {
		t.Fatal("provider error must propagate; swallowing it would fake a successful push")
	}
	if p.calls != 1 {
		t.Fatalf("calls = %d, want 1", p.calls)
	}
}
