// 文件用途：规则链输入回放与副作用确认（ROADMAP P1.2）。
//
// 核心逻辑：执行时可注入 recorder 捕获"节点 + 当时输入"快照；回放按记录顺序
// 用记录到的输入重跑每个节点。命中会产生副作用的节点时，必须显式确认才放行。
//
// 关键注意事项：
//   - 回放必然要留存输入，这与 trace 的"审计最小化"不是一回事：trace 只记事实，
//     replay 记输入。因此 recorder **默认不接线**（nil 即旁路、热路径零开销），
//     只有运维显式接入持久化时才留存载荷——默认状态不产生第二份数据副本。
//   - 回放不沿图继续遍历：后继节点各有自己的记录，跟着边走会把下游重复执行 N 遍。
//   - 节点类型发生漂移时拒绝用旧输入重跑——输入是按旧类型语义捕获的。
//   - 放行副作用重放时会在 metadata 打 rc_replay / rc_replay_of，
//     让下游与审计能区分"首次执行"与"重放产生的副作用"。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// 回放产生的副作用标记（下游与审计据此区分首次执行与重放）。
const (
	ruleChainMetaReplay   = "rc_replay"
	ruleChainMetaReplayOf = "rc_replay_of"
)

// RuleChainReplayRecord 单节点一次执行的输入快照。
type RuleChainReplayRecord struct {
	ExecID   string
	ChainID  string
	NodeID   string
	NodeType string
	// TenantID 记录所属租户。回放留存的是原始输入载荷，敏感性与遥测同级，
	// 落库必须带租户隔离，否则跨租户可回放别人的输入。
	TenantID string
	Payload  map[string]any
	Metadata map[string]any
	Pass     bool
	Error    string
	At       time.Time
}

// RuleChainReplayOptions 回放选项。
type RuleChainReplayOptions struct {
	// ReplayOf 被回放的原始执行 ID，必填——没有来源的回放无法审计。
	ReplayOf string
	// ConfirmSideEffects 显式确认"允许重放副作用"。命中副作用节点且为 false 时直接拒绝。
	ConfirmSideEffects bool
}

// ruleChainReplayRecorder 回放记录落点注入点；nil 表示未接线（旁路）。
var ruleChainReplayRecorder func(rec RuleChainReplayRecord)

// recordRuleChainReplayInput 引擎每次节点执行后的回放记录挂点（未接线时零开销）。
func recordRuleChainReplayInput(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage, result ruleChainNodeResult, nodeErr error) {
	if ruleChainReplayRecorder == nil || e == nil || node == nil {
		return
	}
	chainID := ""
	if e.graph != nil {
		chainID = e.graph.ChainID
	}
	errText := ""
	if nodeErr != nil {
		errText = nodeErr.Error()
	}
	tenantID := ""
	if msg.Rcc != nil {
		tenantID = msg.Rcc.TenantID
	}
	ruleChainReplayRecorder(RuleChainReplayRecord{
		ExecID:   e.execID,
		ChainID:  chainID,
		NodeID:   node.ID,
		NodeType: node.Type,
		TenantID: tenantID,
		Payload:  msg.Payload,
		Metadata: msg.Metadata,
		Pass:     result.pass && nodeErr == nil,
		Error:    errText,
		At:       time.Now().UTC(),
	})
}

// ReplayRuleChainExecution 按捕获记录重跑节点。
// 返回 (节点错误列表, 闸门错误)；闸门错误表示回放被拒绝，一个节点都没跑。
func ReplayRuleChainExecution(ctx context.Context, graph *RuleChainGraph, records []RuleChainReplayRecord, rcc *RuleChainContext, opts RuleChainReplayOptions) ([]error, error) {
	if graph == nil {
		return nil, fmt.Errorf("replay requires a graph")
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("replay requires at least one record")
	}
	if strings.TrimSpace(opts.ReplayOf) == "" {
		return nil, fmt.Errorf("replay requires the source execution id")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 副作用闸门：先用全部记录判定，命中即整体拒绝——绝不放行到一半才发现有副作用。
	sideEffectNodes := make([]string, 0, 2)
	for _, rec := range records {
		if ruleChainNodeSideEffectful(rec.NodeType) {
			sideEffectNodes = append(sideEffectNodes, fmt.Sprintf("%s(%s)", rec.NodeID, rec.NodeType))
		}
	}
	if len(sideEffectNodes) > 0 && !opts.ConfirmSideEffects {
		return nil, fmt.Errorf("replay would re-run side-effect nodes [%s]; set ConfirmSideEffects to proceed",
			strings.Join(sideEffectNodes, ", "))
	}

	execCtx, cancel := context.WithTimeout(ctx, ruleChainExecTimeout)
	defer cancel()
	exec := newRuleChainExecution(execCtx, graph, "")
	errs := make([]error, 0)

	for _, rec := range records {
		if err := exec.ctx.Err(); err != nil {
			return errs, err
		}
		node := graph.NodeByID(rec.NodeID)
		if node == nil {
			errs = append(errs, fmt.Errorf("replay: node %s no longer exists in graph", rec.NodeID))
			continue
		}
		// 类型漂移：输入是按旧类型语义捕获的，换类型后重跑可能语义错乱。
		if rec.NodeType != node.Type {
			errs = append(errs, fmt.Errorf("replay: node %s type changed %s -> %s, captured input no longer applies",
				rec.NodeID, rec.NodeType, node.Type))
			continue
		}
		msg := ruleChainMessage{Payload: rec.Payload, Metadata: rec.Metadata, Rcc: rcc}
		if ruleChainNodeSideEffectful(node.Type) {
			msg.Metadata = replayMarkedMetadata(rec.Metadata, opts.ReplayOf)
		}
		if _, _, err := exec.executeNodeWithPolicy(node, msg); err != nil {
			errs = append(errs, fmt.Errorf("replay: node %s(%s): %w", node.ID, node.Type, err))
		}
	}
	return errs, nil
}

// replayMarkedMetadata 给副作用节点打重放标记，使重放产生的副作用可被追溯。
func replayMarkedMetadata(base map[string]any, replayOf string) map[string]any {
	meta := make(map[string]any, len(base)+2)
	for k, v := range base {
		meta[k] = v
	}
	meta[ruleChainMetaReplay] = true
	meta[ruleChainMetaReplayOf] = replayOf
	return meta
}
