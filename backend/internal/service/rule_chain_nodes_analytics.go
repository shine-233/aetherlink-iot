package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// PHASE-D-D1 BEGIN Analytics 分析节点 handler（D1 规则引擎 2.0）

// ruleChainAnalyticsGenerator analytics.generator：按 values 配置生成模拟遥测载荷。
// 标量直出；{min,max} 区间输出均匀随机数（两位小数）。作为 trigger kind 使用。
func ruleChainAnalyticsGenerator(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	values, ok := node.Config["values"].(map[string]any)
	if !ok || len(values) == 0 {
		return ruleChainNodeResult{}, fmt.Errorf("generator config requires non-empty values")
	}
	generated := make(map[string]any, len(values))
	for key, raw := range values {
		switch spec := raw.(type) {
		case map[string]any:
			minV, okMin := toFloat(spec["min"])
			maxV, okMax := toFloat(spec["max"])
			if !okMin || !okMax || maxV < minV {
				return ruleChainNodeResult{}, fmt.Errorf("generator range for %q invalid", key)
			}
			// 区间单点时直接输出，避免除零与随机抖动。
			if maxV == minV {
				generated[key] = minV
				continue
			}
			generated[key] = roundTwoDecimals(minV + rand.Float64()*(maxV-minV))
		default:
			generated[key] = raw
		}
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	for k, v := range generated {
		out[k] = v
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: out, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainAnalyticsLatest analytics.latest：把 originator 最新遥测按 keys 聚合进载荷。
func ruleChainAnalyticsLatest(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	keys, err := configStringList(node.Config, "keys")
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	latest, err := ruleChainLatestTelemetryFetcher(ctx, rcc.TenantID, rcc.DeviceID, keys)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("fetch latest telemetry: %w", err)
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	for k, v := range latest {
		out[k] = v
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: out, metadata: metadata, rcc: rcc}}}, nil
}

// ---- message_count 滑窗计数 ----

type ruleChainCountEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

var (
	ruleChainCountMu       sync.Mutex
	ruleChainCountRegistry = map[string]*ruleChainCountEntry{}
)

const (
	ruleChainCountDefaultWindow = time.Minute
	ruleChainCountMaxWindow     = time.Hour
	ruleChainCountMaxKept       = 1024
)

// countEntry 取（或创建）scope 计数状态。
func countEntry(scopeKey string) *ruleChainCountEntry {
	ruleChainCountMu.Lock()
	defer ruleChainCountMu.Unlock()
	entry, ok := ruleChainCountRegistry[scopeKey]
	if !ok {
		entry = &ruleChainCountEntry{}
		ruleChainCountRegistry[scopeKey] = entry
	}
	return entry
}

// ruleChainAnalyticsMessageCount analytics.message_count：窗口内到达消息数写入载荷 message_count。
// 配置 {window_ms?}（默认 60s，上限 1h）；惰性清理窗口外时间戳。
func ruleChainAnalyticsMessageCount(e *ruleChainExecution, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	window := ruleChainCountDefaultWindow
	if raw, ok := toFloat(node.Config["window_ms"]); ok && raw > 0 {
		if raw > float64(ruleChainCountMaxWindow.Milliseconds()) {
			return ruleChainNodeResult{}, fmt.Errorf("message_count window_ms exceeds limit %d", ruleChainCountMaxWindow.Milliseconds())
		}
		window = time.Duration(raw * float64(time.Millisecond))
	}
	scopeKey := fmt.Sprintf("%s|%s|%s", chainIDOfExecution(e), node.ID, rcc.TenantID)
	now := time.Now()
	entry := countEntry(scopeKey)
	entry.mu.Lock()
	kept := entry.timestamps[:0]
	for _, ts := range entry.timestamps {
		if now.Sub(ts) <= window {
			kept = append(kept, ts)
		}
	}
	entry.timestamps = append(kept, now)
	count := len(entry.timestamps)
	// 硬上限兜底防窗口内洪泛撑爆内存。
	if len(entry.timestamps) > ruleChainCountMaxKept {
		entry.timestamps = entry.timestamps[len(entry.timestamps)-ruleChainCountMaxKept:]
	}
	entry.mu.Unlock()

	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	out["message_count"] = count
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: out, metadata: metadata, rcc: rcc}}}, nil
}

// roundTwoDecimals 模拟值保留两位小数，保持遥测观感稳定。
func roundTwoDecimals(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// PHASE-D-D1 END
