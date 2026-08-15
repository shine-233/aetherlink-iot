package dal

import (
	"testing"
	"time"
)

func TestCommandJobDispatchRateIntervalUsesDurableSlotSpacing(t *testing.T) {
	if got := commandJobDispatchRateInterval(4); got != 250*time.Millisecond {
		t.Fatalf("four dispatches per second interval = %s", got)
	}
	if got := commandJobDispatchRateInterval(20); got != 50*time.Millisecond {
		t.Fatalf("twenty dispatches per second interval = %s", got)
	}
}

func TestNextCommandJobDispatchQuotaSlotDoesNotMoveCursorBackward(t *testing.T) {
	now := time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC)
	futureCursor := now.Add(2 * time.Second)
	if got := nextCommandJobDispatchQuotaSlot(futureCursor, now, 4); !got.Equal(futureCursor.Add(250 * time.Millisecond)) {
		t.Fatalf("future cursor next slot = %s", got)
	}
	if got := nextCommandJobDispatchQuotaSlot(now.Add(-time.Second), now, 4); !got.Equal(now.Add(250 * time.Millisecond)) {
		t.Fatalf("expired cursor next slot = %s", got)
	}
}

func TestCommandJobDispatchPolicyRejectsNonPositiveLimits(t *testing.T) {
	valid := CommandJobDispatchPolicy{
		GlobalMaxConcurrent:     16,
		TenantMaxConcurrent:     4,
		GlobalRatePerSecond:     20,
		TenantRatePerSecond:     5,
		ContentionRetryInterval: 500 * time.Millisecond,
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid dispatch policy: %v", err)
	}
	valid.TenantRatePerSecond = 0
	if err := valid.validate(); err == nil {
		t.Fatal("zero tenant rate should be rejected")
	}
}
