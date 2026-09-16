// 文件用途：规则链 AI 推理节点（`ai.inference`）的契约测试。
//
// 背景：该节点在路线图里**从未记载**，长期被当作"未实现"。实测代码、注册表、
// 分派点、前端节点面板四处齐全，唯独**一条测试都没有**——这正是它最真实的缺口。
// 本文件补上纯逻辑与 fail-fast 语义的契约。
//
// 核心逻辑：分三组——配置校验、模板渲染、执行期 fail-fast。
//
// 关键注意事项：
//   - 刻意**不测成功推理**：那需要真实模型端点。把不可执行的路径写成 skip
//     等于用绿灯掩盖"从未跑过"，所以只测"在任何外部调用之前就该拒绝"的路径。
//   - `ai.inference` 的语义是 **fail-fast，不允许静默丢消息**：配置缺失、
//     prompt 渲染为空、模型不可用都必须报错。把"AI 没跑成"变成"消息照常往下走"
//     会让下游拿到没有 AI 富化的数据却完全看不出来。
package service

import (
	"context"
	"strings"
	"testing"
)

func TestValidateAiInferenceConfig(t *testing.T) {
	cases := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{name: "仅 prompt", config: map[string]any{"prompt": "summarize {{value}}"}},
		{name: "仅 input_key", config: map[string]any{"input_key": "raw"}},
		{name: "两者都有", config: map[string]any{"prompt": "p", "input_key": "k"}},
		{name: "合法 output_key", config: map[string]any{"prompt": "p", "output_key": "ai_reply"}},
		{name: "既无 prompt 也无 input_key", config: map[string]any{}, wantErr: true},
		{name: "prompt 全空白", config: map[string]any{"prompt": "   "}, wantErr: true},
		{name: "input_key 全空白", config: map[string]any{"input_key": "\t\n"}, wantErr: true},
		{name: "output_key 非字符串", config: map[string]any{"prompt": "p", "output_key": 123}, wantErr: true},
		{name: "output_key 为空串", config: map[string]any{"prompt": "p", "output_key": ""}, wantErr: true},
		{name: "output_key 全空白", config: map[string]any{"prompt": "p", "output_key": "  "}, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateAiInferenceConfig(testCase.config)
			if testCase.wantErr && err == nil {
				t.Fatal("期望校验失败，实际通过——配置错误必须在保存期暴露，而不是等运行期丢消息")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("期望校验通过，实际报错：%v", err)
			}
		})
	}
}

func TestRenderAiTemplate(t *testing.T) {
	payload := map[string]any{"value": 21.5, "name": "sensor-a"}
	metadata := map[string]any{"device_id": "dev-1", "value": "来自元数据的值", "extra": true}

	cases := []struct {
		name string
		tpl  string
		want string
	}{
		{name: "无占位符原样返回", tpl: "plain text", want: "plain text"},
		{name: "空模板返回空串", tpl: "", want: ""},
		{name: "单占位符取自 payload", tpl: "value={{value}}", want: "value=21.5"},
		{name: "payload 优先于 metadata", tpl: "{{value}}", want: "21.5"},
		{name: "payload 缺失时回落 metadata", tpl: "{{device_id}}", want: "dev-1"},
		{name: "两侧都缺失渲染为空串", tpl: "[{{nope}}]", want: "[]"},
		{name: "多个占位符", tpl: "{{name}}/{{device_id}}", want: "sensor-a/dev-1"},
		{name: "占位符两侧空白被裁剪", tpl: "{{  name  }}", want: "sensor-a"},
		{name: "同一占位符重复出现", tpl: "{{name}}-{{name}}", want: "sensor-a-sensor-a"},
		{name: "布尔值格式化", tpl: "{{extra}}", want: "true"},
		{name: "未闭合的占位符原样保留", tpl: "a {{value", want: "a {{value"},
		{name: "占位符后无结束标记", tpl: "{{", want: "{{"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := renderAiTemplate(testCase.tpl, payload, metadata); got != testCase.want {
				t.Fatalf("renderAiTemplate(%q) = %q，期望 %q", testCase.tpl, got, testCase.want)
			}
		})
	}

	// nil map 不得 panic：规则链上游可能给出空载荷。
	t.Run("nil 映射不 panic", func(t *testing.T) {
		if got := renderAiTemplate("{{a}}", nil, nil); got != "" {
			t.Fatalf("nil 映射应渲染为空串，实际 %q", got)
		}
	})
}

