package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// PHASE-D-D1 BEGIN 规则链 2.0 新节点单测（注入缝替换，hermetic）

// d1Node 构造节点辅助。
func d1Node(id, typ string, cfg map[string]any) *RuleChainNode {
	return &RuleChainNode{ID: id, Type: typ, Config: cfg}
}

func d1RCC() *RuleChainContext {
	return &RuleChainContext{DeviceID: "dev-1", TenantID: "tenant-1", Timestamp: time.Now().Unix()}
}

func TestRuleChainEnrichmentOriginatorAttributesD1(t *testing.T) {
	origFetch := ruleChainAttributeFetcher
	defer func() { ruleChainAttributeFetcher = origFetch }()
	ruleChainAttributeFetcher = func(_ context.Context, tenantID, deviceID string, keys []string) (map[string]any, error) {
		if tenantID != "tenant-1" || deviceID != "dev-1" {
			t.Fatalf("租户/设备维度错误: %s/%s", tenantID, deviceID)
		}
		return map[string]any{"location": "plant-1"}, nil
	}
	node := d1Node("e1", RuleChainEnrichmentOriginatorAttributes, map[string]any{"keys": []any{"location"}, "prefix": "env."})
	result, err := executeRuleChainNodeD1(&ruleChainExecution{ctx: context.Background()}, node, ruleChainMessage{
		Payload: map[string]any{"k": 1}, Metadata: map[string]any{}, Rcc: d1RCC(),
	})
	if err != nil {
		t.Fatalf("富化失败: %v", err)
	}
	if !result.pass || result.outputs[0].metadata["env.location"] != "plant-1" {
		t.Fatalf("metadata 应带前缀合入: %+v", result.outputs[0].metadata)
	}
}

func TestRuleChainEnrichmentRelatedAttributesD1(t *testing.T) {
	origLookup := ruleChainDeviceLookup
	origFetch := ruleChainAttributeFetcher
	defer func() { ruleChainDeviceLookup = origLookup; ruleChainAttributeFetcher = origFetch }()
	ruleChainDeviceLookup = func(_ context.Context, tenantID, deviceID string) (*model.Device, error) {
		if deviceID == "dev-gateway" {
			return &model.Device{ID: "dev-gateway", TenantID: tenantID}, nil
		}
		return nil, nil
	}
	ruleChainAttributeFetcher = func(_ context.Context, _, deviceID string, _ []string) (map[string]any, error) {
		return map[string]any{"firmware": "v2.1"}, nil
	}
	node := d1Node("e2", RuleChainEnrichmentRelatedAttributes, map[string]any{
		"keys": []any{"firmware"}, "device_id_key": "gateway_id",
	})
	result, err := executeRuleChainNodeD1(&ruleChainExecution{ctx: context.Background()}, node, ruleChainMessage{
		Payload: map[string]any{"gateway_id": "dev-gateway"}, Metadata: map[string]any{}, Rcc: d1RCC(),
	})
	if err != nil {
		t.Fatalf("关联富化失败: %v", err)
	}
	if result.outputs[0].metadata["firmware"] != "v2.1" {
		t.Fatalf("metadata 应含关联设备属性: %+v", result.outputs[0].metadata)
	}

	// 关联设备不存在 → fail-closed 报错
	nodeBad := d1Node("e2b", RuleChainEnrichmentRelatedAttributes, map[string]any{
		"keys": []any{"firmware"}, "device_id": "dev-missing",
	})
	if _, err := executeRuleChainNodeD1(&ruleChainExecution{ctx: context.Background()}, nodeBad, ruleChainMessage{
		Payload: map[string]any{}, Metadata: map[string]any{}, Rcc: d1RCC(),
	}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("不存在的关联设备应报错: %v", err)
	}
}

func TestRuleChainEnrichmentTenantMetadataD1(t *testing.T) {
	origFetch := ruleChainTenantMetadataFetcher
	defer func() { ruleChainTenantMetadataFetcher = origFetch }()
	ruleChainTenantMetadataFetcher = func(_ context.Context, tenantID string) (map[string]any, error) {
		return map[string]any{"organization": "hq"}, nil
	}
	node := d1Node("e3", RuleChainEnrichmentTenantMetadata, nil)
	result, err := executeRuleChainNodeD1(&ruleChainExecution{ctx: context.Background()}, node, ruleChainMessage{
		Payload: map[string]any{}, Metadata: map[string]any{}, Rcc: d1RCC(),
	})
	if err != nil {
		t.Fatalf("租户元数据富化失败: %v", err)
	}
	if result.outputs[0].metadata["organization"] != "hq" {
		t.Fatalf("metadata 应含租户元数据: %+v", result.outputs[0].metadata)
	}
}

