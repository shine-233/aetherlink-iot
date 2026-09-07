// 文件用途：规则链 AI 推理节点（ROADMAP D7，PHASE-D AI 2.0）。
// 核心逻辑：ai.inference——把载荷/元数据渲染进 prompt 模板，调用模型中心（或全局 ai.llm.* 回退）
//          的 OpenAI 兼容端点做推理，把回复写回 metadata 与 payload 的 output_key。
// 关键注意事项：
//   - 未配置（无 model 档案且无全局 api_key）时 fail-fast 报错——AI 语义不允许静默丢消息；
//   - 模板占位符 {{key}} 依次从 payload、metadata 取值渲染，缺失渲染为空串；
//   - 推理为同步阻塞调用（超时 30s），执行器以 ctx 贯穿，取消语义随上游。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"gorm.io/gorm"
)

// PHASE-D-D7 BEGIN AI 节点（规则引擎 2.0 扩展）

// 规则链 AI 节点类型常量。
const RuleChainAiInference = "ai.inference"

// aiInferenceDefaultOutputKey 推理回复默认写回键。
const aiInferenceDefaultOutputKey = "ai_reply"

// validateAiInferenceConfig ai.inference 配置校验：prompt 与 input_key 至少其一。
func validateAiInferenceConfig(cfg map[string]any) error {
	prompt, _ := cfg["prompt"].(string)
	inputKey, _ := cfg["input_key"].(string)
	if strings.TrimSpace(prompt) == "" && strings.TrimSpace(inputKey) == "" {
		return fmt.Errorf("ai.inference config requires prompt or input_key")
	}
	if ok := cfg["output_key"]; ok != nil {
		if s, isStr := ok.(string); !isStr || strings.TrimSpace(s) == "" {
			return fmt.Errorf("ai.inference output_key must be a non-empty string")
		}
	}
	return nil
}

// ruleChainAiInference ai.inference 执行 handler。
func ruleChainAiInference(e *ruleChainExecution, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	modelID, _ := node.Config["model_id"].(string)
	promptTpl, _ := node.Config["prompt"].(string)
	inputKey, _ := node.Config["input_key"].(string)
	systemPrompt, _ := node.Config["system_prompt"].(string)
	outputKey := aiInferenceDefaultOutputKey
	if v, ok := node.Config["output_key"].(string); ok && strings.TrimSpace(v) != "" {
		outputKey = strings.TrimSpace(v)
	}

	// 取用户提示词：input_key 优先，其次模板渲染。
	userPrompt := ""
	if inputKey != "" {
		if raw, ok := payload[inputKey]; ok {
			userPrompt = fmt.Sprintf("%v", raw)
		} else if raw, ok := metadata[inputKey]; ok {
			userPrompt = fmt.Sprintf("%v", raw)
		}
	}
	if userPrompt == "" {
		userPrompt = renderAiTemplate(promptTpl, payload, metadata)
	}
	if strings.TrimSpace(userPrompt) == "" {
		return ruleChainNodeResult{}, fmt.Errorf("ai.inference produced empty prompt (check input_key/payload)")
	}

	// 解析模型端点：模型中心档案优先，回退全局 ai.llm.* 配置。
	var baseURL, apiKey, modelName string
	if modelID != "" {
		m, err := dal.GetAiModelInTenant(modelID, rcc.TenantID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ruleChainNodeResult{}, fmt.Errorf("ai.inference model %q not found in tenant", modelID)
			}
			return ruleChainNodeResult{}, fmt.Errorf("ai.inference load model: %w", err)
		}
		if !m.Enabled {
			return ruleChainNodeResult{}, fmt.Errorf("ai.inference model %q is disabled", modelID)
		}
		baseURL, apiKey, modelName = m.BaseURL, m.APIKey, m.Model
	} else {
		baseURL, apiKey, modelName = aiLLMConfigFallback()
	}

	msgs := make([]aiLLMChatMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		msgs = append(msgs, aiLLMChatMessage{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, aiLLMChatMessage{Role: "user", Content: userPrompt})

	ctx, cancel := context.WithTimeout(e.ctx, aiModelHTTPTimeout+10*time.Second)
	defer cancel()
	reply, err := aiChatCompletion(ctx, baseURL, apiKey, modelName, msgs, 0, nil)
	if err != nil {
		return ruleChainNodeResult{}, fmt.Errorf("ai.inference call failed: %w", err)
	}

	// 回复写回：metadata（富化语义）+ payload（下游可直接消费）。
	metadata[outputKey] = reply
	out := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		out[k] = v
	}
	out[outputKey] = reply
	logNodeDebug(chainIDOfExecution(e), node.ID, "ai inference done model=%s reply_chars=%d", modelName, len(reply))
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: out, metadata: metadata, rcc: rcc}}}, nil
}

// renderAiTemplate 渲染 {{key}} 占位符：payload 优先，metadata 兜底，缺失置空。
func renderAiTemplate(tpl string, payload, metadata map[string]any) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	var b strings.Builder
	for {
		start := strings.Index(tpl, "{{")
		if start < 0 {
			b.WriteString(tpl)
			break
		}
		end := strings.Index(tpl[start:], "}}")
		if end < 0 {
			b.WriteString(tpl)
			break
		}
		end += start
		b.WriteString(tpl[:start])
		key := strings.TrimSpace(tpl[start+2 : end])
		if v, ok := payload[key]; ok {
			b.WriteString(fmt.Sprintf("%v", v))
		} else if v, ok := metadata[key]; ok {
			b.WriteString(fmt.Sprintf("%v", v))
		}
		tpl = tpl[end+2:]
	}
	return b.String()
}
