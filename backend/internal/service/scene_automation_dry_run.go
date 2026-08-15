package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*SceneAutomation) DryRunSceneAutomation(req *model.DryRunSceneAutomationReq, claims *utils.UserClaims) (*model.SceneAutomationDryRunResult, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to preview scene automation")
	}

	tenantID := claims.TenantID
	if req.ID != nil && *req.ID != "" {
		sceneAutomation, err := ensureSceneAutomationWriteAccess(*req.ID, claims)
		if err != nil {
			return nil, err
		}
		tenantID = sceneAutomation.TenantID
	}

	result := &model.SceneAutomationDryRunResult{
		Supported:          true,
		Valid:              true,
		CanSave:            true,
		Summary:            "自动化预演只校验引用关系并汇总执行约束，不会保存、执行、发布 MQTT 消息或刷新运行缓存。",
		DryRun:             buildSceneAutomationDryRunSummary(req.TriggerConditionGroups, req.Actions),
		ReferenceCounts:    buildSceneAutomationReferenceCounts(req.TriggerConditionGroups, req.Actions),
		Warnings:           buildSceneAutomationDryRunWarnings(req),
		Errors:             []string{},
		BlockingErrors:     []string{},
		SkippedConditions:  buildSceneAutomationDryRunSkippedConditions(req),
		UnavailableActions: buildSceneAutomationDryRunUnavailableActions(req),
	}

	if err := validateSceneAutomationReferences(req.TriggerConditionGroups, req.Actions, claims, tenantID); err != nil {
		result.Valid = false
		contextual := fmt.Sprintf("scene automation reference validation failed: %s", err.Error())
		result.Errors = append(result.Errors, contextual)
		result.BlockingErrors = append(result.BlockingErrors, contextual)
	}
	result.BlockingErrors = append(result.BlockingErrors, buildSceneAutomationDryRunSaveBlockers(req)...)
	result.CanSave = len(result.BlockingErrors) == 0

	if claims.Authority == constant.SYS_ADMIN && tenantID == "" {
		result.Warnings = append(result.Warnings, "system admin preview has no tenant scope until the rule is saved")
	}

	result.Diagnostics = buildSceneAutomationDryRunDiagnostics(result.Warnings, result.BlockingErrors)
	result.NextSteps = buildSceneAutomationDryRunNextSteps(result.CanSave, result.Warnings)
	result.ExecutionTrace = buildSceneAutomationDryRunTrace(req, result)

	return result, nil
}

