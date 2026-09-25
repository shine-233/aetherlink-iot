// 文件用途：边缘本地规则执行器（TB-21 scoped v1）——规则链 Graph JSON 的边缘子集解析。
// 核心逻辑：解析 edge_sync 下发的 chain.Graph 快照（{nodes:[{id,type,config}],edges:[{from,to,kind}]}），
//
//	仅支持 trigger.telemetry / filter.threshold / action.alarm 三种节点；
//	结构校验（唯一性/边引用/自环/上限/Kahn 无环）与配置校验（阈值/告警）在装载期一次完成。
//
// 关键注意事项：
//   - 云端 28 种节点类型在此只认子集；未知类型节点解析通过但在求值时阻断该路径（fail-closed，
//     告警绝不跨过未求值的条件节点），见 eval.go。
//   - 语义对照 internal/service/rule_chain_graph.go（云端权威实现）；此处刻意不 import service，
//     边缘包保持零 DB/零 broker 依赖（纯 stdlib + uuid）。
//   - 阈值 op 白名单在装载期强制（云端是运行期报错）：边缘 fail-closed，坏配置宁可拒收快照。
package edgerules

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 边缘支持的节点类型（与云端常量同值；对照 service/rule_chain_graph.go 与 rule_chain_nodes.go）。
const (
	NodeTypeTriggerTelemetry = "trigger.telemetry"
	NodeTypeFilterThreshold  = "filter.threshold"
	NodeTypeActionAlarm      = "action.alarm"
)

// 边类型（对照云端 RuleChainEdgeKindSuccess/Failure；空值与 success 等价）。
const (
	EdgeKindSuccess = "success"
	EdgeKindFailure = "failure"
)

// 图规模上限（对照云端 ruleChainMaxNodes/Edges/GraphBytes，防止恶意快照撑爆边缘进程）。
const (
	MaxNodes      = 64
	MaxEdges      = 128
	MaxGraphBytes = 256 * 1024
)

// thresholdOps 边缘支持的阈值比较算子（云端求值支持的同六种；装载期白名单）。
var thresholdOps = map[string]bool{">": true, ">=": true, "<": true, "<=": true, "==": true, "!=": true}

// Graph 边缘视角的规则链 DAG 定义。
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node 单个节点（config 保留原始 map，求值时按类型解读）。
type Node struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

// Edge 有向边。v1 求值只走 success（含空值）边：filter.threshold 在边缘只产出
// 通过/不通过（缺失点位=不通过，对照云端），没有错误分支，failure 边结构上保留但不会走到。
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind,omitempty"`
}

// SupportedNodeTypes 返回边缘 v1 支持的节点类型集合（供调用方/测试探查）。
func SupportedNodeTypes() []string {
	return []string{NodeTypeTriggerTelemetry, NodeTypeFilterThreshold, NodeTypeActionAlarm}
}

// IsSupportedNodeType 类型是否在边缘子集内。
func IsSupportedNodeType(t string) bool {
	switch t {
	case NodeTypeTriggerTelemetry, NodeTypeFilterThreshold, NodeTypeActionAlarm:
		return true
	default:
		return false
	}
}

// ParseGraph 解析并校验边缘子集的 graph 文本。
// 结构性非法（坏 JSON/空节点/重复 ID/悬空边/自环/超限/成环）返回错误；
// 未知节点类型不算解析错误（装载成功、求值阻断），由 UnsupportedNodeTypes 暴露给调用方。
func ParseGraph(raw []byte) (*Graph, []string, error) {
	if len(raw) > MaxGraphBytes {
		return nil, nil, fmt.Errorf("graph exceeds size limit %d bytes", MaxGraphBytes)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, nil, fmt.Errorf("graph is empty")
	}
	var g Graph
	if err := json.Unmarshal([]byte(trimmed), &g); err != nil {
		return nil, nil, fmt.Errorf("graph is not valid json: %w", err)
	}
	unsupported, err := g.validate()
	if err != nil {
		return nil, nil, err
	}
	return &g, unsupported, nil
}

