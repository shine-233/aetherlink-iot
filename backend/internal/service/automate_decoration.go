// 文件用途：维护自动化执行结果的装饰和展示数据生成。
// 核心逻辑：读取场景、设备和模型信息，把执行动作结果补充成前端可读的名称与上下文。
// 关键注意事项：装饰逻辑不应改变执行结果本身，缺失设备或模型时要保持可降级输出。
// 重构建议：将装饰查询从执行路径剥离，补齐缺失引用、跨租户数据和查询失败边界测试。
package service

import (
	"aetherlink-iot/backend/initialize"
	model "aetherlink-iot/backend/internal/model"

	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ActionAfterAlarm(actions []model.ActionInfo, deviceID string, actionResultErr error) error {
	if len(actions) == 0 {
		return nil
	}

	sceneAutomationID := actions[0].SceneAutomationID
	alarmCache := initialize.NewAlarmCache()
	groupIDs, err := alarmCache.GetBySceneAutomationId(sceneAutomationID)
	if err != nil {
		return pkgerrors.Wrap(err, "get alarm cache by scene automation id failed")
	}
	if len(groupIDs) == 0 {
		return nil
	}
	logrus.WithFields(logrus.Fields{
		"scene_automation_id": sceneAutomationID,
		"group_count":         len(groupIDs),
	}).Debug("automation alarm action cache groups loaded")

	alarmConfigIDs := make([]string, 0, len(actions))
	for _, act := range actions {
		if act.ActionType == model.AUTOMATE_ACTION_TYPE_ALARM && act.ActionTarget != nil && *act.ActionTarget != "" {
			alarmConfigIDs = append(alarmConfigIDs, *act.ActionTarget)
		}
	}

	for _, groupID := range groupIDs {
		if len(alarmConfigIDs) == 0 {
			err = alarmCache.DeleteBygroupId(groupID)
		} else if actionResultErr == nil {
			err = alarmCache.SetAlarm(groupID, alarmConfigIDs, deviceID)
		}
		if err != nil {
			return pkgerrors.Wrap(err, "update alarm cache after automation action failed")
		}
	}

	return nil
}

func ConditionAfterAlarm(ok bool, conditions initialize.DTConditions, deviceID string, contents []string) error {
	var (
		deviceIDs         []string
		groupID           string
		sceneAutomationID string
		alarmCache        = initialize.NewAlarmCache()
	)

	for _, cond := range conditions {
		groupID = cond.GroupID
		sceneAutomationID = cond.SceneAutomationID
		switch cond.TriggerConditionType {
		case model.DEVICE_TRIGGER_CONDITION_TYPE_ONE:
			if cond.TriggerSource == nil || *cond.TriggerSource == "" {
				continue
			}
			deviceIDs = append(deviceIDs, *cond.TriggerSource)
		case model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE:
			if deviceID != "" {
				deviceIDs = append(deviceIDs, deviceID)
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"group_id":            groupID,
		"scene_automation_id": sceneAutomationID,
		"device_count":        len(deviceIDs),
		"condition_ok":        ok,
		"content_count":       len(contents),
	}).Debug("automation alarm condition cache update")
	if len(deviceIDs) == 0 {
		return nil
	}

	if ok {
		err := alarmCache.SetDevice(groupID, sceneAutomationID, deviceIDs, contents)
		if err != nil {
			return pkgerrors.Wrap(err, "set alarm cache after automation condition failed")
		}
		groupIDs, _ := alarmCache.GetBySceneAutomationId(sceneAutomationID)
		logrus.WithFields(logrus.Fields{
			"scene_automation_id": sceneAutomationID,
			"group_count":         len(groupIDs),
		}).Debug("automation alarm scene cache groups refreshed")
	} else {
		err := AlarmRecovery(groupID, contents)
		if err != nil {
			return pkgerrors.WithMessage(err, "recover automation alarm failed")
		}
		err = alarmCache.DeleteBygroupId(groupID)
		if err != nil {
			return pkgerrors.Wrap(err, "delete alarm cache after automation condition failed")
		}
		logrus.WithField("group_id", groupID).Debug("automation alarm cache group deleted")
	}
	return nil
}

func AlarmExecute(alarmConfigID, sceneAutomationID string) (bool, string, string) {
	var (
		alarmName string
		resultOK  bool
		reason    string
	)

	alarmCache := initialize.NewAlarmCache()
	groupIDs, err := alarmCache.GetBySceneAutomationId(sceneAutomationID)
	logrus.WithFields(logrus.Fields{
		"scene_automation_id": sceneAutomationID,
		"group_count":         len(groupIDs),
	}).Debug("automation alarm execute cache lookup")
	if err != nil || len(groupIDs) == 0 {
		return resultOK, alarmName, "alarm cache does not exist"
	}

	for _, groupID := range groupIDs {
		cache, err := alarmCache.GetByGroupId(groupID)
		if err != nil {
			return resultOK, alarmName, "alarm cache does not exist"
		}
		logrus.WithFields(logrus.Fields{
			"group_id":              groupID,
			"alarm_config_count":    len(cache.AlarmConfigIdList),
			"alarm_device_id_count": len(cache.AlaramDeviceIdList),
		}).Debug("automation alarm execute cache group loaded")

		isOK, err := alarmCache.GetAlarmDeviceExists(cache.AlaramDeviceIdList, alarmConfigID, groupID)
		if err != nil {
			return resultOK, alarmName, "query alarm cache failed"
		}
		if isOK {
			reason = "alarm already exists"
			logrus.WithFields(logrus.Fields{
				"group_id":        groupID,
				"alarm_config_id": alarmConfigID,
			}).Debug("automation alarm already cached; skipping execute")
			continue
		}

		// 告警需要“条件持续 N 秒”才触发时（trigger_duration > 0），条件成立起点由缓存
		// 分组记录（ConditionTrueSince），必须自该起点起连续成立足够秒数才触发。未满足
		// 持续时长的观测直接跳过、不算失败，保持缓存不动，等下一次观测再判断；条件中断时
		// ConditionAfterAlarm 会删除分组，起点随之重置。
		if !alarmTriggerDurationHeld(alarmConfigID, cache.ConditionTrueSince) {
			reason = alarmTriggerDurationNotHeldReason
			logrus.WithFields(logrus.Fields{
				"group_id":             groupID,
				"alarm_config_id":      alarmConfigID,
				"condition_true_since": cache.ConditionTrueSince,
			}).Debug("automation alarm trigger duration not satisfied; skipping execute")
			continue
		}

		content := "scene automation triggered alarm"
		for _, strval := range cache.Contents {
			content += ";" + strval
		}
		resultOK, alarmName, reason = GroupApp.AlarmExecute(alarmConfigID, content, sceneAutomationID, groupID, cache.AlaramDeviceIdList)
	}
	return resultOK, alarmName, reason
}

func AlarmRecovery(groupID string, contents []string) error {
	alarmCache := initialize.NewAlarmCache()
	cache, err := alarmCache.GetByGroupId(groupID)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"group_id":              groupID,
		"alarm_config_count":    len(cache.AlarmConfigIdList),
		"alarm_device_id_count": len(cache.AlaramDeviceIdList),
		"content_count":         len(contents),
	}).Debug("automation alarm recovery cache group loaded")

	for _, alarmConfigID := range cache.AlarmConfigIdList {
		content := "scene automation recovered alarm"
		for _, strval := range contents {
			content += ";" + strval
		}
		if _, err := GroupApp.AlarmRecovery(alarmConfigID, content, cache.SceneAutomationId, groupID, cache.AlaramDeviceIdList); err != nil {
			return pkgerrors.Wrapf(err, "persist alarm recovery for config %s", alarmConfigID)
		}
	}

	return nil
}
