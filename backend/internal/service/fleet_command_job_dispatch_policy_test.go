package service

import (
	"testing"
	"time"
)

func TestCommandJobDispatchGateWaitDurationUsesOnlyShortEfficiencyWaits(t *testing.T) {
	now := time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC)
	shortRetry := now.Add(500 * time.Millisecond)
	wait, ok := commandJobDispatchGateWaitDuration(&shortRetry, now, 2*time.Second)
	if !ok || wait != 500*time.Millisecond {
		t.Fatalf("short gate wait = %s, %t", wait, ok)
	}
	longRetry := now.Add(10 * time.Second)
	if wait, ok := commandJobDispatchGateWaitDuration(&longRetry, now, 2*time.Second); ok || wait != 10*time.Second {
		t.Fatalf("long gate wait = %s, %t", wait, ok)
	}
}

func TestCommandJobDispatchConfigBoundsTenantPolicyByGlobalPolicy(t *testing.T) {
	if got := boundedPositiveInt(8, 4, 3); got != 3 {
		t.Fatalf("bounded tenant concurrency = %d", got)
	}
	if got := boundedPositiveFloat(12, 5, 0.1, 7); got != 7 {
		t.Fatalf("bounded tenant rate = %f", got)
	}
	if got := boundedPositiveDuration(0, 500*time.Millisecond, 50*time.Millisecond, 5*time.Second); got != 500*time.Millisecond {
		t.Fatalf("fallback contention interval = %s", got)
	}
}

func TestCommandJobTerminalStatusExcludesRunningAndScheduled(t *testing.T) {
	for _, status := range []string{commandJobStatusRunning, commandJobStatusScheduled} {
		if commandJobStatusIsTerminal(status) {
			t.Fatalf("status %q should remain active", status)
		}
	}
	for _, status := range []string{commandJobStatusCompleted, commandJobStatusPartiallyFailed, commandJobStatusFailed, commandJobStatusCanceled} {
		if !commandJobStatusIsTerminal(status) {
			t.Fatalf("status %q should be terminal", status)
		}
	}
}