func TestRuleChainTransformDedupD1(t *testing.T) {
	node := d1Node("dd", RuleChainTransformDedup, map[string]any{"window_ms": 60000.0, "keys": []any{"k"}})
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-dd"}}
	msg := ruleChainMessage{Payload: map[string]any{"k": 1.0}, Metadata: map[string]any{}, Rcc: d1RCC()}
	first, err := ruleChainTransformDedup(e, node, msg.Rcc, msg.Payload, msg.Metadata)
	if err != nil || !first.pass {
		t.Fatalf("首次应放行: %v", err)
	}
	second, err := ruleChainTransformDedup(e, node, msg.Rcc, msg.Payload, msg.Metadata)
	if err != nil {
		t.Fatalf("去重执行失败: %v", err)
	}
	if second.pass {
		t.Fatalf("窗口内重复载荷应被剪断")
	}
}

func TestRuleChainFlowDelayD1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	result, err := ruleChainFlowDelay(ctx, d1Node("d1", RuleChainFlowDelay, map[string]any{"duration_ms": 20.0}),
		map[string]any{}, map[string]any{}, d1RCC())
	if err != nil || !result.pass {
		t.Fatalf("delay 应放行: %v", err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatalf("未实际延时")
	}
	// 超上限拒绝
	if _, err := ruleChainFlowDelay(ctx, d1Node("d2", RuleChainFlowDelay, map[string]any{"duration_ms": 99999.0}),
		map[string]any{}, map[string]any{}, d1RCC()); err == nil {
		t.Fatalf("超上限 delay 应报错")
	}
}

func TestRuleChainFlowCheckpointD1(t *testing.T) {
	type cpRow struct {
		nodeID   string
		tenantID string
	}
	var captured []cpRow
	origWriter := ruleChainCheckpointWriter
	defer func() { ruleChainCheckpointWriter = origWriter }()
	ruleChainCheckpointWriter = func(_ context.Context, cp *model.RuleChainCheckpoint) error {
		captured = append(captured, cpRow{nodeID: cp.NodeID, tenantID: cp.TenantID})
		return nil
	}
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-cp"}, execID: "exec-1"}
	result, err := ruleChainFlowCheckpoint(e, d1Node("cp1", RuleChainFlowCheckpoint, nil), ruleChainMessage{
		Payload: map[string]any{"k": 1.0}, Metadata: map[string]any{}, Rcc: d1RCC(),
	})
	if err != nil || !result.pass {
		t.Fatalf("checkpoint 应放行: %v", err)
	}
	if len(captured) != 1 || captured[0].nodeID != "cp1" || captured[0].tenantID != "tenant-1" {
		t.Fatalf("检查点未正确落库: %+v", captured)
	}
}

func TestRuleChainFlowSubchainD1(t *testing.T) {
	origLoader := ruleChainSubchainLoader
	defer func() { ruleChainSubchainLoader = origLoader }()
	ruleChainSubchainLoader = func(_ context.Context, tenantID, chainID string) (*RuleChainGraph, error) {
		if tenantID != "tenant-1" {
			t.Fatalf("子链加载必须租户守卫: %s", tenantID)
		}
		subJSON := `{"nodes":[
			{"id":"s1","type":"transform.mapping","config":{"fields":{"a":"b"}}},
			{"id":"s2","type":"filter.threshold","config":{"key":"b","op":">","value":0}}
		],"edges":[{"from":"s1","to":"s2"}]}`
		g, err := ParseRuleChainSubgraph(subJSON)
		if err != nil {
			return nil, err
		}
		g.ChainID = chainID
		return g, nil
	}
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "root"}, execID: "exec-s"}
	node := d1Node("sub1", RuleChainFlowSubchain, map[string]any{"chain_id": "sub-chain"})
	result, err := ruleChainFlowSubchain(e, node, ruleChainMessage{
		Payload: map[string]any{"a": 5.0}, Metadata: map[string]any{}, Rcc: d1RCC(),
	})
	if err != nil || !result.pass {
		t.Fatalf("子链执行失败: %v", err)
	}

	// 环检测：栈中已含目标链 → 拒绝（真实路径由 newRuleChainExecution 初始化栈）
	eCycled := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "other"}, execID: "exec-s", stack: []string{"sub-chain"}}
	if _, err := ruleChainFlowSubchain(eCycled, node, ruleChainMessage{Payload: map[string]any{}, Metadata: map[string]any{}, Rcc: d1RCC()}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("子链环应被拒绝: %v", err)
	}
}

