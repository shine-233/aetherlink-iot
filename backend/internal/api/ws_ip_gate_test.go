// 文件用途：验证每来源 IP 并发闸门的配额语义。
// 核心逻辑：覆盖正常占用/释放、上限拒绝、空 IP 放行、重复释放防负数四条路径。

package api

import "testing"

func TestWSIPGateAcquireReleaseCycle(t *testing.T) {
	gate := newWSIPGate(2)

	for i := 0; i < 2; i++ {
		if !gate.tryAcquire("10.0.0.1") {
			t.Fatalf("acquire #%d should succeed under the limit", i+1)
		}
	}
	if gate.tryAcquire("10.0.0.1") {
		t.Fatal("third acquire should be rejected at the per-IP limit")
	}
	if !gate.tryAcquire("10.0.0.2") {
		t.Fatal("a different IP must not be affected by another IP's quota")
	}

	gate.release("10.0.0.1")
	if !gate.tryAcquire("10.0.0.1") {
		t.Fatal("acquire after release should succeed")
	}
}

func TestWSIPGateAllowsUnattributableIP(t *testing.T) {
	gate := newWSIPGate(1)
	for i := 0; i < 10; i++ {
		if !gate.tryAcquire("   ") {
			t.Fatalf("empty-ip acquire #%d should always pass", i+1)
		}
	}
	if gate.current("") != 0 {
		t.Fatal("empty ip must not be tracked")
	}
}

func TestWSIPGateReleaseIsBoundedAtZero(t *testing.T) {
	gate := newWSIPGate(1)
	gate.release("10.0.0.9")
	if got := gate.current("10.0.0.9"); got != 0 {
		t.Fatalf("release on unknown ip produced count %d, want 0", got)
	}
	gate.tryAcquire("10.0.0.9")
	gate.release("10.0.0.9")
	gate.release("10.0.0.9")
	if got := gate.current("10.0.0.9"); got != 0 {
		t.Fatalf("double release produced count %d, want clamped 0", got)
	}
}

func TestNewWSIPGateFallsBackToDefaultOnInvalidMax(t *testing.T) {
	gate := newWSIPGate(0)
	if gate.max != defaultWSMaxConnsPerIP {
		t.Fatalf("invalid max resolved to %d, want default %d", gate.max, defaultWSMaxConnsPerIP)
	}
}
