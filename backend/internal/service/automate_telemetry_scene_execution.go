package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const (
	sceneLimiterKeyFormat  = "SceneAutomationId:%s"
	sceneDeviceLimiterKey  = "%s:%s"
	executionResultSuccess = "S"
	executionResultFailure = "F"
)

var (
	executeRunLimiterAllow = func(a *Automate, id string) bool {
		return a.LimiterAllow(id)
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		return a.CheckSceneAutomationHasClose(sceneAutomationId)
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		return a.AutomateConditionCheck(conditions, deviceId)
	}
	executeRunSceneAutomateExecute = func(a *Automate, sceneAutomationId string, deviceIds []string, actions []model.ActionInfo) error {
		return a.SceneAutomateExecute(sceneAutomationId, deviceIds, actions)
	}
	executeRunActionAfterDecoration = func(a *Automate, actions []model.ActionInfo, deviceId string, err error) {
		a.actionAfterDecorationRun(actions, deviceId, err)
	}
	sceneExecutionChecks = []sceneExecutionCheck{
		(*Automate).canAttemptScene,
		(*Automate).sceneIsActive,
		(*Automate).conditionsMatchScene,
		(*Automate).allowSceneExecutionRate,
	}
)

type sceneExecutionCandidate struct {
	sceneAutomationID string
	deviceID          string
	conditions        initialize.DTConditions
	actions           []model.ActionInfo
}

type sceneExecutionOutcome struct {
	details         string
	err             error
	executionResult string
	logDetail       string
}

type sceneExecutionCheck func(*Automate, sceneExecutionCandidate) bool

var (
	getActionInfoListBySceneID       = dal.GetActionInfoListBySceneId
	persistActiveSceneExecutionLogFn = dal.SceneLogInsert
)

func (a *Automate) resetExecutionState() {
	a.attemptedSceneIDs = make(map[string]bool)
	a.executedSceneIDs = make(map[string]bool)
}

func (a *Automate) hasAttemptedScene(sceneAutomationID string) bool {
	return a.attemptedSceneIDs != nil && a.attemptedSceneIDs[sceneAutomationID]
}

func (a *Automate) markSceneAttempted(sceneAutomationID string) {
	if a.attemptedSceneIDs == nil {
		a.attemptedSceneIDs = make(map[string]bool)
	}
	a.attemptedSceneIDs[sceneAutomationID] = true
}

func (a *Automate) markSceneExecuted(sceneAutomationID string) {
	if a.executedSceneIDs == nil {
		a.executedSceneIDs = make(map[string]bool)
	}
	a.executedSceneIDs[sceneAutomationID] = true
}

func (*Automate) LimiterAllow(id string) bool {
	return initialize.NewAutomateLimiter().GetLimiter(fmt.Sprintf(sceneLimiterKeyFormat, id)).Allow()
}

func (a *Automate) ExecuteRun(info initialize.AutomateExecteParams) error {
	logrus.Trace("automation execute run start")
	var executionErrors []error
	for _, v := range info.AutomateExecteSceeInfos {
		candidate := newSceneExecutionCandidate(info, v)
		if !a.prepareSceneExecution(candidate) {
			continue
		}
		if err := a.executeSceneAutomation(candidate); err != nil {
			executionErrors = append(executionErrors, err)
		}
	}
	logrus.Trace("automation execute run finish")
	return errors.Join(executionErrors...)
}

func newSceneExecutionCandidate(info initialize.AutomateExecteParams, scene initialize.AutomateExecteSceneInfo) sceneExecutionCandidate {
	return sceneExecutionCandidate{
		sceneAutomationID: scene.SceneAutomationId,
		deviceID:          info.DeviceId,
		conditions:        scene.GroupsCondition,
		actions:           scene.Actions,
	}
}

// prepareSceneExecution 在真正执行场景前串行通过一组轻量级守卫，
// 尽量把重复触发、停用场景和超频执行挡在动作执行之前。
func (a *Automate) prepareSceneExecution(candidate sceneExecutionCandidate) bool {
	for _, check := range sceneExecutionChecks {
		if !check(a, candidate) {
			return false
		}
	}
	a.markSceneAttempted(candidate.sceneAutomationID)
	return true
}

func (a *Automate) canAttemptScene(candidate sceneExecutionCandidate) bool {
	// 同一条上报命中重复缓存项时，只允许第一次进入后续检查和副作用流程。
	if a.hasAttemptedScene(candidate.sceneAutomationID) {
		logrus.Tracef("skip duplicate scene attempt for sceneAutomationID=%s", candidate.sceneAutomationID)
		return false
	}
	return true
}

func (a *Automate) sceneIsActive(candidate sceneExecutionCandidate) bool {
	logrus.Tracef("checking whether scene is closed: sceneAutomationID=%s", candidate.sceneAutomationID)
	return !executeRunCheckSceneAutomationHasClose(a, candidate.sceneAutomationID)
}

func (a *Automate) conditionsMatchScene(candidate sceneExecutionCandidate) bool {
	logrus.Tracef("checking scene conditions for candidate=%#v", candidate)
	return executeRunConditionCheck(a, candidate.conditions, candidate.deviceID)
}

func (a *Automate) allowSceneExecutionRate(candidate sceneExecutionCandidate) bool {
	return executeRunLimiterAllow(a, sceneLimiterDeviceID(candidate.sceneAutomationID, candidate.deviceID))
}

func sceneLimiterDeviceID(sceneAutomationID, deviceID string) string {
	return fmt.Sprintf(sceneDeviceLimiterKey, sceneAutomationID, deviceID)
}

