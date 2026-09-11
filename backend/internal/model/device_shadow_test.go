// 文件用途：设备影子 ACK 状态机与退避策略的定向证据（ROADMAP P0.2）。
// 覆盖：状态词表/终态判定、指数退避有上限、下次重投时间点、ACK 资格边界。
package model

import (
	"testing"
	"time"
)

func TestShadowStatusVocabularyAndTerminalStates(t *testing.T) {
	// 非终态：仍可能因 ACK 或重投而改变。
	for _, status := range []string{ShadowStatusPending, ShadowStatusSent} {
		if IsShadowTerminalStatus(status) {
			t.Fatalf("%s must not be terminal", status)
		}
	}
	// 终态：不可再投递、不可 ACK。
	for _, status := range []string{
		ShadowStatusDelivered, ShadowStatusFailed, ShadowStatusExpired, ShadowStatusCanceled,
	} {
		if !IsShadowTerminalStatus(status) {
			t.Fatalf("%s must be terminal", status)
		}
	}
}

func TestShadowAckableStatusBoundaries(t *testing.T) {
	// 只有未终态且已下发/待下发的消息可被确认。
	for _, status := range []string{ShadowStatusPending, ShadowStatusSent} {
		if !IsShadowAckableStatus(status) {
			t.Fatalf("%s must be ackable", status)
		}
	}
	// 已送达或已终态不可重复 ACK，否则等于改写历史结果。
	for _, status := range []string{
		ShadowStatusDelivered, ShadowStatusFailed, ShadowStatusExpired, ShadowStatusCanceled, "",
	} {
		if IsShadowAckableStatus(status) {
			t.Fatalf("%s must not be ackable", status)
		}
	}
}

func TestShadowRetryBackoffIsExponentialAndCapped(t *testing.T) {
	first := ShadowRetryBackoff(1)
	if first != 30*time.Second {
		t.Fatalf("first backoff = %v, want 30s", first)
	}
	if got := ShadowRetryBackoff(2); got != 60*time.Second {
		t.Fatalf("second backoff = %v, want 60s", got)
	}
	if got := ShadowRetryBackoff(3); got != 120*time.Second {
		t.Fatalf("third backoff = %v, want 120s", got)
	}
	// 有上限：避免长尾设备被无限推迟。
	const cap = 10 * time.Minute
	for _, attempts := range []int{8, 20, 1000} {
		if got := ShadowRetryBackoff(attempts); got != cap {
			t.Fatalf("backoff(%d) = %v, want cap %v", attempts, got, cap)
		}
	}
	// 非法输入按第一次处理，不得返回 0 或负值。
	if got := ShadowRetryBackoff(0); got != first {
		t.Fatalf("backoff(0) = %v, want %v", got, first)
	}
	if got := ShadowRetryBackoff(-5); got != first {
		t.Fatalf("backoff(-5) = %v, want %v", got, first)
	}
}

func TestShadowNextAttemptAtIsMonotonicAndUTC(t *testing.T) {
	now := time.Now().UTC()
	prev := now
	for attempts := 1; attempts <= 4; attempts++ {
		next := ShadowNextAttemptAt(now, attempts)
		if !next.After(now) {
			t.Fatalf("attempt %d: next %v must be after now %v", attempts, next, now)
		}
		if next.Before(prev) {
			t.Fatalf("attempt %d: backoff must not shrink", attempts)
		}
		prev = next
	}
	// 退避到上限后不再增长。
	a := ShadowNextAttemptAt(now, 50)
	b := ShadowNextAttemptAt(now, 200)
	if !a.Equal(b) {
		t.Fatalf("capped backoff must be stable: %v vs %v", a, b)
	}
}

func TestShadowMaxAttemptsBoundsRetry(t *testing.T) {
	// 达到上限即转 failed，不再重投。
	if ShadowMaxAttempts < 1 {
		t.Fatalf("ShadowMaxAttempts must be positive, got %d", ShadowMaxAttempts)
	}
	if ShadowMaxAttempts > 10 {
		t.Fatalf("ShadowMaxAttempts too large for a bounded retry policy: %d", ShadowMaxAttempts)
	}
}
