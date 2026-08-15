// automate_telemetry_device_condition.go 负责设备类触发条件的目标解析、
// 实时值读取、结果文案拼装和特殊事件参数比较，是自动化条件求值的独立子模块。
package service

import (
	"fmt"
	"strings"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

type deviceConditionEvaluation struct {
	actualValue     interface{}
	triggerValue    string
	triggerOperator string
	dataValue       string
	result          string
	handled         bool
	ok              bool
}

type deviceConditionTarget struct {
	deviceID   string
	deviceName string
}

type deviceConditionRequest struct {
	paramType string
	operator  string
}

type DataIdentifierName func(device_template_id, identifier string) string

func (a *Automate) getActualValue(deviceId string, key string, triggerParamType string) (interface{}, error) {
	if value, ok := a.getActualValueFromTriggerOverrides(key); ok {
		return value, nil
	}
	return getActualValueFromDeviceStore(deviceId, key, triggerParamType)
}

func (a *Automate) getActualValueFromTriggerOverrides(key string) (interface{}, bool) {
	for k, v := range a.formExt.TriggerValues {
		if key == k {
			return v, true
		}
	}
	return nil, false
}

func getActualValueFromDeviceStore(deviceId string, key string, triggerParamType string) (interface{}, error) {
	switch triggerParamType {
	case model.TRIGGER_PARAM_TYPE_TEL:
		return dal.GetCurrentTelemetryDataOneKeys(deviceId, key)
	case model.TRIGGER_PARAM_TYPE_ATTR:
		return dal.GetAttributeOneKeys(deviceId, key)
	case model.TRIGGER_PARAM_TYPE_EVT:
		return dal.GetDeviceEventOneKeys(deviceId, key)
	case model.TRIGGER_PARAM_TYPE_STATUS:
		return dal.GetDeviceCurrentStatus(deviceId)
	}

	return nil, nil
}

func (a *Automate) automateConditionCheckWithDevice(cond model.DeviceTriggerCondition, deviceId string) (bool, string) {
	logrus.Trace("device automation condition check started")
	if !hasDeviceConditionInputs(cond) {
		return false, ""
	}

	target, ok := a.resolveDeviceConditionTarget(cond, deviceId)
	if !ok {
		return false, ""
	}

	request := newDeviceConditionRequest(cond)
	logrus.Trace("device automation condition type:", request.paramType)

	evaluation, ok := a.buildDeviceConditionEvaluation(cond, target, request)
	if !ok {
		return false, ""
	}
	return a.formatDeviceConditionResult(evaluation)
}

func (a *Automate) formatDeviceConditionResult(evaluation deviceConditionEvaluation) (bool, string) {
	if evaluation.handled {
		return evaluation.ok, evaluation.result
	}

	logrus.Trace("automateConditionCheckByOperator params:", evaluation.triggerOperator, evaluation.triggerValue, evaluation.actualValue)
	ok := a.automateConditionCheckByOperator(evaluation.triggerOperator, evaluation.triggerValue, evaluation.actualValue)
	logrus.Tracef("condition compare result:%t", ok)
	return ok, evaluation.result
}

func hasDeviceConditionInputs(cond model.DeviceTriggerCondition) bool {
	return cond.TriggerSource != nil && cond.TriggerParamType != nil && cond.TriggerParam != nil
}

func (a *Automate) resolveDeviceConditionTarget(cond model.DeviceTriggerCondition, deviceId string) (deviceConditionTarget, bool) {
	if cond.TriggerConditionType == model.DEVICE_TRIGGER_CONDITION_TYPE_ONE {
		deviceId = *cond.TriggerSource
		device, err := initialize.GetDeviceCacheById(deviceId)
		if err != nil {
			logrus.WithError(err).Error("get device cache failed")
			return deviceConditionTarget{}, false
		}
		if device == nil || device.Name == nil {
			return deviceConditionTarget{}, false
		}
		return deviceConditionTarget{deviceID: deviceId, deviceName: *device.Name}, true
	}

	if a.device == nil || a.device.Name == nil {
		return deviceConditionTarget{}, false
	}
	return deviceConditionTarget{deviceID: deviceId, deviceName: *a.device.Name}, true
}

func (a *Automate) resolveConditionDevice(cond model.DeviceTriggerCondition, deviceId string) (string, string, bool) {
	target, ok := a.resolveDeviceConditionTarget(cond, deviceId)
	return target.deviceID, target.deviceName, ok
}

func newDeviceConditionRequest(cond model.DeviceTriggerCondition) deviceConditionRequest {
	return deviceConditionRequest{
		paramType: strings.ToUpper(*cond.TriggerParamType),
		operator:  conditionTriggerOperator(cond),
	}
}

func conditionTriggerOperator(cond model.DeviceTriggerCondition) string {
	if cond.TriggerOperator == nil {
		return "="
	}
	return *cond.TriggerOperator
}

func (a *Automate) buildDeviceConditionEvaluation(cond model.DeviceTriggerCondition, target deviceConditionTarget, request deviceConditionRequest) (deviceConditionEvaluation, bool) {
	switch request.paramType {
	case model.TRIGGER_PARAM_TYPE_TEL, model.TRIGGER_PARAM_TYPE_TELEMETRY:
		return a.buildDataConditionEvaluation(
			target, "telemetry", model.TRIGGER_PARAM_TYPE_TEL,
			*cond.TriggerParam, cond.TriggerValue, request.operator, dal.GetIdentifierNameTelemetry(),
		), true
	case model.TRIGGER_PARAM_TYPE_ATTR, model.TRIGGER_PARAM_TYPE_ATTRIBUTES:
		return a.buildDataConditionEvaluation(
			target, "attribute", model.TRIGGER_PARAM_TYPE_ATTR,
			*cond.TriggerParam, cond.TriggerValue, request.operator, dal.GetIdentifierNameAttribute(),
		), true
	case model.TRIGGER_PARAM_TYPE_EVT, model.TRIGGER_PARAM_TYPE_EVENT:
		return a.buildEventConditionEvaluation(target, cond), true
	case model.TRIGGER_PARAM_TYPE_STATUS:
		return a.buildStatusConditionEvaluation(target, *cond.TriggerParam), true
	default:
		return deviceConditionEvaluation{}, false
	}
}

func (a *Automate) buildDataConditionEvaluation(
	target deviceConditionTarget,
	trigger, actualParamType, triggerKey, triggerValue, triggerOperator string,
	dataName DataIdentifierName,
) deviceConditionEvaluation {
	actualValue, _ := a.getActualValue(target.deviceID, triggerKey, actualParamType)
	dataValue := a.getTriggerParamsValue(triggerKey, dataName)
	return deviceConditionEvaluation{
		actualValue:     actualValue,
		triggerValue:    triggerValue,
		triggerOperator: triggerOperator,
		dataValue:       dataValue,
		result:          formatDataConditionResult(target.deviceName, trigger, dataValue, actualValue, triggerOperator, triggerValue),
	}
}

func formatDataConditionResult(deviceName, trigger, dataValue string, actualValue interface{}, triggerOperator, triggerValue string) string {
	return fmt.Sprintf("device(%s)%s [%s]: %v %s %v", deviceName, trigger, dataValue, actualValue, triggerOperator, triggerValue)
}

func (a *Automate) buildEventConditionEvaluation(target deviceConditionTarget, cond model.DeviceTriggerCondition) deviceConditionEvaluation {
	evaluation := a.buildDataConditionEvaluation(
		target, "event", model.TRIGGER_PARAM_TYPE_EVT,
		*cond.TriggerParam, cond.TriggerValue, "=", dal.GetIdentifierNameEvent(),
	)
	return a.applyEventParamConditionEvaluation(target.deviceName, evaluation)
}

func (a *Automate) applyEventParamConditionEvaluation(deviceName string, evaluation deviceConditionEvaluation) deviceConditionEvaluation {
	ok, detail, handled := a.automateEventParamConditionCheck(evaluation.triggerValue, evaluation.actualValue)
	if !handled {
		return evaluation
	}
	if detail != "" {
		evaluation.result = formatEventConditionResult(deviceName, evaluation.dataValue, detail)
	}
	logrus.Tracef("event field condition result=%t detail=%s", ok, detail)
	evaluation.handled = true
	evaluation.ok = ok
	return evaluation
}

func formatEventConditionResult(deviceName, dataValue, detail string) string {
	return fmt.Sprintf("device(%s)%s [%s]: %s", deviceName, "event", dataValue, detail)
}

func (a *Automate) buildStatusConditionEvaluation(target deviceConditionTarget, triggerValue string) deviceConditionEvaluation {
	trigger := "offline"
	actualValue, _ := a.getActualValue(target.deviceID, "login", model.TRIGGER_PARAM_TYPE_STATUS)
	actualStatus, ok := actualValue.(string)
	if !ok {
		return deviceConditionEvaluation{
			handled: true,
			ok:      false,
			result:  formatStatusTypeErrorResult(target.deviceName),
		}
	}
	if strings.ToUpper(actualStatus) == "ON-LINE" {
		trigger = "online"
	}
	result := formatStatusConditionResult(target.deviceName, trigger)
	if strings.ToUpper(triggerValue) == "ALL" {
		return deviceConditionEvaluation{
			handled: true,
			ok:      true,
			result:  result,
		}
	}
	return deviceConditionEvaluation{
		actualValue:     actualValue,
		triggerValue:    triggerValue,
		triggerOperator: "=",
		result:          result,
	}
}

func formatStatusConditionResult(deviceName, trigger string) string {
	return fmt.Sprintf("device(%s) is %s", deviceName, trigger)
}

func formatStatusTypeErrorResult(deviceName string) string {
	return fmt.Sprintf("device(%s) status value is not a string", deviceName)
}

func (*Automate) getTriggerParamsValue(triggerKey string, fc DataIdentifierName) string {
	tempId, _ := dal.GetDeviceTemplateIdByDeviceId(triggerKey)
	if tempId == "" {
		return triggerKey
	}

	return fc(tempId, triggerKey)
}
