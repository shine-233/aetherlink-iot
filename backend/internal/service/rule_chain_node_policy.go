// 文件用途：规则链节点级执行策略（ROADMAP P1.2 规则链可靠性）。
//
// 核心逻辑：节点可用 config.policy 声明 {timeout_ms, max_attempts, backoff_ms,
// dead_letter, retry_safe}，引擎据此施加独立超时、指数退避重试与终局死信下沉。
//
// 关键注意事项：
//   - 会产生不可逆外部副作用的节点（action/external，以及注册表外的未知类型）
//     默认**禁止重试**，除非显式声明 retry_safe=true。重试一次设备命令等于两次下发，
//     重试一次 webhook 等于两次业务投递——这类静默重试会把重复副作用伪装成成功。
//   - 退避等待必须响应上下文取消；绝不越过 deadline 继续睡眠，宁可放弃剩余重试。
//   - 死信只落标识与错误摘要，不落完整载荷（沿用 trace 的审计最小化约定）。
//   - 策略值非法一律 fail closed 返回错误，不静默截断或按默认值兜底。
package service

import (
	"context"
	"fmt"
	"time"
)

const (
	ruleChainPolicyTimeoutDefault     = 2 * time.Second
	ruleChainPolicyMaxAttemptsDefault = 1
	ruleChainPolicyBackoffDefault     = 200 * time.Millisecond

	// ruleChainPolicyMaxAttemptsLimit 重试上限：含首次执行，超出即拒绝。
	ruleChainPolicyMaxAttemptsLimit = 5
	// ruleChainPolicyBackoffCeiling 单次退避等待上限，避免指数增长后长时间挂住节点。
	ruleChainPolicyBackoffCeiling = 5 * time.Second
	// ruleChainPolicyTimeoutFloor 超时下限，防止 0/负值退化成立即超时。
	ruleChainPolicyTimeoutFloor = time.Millisecond
)

// RuleChainNodePolicy 单节点执行策略。
type RuleChainNodePolicy struct {
	Timeout time.Duration
	// MaxAttempts 含首次执行的总尝试次数。
	MaxAttempts int
	// Backoff 首次退避基数，实际等待按 Backoff * 2^(n-1) 指数增长并受上限约束。
	Backoff time.Duration
	// DeadLetter 终局失败是否下沉死信（默认 true）。
	DeadLetter bool
	// RetrySafe 仅对有副作用节点生效：显式声明"重试不会产生重复副作用"。
	RetrySafe bool
}

// RuleChainNodeAttempt 一次节点执行的重试观测（P1.2 门禁：重试次数与延迟可观测）。
type RuleChainNodeAttempt struct {
	Attempts  int
	BackoffMs int64
}

// RuleChainDeadLetter 终局失败下沉记录。刻意不含消息载荷，避免死信成为第二份数据副本。
type RuleChainDeadLetter struct {
	ExecID    string
	ChainID   string
	NodeID    string
	NodeType  string
	TenantID  string
	DeviceID  string
	Error     string
	Attempts  int
	CreatedAt time.Time
}

// ruleChainDeadLetterSink 死信落点注入点；nil 表示未接线（旁路，不改变现有行为）。
var ruleChainDeadLetterSink func(dl RuleChainDeadLetter)

func defaultRuleChainNodePolicy() RuleChainNodePolicy {
	return RuleChainNodePolicy{
		Timeout:     ruleChainPolicyTimeoutDefault,
		MaxAttempts: ruleChainPolicyMaxAttemptsDefault,
		Backoff:     ruleChainPolicyBackoffDefault,
		DeadLetter:  true,
	}
}

// parseRuleChainNodePolicy 解析节点 config.policy。字段缺失用默认值，出现即必须合法。
func parseRuleChainNodePolicy(cfg map[string]any) (RuleChainNodePolicy, error) {
	policy := defaultRuleChainNodePolicy()
	if len(cfg) == 0 {
		return policy, nil
	}
	raw, ok := cfg["policy"]
	if !ok || raw == nil {
		return policy, nil
	}
	section, ok := raw.(map[string]any)
	if !ok {
		return RuleChainNodePolicy{}, fmt.Errorf("policy must be an object")
	}

	if v, ok := section["timeout_ms"]; ok {
		ms, err := policyInt(v, "timeout_ms")
		if err != nil {
			return RuleChainNodePolicy{}, err
		}
		d := time.Duration(ms) * time.Millisecond
		if d < ruleChainPolicyTimeoutFloor {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.timeout_ms must be >= %d", ruleChainPolicyTimeoutFloor.Milliseconds())
		}
		if d > ruleChainExecTimeout {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.timeout_ms must not exceed chain timeout %dms", ruleChainExecTimeout.Milliseconds())
		}
		policy.Timeout = d
	}

	if v, ok := section["max_attempts"]; ok {
		n, err := policyInt(v, "max_attempts")
		if err != nil {
			return RuleChainNodePolicy{}, err
		}
		if n < 1 || n > ruleChainPolicyMaxAttemptsLimit {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.max_attempts must be within [1,%d]", ruleChainPolicyMaxAttemptsLimit)
		}
		policy.MaxAttempts = n
	}

	if v, ok := section["backoff_ms"]; ok {
		ms, err := policyInt(v, "backoff_ms")
		if err != nil {
			return RuleChainNodePolicy{}, err
		}
		if ms < 0 {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.backoff_ms must be >= 0")
		}
		d := time.Duration(ms) * time.Millisecond
		if d > ruleChainPolicyBackoffCeiling {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.backoff_ms must not exceed %d", ruleChainPolicyBackoffCeiling.Milliseconds())
		}
		policy.Backoff = d
	}

	if v, ok := section["dead_letter"]; ok {
		b, ok := v.(bool)
		if !ok {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.dead_letter must be a bool")
		}
		policy.DeadLetter = b
	}

	if v, ok := section["retry_safe"]; ok {
		b, ok := v.(bool)
		if !ok {
			return RuleChainNodePolicy{}, fmt.Errorf("policy.retry_safe must be a bool")
		}
		policy.RetrySafe = b
	}

	return policy, nil
}

