// 文件用途：边缘本地规则求值（TB-21 scoped v1）——遥测到达时从触发器沿链走图。
// 核心逻辑：从所有 trigger.telemetry 根出发沿 success 边 BFS（节点级 visited 防环/防指数扇出），
//
//	队列项携带"路径上最后通过的阈值条件"文本；filter.threshold 求值语义逐条对照云端
//	ruleChainFilterThreshold（key 缺失/非数值=不通过，六种比较算子）；走到 action.alarm 即命中。
//
// 关键注意事项：
//   - 未知类型节点=阻断该路径并记入 Blocked（fail-closed：告警绝不跨过未求值的条件节点）；
//     trigger.device_online 不在 v1 范围（边缘执行器由遥测驱动），其下游同样视为阻断。
//   - 简化版告警语义（对照 calcfield 多严重度阶梯/升级/自动清除）：单节点单条件，
//     表达式只支持"遥测key 比较阈值"；条件文本随事件留痕（对照 calcfield 的 detail content）。
//   - 纯函数：不碰时钟/存储，去重与落盘都在 executor.go。
package edgerules

import "fmt"

// Hit 一次告警命中（图视角的原始产物，未带设备/去重上下文）。
type Hit struct {
	NodeID      string         `json:"node_id"`
	AlarmName   string         `json:"alarm_name"`
	Severity    string         `json:"severity"`
	Description string         `json:"description,omitempty"`
	Content     string         `json:"content,omitempty"`
	Condition   string         `json:"condition,omitempty"` // 路径上最后通过的阈值条件，形如 "temperature > 80"
	Values      map[string]any `json:"values"`              // 命中时的遥测快照（浅拷贝）
}

// EvalResult 一次遥测求值的完整产物（供调用方观测/测试断言）。
type EvalResult struct {
	Fired     []Hit    `json:"fired,omitempty"`     // 命中的告警节点
	Blocked   []string `json:"blocked,omitempty"`   // 因未知类型被阻断的节点
	Evaluated []string `json:"evaluated,omitempty"` // 实际求值过的节点（含未通过的过滤）
}

// walkItem BFS 队列项：节点 ID + 路径上最后通过的阈值条件文本（随路径传递，菱形图不串味）。
type walkItem struct {
	nodeID string
	cond   string
}

// Evaluate 用一份遥测对一张图求值。graph 必须来自 ParseGraph（未校验的图行为未定义）。
func Evaluate(g *Graph, values map[string]any) *EvalResult {
	result := &EvalResult{}
	if g == nil || len(g.Nodes) == 0 || len(values) == 0 {
		return result
	}
	byID := make(map[string]*Node, len(g.Nodes))
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	visited := make(map[string]bool, len(g.Nodes))
	queue := make([]walkItem, 0, 1)
	for i := range g.Nodes {
		// 只有 trigger.telemetry 是 v1 的入口；其余触发类型（device_online 等）不驱动遥测求值。
		if g.Nodes[i].Type == NodeTypeTriggerTelemetry {
			queue = append(queue, walkItem{nodeID: g.Nodes[i].ID})
		}
	}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item.nodeID] {
			continue
		}
		visited[item.nodeID] = true
		node := byID[item.nodeID]
		switch node.Type {
		case NodeTypeTriggerTelemetry:
			result.Evaluated = append(result.Evaluated, item.nodeID)
			queue = appendSuccessors(g, item, byID, queue, item.cond)
		case NodeTypeFilterThreshold:
			result.Evaluated = append(result.Evaluated, item.nodeID)
			if cond, pass := passThreshold(node.Config, values); pass {
				queue = appendSuccessors(g, item, byID, queue, cond)
			}
			// 不通过=路径剪断（对照云端 pass=false），不是异常也不计入 Blocked。
		case NodeTypeActionAlarm:
			result.Evaluated = append(result.Evaluated, item.nodeID)
			cfg, err := parseAlarmConfig(node.Config)
			if err != nil {
				// 装载期已校验；走到这里说明图被就地篡改，按阻断处理而不是 panic。
				result.Blocked = append(result.Blocked, item.nodeID)
				continue
			}
			result.Fired = append(result.Fired, Hit{
				NodeID:      item.nodeID,
				AlarmName:   cfg.Name,
				Severity:    cfg.Severity,
				Description: cfg.Description,
				Content:     cfg.Content,
				Condition:   item.cond,
				Values:      copyValues(values),
			})
			queue = appendSuccessors(g, item, byID, queue, item.cond)
		default:
			// 未知类型（含 trigger.device_online 等未支持触发器）：阻断，不向下传播。
			result.Blocked = append(result.Blocked, item.nodeID)
		}
	}
	return result
}

// appendSuccessors 把节点的 success（含空 kind）下游以携带条件文本的方式追加进队列；
// failure 边 v1 不走。
func appendSuccessors(g *Graph, item walkItem, byID map[string]*Node, queue []walkItem, cond string) []walkItem {
	for _, edge := range g.Edges {
		if edge.From != item.nodeID || edge.Kind == EdgeKindFailure {
			continue
		}
		if _, ok := byID[edge.To]; ok {
			queue = append(queue, walkItem{nodeID: edge.To, cond: cond})
		}
	}
	return queue
}

// passThreshold 单条阈值判定（逐行对照云端 ruleChainFilterThreshold）：
// 点位缺失或非数值一律不通过；通过时返回人类可读条件文本（事件留痕用）。
func passThreshold(cfg map[string]any, values map[string]any) (string, bool) {
	key, op, threshold, err := parseThresholdConfig(cfg)
	if err != nil {
		return "", false
	}
	raw, ok := values[key]
	if !ok {
		return "", false
	}
	num, ok := toFloat(raw)
	if !ok {
		return "", false
	}
	cond := fmt.Sprintf("%s %s %s", key, op, formatNumber(threshold))
	pass := false
	switch op {
	case ">":
		pass = num > threshold
	case ">=":
		pass = num >= threshold
	case "<":
		pass = num < threshold
	case "<=":
		pass = num <= threshold
	case "==":
		pass = num == threshold
	case "!=":
		pass = num != threshold
	default:
		return "", false
	}
	if !pass {
		return "", false
	}
	return cond, true
}

// formatNumber 条件文本里的数值格式化：整数值不带小数点，其余走默认精度。
func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// copyValues 浅拷贝遥测 map，事件快照与调用方后续修改解耦。
func copyValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}