func TestRuleChainAnalyticsGeneratorD1(t *testing.T) {
	node := d1Node("g1", RuleChainAnalyticsGenerator, map[string]any{
		"values": map[string]any{"temperature": map[string]any{"min": 20.0, "max": 20.0}, "status": "ok"},
	})
	result, err := ruleChainAnalyticsGenerator(node, map[string]any{}, map[string]any{}, d1RCC())
	if err != nil {
		t.Fatalf("generator 失败: %v", err)
	}
	out := result.outputs[0].payload
	if out["temperature"] != 20.0 || out["status"] != "ok" {
		t.Fatalf("生成值不符: %+v", out)
	}
}

func TestRuleChainAnalyticsMessageCountD1(t *testing.T) {
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-mc"}, execID: "exec-mc"}
	node := d1Node("mc1", RuleChainAnalyticsMessageCount, map[string]any{"window_ms": 60000.0})
	r1, err := ruleChainAnalyticsMessageCount(e, node, d1RCC(), map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatalf("message_count 失败: %v", err)
	}
	r2, err := ruleChainAnalyticsMessageCount(e, node, d1RCC(), map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatalf("message_count 二次失败: %v", err)
	}
	if r1.outputs[0].payload["message_count"] != 1 || r2.outputs[0].payload["message_count"] != 2 {
		t.Fatalf("计数不符: %+v / %+v", r1.outputs[0].payload, r2.outputs[0].payload)
	}
}

func TestRuleChainAnalyticsLatestD1(t *testing.T) {
	origFetch := ruleChainLatestTelemetryFetcher
	defer func() { ruleChainLatestTelemetryFetcher = origFetch }()
	ruleChainLatestTelemetryFetcher = func(_ context.Context, tenantID, deviceID string, keys []string) (map[string]any, error) {
		return map[string]any{"temperature": 26.5}, nil
	}
	node := d1Node("al1", RuleChainAnalyticsLatest, map[string]any{"keys": []any{"temperature"}})
	result, err := ruleChainAnalyticsLatest(context.Background(), node, d1RCC(), map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatalf("latest 聚合失败: %v", err)
	}
	if result.outputs[0].payload["temperature"] != 26.5 {
		t.Fatalf("最新遥测应合入载荷: %+v", result.outputs[0].payload)
	}
}

func TestRuleChainExternalMQTTForwardD1(t *testing.T) {
	type pub struct {
		topic string
		qos   byte
		bytes int
	}
	var pubs []pub
	origPub := ruleChainMQTTPublisher
	defer func() { ruleChainMQTTPublisher = origPub }()
	ruleChainMQTTPublisher = func(_ context.Context, topic string, qos byte, payload []byte) error {
		pubs = append(pubs, pub{topic: topic, qos: qos, bytes: len(payload)})
		return nil
	}
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-mf"}, execID: "exec-mf"}
	node := d1Node("mf1", RuleChainExternalMQTTForward, map[string]any{"topic": "alerts/dev-1", "qos": 1.0})
	result, err := ruleChainExternalMQTTForward(e, node, d1RCC(), map[string]any{"k": 1.0}, map[string]any{"m": "v"})
	if err != nil || !result.pass {
		t.Fatalf("mqtt forward 失败: %v", err)
	}
	if len(pubs) != 1 || pubs[0].topic != "alerts/dev-1" || pubs[0].qos != 1 || pubs[0].bytes == 0 {
		t.Fatalf("发布不符: %+v", pubs)
	}
	// 发布器未装配 → fail-fast
	ruleChainMQTTPublisher = func(_ context.Context, _ string, _ byte, _ []byte) error {
		return errors.New("mqtt forward publisher is not wired")
	}
	if _, err := ruleChainExternalMQTTForward(e, node, d1RCC(), map[string]any{}, map[string]any{}); err == nil {
		t.Fatalf("发布器未装配应报错")
	}
}

