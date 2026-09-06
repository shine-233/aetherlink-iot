package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/go-basic/uuid"
)

// PHASE-D-D1 BEGIN Flow 流控节点 handler（D1 规则引擎 2.0）

// ruleChainSubchainLoader 子链加载注入点：按租户+chain_id 读取启用链并解析图；
// 默认实现见 ruleChainLoadSubchainGraph。
var ruleChainSubchainLoader = func(ctx context.Context, tenantID, chainID string) (*RuleChainGraph, error) {
	return ruleChainLoadSubchainGraph(ctx, tenantID, chainID)
}

// ruleChainFlowSubchain flow.subchain：执行嵌套子链（同 rcc 上下文）。
// 防环：chain_id 已在执行栈中则拒绝；深度超 ruleChainMaxSubchainDepth 拒绝。
// 子链错误聚合上抛（不改写根因），分支通过语义 = 子链无错误。
func ruleChainFlowSubchain(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, error) {
	rcc := msg.Rcc
	if rcc == nil {
		rcc = &RuleChainContext{}
	}
	chainID, _ := node.Config["chain_id"].(string)
	chainID = strings.TrimSpace(chainID)
	if chainID == "" {
		return ruleChainNodeResult{}, fmt.Errorf("subchain config requires chain_id")
	}
	for _, entered := range e.stack {
		if entered == chainID {
			return ruleChainNodeResult{}, fmt.Errorf("subchain cycle detected: %s already in stack", chainID)
		}
	}
	if len(e.stack) >= ruleChainMaxSubchainDepth {
		return ruleChainNodeResult{}, fmt.Errorf("subchain depth exceeds limit %d", ruleChainMaxSubchainDepth)
	}

	subGraph, err := ruleChainSubchainLoader(e.ctx, rcc.TenantID, chainID)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("load subchain %s: %w", chainID, err)
	}
	if subGraph == nil {
		return ruleChainNodeResult{}, fmt.Errorf("subchain %s not found or disabled", chainID)
	}

	subExec := &ruleChainExecution{ctx: e.ctx, graph: subGraph, execID: e.execID, stack: append(append([]string{}, e.stack...), chainID)}
	subErrs := subExec.walkAndCollect(msg.Payload, "", rcc)
	if len(subErrs) > 0 {
		return ruleChainNodeResult{}, fmt.Errorf("subchain %s: %w", chainID, subErrs[0])
	}
	// 子链错误已聚合上抛，这里以入站消息继续父链分支（父链后续节点接原始载荷）。
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: msg.Payload, metadata: msg.Metadata, rcc: rcc}}}, nil
}

// ruleChainLoadSubchainGraph 默认子链加载：租户守卫 + 仅启用链。
func ruleChainLoadSubchainGraph(ctx context.Context, tenantID, chainID string) (*RuleChainGraph, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	graph, err := dalGetRuleChainGraphEnabled(ctx, tenantID, chainID)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

// dalGetRuleChainGraphEnabled 从 rule_chains 读启用链 graph 并解析（回填 ChainID）。
func dalGetRuleChainGraphEnabled(ctx context.Context, tenantID, chainID string) (*RuleChainGraph, error) {
	var row model.RuleChain
	err := global.DB.WithContext(ctx).
		Table(model.TableNameRuleChain).
		Where("id = ? AND tenant_id = ? AND enabled = ?", chainID, tenantID, true).
		Take(&row).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	graph, err := ParseRuleChainSubgraph(string(row.Graph))
	if err != nil {
		return nil, err
	}
	graph.ChainID = row.ID
	return graph, nil
}

// ruleChainFlowDelay flow.delay：可配置延时（上限 ruleChainMaxDelayMs），
// 等待期间响应执行超时取消。
func ruleChainFlowDelay(ctx context.Context, node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	durationMs, ok := toFloat(node.Config["duration_ms"])
	if !ok || durationMs <= 0 || durationMs > float64(ruleChainMaxDelayMs) {
		return ruleChainNodeResult{}, fmt.Errorf("delay duration_ms must be in (0,%d]", ruleChainMaxDelayMs)
	}
	timer := time.NewTimer(time.Duration(durationMs * float64(time.Millisecond)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ruleChainNodeResult{}, ctx.Err()
	case <-timer.C:
		return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
}

// ruleChainFlowCheckpoint flow.checkpoint：把当前消息持久化为检查点后继续分支。
// 落库失败即节点失败（检查点语义 = 必须落住才能向下走）。
func ruleChainFlowCheckpoint(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, error) {
	rcc := msg.Rcc
	if rcc == nil {
		rcc = &RuleChainContext{}
	}
	payloadJSON, err := marshalRuleChainPayload(msg.Payload)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("marshal checkpoint payload: %w", err)
	}
	cp := &model.RuleChainCheckpoint{
		ID:       uuid.New(),
		ExecID:   e.execID,
		ChainID:  chainIDOfExecution(e),
		NodeID:   node.ID,
		TenantID: rcc.TenantID,
		DeviceID: rcc.DeviceID,
		Payload:  payloadJSON,
	}
	if err := ruleChainCheckpointWriter(e.ctx, cp); err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("write checkpoint: %w", err)
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: msg.Payload, metadata: msg.Metadata, rcc: rcc}}}, nil
}

// createRuleChainCheckpointRow 默认检查点落库。
func createRuleChainCheckpointRow(ctx context.Context, cp *model.RuleChainCheckpoint) error {
	return createRuleChainRow(model.TableNameRuleChainCheckpoint, cp)
}

// chainIDOfExecution 取执行当前栈顶链 ID。
func chainIDOfExecution(e *ruleChainExecution) string {
	if e == nil || e.graph == nil {
		return ""
	}
	return e.graph.ChainID
}

// PHASE-D-D1 END
