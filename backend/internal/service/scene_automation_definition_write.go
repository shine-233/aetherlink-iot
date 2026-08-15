package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func createSceneAutomationRecord(tx *query.QueryTx, req *model.CreateSceneAutomationReq, claims *utils.UserClaims, enabled string) (string, error) {
	now := utils.GetUTCTime()
	sceneAutomation := model.SceneAutomation{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: &req.Description,
		Enabled:     enabled,
		TenantID:    claims.TenantID,
		Creator:     claims.ID,
		Updator:     claims.ID,
		CreatedAt:   now,
		UpdatedAt:   &now,
		Remark:      &req.Remark,
	}
	logrus.Info("create scene automation")
	if err := dal.CreateSceneAutomation(&sceneAutomation, tx); err != nil {
		return "", sceneAutomationDBError(err)
	}
	return sceneAutomation.ID, nil
}

func switchSceneAutomationDefinition(sceneAutomationID string, target string) error {
	return withSceneAutomationTransaction(func(tx *query.QueryTx) error {
		switchSteps := []func(string, string, *query.QueryTx) error{
			dal.SwitchSceneAutomation,
			dal.SwitchDeviceTriggerCondition,
			dal.SwitchOneTimeTask,
			dal.SwitchPeriodicTask,
		}
		for _, switchStep := range switchSteps {
			if err := switchStep(sceneAutomationID, target, tx); err != nil {
				return sceneAutomationDBError(err)
			}
		}
		return nil
	})
}

func replaceSceneAutomationDefinition(tx *query.QueryTx, sceneAutomationID string, req *model.UpdateSceneAutomationReq, tenantID string, updatorID string) error {
	if err := clearSceneAutomationDefinition(tx, sceneAutomationID); err != nil {
		return err
	}
	if err := saveUpdatedSceneAutomation(tx, sceneAutomationID, req, tenantID, updatorID); err != nil {
		return err
	}
	if err := writeSceneAutomationTriggerGroups(tx, sceneAutomationID, req.Enabled, tenantID, req.TriggerConditionGroups); err != nil {
		return err
	}
	return writeSceneAutomationActions(tx, sceneAutomationID, req.Actions)
}

func saveUpdatedSceneAutomation(tx *query.QueryTx, sceneAutomationID string, req *model.UpdateSceneAutomationReq, tenantID string, updatorID string) error {
	sceneAutomation := model.SceneAutomation{
		ID:          sceneAutomationID,
		Name:        req.Name,
		Description: &req.Description,
		Enabled:     req.Enabled,
		TenantID:    tenantID,
		Updator:     updatorID,
		Remark:      &req.Remark,
	}
	logrus.Info("update scene automation")
	if err := dal.SaveSceneAutomation(&sceneAutomation, tx); err != nil {
		return sceneAutomationDBError(err)
	}
	return nil
}

func clearSceneAutomationDefinition(tx *query.QueryTx, sceneAutomationID string) error {
	clearSteps := []func(string, *query.QueryTx) error{
		dal.DeleteDeviceTriggerCondition,
		dal.DeleteOneTimeTask,
		dal.DeletePeriodicTask,
		dal.DeleteActionInfo,
	}
	for _, clearStep := range clearSteps {
		if err := clearStep(sceneAutomationID, tx); err != nil {
			return sceneAutomationDBError(err)
		}
	}
	return nil
}

func writeSceneAutomationTriggerGroups(tx *query.QueryTx, sceneAutomationID string, enabled string, tenantID string, groups [][]model.Condition) error {
	for _, group := range groups {
		if err := writeSceneAutomationTriggerGroup(tx, sceneAutomationID, enabled, tenantID, group); err != nil {
			return err
		}
	}
	return nil
}

func writeSceneAutomationTriggerGroup(tx *query.QueryTx, sceneAutomationID string, enabled string, tenantID string, group []model.Condition) error {
	groupID := uuid.New()
	for _, condition := range group {
		if err := writeSceneAutomationTriggerCondition(tx, sceneAutomationID, enabled, tenantID, groupID, condition); err != nil {
			return err
		}
	}
	return validateSceneAutomationTriggerGroupMix(group)
}

func validateSceneAutomationTriggerGroupMix(group []model.Condition) error {
	var oneCondition, multipleCondition bool
	for _, condition := range group {
		switch condition.TriggerConditionsType {
		case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE:
			oneCondition = true
		case model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
			multipleCondition = true
		}
	}
	if oneCondition && multipleCondition {
		return errcode.New(200060)
	}
	return nil
}