// validate 结构与配置校验，返回未知类型清单。
func (g *Graph) validate() ([]string, error) {
	if len(g.Nodes) == 0 {
		return nil, fmt.Errorf("graph.nodes must not be empty")
	}
	if len(g.Nodes) > MaxNodes {
		return nil, fmt.Errorf("graph.nodes exceeds limit %d", MaxNodes)
	}
	if len(g.Edges) > MaxEdges {
		return nil, fmt.Errorf("graph.edges exceeds limit %d", MaxEdges)
	}
	seen := make(map[string]bool, len(g.Nodes))
	unsupportedSet := map[string]bool{}
	for i := range g.Nodes {
		node := &g.Nodes[i]
		if strings.TrimSpace(node.ID) == "" {
			return nil, fmt.Errorf("nodes[%d].id is required", i)
		}
		if seen[node.ID] {
			return nil, fmt.Errorf("duplicate node id %q", node.ID)
		}
		seen[node.ID] = true
		switch node.Type {
		case NodeTypeFilterThreshold:
			if _, _, _, err := parseThresholdConfig(node.Config); err != nil {
				return nil, fmt.Errorf("threshold node %q: %w", node.ID, err)
			}
		case NodeTypeActionAlarm:
			if _, err := parseAlarmConfig(node.Config); err != nil {
				return nil, fmt.Errorf("alarm node %q: %w", node.ID, err)
			}
		case NodeTypeTriggerTelemetry:
			// 无配置约束。
		default:
			unsupportedSet[node.Type] = true
		}
	}
	for i := range g.Edges {
		edge := &g.Edges[i]
		if !seen[edge.From] || !seen[edge.To] {
			return nil, fmt.Errorf("edge %d references unknown node (%q -> %q)", i, edge.From, edge.To)
		}
		if edge.From == edge.To {
			return nil, fmt.Errorf("edge %d is self-loop", i)
		}
		switch edge.Kind {
		case "", EdgeKindSuccess, EdgeKindFailure:
		default:
			return nil, fmt.Errorf("edge %d has unknown kind %q", i, edge.Kind)
		}
	}
	// 根约束（对照云端根链校验）：已知触发节点必须是根；已知非触发节点不能是根。
	indegree := make(map[string]int, len(g.Nodes))
	for _, edge := range g.Edges {
		indegree[edge.To]++
	}
	for i := range g.Nodes {
		node := &g.Nodes[i]
		switch node.Type {
		case NodeTypeTriggerTelemetry:
			if indegree[node.ID] != 0 {
				return nil, fmt.Errorf("trigger node %q must be a root", node.ID)
			}
		case NodeTypeFilterThreshold, NodeTypeActionAlarm:
			if indegree[node.ID] == 0 {
				return nil, fmt.Errorf("non-trigger node %q cannot be a root", node.ID)
			}
		}
	}
	if err := g.validateAcyclic(); err != nil {
		return nil, err
	}
	unsupported := make([]string, 0, len(unsupportedSet))
	for t := range unsupportedSet {
		unsupported = append(unsupported, t)
	}
	return unsupported, nil
}

// validateAcyclic Kahn 拓扑排序；消不完的节点即在环中（对照云端实现）。
func (g *Graph) validateAcyclic() error {
	indegree := make(map[string]int, len(g.Nodes))
	adjacency := make(map[string][]string, len(g.Nodes))
	for _, edge := range g.Edges {
		indegree[edge.To]++
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}
	queue := make([]string, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		if indegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}
	consumed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		consumed++
		for _, next := range adjacency[current] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if consumed != len(g.Nodes) {
		return fmt.Errorf("graph contains a cycle")
	}
	return nil
}

// NodeByID 按 ID 查节点（执行器取节点配置用）。
func (g *Graph) NodeByID(id string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}

// parseThresholdConfig 解析 filter.threshold 配置 {key,op,value}（对照云端 parseThresholdConfig，
// 叠加 op 白名单与 value 必须有限的约束——边缘拒绝 NaN/Inf 阈值，避免比较结果不可预测）。
func parseThresholdConfig(cfg map[string]any) (string, string, float64, error) {
	if cfg == nil {
		return "", "", 0, fmt.Errorf("threshold config is required")
	}
	key, _ := cfg["key"].(string)
	op, _ := cfg["op"].(string)
	if strings.TrimSpace(key) == "" || op == "" {
		return "", "", 0, fmt.Errorf("threshold config requires key and op")
	}
	if !thresholdOps[op] {
		return "", "", 0, fmt.Errorf("unsupported operator %q", op)
	}
	threshold, ok := toFloat(cfg["value"])
	if !ok {
		return "", "", 0, fmt.Errorf("threshold config value must be a number")
	}
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return "", "", 0, fmt.Errorf("threshold config value must be finite")
	}
	return key, op, threshold, nil
}

// alarmConfig 解析后的 action.alarm 配置（对照云端 validateAlarmConfig 的 L/M/H 语义）。
type alarmConfig struct {
	Name             string
	Severity         string // "" 已归一化为 "H"
	Description      string
	Content          string
	RetriggerDedupMs float64 // 0=不去重；>0 窗口毫秒数
}

// parseAlarmConfig 解析 action.alarm 配置 {name(必填), severity(L/M/H,默认H), description,
// content, retrigger_dedup_ms(>=0)}；对照云端 rule_chain_nodes.go validateAlarmConfig。
func parseAlarmConfig(cfg map[string]any) (*alarmConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("alarm config is required")
	}
	name, _ := cfg["name"].(string)
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("alarm action requires name")
	}
	severity, _ := cfg["severity"].(string)
	switch severity {
	case "":
		severity = "H"
	case "L", "M", "H":
	default:
		return nil, fmt.Errorf("alarm action severity must be one of L/M/H")
	}
	dedupMs := 0.0
	if raw, exists := cfg["retrigger_dedup_ms"]; exists {
		v, ok := toFloat(raw)
		if !ok {
			return nil, fmt.Errorf("alarm retrigger_dedup_ms must be a number")
		}
		if v < 0 {
			return nil, fmt.Errorf("alarm retrigger_dedup_ms must not be negative")
		}
		dedupMs = v
	}
	description, _ := cfg["description"].(string)
	content, _ := cfg["content"].(string)
	return &alarmConfig{
		Name:             name,
		Severity:         severity,
		Description:      description,
		Content:          content,
		RetriggerDedupMs: dedupMs,
	}, nil
}

// toFloat 数值归一化（对照云端 rule_chain_engine.go toFloat：支持
// float/int/json.Number/可解析数字字符串，其余不认）。
func toFloat(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
