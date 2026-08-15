package service

import (
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

const timeOfDayLayout = "15:04:05-07:00"

var automateNow = time.Now

type conditionGroupEvaluation struct {
	ok       bool
	contents []string
}

func (a *Automate) AutomateFilter(info initialize.AutomateExecteParams, fromExt AutomateFromExt) initialize.AutomateExecteParams {
	var sceneInfo []initialize.AutomateExecteSceneInfo
	for _, scene := range info.AutomateExecteSceeInfos {
		var isExists bool
		for _, cond := range scene.GroupsCondition {
			if a.sceneConditionMatchesTrigger(cond, fromExt) {
				isExists = true
			}
		}
		if isExists {
			sceneInfo = append(sceneInfo, scene)
		}
	}
	info.AutomateExecteSceeInfos = sceneInfo
	return info
}

func (a *Automate) sceneConditionMatchesTrigger(cond model.DeviceTriggerCondition, fromExt AutomateFromExt) bool {
	if cond.TriggerParamType == nil || cond.TriggerParam == nil {
		return false
	}

	condTriggerParamType := strings.ToUpper(*cond.TriggerParamType)
	logrus.Tracef("TriggerParamType: %s", condTriggerParamType)
	logrus.Tracef("TriggerParam: %s", *cond.TriggerParam)
	logrus.Tracef("fromExt.TriggerParamType: %s", fromExt.TriggerParamType)

	return a.triggerParamTypeMatches(condTriggerParamType, fromExt, *cond.TriggerParam)
}

func (a *Automate) triggerParamTypeMatches(condTriggerParamType string, fromExt AutomateFromExt, condTriggerParam string) bool {
	switch fromExt.TriggerParamType {
	case model.TRIGGER_PARAM_TYPE_TEL:
		return triggerParamTypeIn(condTriggerParamType, model.TRIGGER_PARAM_TYPE_TEL, model.TRIGGER_PARAM_TYPE_TELEMETRY) &&
			a.containString(fromExt.TriggerParam, condTriggerParam)
	case model.TRIGGER_PARAM_TYPE_STATUS:
		return condTriggerParamType == model.TRIGGER_PARAM_TYPE_STATUS
	case model.TRIGGER_PARAM_TYPE_EVT:
		return triggerParamTypeIn(condTriggerParamType, model.TRIGGER_PARAM_TYPE_EVT, model.TRIGGER_PARAM_TYPE_EVENT) &&
			a.containString(fromExt.TriggerParam, condTriggerParam)
	case model.TRIGGER_PARAM_TYPE_ATTR:
		return triggerParamTypeIn(condTriggerParamType, model.TRIGGER_PARAM_TYPE_ATTR, model.TRIGGER_PARAM_TYPE_ATTRIBUTES) &&
			a.containString(fromExt.TriggerParam, condTriggerParam)
	default:
		return false
	}
}

func triggerParamTypeIn(actual string, expected ...string) bool {
	for _, value := range expected {
		if actual == value {
			return true
		}
	}
	return false
}

func (*Automate) containString(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func groupConditionsByGroupID(conditions initialize.DTConditions) map[string]initialize.DTConditions {
	conditionsByGroupID := make(map[string]initialize.DTConditions)
	for _, condition := range conditions {
		conditionsByGroupID[condition.GroupID] = append(conditionsByGroupID[condition.GroupID], condition)
	}
	return conditionsByGroupID
}

func (a *Automate) anyConditionGroupMatches(groupedConditions map[string]initialize.DTConditions, deviceId string) bool {
	for _, groupConditions := range groupedConditions {
		if a.evaluateConditionGroup(groupConditions, deviceId) {
			return true
		}
	}
	return false
}

func (a *Automate) evaluateConditionGroup(conditions initialize.DTConditions, deviceId string) bool {
	evaluation := a.evaluateConditionGroupResult(conditions, deviceId)
	a.conditionAfterDecorationRun(evaluation, conditions, deviceId)
	return evaluation.ok
}

func (a *Automate) evaluateConditionGroupResult(conditions initialize.DTConditions, deviceId string) conditionGroupEvaluation {
	evaluation := conditionGroupEvaluation{ok: true}
	for _, val := range conditions {
		ok, content := a.AutomateConditionCheckWithGroupOne(val, deviceId)
		evaluation.contents = append(evaluation.contents, content)
		if !ok {
			evaluation.ok = false
			break
		}
	}

	return evaluation
}

func (a *Automate) AutomateConditionCheckWithGroupOne(cond model.DeviceTriggerCondition, deviceId string) (bool, string) {
	logrus.Trace("automation condition type:", cond.TriggerConditionType)
	return a.evaluateConditionByType(cond, deviceId)
}

func (a *Automate) evaluateConditionByType(cond model.DeviceTriggerCondition, deviceId string) (bool, string) {
	switch cond.TriggerConditionType {
	case model.DEVICE_TRIGGER_CONDITION_TYPE_TIME:
		return a.automateConditionCheckWithTime(cond), ""
	case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
		return a.automateConditionCheckWithDevice(cond, deviceId)
	default:
		return true, ""
	}
}
