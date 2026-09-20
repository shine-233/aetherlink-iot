package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// policyTestExec 构造一次最小链执行上下文。
func policyTestExec(ctx context.Context, chainID string) *ruleChainExecution {
	graph := &RuleChainGraph{ChainID: chainID}
	return newRuleChainExecution(ctx, graph, "exec-1")
}

func policyTestMsg() ruleChainMessage {
	return ruleChainMessage{
		Payload:  map[string]any{},
		Metadata: map[string]any{},
		Rcc:      &RuleChainContext{TenantID: "t1", DeviceID: "d1"},
	}
}

// withFailingWebhook 注入必然失败的 webhook poster 并返回调用计数指针。
func withFailingWebhook(t *testing.T, fail func() error) *int {
	t.Helper()
	orig := ruleChainWebhookPoster
	calls := 0
	ruleChainWebhookPoster = func(ctx context.Context, url string, body []byte) (*http.Response, error) {
		calls++
		return nil, fail()
	}
	t.Cleanup(func() { ruleChainWebhookPoster = orig })
	return &calls
}

func TestParseRuleChainNodePolicyDefaults(t *testing.T) {
	p, err := parseRuleChainNodePolicy(nil)
	require.NoError(t, err)
	require.Equal(t, 2*time.Second, p.Timeout)
	require.Equal(t, 1, p.MaxAttempts)
	require.Equal(t, 200*time.Millisecond, p.Backoff)
	require.True(t, p.DeadLetter, "终局失败默认应下沉死信")
	require.False(t, p.RetrySafe)

	p2, err := parseRuleChainNodePolicy(map[string]any{"url": "http://x"})
	require.NoError(t, err)
	require.Equal(t, p, p2, "无 policy 段时不应改变默认值")
}

func TestParseRuleChainNodePolicyRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		policy  map[string]any
		wantMsg string
	}{
		{"policy 非对象", map[string]any{"policy": "nope"}, "must be an object"},
		{"max_attempts 超上限", map[string]any{"policy": map[string]any{"max_attempts": float64(99)}}, "max_attempts"},
		{"max_attempts 为 0", map[string]any{"policy": map[string]any{"max_attempts": float64(0)}}, "max_attempts"},
		{"max_attempts 非整数", map[string]any{"policy": map[string]any{"max_attempts": 1.5}}, "integer"},
		{"timeout_ms 超整链上限", map[string]any{"policy": map[string]any{"timeout_ms": float64(999999)}}, "chain timeout"},
		{"timeout_ms 为 0", map[string]any{"policy": map[string]any{"timeout_ms": float64(0)}}, "timeout_ms"},
		{"backoff_ms 为负", map[string]any{"policy": map[string]any{"backoff_ms": float64(-1)}}, "backoff_ms"},
		{"dead_letter 类型错", map[string]any{"policy": map[string]any{"dead_letter": "yes"}}, "dead_letter"},
		{"retry_safe 类型错", map[string]any{"policy": map[string]any{"retry_safe": "yes"}}, "retry_safe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRuleChainNodePolicy(tc.policy)
			require.Error(t, err, "非法策略必须被拒绝，不得静默兜底")
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestRuleChainNodePolicyRetriesNonSideEffectNode(t *testing.T) {
	node := &RuleChainNode{ID: "n1", Type: RuleChainFilterThreshold, Config: map[string]any{
		"policy": map[string]any{"max_attempts": float64(3), "backoff_ms": float64(0)},
	}}
	_, attempt, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.Equal(t, 3, attempt.Attempts, "无副作用节点应按策略重试到上限")
	require.Zero(t, attempt.BackoffMs)
}

// 有副作用节点禁止自动重试：重试一次设备命令等于两次下发。
func TestRuleChainNodePolicyDoesNotRetrySideEffectNode(t *testing.T) {
	calls := withFailingWebhook(t, func() error { return errors.New("boom") })
	node := &RuleChainNode{ID: "n1", Type: RuleChainActionWebhook, Config: map[string]any{
		"url":    "http://example.invalid",
		"policy": map[string]any{"max_attempts": float64(3), "backoff_ms": float64(0)},
	}}
	_, attempt, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.Equal(t, 1, *calls, "webhook 有副作用，禁止自动重试，否则业务被重复投递")
	require.Equal(t, 1, attempt.Attempts)
}

func TestRuleChainNodePolicyRetrySafeOptsInAllowsRetry(t *testing.T) {
	calls := withFailingWebhook(t, func() error { return errors.New("boom") })
	node := &RuleChainNode{ID: "n1", Type: RuleChainActionWebhook, Config: map[string]any{
		"url":    "http://example.invalid",
		"policy": map[string]any{"max_attempts": float64(3), "backoff_ms": float64(0), "retry_safe": true},
	}}
	_, attempt, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.Equal(t, 3, *calls, "显式声明 retry_safe 后才允许重试")
	require.Equal(t, 3, attempt.Attempts)
}

// 注册表外类型无法证明重试安全，按有副作用处理。
func TestRuleChainNodePolicyUnknownTypeTreatedAsSideEffectful(t *testing.T) {
	require.True(t, ruleChainNodeSideEffectful("custom.unknown"))
	node := &RuleChainNode{ID: "n1", Type: "custom.unknown", Config: map[string]any{
		"policy": map[string]any{"max_attempts": float64(4), "backoff_ms": float64(0)},
	}}
	_, attempt, _ := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Equal(t, 1, attempt.Attempts, "未知类型不得被自动重试")
}

func TestRuleChainNodePolicyEnforcesPerNodeTimeout(t *testing.T) {
	orig := ruleChainWebhookPoster
	ruleChainWebhookPoster = func(ctx context.Context, url string, body []byte) (*http.Response, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { ruleChainWebhookPoster = orig })

	node := &RuleChainNode{ID: "n1", Type: RuleChainActionWebhook, Config: map[string]any{
		"url":    "http://example.invalid",
		"policy": map[string]any{"timeout_ms": float64(20)},
	}}
	_, _, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "节点级超时必须生效，实际: %v", err)
}

// 退避等待必须响应取消，绝不越过 deadline 继续睡眠。
func TestRuleChainNodePolicyBackoffRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	orig := ruleChainWebhookPoster
	ruleChainWebhookPoster = func(ctx context.Context, url string, body []byte) (*http.Response, error) {
		calls++
		cancel()
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { ruleChainWebhookPoster = orig })

	node := &RuleChainNode{ID: "n1", Type: RuleChainActionWebhook, Config: map[string]any{
		"url": "http://example.invalid",
		"policy": map[string]any{
			"max_attempts": float64(3), "backoff_ms": float64(500), "retry_safe": true,
		},
	}}
	start := time.Now()
	_, _, err := policyTestExec(ctx, "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "取消后应立即返回，实际: %v", err)
	require.Equal(t, 1, calls, "取消后不得再发起后续尝试")
	require.Less(t, time.Since(start), 500*time.Millisecond, "不得睡完整段退避时间")
}

func TestRuleChainNodePolicyDeadLetterSinkOnFinalFailure(t *testing.T) {
	var got []RuleChainDeadLetter
	ruleChainDeadLetterSink = func(dl RuleChainDeadLetter) { got = append(got, dl) }
	t.Cleanup(func() { ruleChainDeadLetterSink = nil })

	node := &RuleChainNode{ID: "n1", Type: RuleChainFilterThreshold, Config: map[string]any{
		"policy": map[string]any{"max_attempts": float64(2), "backoff_ms": float64(0)},
	}}
	_, _, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.Len(t, got, 1, "终局失败应恰好下沉一条死信")
	require.Equal(t, "n1", got[0].NodeID)
	require.Equal(t, RuleChainFilterThreshold, got[0].NodeType)
	require.Equal(t, "c1", got[0].ChainID)
	require.Equal(t, "exec-1", got[0].ExecID)
	require.Equal(t, "t1", got[0].TenantID)
	require.Equal(t, "d1", got[0].DeviceID)
	require.Equal(t, 2, got[0].Attempts)
	require.NotEmpty(t, got[0].Error)
	require.False(t, got[0].CreatedAt.IsZero())
}

func TestRuleChainNodePolicyDeadLetterCanBeOptedOut(t *testing.T) {
	var got []RuleChainDeadLetter
	ruleChainDeadLetterSink = func(dl RuleChainDeadLetter) { got = append(got, dl) }
	t.Cleanup(func() { ruleChainDeadLetterSink = nil })

	node := &RuleChainNode{ID: "n1", Type: RuleChainFilterThreshold, Config: map[string]any{
		"policy": map[string]any{"max_attempts": float64(2), "backoff_ms": float64(0), "dead_letter": false},
	}}
	_, _, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.Error(t, err)
	require.Empty(t, got, "显式关闭死信后不应下沉")
}

func TestRuleChainNodePolicySuccessDoesNotEmitDeadLetter(t *testing.T) {
	var got []RuleChainDeadLetter
	ruleChainDeadLetterSink = func(dl RuleChainDeadLetter) { got = append(got, dl) }
	t.Cleanup(func() { ruleChainDeadLetterSink = nil })

	node := &RuleChainNode{ID: "n1", Type: RuleChainTriggerTelemetry}
	_, attempt, err := policyTestExec(context.Background(), "c1").executeNodeWithPolicy(node, policyTestMsg())
	require.NoError(t, err)
	require.Equal(t, 1, attempt.Attempts)
	require.Empty(t, got, "成功路径不得产生死信")
}