// buildSceneAutomationDryRunTrace 把已校验的条件与动作按"先触发后动作"的真实评估顺序
// 展开为有序步骤序列。它不执行规则、不发布 MQTT、不读取实时遥测,只是对当前草稿做静态推演,
// 方便前端逐步呈现"保存并启用后会怎样评估"。每一步的 Status 复用已算出的 skipped/blocked 结论。
func buildSceneAutomationDryRunTrace(req *model.DryRunSceneAutomationReq, result *model.SceneAutomationDryRunResult) model.SceneAutomationDryRunTrace {
	skippedSet := map[string]struct{}{}
	for _, s := range result.SkippedConditions {
		skippedSet[s] = struct{}{}
	}
	unavailableSet := map[string]struct{}{}
	for _, s := range result.UnavailableActions {
		unavailableSet[s] = struct{}{}
	}

	steps := make([]model.SceneAutomationDryRunTraceStep, 0)
	index := 0

	for groupIndex, group := range req.TriggerConditionGroups {
		gi := groupIndex
		for conditionIndex, condition := range group {
			index++
			rowLabel := fmt.Sprintf("condition group #%d row #%d", groupIndex+1, conditionIndex+1)
			kind := sceneAutomationConditionTypeName(condition.TriggerConditionsType)
			target := sceneAutomationConditionReferenceKind(condition.TriggerConditionsType)

			status := "evaluated"
			notes := []string{}
			for skipped := range skippedSet {
				if strings.Contains(skipped, rowLabel) {
					status = "skipped"
					notes = append(notes, skipped)
				}
			}
			sort.Strings(notes)

			detail := "引用关系在预演中校验;实时遥测/在线状态/定时时机不判断"
			if condition.TriggerSource != nil && *condition.TriggerSource != "" {
				detail = fmt.Sprintf("references %s %q", target, *condition.TriggerSource)
			}

			steps = append(steps, model.SceneAutomationDryRunTraceStep{
				Index:      index,
				Phase:      "trigger",
				Kind:       kind,
				Target:     target,
				Label:      rowLabel,
				Status:     status,
				Detail:     detail,
				Notes:      notes,
				GroupIndex: &gi,
			})
		}
	}

	for actionIndex, action := range req.Actions {
		index++
		actionLabel := fmt.Sprintf("action #%d", actionIndex+1)
		kind := sceneAutomationActionTypeName(action.ActionType)
		target := sceneAutomationActionTargetKind(action)

		status := "evaluated"
		notes := []string{}
		for unavailable := range unavailableSet {
			if strings.Contains(unavailable, actionLabel) {
				status = "blocked"
				notes = append(notes, unavailable)
			}
		}
		sort.Strings(notes)

		detail := "动作目标引用已在预演中校验;不会真实下发或发布 MQTT"
		if action.ActionTarget != "" {
			detail = fmt.Sprintf("targets %s %q", target, action.ActionTarget)
		}

		steps = append(steps, model.SceneAutomationDryRunTraceStep{
			Index:  index,
			Phase:  "action",
			Kind:   kind,
			Target: target,
			Label:  actionLabel,
			Status: status,
			Detail: detail,
			Notes:  notes,
		})
	}

	return model.SceneAutomationDryRunTrace{
		Steps:        steps,
		StepCount:    len(steps),
		EvaluatedAt:  time.Now().UTC().Format(time.RFC3339),
		Explanation:  "执行 trace 按先触发条件后动作的顺序静态推演,仅反映草稿结构与引用校验,不代表真实运行结果。",
		IsSimulation: true,
	}
}

func buildSceneAutomationDryRunSummary(triggerConditionGroups [][]model.Condition, actions []model.Action) model.SceneAutomationDryRunSummary {
	summary := model.SceneAutomationDryRunSummary{
		ConditionGroupCount: len(triggerConditionGroups),
		ConditionTypes:      map[string]int{},
		ActionTypes:         map[string]int{},
		TargetKinds:         map[string]int{},
	}

	for _, group := range triggerConditionGroups {
		summary.ConditionCount += len(group)
		for _, condition := range group {
			summary.ConditionTypes[sceneAutomationConditionTypeName(condition.TriggerConditionsType)]++
		}
	}

	for _, action := range actions {
		summary.ActionCount++
		summary.ActionTypes[sceneAutomationActionTypeName(action.ActionType)]++
		summary.TargetKinds[sceneAutomationActionTargetKind(action)]++
	}

	return summary
}

func buildSceneAutomationReferenceCounts(triggerConditionGroups [][]model.Condition, actions []model.Action) map[string]int {
	referencesByKind := map[string]map[string]struct{}{}
	addReference := func(kind string, id string) {
		if kind == "" || id == "" {
			return
		}
		if referencesByKind[kind] == nil {
			referencesByKind[kind] = map[string]struct{}{}
		}
		referencesByKind[kind][id] = struct{}{}
	}

	for _, group := range triggerConditionGroups {
		for _, condition := range group {
			if condition.TriggerSource == nil {
				continue
			}
			addReference(sceneAutomationConditionReferenceKind(condition.TriggerConditionsType), *condition.TriggerSource)
		}
	}

	for _, action := range actions {
		addReference(sceneAutomationActionTargetKind(action), action.ActionTarget)
	}

	counts := map[string]int{}
	for kind, references := range referencesByKind {
		counts[kind] = len(references)
	}
	return counts
}

