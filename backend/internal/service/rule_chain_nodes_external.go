package service

import (
	"context"
	"fmt"
	"strings"
)

// PHASE-D-D1 BEGIN External 外发节点 handler（D1 规则引擎 2.0）

// ruleChainExternalMQTTForward external.mqtt_forward：把消息外发到 MQTT 主题。
// 载荷契约：{"payload":<业务载荷>,"metadata":<富化元数据>} 信封（无损往返）。
// 发布器未装配（app 注入缺失）时 fail-fast 返回错误——外发语义不允许静默丢。
func ruleChainExternalMQTTForward(e *ruleChainExecution, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	topic, _ := node.Config["topic"].(string)
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ruleChainNodeResult{}, fmt.Errorf("mqtt_forward config requires topic")
	}
	qosByte := byte(0)
	if qos, ok := toFloat(node.Config["qos"]); ok && qos == 1 {
		qosByte = 1
	}
	envelope, err := marshalRuleChainEnvelope(payload, metadata)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("marshal mqtt forward envelope: %w", err)
	}
	if err := ruleChainMQTTPublisher(e.ctx, topic, qosByte, envelope); err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("mqtt forward publish: %w", err)
	}
	logNodeDebug(chainIDOfExecution(e), node.ID, "mqtt forward published topic=%s bytes=%d", topic, len(envelope))
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainExternalKafka external.kafka：Kafka 外发骨架。
// 生产者注入点（ruleChainKafkaProducer）由 app 装配层按配置门控注入；
// 未注入时 noop 放行（配置门控默认关闭该节点不应进入执行路径，此处兜底放行不丢消息管道）。
func ruleChainExternalKafka(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	topic, _ := node.Config["topic"].(string)
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ruleChainNodeResult{}, fmt.Errorf("kafka config requires topic")
	}
	if ruleChainKafkaProducer == nil {
		logNodeDebug("", node.ID, "kafka producer not wired; skip (config-gated)")
		return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
	}
	envelope, err := marshalRuleChainEnvelope(payload, metadata)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("marshal kafka envelope: %w", err)
	}
	if err := ruleChainKafkaProducer(ctx, topic, rcc.DeviceID, envelope); err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("kafka produce: %w", err)
	}
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// marshalRuleChainPayload 载荷序列化（checkpoint 用）。
func marshalRuleChainPayload(payload map[string]any) ([]byte, error) {
	return marshalRuleChainJSON(payload)
}

// marshalRuleChainEnvelope 外发信封序列化。
func marshalRuleChainEnvelope(payload, metadata map[string]any) ([]byte, error) {
	return marshalRuleChainJSON(map[string]any{"payload": payload, "metadata": metadata})
}

// marshalRuleChainJSON 统一 JSON 编码（禁 HTML 转义，保持遥测可读）。
func marshalRuleChainJSON(v any) ([]byte, error) {
	return jsonMarshalNoEscape(v)
}

// PHASE-D-D1 END
