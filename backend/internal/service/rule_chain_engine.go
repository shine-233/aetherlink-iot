// 文件用途：规则链执行引擎（ROADMAP B2 + PHASE-D-D1 2.0）。
// 核心逻辑：从入度为零的触发节点开始拓扑遍历；过滤器不通过则剪断该分支；
//
//	转换节点产出新载荷向下游传递；动作节点执行副作用（webhook/设备命令）；
//	D1 扩展：节点间消息升级为 {payload, metadata, originator} 三元组，
//	支持富化（metadata 合入）、分叉输出（split）、originator 切换与节点级调试 trace。
//
// 关键注意事项：单次执行整体超时 10s、webhook 单节点 5s；
//
//	命令动作经 ruleChainCommandSender 注入，便于测试替换；
//	对外入口 ExecuteRuleChainGraph/ExecuteRuleChainGraphForTrigger 语义保持不变。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/safehttp"
	"github.com/go-basic/uuid"
)

const (
	ruleChainExecTimeout = 10 * time.Second
	ruleChainWebhookWait = 5 * time.Second
	// ruleChainMaxSubchainDepth 嵌套子链最大深度（防组合爆炸）。
	ruleChainMaxSubchainDepth = 5
	// ruleChainMaxNodeOutputs 单节点最大输出分支数（split 数组上限）。
	ruleChainMaxNodeOutputs = 100
)

// RuleChainContext 一次链执行的运行时上下文。
type RuleChainContext struct {
	DeviceID     string
	DeviceNumber string
	TenantID     string
	Timestamp    int64
}

// ruleChainMessage 节点间流转的消息（D1 2.0）：
// payload 为业务载荷；metadata 承载富化结果；rcc 为该消息当前生效的 originator 上下文。
type ruleChainMessage struct {
	Payload  map[string]any
	Metadata map[string]any
	Rcc      *RuleChainContext
}

// ruleChainNodeOutput 单个输出分支。
type ruleChainNodeOutput struct {
	payload  map[string]any
	metadata map[string]any
	rcc      *RuleChainContext
}

// ruleChainNodeResult 单节点执行结果：pass=false 表示分支被过滤剪断。
type ruleChainNodeResult struct {
	pass    bool
	outputs []ruleChainNodeOutput
}

// ruleChainExecution 单次链执行的就地状态（超时/子链栈/trace 归属）。
type ruleChainExecution struct {
	ctx    context.Context
	graph  *RuleChainGraph
	execID string
	stack  []string // 已进入的子链 chain_id 栈（含根链），防环
}

func newRuleChainExecution(ctx context.Context, graph *RuleChainGraph, execID string) *ruleChainExecution {
	if execID == "" {
		execID = uuid.New()
	}
	e := &ruleChainExecution{ctx: ctx, graph: graph, execID: execID}
	if graph != nil && strings.TrimSpace(graph.ChainID) != "" {
		e.stack = append(e.stack, graph.ChainID)
	}
	return e
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
	exec := newRuleChainExecution(execCtx, graph, "")
	errs = append(errs, exec.walkAndCollect(values, triggerType, rcc)...)
	return errs
}

// walkAndCollect 从触发根开始遍历（供外层执行与子链复用）。
func (e *ruleChainExecution) walkAndCollect(values map[string]any, triggerType string, rcc *RuleChainContext) []error {
	errs := make([]error, 0)
	var walk func(node *RuleChainNode, msg ruleChainMessage)
	walk = func(node *RuleChainNode, msg ruleChainMessage) {
		if e.ctx.Err() != nil {
			return
		}
		start := time.Now()
		result, nodeErr := e.executeNode(node, msg)
		recordRuleChainNodeTrace(e, node, msg, result, nodeErr, time.Since(start))
		if nodeErr != nil {
			errs = append(errs, fmt.Errorf("node %s(%s): %w", node.ID, node.Type, nodeErr))
			return
		}
		if !result.pass {
			return
		}
		outputs := result.outputs
		if len(outputs) == 0 {
			outputs = []ruleChainNodeOutput{{payload: msg.Payload, metadata: msg.Metadata, rcc: msg.Rcc}}
		}
		for _, output := range outputs {
			if e.ctx.Err() != nil {
				return
			}
			nextMsg := ruleChainMessage{Payload: output.payload, Metadata: output.metadata, Rcc: output.rcc}
			for _, next := range e.graph.Successors(node.ID) {
				walk(next, nextMsg)
			}
		}
	}
	for _, root := range e.graph.Roots() {
		if triggerType != "" && root.Type != triggerType {
			continue
		}
		walk(root, ruleChainMessage{Payload: values, Metadata: map[string]any{}, Rcc: rcc})
	}
	return errs
}