func (a *Automate) executeSceneAutomation(candidate sceneExecutionCandidate) error {
	err := executeRunSceneAutomateExecute(a, candidate.sceneAutomationID, []string{candidate.deviceID}, candidate.actions)
	executeRunActionAfterDecoration(a, candidate.actions, candidate.deviceID, err)
	return a.finishSceneExecution(candidate.sceneAutomationID, err)
}

func (a *Automate) finishSceneExecution(sceneAutomationID string, err error) error {
	if err == nil {
		a.markSceneExecuted(sceneAutomationID)
		return nil
	}
	return fmt.Errorf("scene %s execution failed: %w", sceneAutomationID, err)
}

// CheckSceneAutomationHasClose
// @description Check whether a scene automation has been disabled or deleted.
func (*Automate) CheckSceneAutomationHasClose(sceneAutomationId string) bool {
	ok := dal.CheckSceneAutomationHasClose(sceneAutomationId)
	// Drop any stale cache entry once the scene is no longer active.
	if ok {
		_ = initialize.NewAutomateCache().DeleteCacheBySceneAutomationId(sceneAutomationId)
	}
	return ok
}

// SceneAutomateExecute
// @description Execute the actions bound to a scene automation.
// @params info initialize.AutomateExecteParams
// @return error
func (a *Automate) SceneAutomateExecute(sceneAutomationId string, deviceIds []string, actions []model.ActionInfo) error {
	tenantID := dal.GetSceneAutomationTenantID(context.Background(), sceneAutomationId)
	// 自动化执行日志要以场景自动化租户为准，避免后续设备归属变化导致日志串租户。
	outcome := a.executeSceneActionFlow(sceneAutomationId, deviceIds, actions, tenantID)
	_ = a.sceneExecuteLogSave(sceneAutomationId, tenantID, outcome)
	return outcome.err
}

// ActiveSceneExecute
// @description Execute an active scene directly and persist its execution log.
// @return error
func (a *Automate) ActiveSceneExecute(scene_id, tenantID string) error {
	actions, deviceIds, err := loadActiveSceneExecutionInputs(scene_id)
	if err != nil {
		return err
	}

	outcome := a.executeManualSceneActionFlow(scene_id, deviceIds, actions, tenantID)
	logErr := insertActiveSceneExecutionLog(scene_id, tenantID, outcome)
	return errors.Join(outcome.err, logErr)
}

func loadActiveSceneExecutionInputs(sceneID string) ([]model.ActionInfo, []string, error) {
	actions, err := getActionInfoListBySceneID([]string{sceneID})
	if err != nil {
		return nil, nil, err
	}
	deviceIds, err := loadSceneActionTargetDeviceIDs(actions)
	if err != nil {
		return nil, nil, err
	}
	return actions, deviceIds, nil
}

func insertActiveSceneExecutionLog(sceneID, tenantID string, outcome sceneExecutionOutcome) error {
	logrus.Debug(outcome.details)
	return persistActiveSceneExecutionLogFn(&model.SceneLog{
		ID:              uuid.New(),
		SceneID:         sceneID,
		ExecutedAt:      time.Now().UTC(),
		Detail:          outcome.logDetail,
		ExecutionResult: outcome.executionResult,
		TenantID:        tenantID,
	})
}

// @description Persist the execution log for an automated scene run.
// @return error
func (*Automate) sceneExecuteLogSave(scene_id string, tenantID string, outcome sceneExecutionOutcome) error {
	logrus.Debug(outcome.logDetail)
	return dal.SceneAutomationLogInsert(&model.SceneAutomationLog{
		SceneAutomationID: scene_id,
		ExecutedAt:        time.Now().UTC(),
		Detail:            outcome.logDetail,
		ExecutionResult:   outcome.executionResult,
		TenantID:          tenantID,
	})
}

func (a *Automate) executeSceneActionFlow(sceneID string, deviceIDs []string, actions []model.ActionInfo, tenantID string) sceneExecutionOutcome {
	details, err := a.AutomateActionExecute(sceneID, deviceIDs, actions, tenantID)
	return buildSceneExecutionOutcome(details, err)
}

func (a *Automate) executeManualSceneActionFlow(sceneID string, deviceIDs []string, actions []model.ActionInfo, tenantID string) sceneExecutionOutcome {
	details, err := a.ManualSceneActionExecute(sceneID, deviceIDs, actions, tenantID)
	return buildSceneExecutionOutcome(details, err)
}

func buildSceneExecutionOutcome(details string, err error) sceneExecutionOutcome {
	executionResult, logDetail := sceneExecutionLogFields(details, err)
	return sceneExecutionOutcome{
		details:         details,
		err:             err,
		executionResult: executionResult,
		logDetail:       logDetail,
	}
}

func sceneExecutionLogFields(details string, err error) (executionResult string, logDetail string) {
	if err == nil {
		return executionResultSuccess, details
	}
	return executionResultFailure, fmt.Sprintf("%s[%s]", details, err.Error())
}

func loadSceneActionTargetDeviceIDs(actions []model.ActionInfo) ([]string, error) {
	deviceConfigIDs := collectSceneActionTargetDeviceConfigIDs(actions)
	if len(deviceConfigIDs) == 0 {
		return nil, nil
	}
	return dal.GetDeviceIdsByDeviceConfigId(deviceConfigIDs)
}

func collectSceneActionTargetDeviceConfigIDs(actions []model.ActionInfo) []string {
	var deviceConfigIDs []string
	for _, action := range actions {
		if action.ActionType == model.AUTOMATE_ACTION_TYPE_MULTIPLE && action.ActionTarget != nil {
			deviceConfigIDs = append(deviceConfigIDs, *action.ActionTarget)
		}
	}
	return deviceConfigIDs
}