func buildSceneAutomationDryRunWarnings(req *model.DryRunSceneAutomationReq) []string {
	warnings := []string{
		"预演不会判断实时遥测值、设备在线状态或定时触发时机",
	}
	if req.Enabled == "N" {
		warnings = append(warnings, "rule is currently marked disabled; saving it disabled will not activate runtime cache")
	}
	if len(req.TriggerConditionGroups) == 0 {
		warnings = append(warnings, "no trigger condition groups were provided")
	}
	if len(req.Actions) == 0 {
		warnings = append(warnings, "no actions were provided")
	}
	for groupIndex, group := range req.TriggerConditionGroups {
		if len(group) == 0 {
			warnings = append(warnings, fmt.Sprintf("condition group #%d is empty", groupIndex+1))
		}
		for conditionIndex, condition := range group {
			if (condition.TriggerConditionsType == "10" || condition.TriggerConditionsType == "11") && condition.TriggerSource == nil {
				warnings = append(warnings, fmt.Sprintf("condition group #%d row #%d has no trigger source", groupIndex+1, conditionIndex+1))
			}
		}
	}
	for actionIndex, action := range req.Actions {
		if action.ActionTarget == "" {
			warnings = append(warnings, fmt.Sprintf("action #%d has no target", actionIndex+1))
		}
		if (action.ActionType == "10" || action.ActionType == "11") && action.ActionParam == "" {
			warnings = append(warnings, fmt.Sprintf("action #%d has no command parameter", actionIndex+1))
		}
	}
	return warnings
}

func buildSceneAutomationDryRunSkippedConditions(req *model.DryRunSceneAutomationReq) []string {
	skipped := []string{}
	if len(req.TriggerConditionGroups) == 0 {
		return []string{"no condition rows are available for match evaluation"}
	}

	for groupIndex, group := range req.TriggerConditionGroups {
		if len(group) == 0 {
			skipped = append(skipped, fmt.Sprintf("condition group #%d has no rows to evaluate", groupIndex+1))
			continue
		}
		for conditionIndex, condition := range group {
			rowLabel := fmt.Sprintf("condition group #%d row #%d", groupIndex+1, conditionIndex+1)
			switch condition.TriggerConditionsType {
			case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
				skipped = append(skipped, fmt.Sprintf("%s 的实时遥测值和在线状态匹配不会在预演中判断", rowLabel))
				if condition.TriggerSource == nil {
					skipped = append(skipped, fmt.Sprintf("%s has no trigger source, so reference validation is limited", rowLabel))
				}
			case "20", "21", model.DEVICE_TRIGGER_CONDITION_TYPE_TIME:
				skipped = append(skipped, fmt.Sprintf("%s 的定时或时间窗口不会在预演中判断", rowLabel))
			case "":
				skipped = append(skipped, fmt.Sprintf("%s has no condition type, so match evaluation is skipped", rowLabel))
			}
		}
	}

	return skipped
}

func buildSceneAutomationDryRunUnavailableActions(req *model.DryRunSceneAutomationReq) []string {
	unavailable := []string{}
	if len(req.Actions) == 0 {
		return []string{"no action rows are available for validation"}
	}

	for actionIndex, action := range req.Actions {
		actionLabel := fmt.Sprintf("action #%d", actionIndex+1)
		if action.ActionTarget == "" {
			unavailable = append(unavailable, fmt.Sprintf("%s has no target", actionLabel))
		}
		if (action.ActionType == model.AUTOMATE_ACTION_TYPE_ONE || action.ActionType == model.AUTOMATE_ACTION_TYPE_MULTIPLE) && action.ActionParam == "" {
			unavailable = append(unavailable, fmt.Sprintf("%s has no command parameter", actionLabel))
		}
		if action.ActionType == "" {
			unavailable = append(unavailable, fmt.Sprintf("%s has no action type", actionLabel))
		}
	}

	return unavailable
}