func writeSceneAutomationTriggerCondition(tx *query.QueryTx, sceneAutomationID string, enabled string, tenantID string, groupID string, condition model.Condition) error {
	switch condition.TriggerConditionsType {
	case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE, model.DEVICE_TRIGGER_CONDITION_TYPE_TIME:
		return writeSceneAutomationDeviceTriggerCondition(tx, sceneAutomationID, enabled, tenantID, groupID, condition)
	case "20":
		return writeSceneAutomationOneTimeTask(tx, sceneAutomationID, enabled, condition)
	case "21":
		return writeSceneAutomationPeriodicTask(tx, sceneAutomationID, condition)
	default:
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"err": "not support trigger type",
		})
	}
}

func writeSceneAutomationDeviceTriggerCondition(tx *query.QueryTx, sceneAutomationID string, enabled string, tenantID string, groupID string, condition model.Condition) error {
	logrus.Info("create device trigger condition")
	if err := dal.CreateDeviceTriggerCondition(buildSceneAutomationDeviceTriggerCondition(sceneAutomationID, enabled, tenantID, groupID, condition), tx); err != nil {
		return sceneAutomationDBError(err)
	}
	return nil
}

func buildSceneAutomationDeviceTriggerCondition(sceneAutomationID string, enabled string, tenantID string, groupID string, condition model.Condition) model.DeviceTriggerCondition {
	dtc := model.DeviceTriggerCondition{
		ID:                   uuid.New(),
		SceneAutomationID:    sceneAutomationID,
		Enabled:              enabled,
		GroupID:              groupID,
		TriggerConditionType: condition.TriggerConditionsType,
		TriggerSource:        condition.TriggerSource,
		TriggerParamType:     condition.TriggerParamType,
		TriggerParam:         condition.TriggerParam,
		TriggerOperator:      condition.TriggerOperator,
		TenantID:             tenantID,
	}
	if condition.TriggerValue != nil {
		dtc.TriggerValue = *condition.TriggerValue
	}
	return dtc
}

func writeSceneAutomationOneTimeTask(tx *query.QueryTx, sceneAutomationID string, enabled string, condition model.Condition) error {
	logrus.Info("create one-time task")
	if err := dal.CreateOneTimeTask(buildSceneAutomationOneTimeTask(sceneAutomationID, enabled, condition), tx); err != nil {
		return sceneAutomationDBError(err)
	}
	return nil
}

func buildSceneAutomationOneTimeTask(sceneAutomationID string, enabled string, condition model.Condition) model.OneTimeTask {
	task := model.OneTimeTask{
		ID:                uuid.New(),
		SceneAutomationID: sceneAutomationID,
		ExecutingState:    "NEX",
		Enabled:           enabled,
	}
	if condition.ExecutionTime != nil {
		task.ExecutionTime = *condition.ExecutionTime
	}
	if condition.ExpirationTime != nil {
		task.ExpirationTime = int64(*condition.ExpirationTime)
	}
	return task
}

func writeSceneAutomationPeriodicTask(tx *query.QueryTx, sceneAutomationID string, condition model.Condition) error {
	logrus.Info("create periodic task")
	if err := dal.CreatePeriodicTask(buildSceneAutomationPeriodicTask(sceneAutomationID, condition), tx); err != nil {
		return sceneAutomationDBError(err)
	}
	return nil
}

func buildSceneAutomationPeriodicTask(sceneAutomationID string, condition model.Condition) model.PeriodicTask {
	task := model.PeriodicTask{
		ID:                uuid.New(),
		SceneAutomationID: sceneAutomationID,
		Enabled:           "Y",
	}
	if condition.TaskType != nil {
		task.TaskType = *condition.TaskType
	}
	if condition.Params != nil {
		task.Param = *condition.Params
	}
	if condition.ExecutionTime != nil {
		task.ExecutionTime = *condition.ExecutionTime
	}
	if condition.ExpirationTime != nil {
		task.ExpirationTime = int64(*condition.ExpirationTime)
	}
	return task
}

func writeSceneAutomationActions(tx *query.QueryTx, sceneAutomationID string, actions []model.Action) error {
	for _, action := range actions {
		if err := dal.CreateActionInfo(buildSceneAutomationActionInfo(sceneAutomationID, action), tx); err != nil {
			return sceneAutomationDBError(err)
		}
	}
	return nil
}

func buildSceneAutomationActionInfo(sceneAutomationID string, action model.Action) model.ActionInfo {
	return model.ActionInfo{
		ID:                uuid.New(),
		SceneAutomationID: sceneAutomationID,
		ActionTarget:      &action.ActionTarget,
		ActionType:        action.ActionType,
		ActionParamType:   &action.ActionParamType,
		ActionParam:       &action.ActionParam,
		ActionValue:       &action.ActionValue,
	}
}
