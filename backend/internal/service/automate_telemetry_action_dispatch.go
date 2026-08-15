// 文件用途：集中维护自动化遥测服务的动作分发与执行结果汇总。
// 核心逻辑：根据 action type 选择对应执行 adapter，串行执行动作列表，并统一拼装结果文案与失败返回。
// 使用注意：这里保留“不支持的动作类型立即终止”的旧行为，避免后续成功动作掩盖配置本身的问题。
// 重构建议：如果后续新增动作类型，优先只扩展本文件的分发表和小 helper，不要重新塞回 automate_telemetry.go 主文件。
package service

import (
	"errors"
	"fmt"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

func newAutomateTelemetryActionService(action model.ActionInfo, deviceIds []string, tenantID string) AutomateTelemetryAction {
	switch action.ActionType {
	case model.AUTOMATE_ACTION_TYPE_ONE:
		return &AutomateTelemetryActionOne{TenantID: tenantID}
	case model.AUTOMATE_ACTION_TYPE_ALARM:
		return &AutomateTelemetryActionAlarm{DeviceIds: deviceIds}
	case model.AUTOMATE_ACTION_TYPE_MULTIPLE:
		return &AutomateTelemetryActionMultiple{DeviceIds: deviceIds, TenantID: tenantID}
	case model.AUTOMATE_ACTION_TYPE_SCENE:
		return &AutomateTelemetryActionScene{TenantID: tenantID}
	case model.AUTOMATE_ACTION_TYPE_SERVICE:
		return &AutomateTelemetryActionService{}
	default:
		return nil
	}
}

type sceneActionRunner func(model.ActionInfo) (string, error)

// AutomateActionExecute 执行自动化场景绑定的全部动作，并汇总动作结果文案。
func (*Automate) AutomateActionExecute(_ string, deviceIds []string, actions []model.ActionInfo, tenantID string) (string, error) {
	return executeSceneActions(actions, func(action model.ActionInfo) (string, error) {
		return executeAutomateTelemetryAction(action, deviceIds, tenantID)
	})
}

func (*Automate) ManualSceneActionExecute(sceneID string, deviceIDs []string, actions []model.ActionInfo, tenantID string) (string, error) {
	return executeSceneActions(actions, func(action model.ActionInfo) (string, error) {
		if action.ActionType == model.AUTOMATE_ACTION_TYPE_ALARM {
			return executeManualSceneAlarmAction(sceneID, deviceIDs, action)
		}
		return executeAutomateTelemetryAction(action, deviceIDs, tenantID)
	})
}

func executeSceneActions(actions []model.ActionInfo, run sceneActionRunner) (string, error) {
	logrus.Debug("automation action execution started")
	var (
		result    string
		resultErr error
	)
	if len(actions) == 0 {
		return "automate action list is empty", errors.New("automate action list is empty")
	}
	for _, action := range actions {
		logrus.Debug("actionType:", action.ActionType)
		actionMessage, err := run(action)
		// 不支持的动作类型直接终止，避免后续成功动作掩盖配置本身的问题。
		if isUnsupportedAutomateAction(actionMessage, err) {
			return actionMessage, err
		}
		if err != nil && resultErr == nil {
			resultErr = err
		}
		result = appendAutomateActionResult(result, actionMessage, err)
	}
	logrus.Debug("result:", result)
	return result, resultErr
}

func executeAutomateTelemetryAction(action model.ActionInfo, deviceIds []string, tenantID string) (string, error) {
	actionService := newAutomateTelemetryActionService(action, deviceIds, tenantID)
	if actionService == nil {
		logrus.Error(unsupportedActionMessage)
		return unsupportedActionMessage, errors.New(unsupportedActionMessage)
	}
	return actionService.AutomateActionRun(action)
}

func isUnsupportedAutomateAction(actionMessage string, err error) bool {
	return err != nil && actionMessage == unsupportedActionMessage
}

func appendAutomateActionResult(result, actionMessage string, err error) string {
	if err != nil {
		return result + fmt.Sprintf("%s execute failed;", actionMessage)
	}
	return result + fmt.Sprintf("%s execute success;", actionMessage)
}
