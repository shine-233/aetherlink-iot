package ratelimit

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 过期窗口必须被清扫，否则每个出现过的 subject 会永久驻留内存。
func TestMemoryEvaluatorSweepsExpiredBuckets(t *testing.T) {
	evaluator := NewMemoryEvaluator()
	current := time.Unix(1_700_000_000, 0)
	evaluator.SetClock(func() time.Time { return current })
	rules, err := ParseRateLimitRules("5:1")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		if ok, _, _, _ := evaluator.Allow(context.Background(), "api", fmt.Sprintf("dev-%d", i), rules); !ok {
			t.Fatalf("subject %d unexpectedly blocked", i)
		}
	}
	if got := len(evaluator.counts); got != 100 {
		t.Fatalf("bucket count = %d, want 100", got)
	}

	// 越过窗口与清扫间隔后，一次新请求即清掉全部过期桶，仅保留新桶。
	current = current.Add(memoryEvaluatorSweepInterval + 2*time.Second)
	if ok, _, _, _ := evaluator.Allow(context.Background(), "api", "fresh", rules); !ok {
		t.Fatal("fresh subject unexpectedly blocked")
	}
	if got := len(evaluator.counts); got != 1 {
		t.Fatalf("bucket count after sweep = %d, want 1", got)
	}
}

// 清扫不得影响仍在窗口内的计数。
func TestMemoryEvaluatorSweepKeepsLiveBuckets(t *testing.T) {
	evaluator := NewMemoryEvaluator()
	current := time.Unix(1_700_000_000, 0)
	evaluator.SetClock(func() time.Time { return current })
	rules, _ := ParseRateLimitRules("2:600")

	ctx := context.Background()
	evaluator.Allow(ctx, "api", "t1", rules)
	evaluator.Allow(ctx, "api", "t1", rules)
	current = current.Add(2 * memoryEvaluatorSweepInterval)
	if ok, _, _, _ := evaluator.Allow(ctx, "api", "t1", rules); ok {
		t.Fatal("live 600s window must still block after sweep")
	}
}
