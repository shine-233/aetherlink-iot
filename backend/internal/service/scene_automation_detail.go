package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*SceneAutomation) GetSceneAutomation(sceneAutomationID string, claims *utils.UserClaims) (interface{}, error) {
	sceneAutomation, err := ensureSceneAutomationReadAccess(sceneAutomationID, claims)
	if err != nil {
		return nil, err
	}

	references, err := loadSceneAutomationDetailReadReferences(sceneAutomationID)
	if err != nil {
		return nil, err
	}

	res := make(map[string]interface{})
	res["id"] = sceneAutomation.ID
	res["name"] = sceneAutomation.Name
	res["description"] = sceneAutomation.Description
	res["enabled"] = sceneAutomation.Enabled
	res["tenant_id"] = sceneAutomation.TenantID
	res["creator"] = sceneAutomation.Creator
	res["updator"] = sceneAutomation.Updator

	triggerConditionGroups := make([][]map[string]interface{}, 0)
	triggerConditionGroups = append(triggerConditionGroups, buildSceneAutomationPeriodicTaskTriggerGroups(references.periodicTasks)...)
	triggerConditionGroups = append(triggerConditionGroups, buildSceneAutomationOneTimeTaskTriggerGroups(references.oneTimeTasks)...)
	triggerConditionGroups = append(triggerConditionGroups, buildSceneAutomationDeviceTriggerConditionGroups(references.deviceTriggerConditions)...)

	res["trigger_condition_groups"] = triggerConditionGroups

	if len(references.actionInfos) > 0 {
		res["actions"] = buildSceneAutomationDetailActionResponse(references.actionInfos)
	}

	return res, nil
}

type sceneAutomationDetailReferences struct {
	deviceTriggerConditions []*model.DeviceTriggerCondition
	oneTimeTasks            []*model.OneTimeTask
	periodicTasks           []*model.PeriodicTask
	actionInfos             []*model.ActionInfo
}

func loadSceneAutomationDetailReadReferences(sceneAutomationID string) (*sceneAutomationDetailReferences, error) {
	deviceTriggerConditions, err := dal.GetDeviceTriggerCondition(sceneAutomationID)
	if err != nil {
		return nil, sceneAutomationDBError(err)
	}

	oneTimeTasks, err := dal.GetOneTimeTask(sceneAutomationID)
	if err != nil {
		return nil, sceneAutomationDBError(err)
	}

	periodicTasks, err := dal.GetPeriodicTask(sceneAutomationID)
	if err != nil {
		return nil, sceneAutomationDBError(err)
	}

	actionInfos, err := dal.GetActionInfo(sceneAutomationID)
	if err != nil {
		return nil, sceneAutomationDBError(err)
	}

	return &sceneAutomationDetailReferences{
		deviceTriggerConditions: deviceTriggerConditions,
		oneTimeTasks:            oneTimeTasks,
		periodicTasks:           periodicTasks,
		actionInfos:             actionInfos,
	}, nil
}

func buildSceneAutomationPeriodicTaskTriggerGroups(periodicTasks []*model.PeriodicTask) [][]map[string]interface{} {
	triggerGroups := make([][]map[string]interface{}, 0)
	for _, periodicTask := range periodicTasks {
		triggerGroups = append(triggerGroups, []map[string]interface{}{
			buildSceneAutomationPeriodicTaskTriggerMap(periodicTask),
		})
	}
	return triggerGroups
}

func buildSceneAutomationPeriodicTaskTriggerMap(periodicTask *model.PeriodicTask) map[string]interface{} {
	return map[string]interface{}{
		"task_type":               periodicTask.TaskType,
		"expiration_time":         periodicTask.ExpirationTime,
		"params":                  periodicTask.Param,
		"trigger_conditions_type": "21",
	}
}

func buildSceneAutomationOneTimeTaskTriggerGroups(oneTimeTasks []*model.OneTimeTask) [][]map[string]interface{} {
	triggerGroups := make([][]map[string]interface{}, 0)
	for _, oneTimeTask := range oneTimeTasks {
		triggerGroups = append(triggerGroups, []map[string]interface{}{
			buildSceneAutomationOneTimeTaskTriggerMap(oneTimeTask),
		})
	}
	return triggerGroups
}

func buildSceneAutomationOneTimeTaskTriggerMap(oneTimeTask *model.OneTimeTask) map[string]interface{} {
	return map[string]interface{}{
		"execution_time":          oneTimeTask.ExecutionTime,
		"expiration_time":         oneTimeTask.ExpirationTime,
		"trigger_conditions_type": "20",
	}
}

func buildSceneAutomationDeviceTriggerConditionGroups(deviceTriggerConditions []*model.DeviceTriggerCondition) [][]map[string]interface{} {
	triggerGroups := make([][]map[string]interface{}, 0)
	for _, group := range groupSceneAutomationDeviceTriggerConditionsByGroupID(deviceTriggerConditions) {
		triggerGroups = append(triggerGroups, buildSceneAutomationDeviceTriggerConditionGroup(group))
	}
	return triggerGroups
}

func groupSceneAutomationDeviceTriggerConditionsByGroupID(deviceTriggerConditions []*model.DeviceTriggerCondition) map[string][]*model.DeviceTriggerCondition {
	groupedConditions := make(map[string][]*model.DeviceTriggerCondition)
	for _, condition := range deviceTriggerConditions {
		groupedConditions[condition.GroupID] = append(groupedConditions[condition.GroupID], condition)
	}
	return groupedConditions
}

func buildSceneAutomationDeviceTriggerConditionGroup(deviceTriggerConditions []*model.DeviceTriggerCondition) []map[string]interface{} {
	group := make([]map[string]interface{}, 0)
	for _, condition := range deviceTriggerConditions {
		group = append(group, buildSceneAutomationDeviceTriggerConditionMap(condition))
	}
	return group
}

func buildSceneAutomationDeviceTriggerConditionMap(condition *model.DeviceTriggerCondition) map[string]interface{} {
	return map[string]interface{}{
		"id":                      condition.ID,
		"group_id":                condition.GroupID,
		"trigger_conditions_type": condition.TriggerConditionType,
		"trigger_source":          condition.TriggerSource,
		"trigger_param_type":      condition.TriggerParamType,
		"trigger_param":           condition.TriggerParam,
		"trigger_operator":        condition.TriggerOperator,
		"trigger_value":           condition.TriggerValue,
	}
}

func buildSceneAutomationDetailActionResponse(actionInfos []*model.ActionInfo) []map[string]interface{} {
	actionInfoMap := make([]map[string]interface{}, 0)
	for _, actionInfo := range actionInfos {
		actionInfoMap = append(actionInfoMap, buildSceneAutomationDetailActionMap(actionInfo))
	}
	return actionInfoMap
}

func buildSceneAutomationDetailActionMap(actionInfo *model.ActionInfo) map[string]interface{} {
	return map[string]interface{}{
		"action_type":       actionInfo.ActionType,
		"action_target":     actionInfo.ActionTarget,
		"action_param_type": actionInfo.ActionParamType,
		"action_param":      actionInfo.ActionParam,
		"action_value":      actionInfo.ActionValue,
	}
}