// executeNode 分发单节点执行。pass=false 表示分支被过滤剪断。
func (e *ruleChainExecution) executeNode(node *RuleChainNode, msg ruleChainMessage) (ruleChainNodeResult, error) {
	rcc := msg.Rcc
	if rcc == nil {
		rcc = &RuleChainContext{}
	}
	payload := msg.Payload
	metadata := msg.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	single := func(pass bool, out map[string]any, err error) (ruleChainNodeResult, error) {
		if err != nil {
			return ruleChainNodeResult{}, err
		}
		return ruleChainNodeResult{pass: pass, outputs: []ruleChainNodeOutput{{
			payload: out, metadata: metadata, rcc: rcc,
		}}}, nil
	}

	switch node.Type {
	case RuleChainTriggerTelemetry, RuleChainTriggerOnline:
		return single(true, payload, nil)
	case RuleChainFilterThreshold:
		return ruleChainFilterThreshold(node.Config, payload, metadata, rcc)
	case RuleChainTransformMapping:
		return single(true, ruleChainTransformMapping(node.Config, payload), nil)
	case RuleChainActionWebhook:
		return single(true, payload, ruleChainActionWebhook(e.ctx, node.Config, rcc, payload))
	case RuleChainActionCommand:
		return single(true, payload, ruleChainActionCommand(e.ctx, node.Config, rcc))
	case RuleChainActionAlarm:
		return single(true, payload, ruleChainActionAlarm(e.ctx, node.Config, rcc, payload))
	default:
		return executeRuleChainNodeD1(e, node, msg)
	}
}

func ruleChainFilterThreshold(cfg map[string]any, payload map[string]any, metadata map[string]any, rcc *RuleChainContext) (ruleChainNodeResult, error) {
	key, op, threshold, err := parseThresholdConfig(cfg)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	raw, ok := payload[key]
	if !ok {
		// 点位缺失视为不通过，避免误触发下游动作。
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	num, ok := toFloat(raw)
	if !ok {
		return ruleChainNodeResult{pass: false, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
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
		return ruleChainNodeResult{}, fmt.Errorf("unsupported operator %q", op)
	}
	return ruleChainNodeResult{pass: pass, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
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

// ruleChainAlarmCreator 告警动作落库注入点（测试可替换）。
var ruleChainAlarmCreator = func(ctx context.Context, history *model.AlarmHistory) error {
	return dal.CreateAlarmHistoryRow(history)
}

// ruleChainAlarmDedupStore 告警 re-trigger 去重存储注入点（测试可替换；nil 用进程内实现）。
var ruleChainAlarmDedupStore = ruleChainInMemoryAlarmDedup{}

// ruleChainActionAlarm 产生一条告警历史（action.alarm）。
// config: {name:string(必填), severity:"L"|"M"|"H"(默认H), description, content,
//
//	retrigger_dedup_ms:int(默认0=不去重；>0 时同 node+device 在窗口内重复触发不再落新告警)}
func ruleChainActionAlarm(ctx context.Context, cfg map[string]any, rcc *RuleChainContext, payload map[string]any) error {
	if cfg == nil {
		return fmt.Errorf("alarm config is required")
	}
	name, _ := cfg["name"].(string)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("alarm action requires name")
	}
	severity, _ := cfg["severity"].(string)
	switch severity {
	case "":
		severity = "H"
	case "L", "M", "H":
	default:
		return fmt.Errorf("alarm action severity must be one of L/M/H")
	}
	// PHASE-D-D1：re-trigger 去重——窗口内同 node+device 的重复触发直接跳过，不落新告警。
	dedupMs, _ := toFloat(cfg["retrigger_dedup_ms"])
	if dedupMs > 0 {
		dedupKey := strings.Join([]string{"alarm", rcc.TenantID, name, rcc.DeviceID}, "|")
		if ruleChainAlarmDedupStore.SeenWithin(dedupKey, time.Duration(dedupMs)*time.Millisecond) {
			return nil
		}
		ruleChainAlarmDedupStore.MarkSeen(dedupKey)
	}
	content, _ := cfg["content"].(string)
	if content == "" {
		if summary, err := json.Marshal(payload); err == nil && len(summary) < 512 {
			content = string(summary)
		}
	}
	description, _ := cfg["description"].(string)
	deviceList := "[]"
	if rcc.DeviceID != "" {
		deviceList = `["` + rcc.DeviceID + `"]`
	}
	history := &model.AlarmHistory{
		ID:                uuid.New(),
		AlarmConfigID:     "",
		GroupID:           "",
		SceneAutomationID: "",
		Name:              name,
		AlarmStatus:       severity,
		TenantID:          rcc.TenantID,
		CreateAt:          time.Now().UTC(),
		AlarmDeviceList:   deviceList,
	}
	if description != "" {
		history.Description = &description
	}
	if content != "" {
		history.Content = &content
	}
	return ruleChainAlarmCreator(ctx, history)
}
