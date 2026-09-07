// 文件用途：规则链 2.0 Transformation 扩充节点与 Filter 扩充节点（PHASE-D-D1）。
// 核心逻辑：script（挂 data_script 的 Lua 引擎）/rename_keys/split_array/dedup/
//
//	change_originator 五个转换节点；exists/string_match/in_range 三个过滤节点。
//
// 关键注意事项：split_array 引擎级多输出（上限 ruleChainMaxNodeOutputs）；
//
//	dedup 依赖进程内时间窗注册表，重启后窗口重置（MVP 语义，交付说明已注明）；
//	change_originator 必须通过租户守卫校验目标设备，跨租户即报错。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/utils"
)

// ruleChainTransformScript script 转换：payload JSON 进 Lua（data_script 引擎），输出对象替换 payload。
// config: {script_id?(租户内数据脚本) | code?(内联脚本,≤8KB)}，二选一。
func ruleChainTransformScript(e *ruleChainExecution, node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, error) {
	rcc := msg.Rcc
	if rcc == nil {
		rcc = &RuleChainContext{}
	}
	code, _ := node.Config["code"].(string)
	if strings.TrimSpace(code) == "" {
		scriptID, _ := node.Config["script_id"].(string)
		scriptID = strings.TrimSpace(scriptID)
		if scriptID == "" {
			return ruleChainNodeResult{}, fmt.Errorf("script node %s requires script_id or code", node.ID)
		}
		if strings.TrimSpace(rcc.TenantID) == "" {
			return ruleChainNodeResult{}, fmt.Errorf("script node %s requires tenant context", node.ID)
		}
		loaded, err := ruleChainScriptLoader(e.ctx, rcc.TenantID, scriptID)
		if err != nil {
			return ruleChainNodeResult{}, err
		}
		if strings.TrimSpace(loaded) == "" {
			return ruleChainNodeResult{}, fmt.Errorf("script node %s: script %q not found in tenant", node.ID, scriptID)
		}
		code = loaded
	}
	payloadJSON, err := json.Marshal(msg.Payload)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	topic := fmt.Sprintf("rule-chain://%s/%s", e.graph.ChainID, node.ID)
	out, err := utils.ScriptDeal(code, payloadJSON, topic)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("script node %s: %w", node.ID, err)
	}
	output := make(map[string]any, 4)
	trimmed := strings.TrimSpace(out)
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &output); err != nil || output == nil {
			// 脚本未返回对象时收敛为 {result: string}，保证下游拿到 map。
			output = map[string]any{"result": out}
		}
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: output, metadata: msg.Metadata, rcc: rcc}}}, nil
}

