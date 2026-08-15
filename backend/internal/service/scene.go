// 文件用途：维护场景服务的基础信息、查询和执行协调。
// 核心逻辑：读取与更新场景定义，组合设备、条件、动作和日志数据供前端场景页使用。
// 关键注意事项：场景跨设备和动作边界，跨租户读取或错误启停会导致错误自动化执行。
// 重构建议：拆分场景 CRUD、执行协调和日志记录，补齐权限、事务、幂等和失败隔离测试。
package service

import (
	"fmt"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type Scene struct{}

func ensureSceneReadAccess(sceneID string, claims *utils.UserClaims) (*model.SceneInfo, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene")
	}
	sceneInfo, err := dal.GetSceneInfo(sceneID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if claims.Authority == constant.SYS_ADMIN || sceneInfo.TenantID == claims.TenantID {
		return sceneInfo, nil
	}
	return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene")
}

func ensureSceneWriteAccess(sceneID string, claims *utils.UserClaims) (*model.SceneInfo, error) {
	sceneInfo, err := ensureSceneReadAccess(sceneID, claims)
	if err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN && sceneInfo.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify scene")
	}
	return sceneInfo, nil
}

func validateSceneActionReferences(actions []model.SceneActionsReq, claims *utils.UserClaims, tenantID string) error {
	for _, action := range actions {
		switch action.ActionType {
		case model.AUTOMATE_ACTION_TYPE_ONE:
			if action.ActionTarget == "" {
				return errcode.NewWithMessage(errcode.CodeParamError, "scene action device target is required")
			}
			device, err := ensureTelemetryDeviceWriteAccess(action.ActionTarget, claims)
			if err != nil {
				return err
			}
			if device.TenantID != tenantID {
				return errcode.NewWithMessage(errcode.CodeNoPermission, "scene action device tenant mismatch")
			}
		case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
			if action.ActionTarget == "" {
				return errcode.NewWithMessage(errcode.CodeParamError, "scene action device config target is required")
			}
			deviceConfig, err := ensureDeviceConfigWriteAccess(action.ActionTarget, claims)
			if err != nil {
				return err
			}
			if deviceConfig.TenantID != tenantID {
				return errcode.NewWithMessage(errcode.CodeNoPermission, "scene action device config tenant mismatch")
			}
		case model.AUTOMATE_ACTION_TYPE_ALARM:
			if action.ActionTarget == "" {
				return errcode.NewWithMessage(errcode.CodeParamError, "scene action alarm target is required")
			}
			alarmConfig, err := ensureAlarmConfigWriteAccess(action.ActionTarget, claims)
			if err != nil {
				return err
			}
			if alarmConfig.TenantID != tenantID {
				return errcode.NewWithMessage(errcode.CodeNoPermission, "scene action alarm config tenant mismatch")
			}
		}
	}
	return nil
}

func dryRunSceneActionsToSceneActions(actions []model.DryRunSceneActionReq) []model.SceneActionsReq {
	sceneActions := make([]model.SceneActionsReq, 0, len(actions))
	for _, action := range actions {
		sceneActions = append(sceneActions, model.SceneActionsReq{
			ActionType:      action.ActionType,
			ActionTarget:    action.ActionTarget,
			ActionParamType: action.ActionParamType,
			ActionParam:     action.ActionParam,
			ActionValue:     action.ActionValue,
			Remark:          action.Remark,
		})
	}
	return sceneActions
}

func buildSceneDryRunSummary(actions []model.SceneActionsReq) model.SceneAutomationDryRunSummary {
	summary := model.SceneAutomationDryRunSummary{
		ConditionTypes: map[string]int{},
		ActionTypes:    map[string]int{},
		TargetKinds:    map[string]int{},
		ActionCount:    len(actions),
	}

	for _, action := range actions {
		actionType := action.ActionType
		summary.ActionTypes[sceneAutomationActionTypeName(actionType)]++
		summary.TargetKinds[sceneActionTargetKind(action)]++
	}

	return summary
}

