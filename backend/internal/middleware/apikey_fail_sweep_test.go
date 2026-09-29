package middleware

import (
	"fmt"
	"testing"
	"time"
)

func countOpenAPIKeyAuthFailEntries() int {
	n := 0
	openAPIKeyAuthFailCounts.Range(func(_, _ any) bool { n++; return true })
	return n
}

// 失败计数表的过期条目必须被清扫，防止轮换来源 IP 造成无界增长；未过期条目与限流语义保持不变。
func TestOpenAPIKeyAuthFailuresSweepExpiredEntries(t *testing.T) {
	openAPIKeyAuthFailCounts.Range(func(k, _ any) bool { openAPIKeyAuthFailCounts.Delete(k); return true })
	openAPIKeyAuthNextSweep.Store(0)
	t.Cleanup(func() {
		openAPIKeyAuthFailCounts.Range(func(k, _ any) bool { openAPIKeyAuthFailCounts.Delete(k); return true })
		openAPIKeyAuthNextSweep.Store(0)
	})

	for i := 0; i < 50; i++ {
		recordOpenAPIKeyAuthFailure(fmt.Sprintf("198.51.100.%d", i))
	}
	for i := 0; i < openAPIKeyAuthFailLimit; i++ {
		recordOpenAPIKeyAuthFailure("203.0.113.9")
	}
	if !openAPIKeyAuthRateLimited("203.0.113.9") {
		t.Fatal("IP over the failure limit must be rate limited")
	}
	if got := countOpenAPIKeyAuthFailEntries(); got != 51 {
		t.Fatalf("entries = %d, want 51", got)
	}

	// 窗口内的清扫不得删除任何条目。
	openAPIKeyAuthNextSweep.Store(0)
	sweepOpenAPIKeyAuthFailures(time.Now())
	if got := countOpenAPIKeyAuthFailEntries(); got != 51 {
		t.Fatalf("entries after in-window sweep = %d, want 51", got)
	}
	if !openAPIKeyAuthRateLimited("203.0.113.9") {
		t.Fatal("in-window sweep must not reset an active limit")
	}

	// 越过窗口后清扫删除全部过期条目。
	sweepOpenAPIKeyAuthFailures(time.Now().Add(2 * openAPIKeyAuthFailWindow))
	if got := countOpenAPIKeyAuthFailEntries(); got != 0 {
		t.Fatalf("entries after expiry sweep = %d, want 0", got)
	}
}
