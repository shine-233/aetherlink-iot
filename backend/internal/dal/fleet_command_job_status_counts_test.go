package dal

import "testing"

func TestNewCommandJobDetailStatusCountsIncludesAllLifecycleStates(t *testing.T) {
	counts := newCommandJobDetailStatusCounts()
	for _, status := range []string{"ready", "dispatching", "submitted", "failed", "blocked", "canceled"} {
		value, ok := counts[status]
		if !ok {
			t.Fatalf("status_counts missing %q", status)
		}
		if value != 0 {
			t.Fatalf("status_counts[%q] = %d, want zero initial value", status, value)
		}
	}
}