// policyInt 兼容 JSON float64 / int / json.Number 三种数值形态。
func policyInt(raw any, field string) (int, error) {
	var f float64
	switch v := raw.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("policy.%s must be a number", field)
	}
	if f != float64(int(f)) {
		return 0, fmt.Errorf("policy.%s must be an integer", field)
	}
	return int(f), nil
}

// ruleChainNodeKind 查节点类型所属分类，未知类型返回空串。
func ruleChainNodeKind(nodeType string) string {
	for _, spec := range ruleChainNodeSpecs {
		if spec.Type == nodeType {
			return spec.Kind
		}
	}
	return ""
}

// ruleChainNodeSideEffectful 判断节点是否会产生不可逆外部副作用。
// 未知类型按有副作用处理——无法证明安全即禁止自动重试。
func ruleChainNodeSideEffectful(nodeType string) bool {
	switch ruleChainNodeKind(nodeType) {
	case RuleChainKindAction, RuleChainKindExternal:
		return true
	case RuleChainKindTrigger, RuleChainKindFilter, RuleChainKindTransform,
		RuleChainKindEnrichment, RuleChainKindFlow, RuleChainKindAnalytics:
		return false
	default:
		return true
	}
}

// effectiveMaxAttempts 最终生效的尝试次数：有副作用且未声明 retry_safe 的节点一律不重试。
func effectiveMaxAttempts(policy RuleChainNodePolicy, nodeType string) int {
	if policy.MaxAttempts <= 1 {
		return 1
	}
	if ruleChainNodeSideEffectful(nodeType) && !policy.RetrySafe {
		return 1
	}
	return policy.MaxAttempts
}

// executeNodeWithPolicy 施加节点级超时、退避重试与死信下沉后执行单节点。
func (e *ruleChainExecution) executeNodeWithPolicy(node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, RuleChainNodeAttempt, error) {
	policy, err := parseRuleChainNodePolicy(node.Config)
	if err != nil {
		return ruleChainNodeResult{}, RuleChainNodeAttempt{}, err
	}
	attempts := effectiveMaxAttempts(policy, node.Type)

	var waited time.Duration
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := e.ctx.Err(); err != nil {
			return ruleChainNodeResult{}, RuleChainNodeAttempt{Attempts: i - 1, BackoffMs: waited.Milliseconds()}, err
		}
		result, err := func() (ruleChainNodeResult, error) {
			nodeCtx, cancel := context.WithTimeout(e.ctx, policy.Timeout)
			defer cancel()
			sub := *e
			sub.ctx = nodeCtx
			return sub.executeNode(node, msg)
		}()
		if err == nil {
			return result, RuleChainNodeAttempt{Attempts: i, BackoffMs: waited.Milliseconds()}, nil
		}
		lastErr = err
		if i == attempts {
			break
		}
		delay := policy.Backoff << (i - 1)
		if delay > ruleChainPolicyBackoffCeiling || delay < 0 {
			delay = ruleChainPolicyBackoffCeiling
		}
		// 不越过整链 deadline 继续等待：剩余时间不足即放弃重试，避免睡过超时窗口。
		if deadline, ok := e.ctx.Deadline(); ok && delay >= time.Until(deadline) {
			break
		}
		select {
		case <-time.After(delay):
			waited += delay
		case <-e.ctx.Done():
			return ruleChainNodeResult{}, RuleChainNodeAttempt{Attempts: i, BackoffMs: waited.Milliseconds()}, e.ctx.Err()
		}
	}

	if policy.DeadLetter {
		e.emitRuleChainDeadLetter(node, msg, lastErr, attempts)
	}
	return ruleChainNodeResult{}, RuleChainNodeAttempt{Attempts: attempts, BackoffMs: waited.Milliseconds()}, lastErr
}

// emitRuleChainDeadLetter 下沉终局失败；sink 未接线时只留调试日志。
func (e *ruleChainExecution) emitRuleChainDeadLetter(node *RuleChainNode, msg ruleChainMessage, err error, attempts int) {
	if ruleChainDeadLetterSink == nil {
		return
	}
	chainID := ""
	if e.graph != nil {
		chainID = e.graph.ChainID
	}
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	tenantID := ""
	deviceID := ""
	if msg.Rcc != nil {
		tenantID = msg.Rcc.TenantID
		deviceID = msg.Rcc.DeviceID
	}
	ruleChainDeadLetterSink(RuleChainDeadLetter{
		ExecID:    e.execID,
		ChainID:   chainID,
		NodeID:    node.ID,
		NodeType:  node.Type,
		TenantID:  tenantID,
		DeviceID:  deviceID,
		Error:     errText,
		Attempts:  attempts,
		CreatedAt: time.Now().UTC(),
	})
}
