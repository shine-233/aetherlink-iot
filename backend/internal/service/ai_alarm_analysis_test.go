// 文件用途：AI 告警分析的纯逻辑单测——作用域守卫、提示词截断/字段、JSON 解析与钳制。
// 核心逻辑：不依赖真实 LLM 与数据库（纯函数直测），保证提示词不会带超长字段、
// 解析器能容错模型输出的代码围栏包裹、条数钳制生效、跨租户/空租户被拒绝。
package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

func TestAlarmAnalysisScopeRejectsEmptyClaims(t *testing.T) {
	if _, err := alarmAnalysisScope(nil); err == nil {
		t.Fatal("nil claims 应被拒绝")
	}
	if _, err := alarmAnalysisScope(&utils.UserClaims{TenantID: "  "}); err == nil {
		t.Fatal("空租户应被拒绝")
	}
	if _, err := alarmAnalysisScope(&utils.UserClaims{TenantID: "tenant-a"}); err != nil {
		t.Fatalf("有效租户不应被拒绝: %v", err)
	}
}

func TestAlarmAnalysisPromptTruncatesLongFields(t *testing.T) {
	longDesc := strings.Repeat("d", 900)
	longContent := strings.Repeat("c", 2000)
	desc := longDesc
	content := longContent
	alarm := &model.AlarmHistory{
		Name:            "高温告警",
		AlarmStatus:     "H",
		Description:     &desc,
		Content:         &content,
		AlarmDeviceList: `["dev-1","dev-2"]`,
	}
	system, user := buildAlarmAnalysisPrompt(alarm, "影响范围？")
	if !strings.Contains(system, "JSON") {
		t.Fatal("system 提示词应要求 JSON 输出")
	}
	if strings.Contains(user, strings.Repeat("d", 500)) {
		t.Fatal("描述字段未被截断到 400")
	}
	if strings.Contains(user, strings.Repeat("c", 900)) {
		t.Fatal("内容字段未被截断到 800")
	}
	if !strings.Contains(user, "dev-1") || !strings.Contains(user, "影响范围") {
		t.Fatal("触发设备与补充问题应进入上下文")
	}
}

func TestAlarmAnalysisParseJSONFencedAndClamps(t *testing.T) {
	fenced := "```json\n{\"summary\":\"s\",\"probable_causes\":[\"a\"],\"recommended_actions\":[\"x\"]}\n```"
	res, err := parseAlarmAnalysisJSON(fenced)
	if err != nil {
		t.Fatalf("围栏包裹的 JSON 应可解析: %v", err)
	}
	if res.Summary != "s" || len(res.ProbableCauses) != 1 {
		t.Fatalf("解析结果不符: %+v", res)
	}

	big := alarmAnalysisResult{
		Summary:            strings.Repeat("s", 600),
		ProbableCauses:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
		RecommendedActions: []string{"a", "b"},
	}
	clamped := clampAlarmAnalysisResult(big)
	if len(clamped.Summary) != 500 {
		t.Fatalf("摘要未截断到 500: %d", len(clamped.Summary))
	}
	if len(clamped.ProbableCauses) != 8 {
		t.Fatalf("根因列表未钳制到 8: %d", len(clamped.ProbableCauses))
	}
	if len(clamped.RecommendedActions) != 2 {
		t.Fatalf("处置建议被误钳制: %d", len(clamped.RecommendedActions))
	}
}

func TestAlarmAnalysisParseJSONRejectsGarbage(t *testing.T) {
	if _, err := parseAlarmAnalysisJSON("抱歉，我无法分析"); err == nil {
		t.Fatal("无 JSON 的输出应报错")
	}
	if _, err := parseAlarmAnalysisJSON("{not-json}"); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}