func buildSceneAutomationDryRunSaveBlockers(req *model.DryRunSceneAutomationReq) []string {
	blockers := []string{}
	if len(req.TriggerConditionGroups) == 0 {
		blockers = append(blockers, "add at least one trigger condition group before saving")
	}
	if len(req.Actions) == 0 {
		blockers = append(blockers, "add at least one action before saving")
	}
	for groupIndex, group := range req.TriggerConditionGroups {
		if len(group) == 0 {
			blockers = append(blockers, fmt.Sprintf("condition group #%d must contain at least one condition before saving", groupIndex+1))
		}
	}
	return blockers
}

func buildSceneAutomationDryRunDiagnostics(warnings []string, errors []string) []model.SceneAutomationDryRunDiagnostic {
	diagnostics := make([]model.SceneAutomationDryRunDiagnostic, 0, len(warnings)+len(errors)+1)
	for _, errText := range errors {
		diagnostics = append(diagnostics, model.SceneAutomationDryRunDiagnostic{
			Severity: "error",
			Scope:    "reference_validation",
			Message:  errText,
		})
	}
	for _, warning := range warnings {
		diagnostics = append(diagnostics, model.SceneAutomationDryRunDiagnostic{
			Severity: "warning",
			Scope:    "preview_limit",
			Message:  warning,
		})
	}
	if len(errors) == 0 {
		diagnostics = append(diagnostics, model.SceneAutomationDryRunDiagnostic{
			Severity: "success",
			Scope:    "reference_validation",
			Message:  "引用的设备、物模型、场景和报警规则已通过预演校验",
		})
	}
	return diagnostics
}

func buildSceneAutomationDryRunNextSteps(valid bool, warnings []string) []string {
	if !valid {
		return []string{
			"fix invalid or inaccessible references before saving the automation",
			"修改条件来源或动作目标后，请重新运行预演",
		}
	}
	if len(warnings) > 0 {
		return []string{
			"review warnings before enabling this automation",
			"保存后请使用执行日志确认实时遥测和定时行为",
		}
	}
	return []string{
		"save the automation when the local explanation matches the intended rule",
		"use execution logs after saving to monitor real runtime matches and action results",
	}
}

func sceneAutomationConditionTypeName(conditionType string) string {
	switch conditionType {
	case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE:
		return "single_device"
	case model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
		return "device_profile"
	case "20":
		return "one_time_schedule"
	case "21":
		return "recurring_schedule"
	case model.DEVICE_TRIGGER_CONDITION_TYPE_TIME:
		return "time_range"
	default:
		if conditionType == "" {
			return "unspecified"
		}
		return conditionType
	}
}

func sceneAutomationConditionReferenceKind(conditionType string) string {
	switch conditionType {
	case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE:
		return "device"
	case model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
		return "device_profile"
	default:
		return ""
	}
}

func sceneAutomationActionTypeName(actionType string) string {
	switch actionType {
	case model.AUTOMATE_ACTION_TYPE_ONE:
		return "single_device"
	case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
		return "device_profile"
	case model.AUTOMATE_ACTION_TYPE_SCENE:
		return "activate_scene"
	case model.AUTOMATE_ACTION_TYPE_ALARM:
		return "trigger_alarm"
	case model.AUTOMATE_ACTION_TYPE_SERVICE:
		return "service"
	default:
		if actionType == "" {
			return "unspecified"
		}
		return actionType
	}
}

func sceneAutomationActionTargetKind(action model.Action) string {
	switch action.ActionType {
	case model.AUTOMATE_ACTION_TYPE_ONE:
		return "device"
	case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
		return "device_profile"
	case model.AUTOMATE_ACTION_TYPE_SCENE:
		return "scene"
	case model.AUTOMATE_ACTION_TYPE_ALARM:
		return "alarm"
	default:
		return "other"
	}
}