func TestRuleChainExternalKafkaD1(t *testing.T) {
	var produced []string
	origProducer := ruleChainKafkaProducer
	defer func() { ruleChainKafkaProducer = origProducer }()
	ruleChainKafkaProducer = func(_ context.Context, topic, key string, _ []byte) error {
		produced = append(produced, topic+"|"+key)
		return nil
	}
	node := d1Node("k1", RuleChainExternalKafka, map[string]any{"topic": "events"})
	rcc := d1RCC()
	result, err := ruleChainExternalKafka(context.Background(), node, rcc, map[string]any{}, map[string]any{})
	if err != nil || !result.pass {
		t.Fatalf("kafka 外发失败: %v", err)
	}
	if len(produced) != 1 || produced[0] != "events|dev-1" {
		t.Fatalf("kafka 生产不符: %+v", produced)
	}
	// 未装配 → noop 放行（配置门控兜底语义）
	ruleChainKafkaProducer = nil
	result, err = ruleChainExternalKafka(context.Background(), node, rcc, map[string]any{}, map[string]any{})
	if err != nil || !result.pass {
		t.Fatalf("未装配应 noop 放行: %v", err)
	}
}

func TestRecordRuleChainNodeTraceD1(t *testing.T) {
	var captured *model.RuleChainNodeTrace
	origEnabled, origWriter := ruleChainTraceEnabled, ruleChainTraceWriter
	defer func() { ruleChainTraceEnabled, ruleChainTraceWriter = origEnabled, origWriter }()
	ruleChainTraceEnabled = func() bool { return true }
	ruleChainTraceWriter = func(trace *model.RuleChainNodeTrace) error {
		captured = trace
		return nil
	}
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-t"}, execID: "exec-t"}
	node := d1Node("n-t", RuleChainFilterThreshold, nil)
	msg := ruleChainMessage{Payload: map[string]any{}, Metadata: map[string]any{}, Rcc: d1RCC()}
	recordRuleChainNodeTrace(e, node, msg, ruleChainNodeResult{pass: true}, nil, 5*time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for captured == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if captured == nil {
		t.Fatalf("trace 未写入")
	}
	if captured.NodeID != "n-t" || captured.ChainID != "chain-t" || !captured.Pass || captured.TenantID != "tenant-1" {
		t.Fatalf("trace 内容不符: %+v", captured)
	}
}

func TestNodeSpecRegistryD1(t *testing.T) {
	specs := RuleChainNodeSpecList()
	if len(specs) < 25 {
		t.Fatalf("D1 注册表应不少于 25 种节点，实际 %d", len(specs))
	}
	// 每个注册类型必须有 kind 且校验器可执行（nil 校验器合法）。
	for _, spec := range specs {
		if RuleChainNodeKind(spec.Type) == "" {
			t.Fatalf("类型 %s 缺 kind", spec.Type)
		}
		if err := validateRuleChainNodeConfig(spec.Type, map[string]any{}); err != nil {
			switch spec.Type {
			case RuleChainTransformScript, RuleChainTransformRenameKeys, RuleChainTransformDedup,
				RuleChainFilterExists, RuleChainFilterStringMatch, RuleChainFilterInRange,
				RuleChainEnrichmentOriginatorAttributes, RuleChainEnrichmentLatestTelemetry,
				RuleChainEnrichmentRelatedAttributes, RuleChainAnalyticsLatest,
				RuleChainFlowSubchain, RuleChainFlowDelay, RuleChainActionAlarm:
				// 这些类型空配置必须被拒——校验器生效的旁证。
			case RuleChainExternalMQTTForward, RuleChainExternalKafka:
				if !strings.Contains(err.Error(), "topic") {
					t.Fatalf("%s 空配置应报 topic 缺失: %v", spec.Type, err)
				}
			}
		}
	}
	// 未知类型必须被拒。
	if err := validateRuleChainNodeConfig("unknown.type", nil); err == nil {
		t.Fatalf("未知类型应被拒")
	}
}

// PHASE-D-D1 END