// ruleChainTransformRenameKeys 键重命名：{mappings:{old:new}}。
func ruleChainTransformRenameKeys(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	mappings, _ := node.Config["mappings"].(map[string]any)
	output := make(map[string]any, len(payload))
	for key, value := range payload {
		output[key] = value
	}
	for from, toAny := range mappings {
		to, _ := toAny.(string)
		if to == "" || from == to {
			continue
		}
		if value, ok := output[from]; ok {
			delete(output, from)
			output[to] = value
		}
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: output, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainTransformSplitArray 数组分叉：数组/分隔字符串拆成多输出，下游每个元素执行一次。
// config: {key, separator?(字符串拆分用), max_splits?(默认 100)}
func ruleChainTransformSplitArray(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	key, _ := node.Config["key"].(string)
	maxSplits := ruleChainMaxNodeOutputs
	if raw, ok := toFloat(node.Config["max_splits"]); ok && raw > 0 && raw <= float64(ruleChainMaxNodeOutputs) {
		maxSplits = int(raw)
	}
	raw, exists := payload[key]
	if !exists {
		// 无可拆分值：与过滤器语义一致，剪断分支。
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	var items []any
	switch value := raw.(type) {
	case []any:
		items = value
	case string:
		separator, _ := node.Config["separator"].(string)
		if separator == "" {
			items = []any{value}
		} else {
			for _, part := range strings.Split(value, separator) {
				items = append(items, part)
			}
		}
	default:
		items = []any{value}
	}
	if len(items) == 0 {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	if len(items) > maxSplits {
		return ruleChainNodeResult{}, fmt.Errorf("split_array node %s: %d items exceeds limit %d", node.ID, len(items), maxSplits)
	}
	outputs := make([]ruleChainNodeOutput, 0, len(items))
	for _, item := range items {
		branch := make(map[string]any, len(payload))
		for k, v := range payload {
			branch[k] = v
		}
		branch[key] = item
		outputs = append(outputs, ruleChainNodeOutput{payload: branch, metadata: metadata, rcc: rcc})
	}
	return ruleChainNodeResult{pass: true, outputs: outputs}, nil
}

// ruleChainTransformDedup 时间窗去重：窗口内同签名（keys 子集或全 payload）重复则剪断。
// config: {window_ms>0, keys?:string[]}
func ruleChainTransformDedup(e *ruleChainExecution, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	windowMs, _ := toFloat(node.Config["window_ms"])
	window := time.Duration(windowMs) * time.Millisecond
	keys, err := configStringList(node.Config, "keys")
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	signature, err := dedupSignature(payload, keys)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	scope := strings.Join([]string{"dedup", rcc.TenantID, e.graph.ChainID, node.ID}, "|")
	if ruleChainDedupChecker(scope, signature, window) {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainTransformChangeOriginator 切换 originator：下游动作面向新设备执行。
// config: {mode:"device_id"(配 device_id) | "from_key"(配 key)}；目标设备必须属于当前租户。
func ruleChainTransformChangeOriginator(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	mode, _ := node.Config["mode"].(string)
	var target string
	switch mode {
	case "device_id":
		target, _ = node.Config["device_id"].(string)
	case "from_key":
		key, _ := node.Config["key"].(string)
		if raw, ok := payload[key]; ok {
			target, _ = raw.(string)
		}
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return ruleChainNodeResult{}, fmt.Errorf("change_originator node %s: target device not resolved", node.ID)
	}
	device, err := ruleChainDeviceLookup(ctx, rcc.TenantID, target)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	if device == nil {
		return ruleChainNodeResult{}, fmt.Errorf("change_originator node %s: device %q not found in tenant", node.ID, target)
	}
	next := &RuleChainContext{
		DeviceID:     device.ID,
		DeviceNumber: device.DeviceNumber,
		TenantID:     rcc.TenantID,
		Timestamp:    rcc.Timestamp,
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: next}}}, nil
}

// ---- Filter 扩充（语义与 filter.threshold 一致：不通过剪断分支）----

// ruleChainFilterExists 键存在性过滤：{key}。
func ruleChainFilterExists(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	key, _ := node.Config["key"].(string)
	_, ok := payload[key]
	return ruleChainNodeResult{pass: ok, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainFilterStringMatch 字符串匹配过滤：{key, op:contains|equals|prefix|suffix, value}。
// 值缺失或非字符串 → 不通过。
func ruleChainFilterStringMatch(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	key, _ := node.Config["key"].(string)
	op, _ := node.Config["op"].(string)
	value, _ := node.Config["value"].(string)
	raw, ok := payload[key]
	if !ok {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	str, ok := raw.(string)
	if !ok {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	pass := false
	switch op {
	case "contains":
		pass = strings.Contains(str, value)
	case "equals":
		pass = str == value
	case "prefix":
		pass = strings.HasPrefix(str, value)
	case "suffix":
		pass = strings.HasSuffix(str, value)
	}
	return ruleChainNodeResult{pass: pass, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainFilterInRange 数值区间过滤：{key, min, max}（闭区间）；值缺失或不可数值化 → 不通过。
func ruleChainFilterInRange(node *RuleChainNode, payload, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	key, _ := node.Config["key"].(string)
	minV, _ := toFloat(node.Config["min"])
	maxV, _ := toFloat(node.Config["max"])
	raw, ok := payload[key]
	if !ok {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	num, ok := toFloat(raw)
	if !ok {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	return ruleChainNodeResult{pass: num >= minV && num <= maxV, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}
