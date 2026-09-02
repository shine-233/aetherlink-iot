// 文件用途：规则链 DAG 图模型、解析与校验（ROADMAP B2）。
// 核心逻辑：graph JSON = {nodes:[{id,type,config}], edges:[{from,to}]}；
//
//	校验含唯一性、边引用完整性、触发器存在性与 Kahn 拓扑无环检查。
//
// 关键注意事项：节点类型注册表是扩展点——新增类型需同时实现执行 handler。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	RuleChainTriggerTelemetry = "trigger.telemetry"
	RuleChainTriggerOnline    = "trigger.device_online"
	RuleChainFilterThreshold  = "filter.threshold"
	RuleChainTransformMapping = "transform.mapping"
	RuleChainActionWebhook    = "action.webhook"
	RuleChainActionCommand    = "action.command"
	RuleChainActionAlarm      = "action.alarm"
	ruleChainMaxNodes         = 64
	ruleChainMaxEdges         = 128
	ruleChainMaxGraphBytes    = 256 * 1024
)

// RuleChainNodeTypeMeta 内置节点类型注册表（前端画布与后端校验共用语义）。
var RuleChainNodeTypeMeta = map[string]string{
	RuleChainTriggerTelemetry: "trigger",
	RuleChainTriggerOnline:    "trigger",
	RuleChainFilterThreshold:  "filter",
	RuleChainTransformMapping: "transform",
	RuleChainActionWebhook:    "action",
	RuleChainActionCommand:    "action",
	RuleChainActionAlarm:      "action",
}

// RuleChainGraph DAG 定义。
type RuleChainGraph struct {
	Nodes []RuleChainNode `json:"nodes"`
	Edges []RuleChainEdge `json:"edges"`
}

// RuleChainNode 单个节点。
type RuleChainNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

// RuleChainEdge 有向边。
type RuleChainEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ParseRuleChainGraph 解析并校验 graph 文本。
func ParseRuleChainGraph(raw string) (*RuleChainGraph, error) {
	if len(raw) > ruleChainMaxGraphBytes {
		return nil, fmt.Errorf("graph exceeds size limit")
	}
	rawTrimmed := strings.TrimSpace(raw)
	if rawTrimmed == "" {
		rawTrimmed = "{}"
	}
	var graph RuleChainGraph
	if err := json.Unmarshal([]byte(rawTrimmed), &graph); err != nil {
		return nil, fmt.Errorf("graph is not valid json: %w", err)
	}
	if err := graph.Validate(); err != nil {
		return nil, err
	}
	return &graph, nil
}

// Validate 校验节点/边结构与 DAG 无环约束。
func (g *RuleChainGraph) Validate() error {
	if len(g.Nodes) == 0 {
		return fmt.Errorf("graph.nodes must not be empty")
	}
	if len(g.Nodes) > ruleChainMaxNodes {
		return fmt.Errorf("graph.nodes exceeds limit %d", ruleChainMaxNodes)
	}
	if len(g.Edges) > ruleChainMaxEdges {
		return fmt.Errorf("graph.edges exceeds limit %d", ruleChainMaxEdges)
	}
	seen := make(map[string]bool, len(g.Nodes))
	hasTrigger := false
	for i := range g.Nodes {
		node := &g.Nodes[i]
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("nodes[%d].id is required", i)
		}
		if seen[node.ID] {
			return fmt.Errorf("duplicate node id %q", node.ID)
		}
		seen[node.ID] = true
		kind, ok := RuleChainNodeTypeMeta[node.Type]
		if !ok {
			return fmt.Errorf("node %q has unknown type %q", node.ID, node.Type)
		}
		if kind == "trigger" {
			hasTrigger = true
		}
	}
	if !hasTrigger {
		return fmt.Errorf("graph needs at least one trigger node")
	}
	for i := range g.Edges {
		edge := &g.Edges[i]
		if !seen[edge.From] || !seen[edge.To] {
			return fmt.Errorf("edge %d references unknown node (%q -> %q)", i, edge.From, edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("edge %d is self-loop", i)
		}
	}
	return g.validateAcyclic()
}

// validateAcyclic Kahn 算法拓扑排序；无法消完的节点即处于环中。
func (g *RuleChainGraph) validateAcyclic() error {
	indegree := make(map[string]int, len(g.Nodes))
	adjacency := make(map[string][]string, len(g.Nodes))
	for _, node := range g.Nodes {
		indegree[node.ID] += 0
	}
	for _, edge := range g.Edges {
		indegree[edge.To]++
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}
	queue := make([]string, 0, len(g.Nodes))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
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

// Roots 返回入度为零的节点（执行入口）。
func (g *RuleChainGraph) Roots() []*RuleChainNode {
	indegree := make(map[string]int, len(g.Nodes))
	for _, node := range g.Nodes {
		indegree[node.ID] += 0
	}
	for _, edge := range g.Edges {
		indegree[edge.To]++
	}
	roots := make([]*RuleChainNode, 0, 1)
	for i := range g.Nodes {
		if indegree[g.Nodes[i].ID] == 0 {
			roots = append(roots, &g.Nodes[i])
		}
	}
	return roots
}

// Successors 返回节点的下游节点（按边顺序）。
func (g *RuleChainGraph) Successors(nodeID string) []*RuleChainNode {
	result := make([]*RuleChainNode, 0, 2)
	for _, edge := range g.Edges {
		if edge.From != nodeID {
			continue
		}
		for i := range g.Nodes {
			if g.Nodes[i].ID == edge.To {
				result = append(result, &g.Nodes[i])
				break
			}
		}
	}
	return result
}

// NodeByID 按ID查找节点。
func (g *RuleChainGraph) NodeByID(id string) *RuleChainNode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}
