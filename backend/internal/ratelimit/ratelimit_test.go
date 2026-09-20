package ratelimit

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

func TestParseRateLimitRules(t *testing.T) {
	// 1. 合法复合表达式
	expr := "100:1,1000:60,20000:3600"
	rules, err := ParseRateLimitRules(expr)
	if err != nil {
		t.Fatalf("ParseRateLimitRules(%q) returned error: %v", expr, err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Limit != 100 || rules[0].WindowSeconds != 1 {
		t.Errorf("rule 0 mismatch: %+v", rules[0])
	}
	if rules[1].Limit != 1000 || rules[1].WindowSeconds != 60 {
		t.Errorf("rule 1 mismatch: %+v", rules[1])
	}
	if rules[2].Limit != 20000 || rules[2].WindowSeconds != 3600 {
		t.Errorf("rule 2 mismatch: %+v", rules[2])
	}

	// 格式化测试
	formatted := FormatRateLimitRules(rules)
	if formatted != expr {
		t.Errorf("FormatRateLimitRules expected %q, got %q", expr, formatted)
	}

	// 2. 乱序自动排序测试（短窗口在前）
	shuffled := "1000:60,100:1"
	sortedRules, err := ParseRateLimitRules(shuffled)
	if err != nil {
		t.Fatalf("ParseRateLimitRules(%q) error: %v", shuffled, err)
	}
	if sortedRules[0].WindowSeconds != 1 || sortedRules[1].WindowSeconds != 60 {
		t.Errorf("expected sorted rules, got %+v", sortedRules)
	}

	// 3. 非法测试
	invalidCases := []string{
		"",
		"   ",
		"100",
		"100:-1",
		"-10:5",
		"abc:10",
		"10:abc",
		"10:1,20:1", // 重复窗口
	}
	for _, tc := range invalidCases {
		if _, err := ParseRateLimitRules(tc); err == nil {
			t.Errorf("expected error for %q, but got nil", tc)
		}
	}
}

func TestMemoryEvaluatorMultiWindow(t *testing.T) {
	evaluator := NewMemoryEvaluator()
	baseTime := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	currentTime := baseTime
	evaluator.SetClock(func() time.Time { return currentTime })

	// 规则：1 秒内最多 2 次，60 秒内最多 4 次
	rules, err := ParseRateLimitRules("2:1,4:60")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ctx := context.Background()
	subject := "tenant-test-1"

	// 第 1 次：允许
	allowed, retry, _, err := evaluator.Allow(ctx, "api", subject, rules)
	if !allowed || err != nil {
		t.Fatalf("req 1 failed: allowed=%v, retry=%d, err=%v", allowed, retry, err)
	}

	// 第 2 次：允许 (当前秒 2 次打满)
	allowed, retry, _, err = evaluator.Allow(ctx, "api", subject, rules)
	if !allowed || err != nil {
		t.Fatalf("req 2 failed: allowed=%v, retry=%d, err=%v", allowed, retry, err)
	}

	// 第 3 次：当前秒超出限制（突发超限），拒绝！
	allowed, retry, violatedRule, err := evaluator.Allow(ctx, "api", subject, rules)
	if allowed || err != nil {
		t.Fatalf("req 3 expected blocked, got allowed=%v", allowed)
	}
	if retry <= 0 {
		t.Errorf("expected positive retry-after, got %d", retry)
	}
	if violatedRule != "2:1" {
		t.Errorf("expected violatedRule 2:1, got %q", violatedRule)
	}

	// 时间快进 2 秒，突破 1 秒突发限制，但仍在 60 秒长窗口内
	currentTime = currentTime.Add(2 * time.Second)

	// 第 4 次：允许（当前秒第 1 次，长窗口累计第 3 次）
	allowed, _, _, _ = evaluator.Allow(ctx, "api", subject, rules)
	if !allowed {
		t.Fatalf("req 4 expected allowed")
	}

	// 第 5 次：允许（当前秒第 2 次，长窗口累计第 4 次，长窗口打满！）
	allowed, _, _, _ = evaluator.Allow(ctx, "api", subject, rules)
	if !allowed {
		t.Fatalf("req 5 expected allowed")
	}

	// 时间再快进 2 秒
	currentTime = currentTime.Add(2 * time.Second)

	// 第 6 次：虽然当前秒没打满，但 60 秒长窗口已经达到 4 次，拒绝！
	allowed, retry, violatedRule, _ = evaluator.Allow(ctx, "api", subject, rules)
	if allowed {
		t.Fatalf("req 6 expected blocked by long window, got allowed")
	}
	if violatedRule != "4:60" {
		t.Errorf("expected violatedRule 4:60, got %q", violatedRule)
	}
	if retry <= 0 {
		t.Errorf("expected positive retry-after, got %d", retry)
	}
}

func TestRateLimitServiceOverrides(t *testing.T) {
	evaluator := NewMemoryEvaluator()
	svc := NewRateLimitService(evaluator, nil)

	ctx := context.Background()
	tenantA := "tenant-alpha"
	tenantB := "tenant-beta"

	// 覆写 tenantA 的 API 限流为严格的 1 次/60 秒
	key := svc.makeOverrideKey(tenantA, model.TargetTypeTenant, tenantA, model.LimitTypeAPI)
	overrideRules, _ := ParseRateLimitRules("1:60")
	svc.overrideMu.Lock()
	svc.overrides[key] = overrideRules
	svc.overrideMu.Unlock()

	// TenantA 第 1 次成功
	allowed, _, _ := svc.CheckAPI(ctx, tenantA, "user-1")
	if !allowed {
		t.Fatalf("tenantA req 1 should be allowed")
	}

	// TenantA 第 2 次拦截 (由于覆写了 1:60)
	allowed, retry, violated := svc.CheckAPI(ctx, tenantA, "user-1")
	if allowed {
		t.Fatalf("tenantA req 2 should be blocked by override")
	}
	if violated != "1:60" || retry <= 0 {
		t.Errorf("tenantA mismatch: retry=%d, violated=%q", retry, violated)
	}

	// TenantB 使用默认配额（默认 600:60），第 2 次依然允许
	allowedB1, _, _ := svc.CheckAPI(ctx, tenantB, "user-2")
	allowedB2, _, _ := svc.CheckAPI(ctx, tenantB, "user-2")
	if !allowedB1 || !allowedB2 {
		t.Fatalf("tenantB requests should both be allowed under default quota")
	}

	// 检查指标统计
	metrics := svc.GetMetrics()
	if metrics.TotalChecked < 4 {
		t.Errorf("expected total_checked >= 4, got %d", metrics.TotalChecked)
	}
	if metrics.TotalBlocked < 1 {
		t.Errorf("expected total_blocked >= 1, got %d", metrics.TotalBlocked)
	}
}

func TestRedisClusterEvaluatorMultiWindow(t *testing.T) {
	// 尝试连接本地 Redis
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available on 127.0.0.1:6379: %v", err)
	}
	defer client.Close()

	evaluator := NewRedisClusterEvaluator(client, nil)
	subject := "test-redis-subject-" + time.Now().Format("150405.000000")
	// 规则：1 秒内最多 2 次，60 秒内最多 5 次
	rules, err := ParseRateLimitRules("2:1,5:60")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// 清理前缀以防脏数据
	client.Del(ctx, redisRateLimitPrefix+"api:"+subject+":1s")
	client.Del(ctx, redisRateLimitPrefix+"api:"+subject+":60s")
	defer func() {
		client.Del(ctx, redisRateLimitPrefix+"api:"+subject+":1s")
		client.Del(ctx, redisRateLimitPrefix+"api:"+subject+":60s")
	}()

	// 第 1 次：放行
	allowed, retry, _, err := evaluator.Allow(ctx, "api", subject, rules)
	if !allowed || err != nil {
		t.Fatalf("req 1 failed: allowed=%v, retry=%d, err=%v", allowed, retry, err)
	}

	// 第 2 次：放行 (1s 窗口打满)
	allowed, retry, _, err = evaluator.Allow(ctx, "api", subject, rules)
	if !allowed || err != nil {
		t.Fatalf("req 2 failed: allowed=%v, retry=%d, err=%v", allowed, retry, err)
	}

	// 第 3 次：拦截 (1s 窗口超限)
	allowed, retry, violatedRule, err := evaluator.Allow(ctx, "api", subject, rules)
	if allowed || err != nil {
		t.Fatalf("req 3 expected blocked, got allowed=%v", allowed)
	}
	if retry <= 0 {
		t.Errorf("expected positive retry-after, got %d", retry)
	}
	if violatedRule != "2:1" {
		t.Errorf("expected violatedRule 2:1, got %q", violatedRule)
	}
}
