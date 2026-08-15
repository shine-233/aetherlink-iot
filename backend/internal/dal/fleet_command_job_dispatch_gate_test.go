package dal

import (
	"testing"
	"time"
)

func testCommandJobDispatchGatePolicy() CommandJobDispatchPolicy {
	return CommandJobDispatchPolicy{
		GlobalMaxConcurrent:     4,
		TenantMaxConcurrent:     2,
		GlobalRatePerSecond:     10,
		TenantRatePerSecond:     5,
		ContentionRetryInterval: 500 * time.Millisecond,
	}
}

// 并发闸门必须在“已达上限”时就拒绝，而不是等到超过上限，否则真实多实例下
// 每个实例都会各自多放一行出去。
func TestCommandJobDispatchGateBlocksAtGlobalConcurrencyLimit(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalDispatching:    int64(policy.GlobalMaxConcurrent),
		TenantDispatching:    0,
		GlobalNextDispatchAt: now,
		TenantNextDispatchAt: now,
	})

	if gate.Allow {
		t.Fatalf("gate allowed dispatch at the global concurrency limit")
	}
	if gate.Reason != CommandJobDispatchGateGlobalConcurrency {
		t.Fatalf("reason = %q, want %q", gate.Reason, CommandJobDispatchGateGlobalConcurrency)
	}
	if want := now.Add(policy.ContentionRetryInterval); !gate.RetryAt.Equal(want) {
		t.Fatalf("retryAt = %s, want %s", gate.RetryAt, want)
	}
}

// 全局闸门优先于租户闸门：两者同时触顶时必须报 global，否则运维会误判成
// 单租户配额问题而去调错的旋钮。
func TestCommandJobDispatchGatePrefersGlobalConcurrencyReason(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalDispatching:    int64(policy.GlobalMaxConcurrent),
		TenantDispatching:    int64(policy.TenantMaxConcurrent),
		GlobalNextDispatchAt: now,
		TenantNextDispatchAt: now,
	})

	if gate.Allow || gate.Reason != CommandJobDispatchGateGlobalConcurrency {
		t.Fatalf("gate = %+v, want blocked by global concurrency", gate)
	}
}

func TestCommandJobDispatchGateBlocksAtTenantConcurrencyLimit(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalDispatching:    0,
		TenantDispatching:    int64(policy.TenantMaxConcurrent),
		GlobalNextDispatchAt: now,
		TenantNextDispatchAt: now,
	})

	if gate.Allow || gate.Reason != CommandJobDispatchGateTenantConcurrency {
		t.Fatalf("gate = %+v, want blocked by tenant concurrency", gate)
	}
}

// 限速游标在未来时必须推迟到该游标，并且 retryAt 取两个游标里更晚的那个，
// 否则会在全局仍未开闸时反复空转。
func TestCommandJobDispatchGateDefersToLaterRateCursor(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()
	globalNext := now.Add(2 * time.Second)
	tenantNext := now.Add(750 * time.Millisecond)

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalNextDispatchAt: globalNext,
		TenantNextDispatchAt: tenantNext,
	})

	if gate.Allow {
		t.Fatalf("gate allowed dispatch while the rate cursor is in the future")
	}
	if gate.Reason != CommandJobDispatchGateGlobalRate {
		t.Fatalf("reason = %q, want %q", gate.Reason, CommandJobDispatchGateGlobalRate)
	}
	if !gate.RetryAt.Equal(globalNext) {
		t.Fatalf("retryAt = %s, want the later cursor %s", gate.RetryAt, globalNext)
	}
}

func TestCommandJobDispatchGateReportsTenantRateWhenTenantCursorIsLater(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()
	tenantNext := now.Add(3 * time.Second)

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalNextDispatchAt: now.Add(500 * time.Millisecond),
		TenantNextDispatchAt: tenantNext,
	})

	if gate.Allow || gate.Reason != CommandJobDispatchGateTenantRate {
		t.Fatalf("gate = %+v, want blocked by tenant rate", gate)
	}
	if !gate.RetryAt.Equal(tenantNext) {
		t.Fatalf("retryAt = %s, want %s", gate.RetryAt, tenantNext)
	}
}

// 并发判定必须先于限速判定：并发触顶时用固定的 contention 间隔重试，而不是
// 沿用可能很远的限速游标，否则一次拥塞会把后续下发拖停很久。
func TestCommandJobDispatchGateChecksConcurrencyBeforeRate(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalDispatching:    int64(policy.GlobalMaxConcurrent),
		GlobalNextDispatchAt: now.Add(time.Hour),
		TenantNextDispatchAt: now.Add(time.Hour),
	})

	if gate.Reason != CommandJobDispatchGateGlobalConcurrency {
		t.Fatalf("reason = %q, want concurrency to win over rate", gate.Reason)
	}
	if want := now.Add(policy.ContentionRetryInterval); !gate.RetryAt.Equal(want) {
		t.Fatalf("retryAt = %s, want the contention interval %s", gate.RetryAt, want)
	}
}

// 游标正好等于 now 不算未来，必须放行；否则限速会永久卡住。
func TestCommandJobDispatchGateAllowsWhenCursorsHaveElapsed(t *testing.T) {
	now := time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	policy := testCommandJobDispatchGatePolicy()

	gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
		Policy:               policy,
		Now:                  now,
		GlobalDispatching:    int64(policy.GlobalMaxConcurrent) - 1,
		TenantDispatching:    int64(policy.TenantMaxConcurrent) - 1,
		GlobalNextDispatchAt: now,
		TenantNextDispatchAt: now.Add(-time.Minute),
	})

	if !gate.Allow {
		t.Fatalf("gate = %+v, want allow", gate)
	}
	if gate.Reason != "" {
		t.Fatalf("reason = %q, want empty on allow", gate.Reason)
	}
}