func sceneActionTargetKind(action model.SceneActionsReq) string {
	switch action.ActionType {
	case model.AUTOMATE_ACTION_TYPE_ONE:
		return "device"
	case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
		return "device_profile"
	case model.AUTOMATE_ACTION_TYPE_ALARM:
		return "alarm"
	default:
		if action.ActionType == "" {
			return "unspecified"
		}
		return "other"
	}
}

func buildSceneDryRunReferenceCounts(actions []model.SceneActionsReq) map[string]int {
	referencesByKind := map[string]map[string]struct{}{}
	for _, action := range actions {
		kind := sceneActionTargetKind(action)
		if kind == "" || kind == "unspecified" || kind == "other" || action.ActionTarget == "" {
			continue
		}
		if referencesByKind[kind] == nil {
			referencesByKind[kind] = map[string]struct{}{}
		}
		referencesByKind[kind][action.ActionTarget] = struct{}{}
	}

	counts := map[string]int{}
	for kind, references := range referencesByKind {
		counts[kind] = len(references)
	}
	return counts
}

func buildSceneDryRunWarnings(req *model.DryRunSceneReq) []string {
	return []string{}
}

func isBlankStringPointer(value *string) bool {
	return value == nil || *value == ""
}

func buildSceneDryRunSaveBlockers(actions []model.SceneActionsReq) []string {
	blockers := []string{}
	if len(actions) == 0 {
		return []string{"add at least one scene action before saving"}
	}

	for actionIndex, action := range actions {
		label := fmt.Sprintf("action #%d", actionIndex+1)
		if action.ActionType == "" {
			blockers = append(blockers, fmt.Sprintf("%s has no action type", label))
		}
		if action.ActionTarget == "" {
			blockers = append(blockers, fmt.Sprintf("%s has no target", label))
		}
		if (action.ActionType == model.AUTOMATE_ACTION_TYPE_ONE || action.ActionType == model.AUTOMATE_ACTION_TYPE_MULTIPLE) && isBlankStringPointer(action.ActionParam) {
			blockers = append(blockers, fmt.Sprintf("%s has no command parameter", label))
		}
		if (action.ActionType == model.AUTOMATE_ACTION_TYPE_ONE || action.ActionType == model.AUTOMATE_ACTION_TYPE_MULTIPLE) && isBlankStringPointer(action.ActionValue) {
			blockers = append(blockers, fmt.Sprintf("%s has no command value", label))
		}
	}

	return blockers
}

func buildSceneDryRunUnavailableActions(actions []model.SceneActionsReq) []string {
	unavailable := []string{}
	for actionIndex, action := range actions {
		label := fmt.Sprintf("action #%d", actionIndex+1)
		if action.ActionType == "" {
			unavailable = append(unavailable, fmt.Sprintf("%s has no action type", label))
		}
		if action.ActionTarget == "" {
			unavailable = append(unavailable, fmt.Sprintf("%s has no target", label))
		}
	}
	if len(actions) == 0 {
		return []string{"no scene action rows are available for validation"}
	}
	return unavailable
}

func buildSceneDryRunNextSteps(canSave bool) []string {
	if !canSave {
		return []string{
			"fix invalid or inaccessible scene action targets before saving",
			"修改动作目标或命令参数后，请重新运行场景预演",
		}
	}
	return []string{
		"save the scene when the action preview matches the intended manual operation",
		"after saving, activate the scene or inspect scene logs to prove real command/alarm execution",
	}
}