// TestRuleChainAiInferenceFailsFastOnEmptyPrompt 覆盖**在任何外部调用之前**就该拒绝的路径。
//
// 这些路径不触达数据库与模型端点，因此可以在无基础设施的情况下真实执行。
func TestRuleChainAiInferenceFailsFastOnEmptyPrompt(t *testing.T) {
	execution := &ruleChainExecution{ctx: context.Background()}
	rcc := &RuleChainContext{TenantID: "tenant-1"}

	cases := []struct {
		name     string
		config   map[string]any
		payload  map[string]any
		metadata map[string]any
	}{
		{name: "既无 prompt 也无 input_key", config: map[string]any{}},
		{name: "prompt 全空白", config: map[string]any{"prompt": "   "}},
		{name: "input_key 指向不存在的键", config: map[string]any{"input_key": "missing"}},
		{name: "占位符全部缺失导致渲染为空", config: map[string]any{"prompt": "{{a}}{{b}}"}},
		{name: "nil 载荷且无模板", config: map[string]any{"prompt": ""}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			node := &RuleChainNode{ID: "node-1", Type: RuleChainAiInference, Config: testCase.config}
			_, err := ruleChainAiInference(execution, node, rcc, testCase.payload, testCase.metadata)
			if err == nil {
				t.Fatal("prompt 为空必须 fail-fast——AI 语义不允许静默丢消息")
			}
			if !strings.Contains(err.Error(), "empty prompt") {
				t.Fatalf("错误应指向空 prompt，实际：%v", err)
			}
		})
	}
}

// TestRuleChainAiInferencePassesPromptGateOnNonEmptyPrompt 是上一条的**反向对照**。
//
// 没有它，"空 prompt 必报错"可能是永远为真的假绿。这里给一个非空 prompt，
// 断言失败原因**不再是** empty prompt——证明前面的用例真的走到了那个分支。
func TestRuleChainAiInferencePassesPromptGateOnNonEmptyPrompt(t *testing.T) {
	execution := &ruleChainExecution{ctx: context.Background()}
	node := &RuleChainNode{
		ID:     "node-1",
		Type:   RuleChainAiInference,
		Config: map[string]any{"prompt": "hello world"},
	}
	rcc := &RuleChainContext{TenantID: "tenant-1"}

	_, err := ruleChainAiInference(execution, node, rcc, map[string]any{}, map[string]any{})
	if err == nil {
		t.Skip("本机配置了可用的全局 AI 端点，反向对照无从构造（跳过而非假绿）")
	}
	if strings.Contains(err.Error(), "empty prompt") {
		t.Fatalf("非空 prompt 不应报 empty prompt，否则前面的用例可能只是永远为真：%v", err)
	}
}

// TestAiInferenceNodeIsRegisteredAndDispatched 锁住"已实现但被当成未实现"这件事不再发生。
//
// 该节点曾在路线图里长期记作 `未实现`，而实际上代码、注册表、分派点、前端面板四处齐全。
// 这条用例把注册与分派两处钉死：任何一处被摘掉都会立刻变红。
func TestAiInferenceNodeIsRegisteredAndDispatched(t *testing.T) {
	spec, ok := ruleChainSpecByType[RuleChainAiInference]
	if !ok {
		t.Fatal("ai.inference 必须留在节点注册表里，否则保存配置时会报 'node type is not registered'")
	}
	if spec.Validate == nil {
		t.Fatal("ai.inference 必须带配置校验器：配置错误应在保存期暴露，而不是运行期丢消息")
	}
	if err := validateRuleChainNodeConfig(RuleChainAiInference, map[string]any{"prompt": "x"}); err != nil {
		t.Fatalf("注册表校验器应接受合法配置，实际：%v", err)
	}
	if err := validateRuleChainNodeConfig(RuleChainAiInference, map[string]any{}); err == nil {
		t.Fatal("注册表校验器应拒绝空配置")
	}
	if kind := RuleChainNodeKind(RuleChainAiInference); kind == "" {
		t.Fatal("ai.inference 必须能解析出节点分类（前端 palette 依赖它）")
	}
}
