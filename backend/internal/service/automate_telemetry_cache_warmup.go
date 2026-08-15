package service

import (
	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (a *Automate) telExecute(deviceId, deviceConfigId string, fromExt AutomateFromExt) error {
	info, resultInt, err := initialize.NewAutomateCache().GetCacheByDeviceId(deviceId, deviceConfigId)
	logrus.Tracef("automation cache lookup result=%d deviceId=%s deviceConfigId=%s", resultInt, deviceId, deviceConfigId)
	if err != nil {
		return pkgerrors.Wrap(err, "get automate cache by device id")
	}
	if resultInt == initialize.AUTOMATE_CACHE_RESULT_NOT_TASK {
		logrus.Trace("automation cache indicates no task")
		return nil
	}
	if resultInt == initialize.AUTOMATE_CACHE_RESULT_NOT_FOUND {
		logrus.Trace("automation cache miss, querying source")
		info, resultInt, err = a.QueryAutomateInfoAndSetCache(deviceId, deviceConfigId)
		if err != nil {
			return pkgerrors.Wrap(err, "query automate info and set cache")
		}
		if resultInt == initialize.AUTOMATE_CACHE_RESULT_NOT_TASK {
			return nil
		}
	}
	logrus.Trace("automation execution info loaded")
	logrus.Tracef("automation scene count before filter: %d", len(info.AutomateExecteSceeInfos))
	logrus.Tracef("automation execution info before filter: %#v", info)
	info = a.AutomateFilter(info, fromExt)
	logrus.Tracef("automation scene count after filter: %d", len(info.AutomateExecteSceeInfos))
	logrus.Tracef("automation execution info after filter: %#v", info)
	return a.ExecuteRun(info)
}

// QueryAutomateInfoAndSetCache
// @description Load automation conditions and actions, then warm the device cache.
// @params deviceId string
// @return initialize.AutomateExecteParams, int, error
func (*Automate) QueryAutomateInfoAndSetCache(deviceId, deviceConfigId string) (initialize.AutomateExecteParams, int, error) {
	automateExecuteParams := initialize.AutomateExecteParams{
		DeviceId:       deviceId,
		DeviceConfigId: deviceConfigId,
	}
	groups, err := loadAutomateWarmupGroups(deviceId, deviceConfigId)
	if err != nil {
		return automateExecuteParams, 0, err
	}
	if len(groups) == 0 {
		err := initialize.NewAutomateCache().SetCacheByDeviceIdWithNoTask(deviceId, deviceConfigId)
		if err != nil {
			return automateExecuteParams, 0, pkgerrors.Wrap(err, "set automate cache no-task state")
		}
		return automateExecuteParams, initialize.AUTOMATE_CACHE_RESULT_NOT_TASK, nil
	}

	sceneAutomateIds, groupIds := collectWarmupSceneAndGroupIDs(groups)
	groups, err = dal.GetDeviceTriggerConditionByGroupIds(groupIds)
	if err != nil {
		return automateExecuteParams, 0, pkgerrors.Wrap(err, "get device trigger conditions by group ids")
	}

	actionInfos, err := dal.GetActionInfoListBySceneAutomationId(sceneAutomateIds)
	if err != nil {
		return automateExecuteParams, 0, pkgerrors.Wrap(err, "get action info list by scene automation id")
	}
	logrus.Debugf("warming automate cache for deviceConfigId=%s groups=%v actionInfos=%v", deviceConfigId, groups, actionInfos)
	err = initialize.NewAutomateCache().SetCacheByDeviceId(deviceId, deviceConfigId, groups, actionInfos)
	if err != nil {
		return automateExecuteParams, 0, pkgerrors.Wrap(err, "set automate cache by device id")
	}

	return initialize.NewAutomateCache().GetCacheByDeviceId(deviceId, deviceConfigId)
}

func loadAutomateWarmupGroups(deviceId, deviceConfigId string) ([]model.DeviceTriggerCondition, error) {
	var (
		groups []model.DeviceTriggerCondition
		err    error
	)
	if deviceConfigId != "" {
		groups, err = dal.GetDeviceTriggerConditionByDeviceId(deviceConfigId, model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE)
	} else {
		groups, err = dal.GetDeviceTriggerConditionByDeviceId(deviceId, model.DEVICE_TRIGGER_CONDITION_TYPE_ONE)
	}
	logrus.Debugf("loaded device trigger groups for deviceConfigId=%s groups=%v", deviceConfigId, groups)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "get device trigger condition by device id")
	}
	return groups, nil
}

func collectWarmupSceneAndGroupIDs(groups []model.DeviceTriggerCondition) ([]string, []string) {
	sceneAutomateGroups := make(map[string]bool)
	var (
		sceneAutomateIds []string
		groupIds         []string
	)

	for _, groupInfo := range groups {
		if !sceneAutomateGroups[groupInfo.SceneAutomationID] {
			sceneAutomateIds = append(sceneAutomateIds, groupInfo.SceneAutomationID)
			sceneAutomateGroups[groupInfo.SceneAutomationID] = true
		}
		groupIds = append(groupIds, groupInfo.GroupID)
	}
	return sceneAutomateIds, groupIds
}