func (*Scene) DryRunScene(req model.DryRunSceneReq, claims *utils.UserClaims) (*model.SceneAutomationDryRunResult, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to preview scene")
	}

	tenantID := claims.TenantID
	if req.ID != nil && *req.ID != "" {
		sceneInfo, err := ensureSceneWriteAccess(*req.ID, claims)
		if err != nil {
			return nil, err
		}
		tenantID = sceneInfo.TenantID
	}

	actions := dryRunSceneActionsToSceneActions(req.Actions)
	warnings := buildSceneDryRunWarnings(&req)
	blockers := buildSceneDryRunSaveBlockers(actions)
	errors := []string{}
	if len(blockers) == 0 {
		if err := validateSceneActionReferences(actions, claims, tenantID); err != nil {
			errors = append(errors, err.Error())
			blockers = append(blockers, err.Error())
		}
	}

	canSave := len(blockers) == 0
	return &model.SceneAutomationDryRunResult{
		Supported:          true,
		Valid:              canSave,
		CanSave:            canSave,
		Summary:            "场景预演只校验动作目标和保存条件，不会保存场景、下发命令、触发报警或执行设备操作。",
		DryRun:             buildSceneDryRunSummary(actions),
		ReferenceCounts:    buildSceneDryRunReferenceCounts(actions),
		Warnings:           warnings,
		Errors:             errors,
		BlockingErrors:     blockers,
		SkippedConditions:  []string{},
		UnavailableActions: buildSceneDryRunUnavailableActions(actions),
		Diagnostics:        buildSceneAutomationDryRunDiagnostics(warnings, blockers),
		NextSteps:          buildSceneDryRunNextSteps(canSave),
	}, nil
}

func (*Scene) CreateScene(req model.CreateSceneReq, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create scene")
	}
	if err := validateSceneActionReferences(req.Actions, claims, claims.TenantID); err != nil {
		return "", err
	}
	id, err := dal.CreateSceneInfo(req, claims)
	if err != nil {
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return id, err
}

func (*Scene) UpdateScene(req model.UpdateSceneReq, claims *utils.UserClaims) (string, error) {
	sceneInfo, err := ensureSceneWriteAccess(req.ID, claims)
	if err != nil {
		return "", err
	}
	if err := validateSceneActionReferences(req.Actions, claims, sceneInfo.TenantID); err != nil {
		return "", err
	}
	id, err := dal.UpdateSceneInfo(req, claims, sceneInfo.TenantID)
	if err != nil {
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return id, err
}

func (*Scene) DeleteScene(scene_id string, claims *utils.UserClaims) error {
	if _, err := ensureSceneWriteAccess(scene_id, claims); err != nil {
		return err
	}
	err := dal.DeleteSceneInfo(scene_id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*Scene) GetScene(scene_id string, claims *utils.UserClaims) (interface{}, error) {
	sceneInfo, err := ensureSceneReadAccess(scene_id, claims)
	if err != nil {
		return nil, err
	}

	sceneActionsInfo, err := dal.GetSceneActionsInfo(scene_id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return sceneDetailResponse(sceneInfo, sceneActionsInfo), nil
}

func (*Scene) GetSceneListByPage(req model.GetSceneListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene")
	}
	total, sceneInfo, err := dal.GetSceneInfoByPage(&req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return sceneListResponse(total, sceneInfo), nil
}

func (*Scene) ActiveScene(scene_id string, claims *utils.UserClaims) error {
	sceneInfo, err := ensureSceneWriteAccess(scene_id, claims)
	if err != nil {
		return err
	}
	err = GroupApp.ActiveSceneExecute(scene_id, sceneInfo.TenantID)
	if err != nil {
		return err
	}
	return nil
}

func (*Scene) GetSceneLog(req model.GetSceneLogListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureSceneReadAccess(req.ID, claims); err != nil {
		return nil, err
	}
	total, data, err := dal.GetSceneLogByPage(req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return sceneLogListResponse(total, data), nil
}

func sceneDetailResponse(sceneInfo *model.SceneInfo, actions []*model.SceneActionInfo) map[string]interface{} {
	return map[string]interface{}{
		"info":    sceneInfo,
		"actions": actions,
	}
}

func sceneListResponse(total int64, list []*model.SceneInfo) map[string]interface{} {
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}
}

func sceneLogListResponse(total int64, list []*model.SceneLog) map[string]interface{} {
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}
}
