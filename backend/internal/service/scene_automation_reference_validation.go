package service

import (
	"encoding/json"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// validateSceneAutomationReferences is shared by save/update and dry-run so
// customer previews fail on the same inaccessible references as persisted rules.
func validateSceneAutomationReferences(triggerConditionGroups [][]model.Condition, actions []model.Action, claims *utils.UserClaims, tenantID string) error {
	if err := validateSceneAutomationTriggerReferences(triggerConditionGroups, claims, tenantID); err != nil {
		return err
	}
	return validateSceneAutomationActionReferences(actions, claims, tenantID)
}

func validateSceneAutomationTriggerReferences(triggerConditionGroups [][]model.Condition, claims *utils.UserClaims, tenantID string) error {
	for _, group := range triggerConditionGroups {
		for _, condition := range group {
			if err := validateSceneAutomationTriggerReference(condition, claims, tenantID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSceneAutomationTriggerReference(condition model.Condition, claims *utils.UserClaims, tenantID string) error {
	if err := validateSceneAutomationEventParamTriggerValue(condition); err != nil {
		return err
	}

	if condition.TriggerSource == nil || *condition.TriggerSource == "" {
		return nil
	}

	switch condition.TriggerConditionsType {
	case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE:
		return validateSceneAutomationTriggerDeviceReference(*condition.TriggerSource, claims, tenantID)
	case model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
		return validateSceneAutomationTriggerDeviceConfigReference(*condition.TriggerSource, claims, tenantID)
	default:
		return nil
	}
}

func validateSceneAutomationEventParamTriggerValue(condition model.Condition) error {
	if condition.TriggerParamType == nil || !isEventTriggerParamType(*condition.TriggerParamType) {
		return nil
	}
	if condition.TriggerValue == nil || strings.TrimSpace(*condition.TriggerValue) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "event trigger_value is required")
	}

	var config eventParamMatchConfig
	if err := json.Unmarshal([]byte(*condition.TriggerValue), &config); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "event trigger_value is not a valid field match JSON")
	}
	if detail := validateEventParamMatchConfigShape(config); detail != "" {
		return errcode.NewWithMessage(errcode.CodeParamError, detail)
	}
	return nil
}

func isEventTriggerParamType(triggerParamType string) bool {
	switch strings.ToUpper(strings.TrimSpace(triggerParamType)) {
	case model.TRIGGER_PARAM_TYPE_EVT, model.TRIGGER_PARAM_TYPE_EVENT, "event":
		return true
	default:
		return false
	}
}

func validateSceneAutomationTriggerDeviceReference(deviceID string, claims *utils.UserClaims, tenantID string) error {
	device, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return err
	}
	if device.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "trigger device tenant mismatch")
	}
	return nil
}

func validateSceneAutomationTriggerDeviceConfigReference(deviceConfigID string, claims *utils.UserClaims, tenantID string) error {
	deviceConfig, err := ensureDeviceConfigReadAccess(deviceConfigID, claims)
	if err != nil {
		return err
	}
	if deviceConfig.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "trigger device config tenant mismatch")
	}
	return nil
}

func validateSceneAutomationActionReferences(actions []model.Action, claims *utils.UserClaims, tenantID string) error {
	for _, action := range actions {
		if err := validateSceneAutomationActionReference(action, claims, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func validateSceneAutomationActionReference(action model.Action, claims *utils.UserClaims, tenantID string) error {
	if action.ActionTarget == "" {
		return nil
	}

	switch action.ActionType {
	case model.AUTOMATE_ACTION_TYPE_ONE:
		return validateSceneAutomationActionDeviceReference(action.ActionTarget, claims, tenantID)
	case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
		return validateSceneAutomationActionDeviceConfigReference(action.ActionTarget, claims, tenantID)
	case model.AUTOMATE_ACTION_TYPE_SCENE:
		return validateSceneAutomationActionSceneReference(action.ActionTarget, claims, tenantID)
	case model.AUTOMATE_ACTION_TYPE_ALARM:
		return validateSceneAutomationActionAlarmReference(action.ActionTarget, claims, tenantID)
	case model.AUTOMATE_ACTION_TYPE_SERVICE:
		return errcode.NewWithMessage(errcode.CodeParamError, "service action is not supported")
	default:
		return nil
	}
}

func validateSceneAutomationActionDeviceReference(deviceID string, claims *utils.UserClaims, tenantID string) error {
	device, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
	if err != nil {
		return err
	}
	if device.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "action device tenant mismatch")
	}
	return nil
}

func validateSceneAutomationActionDeviceConfigReference(deviceConfigID string, claims *utils.UserClaims, tenantID string) error {
	deviceConfig, err := ensureDeviceConfigWriteAccess(deviceConfigID, claims)
	if err != nil {
		return err
	}
	if deviceConfig.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "action device config tenant mismatch")
	}
	return nil
}

func validateSceneAutomationActionSceneReference(sceneAutomationID string, claims *utils.UserClaims, tenantID string) error {
	sceneAutomation, err := ensureSceneAutomationWriteAccess(sceneAutomationID, claims)
	if err != nil {
		return err
	}
	if sceneAutomation.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "action scene tenant mismatch")
	}
	return nil
}

func validateSceneAutomationActionAlarmReference(alarmConfigID string, claims *utils.UserClaims, tenantID string) error {
	alarmConfig, err := ensureAlarmConfigWriteAccess(alarmConfigID, claims)
	if err != nil {
		return err
	}
	if alarmConfig.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "action alarm config tenant mismatch")
	}
	return nil
}
