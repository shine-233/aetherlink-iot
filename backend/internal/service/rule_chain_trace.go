package service

import (
	"os"
	"sync/atomic"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/spf13/viper"
)

// PHASE-D-D1 BEGIN 节点级调试 trace（D1 规则引擎 2.0）
//
// 设计：
//   - 默认关闭（ruleChainTraceEnabled=false），热路径零开销旁路；
//   - 开启且 writer 就绪时，trace 异步落库（独立 goroutine + recover，
//     绝不让 trace 失败影响消息执行）；
//   - 错误摘要截断到 500 字符，且只落错误文本，不落完整载荷（审计最小化）。

var traceEnabledCached = atomic.Bool{}

func isTraceGloballyEnabled() bool {
	if traceEnabledCached.Load() {
		return true
	}
	if viper.GetBool("rule-chain.trace-enabled") || viper.GetBool("rule_chain.trace_enabled") {
		return true
	}
	val := os.Getenv("AETHERLINK_RULE_CHAIN_TRACE_ENABLED")
	return val == "true" || val == "1"
}

// ruleChainTraceEnabled trace 总开关（viper rule-chain.trace-enabled 或环境变量，默认关闭）。
var ruleChainTraceEnabled func() bool = isTraceGloballyEnabled

// SetRuleChainTraceEnabled 动态调整 trace 开关（供测试与运维控制）。
func SetRuleChainTraceEnabled(enabled bool) {
	traceEnabledCached.Store(enabled)
	ruleChainTraceEnabled = func() bool { return enabled }
}

func shouldRecordNodeTrace(node *RuleChainNode) bool {
	if ruleChainTraceEnabled() {
		return true
	}
	if node != nil && node.Config != nil {
		if d, ok := node.Config["debug"].(bool); ok && d {
			return true
		}
		if t, ok := node.Config["trace"].(bool); ok && t {
			return true
		}
	}
	return false
}

// ruleChainTraceWriter trace 落库注入点（67.sql 表）；nil 时即使开关打开也旁路。
var ruleChainTraceWriter = func(trace *model.RuleChainNodeTrace) error {
	return createRuleChainNodeTraceRow(trace)
}

// ruleChainTraceErrorMax 错误摘要截断长度。
const ruleChainTraceErrorMax = 500

// recordRuleChainNodeTrace 引擎每次节点执行后的 trace 挂点。
func recordRuleChainNodeTrace(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage, result ruleChainNodeResult, nodeErr error, elapsed time.Duration) {
	if !shouldRecordNodeTrace(node) || ruleChainTraceWriter == nil {
		return
	}
	chainID := ""
	tenantID := ""
	if e != nil && e.graph != nil {
		chainID = e.graph.ChainID
	}
	rcc := msg.Rcc
	if rcc != nil {
		tenantID = rcc.TenantID
	}
	trace := &model.RuleChainNodeTrace{
		ID:        uuid.New(),
		ExecID:    e.execID,
		ChainID:   chainID,
		NodeID:    node.ID,
		NodeType:  node.Type,
		Pass:      result.pass && nodeErr == nil,
		ElapsedMs: elapsed.Milliseconds(),
		TenantID:  tenantID,
		CreatedAt: time.Now().UTC(),
	}
	if nodeErr != nil {
		msgText := nodeErr.Error()
		if len(msgText) > ruleChainTraceErrorMax {
			msgText = msgText[:ruleChainTraceErrorMax]
		}
		trace.ErrorMsg = &msgText
	}
	go func() {
		defer func() {
			// trace 绝不反向影响消息管道。
			_ = recover()
		}()
		if err := ruleChainTraceWriter(trace); err != nil {
			logNodeDebug(chainID, node.ID, "write node trace failed: %v", err)
		}
	}()
}

// createRuleChainNodeTraceRow 默认落库实现（租户维度由写入值自带）。
func createRuleChainNodeTraceRow(trace *model.RuleChainNodeTrace) error {
	return createRuleChainRow(model.TableNameRuleChainNodeTrace, trace)
}

// PHASE-D-D1 END
