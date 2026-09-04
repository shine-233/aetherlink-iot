// 文件用途：规则链执行引擎（ROADMAP B2）。
// 核心逻辑：从入度为零的触发节点开始拓扑遍历；过滤器不通过则剪断该分支；
//
//	转换节点产出新载荷向下游传递；动作节点执行副作用（webhook/设备命令）。
//
// 关键注意事项：单次执行整体超时 10s、webhook 单节点 5s；
//
//	命令动作经 ruleChainCommandSender 注入，便于测试替换。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/safehttp"
)

const (
	ruleChainExecTimeout = 10 * time.Second
	ruleChainWebhookWait = 5 * time.Second
)

// RuleChainContext 一次链执行的运行时上下文。
type RuleChainContext struct {
	DeviceID     string
	DeviceNumber string
	TenantID     string
	Timestamp    int64
}

// ruleChainCommandSender 设备命令发送注入点（测试可替换）。
var ruleChainCommandSender = func(ctx context.Context, deviceID, identify, paramsJSON string) error {
	params := paramsJSON
	putMessage := &model.PutMessageForCommand{
		DeviceID: deviceID,
		Identify: identify,
		Value:    &params,
	}
	return GroupApp.CommandData.CommandPutMessage(ctx, "", putMessage, "2")
}

var ruleChainWebhookPoster = safehttp.PostWebhookJSON

// ExecuteRuleChainGraph 执行一条规则链，返回聚合错误（节点失败记录但继续其他分支）。
func ExecuteRuleChainGraph(ctx context.Context, graph *RuleChainGraph, rcc *RuleChainContext, values map[string]any) []error {
	return executeRuleChainGraphFromTrigger(ctx, graph, rcc, values, "")
}

// ExecuteRuleChainGraphForTrigger executes only roots matching the current runtime event.
func ExecuteRuleChainGraphForTrigger(ctx context.Context, graph *RuleChainGraph, rcc *RuleChainContext, values map[string]any, triggerType string) []error {
	return executeRuleChainGraphFromTrigger(ctx, graph, rcc, values, triggerType)
}

func executeRuleChainGraphFromTrigger(ctx context.Context, graph *RuleChainGraph, rcc *RuleChainContext, values map[string]any, triggerType string) []error {
	if graph == nil || rcc == nil {
		return nil
	}
	execCtx, cancel := context.WithTimeout(ctx, ruleChainExecTimeout)
	defer cancel()

	errs := make([]error, 0)
	var walk func(node *RuleChainNode, payload map[string]any)
	walk = func(node *RuleChainNode, payload map[string]any) {
		if err := execCtx.Err(); err != nil {
			return
		}
		pass, output, nodeErr := executeRuleChainNode(execCtx, node, rcc, payload)
		if nodeErr != nil {
			errs = append(errs, fmt.Errorf("node %s(%s): %w", node.ID, node.Type, nodeErr))
			return
		}
		if !pass {
			return
		}
		for _, next := range graph.Successors(node.ID) {
			walk(next, output)
		}
	}
	for _, root := range graph.Roots() {
		if triggerType != "" && root.Type != triggerType {
			continue
		}
		walk(root, values)
	}
	return errs
}

// executeRuleChainNode 分发单节点执行。pass=false 表示分支被过滤剪断。
func executeRuleChainNode(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload map[string]any) (bool, map[string]any, error) {
	switch node.Type {
	case RuleChainTriggerTelemetry, RuleChainTriggerOnline:
		return true, payload, nil
	case RuleChainFilterThreshold:
		return ruleChainFilterThreshold(node.Config, payload)
	case RuleChainTransformMapping:
		return true, ruleChainTransformMapping(node.Config, payload), nil
	case RuleChainActionWebhook:
		return true, payload, ruleChainActionWebhook(ctx, node.Config, rcc, payload)
	case RuleChainActionCommand:
		return true, payload, ruleChainActionCommand(ctx, node.Config, rcc)
	default:
		return false, payload, fmt.Errorf("unknown node type %q", node.Type)
	}
}

func ruleChainFilterThreshold(cfg map[string]any, payload map[string]any) (bool, map[string]any, error) {
	key, op, threshold, err := parseThresholdConfig(cfg)
	if err != nil {
		return false, payload, err
	}
	raw, ok := payload[key]
	if !ok {
		// 点位缺失视为不通过，避免误触发下游动作。
		return false, payload, nil
	}
	num, ok := toFloat(raw)
	if !ok {
		return false, payload, nil
	}
	switch op {
	case ">":
		return num > threshold, payload, nil
	case ">=":
		return num >= threshold, payload, nil
	case "<":
		return num < threshold, payload, nil
	case "<=":
		return num <= threshold, payload, nil
	case "==":
		return num == threshold, payload, nil
	case "!=":
		return num != threshold, payload, nil
	default:
		return false, payload, fmt.Errorf("unsupported operator %q", op)
	}
}

func parseThresholdConfig(cfg map[string]any) (string, string, float64, error) {
	if cfg == nil {
		return "", "", 0, fmt.Errorf("threshold config is required")
	}
	key, _ := cfg["key"].(string)
	op, _ := cfg["op"].(string)
	if key == "" || op == "" {
		return "", "", 0, fmt.Errorf("threshold config requires key and op")
	}
	threshold, ok := toFloat(cfg["value"])
	if !ok {
		return "", "", 0, fmt.Errorf("threshold config value must be a number")
	}
	return key, op, threshold, nil
}

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

func ruleChainTransformMapping(cfg map[string]any, payload map[string]any) map[string]any {
	fieldsRaw, ok := cfg["fields"]
	if !ok {
		return payload
	}
	fields, ok := fieldsRaw.(map[string]any)
	if !ok || len(fields) == 0 {
		return payload
	}
	output := make(map[string]any, len(fields))
	for from, toAny := range fields {
		to, _ := toAny.(string)
		if to == "" {
			to = from
		}
		if value, exists := payload[from]; exists {
			output[to] = value
		}
	}
	return output
}

func ruleChainActionWebhook(ctx context.Context, cfg map[string]any, rcc *RuleChainContext, payload map[string]any) error {
	urlRaw, _ := cfg["url"]
	url, _ := urlRaw.(string)
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("webhook config requires url")
	}
	body, err := json.Marshal(map[string]any{
		"device_id":     rcc.DeviceID,
		"device_number": rcc.DeviceNumber,
		"tenant_id":     rcc.TenantID,
		"timestamp":     rcc.Timestamp,
		"values":        payload,
	})
	if err != nil {
		return err
	}
	timeoutMs, ok := toFloat(cfg["timeout_ms"])
	wait := ruleChainWebhookWait
	if ok && timeoutMs > 0 && timeoutMs < float64(ruleChainWebhookWait.Milliseconds()) {
		wait = time.Duration(timeoutMs) * time.Millisecond
	}
	callCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	resp, err := ruleChainWebhookPoster(callCtx, url, body)
	if err != nil {
		return err
	}
	defer safehttp.DrainAndClose(resp)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook responded %d", resp.StatusCode)
	}
	return nil
}

func ruleChainActionCommand(ctx context.Context, cfg map[string]any, rcc *RuleChainContext) error {
	identify, _ := cfg["identify"].(string)
	if identify == "" {
		return fmt.Errorf("command config requires identify")
	}
	params := cfg["params"]
	if params == nil {
		params = map[string]any{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if rcc.DeviceID == "" {
		return fmt.Errorf("command action requires a device context")
	}
	return ruleChainCommandSender(ctx, rcc.DeviceID, identify, string(paramsJSON))
}
