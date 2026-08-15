package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

// TestBuildSceneAutomationDryRunTraceOrdersTriggersBeforeActions 验证执行 trace
// 按"先触发条件后动作"的顺序展开,索引连续,且复用已算出的 skipped/blocked 结论。
func TestBuildSceneAutomationDryRunTraceOrdersTriggersBeforeActions(t *testing.T) {
	t.Parallel()

	deviceSource := "device-123"
	req := &model.DryRunSceneAutomationReq{
		TriggerConditionGroups: [][]model.Condition{
			{
				{TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &deviceSource},
				{TriggerConditionsType: model.DEVICE_TRIGGER_CONDITION_TYPE_TIME},
			},
		},
		Actions: []model.Action{
			{ActionType: model.AUTOMATE_ACTION_TYPE_ONE, ActionTarget: "device-999", ActionParam: "on"},
			{ActionType: model.AUTOMATE_ACTION_TYPE_ONE, ActionTarget: ""},
		},
	}

	result := &model.SceneAutomationDryRunResult{
		SkippedConditions: []string{
			"condition group #1 row #2 的定时或时间窗口不会在预演中判断",
		},
		UnavailableActions: []string{
			"action #2 has no target",
		},
	}

	trace := buildSceneAutomationDryRunTrace(req, result)

	if !trace.IsSimulation {
		t.Fatalf("expected trace to be flagged as simulation")
	}
	if trace.StepCount != 4 {
		t.Fatalf("expected 4 steps (2 triggers + 2 actions), got %d", trace.StepCount)
	}
	if len(trace.Steps) != 4 {
		t.Fatalf("expected 4 steps in slice, got %d", len(trace.Steps))
	}

	// 索引连续从 1 递增
	for i, step := range trace.Steps {
		if step.Index != i+1 {
			t.Fatalf("step %d has non-sequential index %d", i, step.Index)
		}
	}

	// 前两步是 trigger,后两步是 action
	if trace.Steps[0].Phase != "trigger" || trace.Steps[1].Phase != "trigger" {
		t.Fatalf("expected first two steps to be trigger phase, got %q, %q", trace.Steps[0].Phase, trace.Steps[1].Phase)
	}
	if trace.Steps[2].Phase != "action" || trace.Steps[3].Phase != "action" {
		t.Fatalf("expected last two steps to be action phase, got %q, %q", trace.Steps[2].Phase, trace.Steps[3].Phase)
	}

	// 第一个 trigger 有 source,应为 evaluated
	if trace.Steps[0].Status != "evaluated" {
		t.Fatalf("expected trigger with source to be evaluated, got %q", trace.Steps[0].Status)
	}
	// 第二个 trigger 是时间窗口,应被标为 skipped
	if trace.Steps[1].Status != "skipped" {
		t.Fatalf("expected time-window trigger to be skipped, got %q", trace.Steps[1].Status)
	}
	if len(trace.Steps[1].Notes) == 0 {
		t.Fatalf("expected skipped trigger to carry a note")
	}

	// 第一个 action 完整,应为 evaluated
	if trace.Steps[2].Status != "evaluated" {
		t.Fatalf("expected complete action to be evaluated, got %q", trace.Steps[2].Status)
	}
	// 第二个 action 无 target,应被标为 blocked
	if trace.Steps[3].Status != "blocked" {
		t.Fatalf("expected action with no target to be blocked, got %q", trace.Steps[3].Status)
	}
	if len(trace.Steps[3].Notes) == 0 {
		t.Fatalf("expected blocked action to carry a note")
	}

	// GroupIndex 只在 trigger 步骤上设置
	if trace.Steps[0].GroupIndex == nil || *trace.Steps[0].GroupIndex != 0 {
		t.Fatalf("expected trigger step to reference group index 0")
	}
	if trace.Steps[2].GroupIndex != nil {
		t.Fatalf("expected action step to have no group index")
	}
}

// TestBuildSceneAutomationDryRunTraceEmptyDraft 验证空草稿产出零步骤但仍是有效 trace。
func TestBuildSceneAutomationDryRunTraceEmptyDraft(t *testing.T) {
	t.Parallel()

	req := &model.DryRunSceneAutomationReq{}
	result := &model.SceneAutomationDryRunResult{}

	trace := buildSceneAutomationDryRunTrace(req, result)

	if trace.StepCount != 0 {
		t.Fatalf("expected 0 steps for empty draft, got %d", trace.StepCount)
	}
	if trace.Steps == nil {
		t.Fatalf("expected non-nil steps slice even when empty")
	}
	if !trace.IsSimulation {
		t.Fatalf("expected empty trace to still be flagged as simulation")
	}
	if trace.EvaluatedAt == "" {
		t.Fatalf("expected evaluated_at timestamp to be set")
	}
}
