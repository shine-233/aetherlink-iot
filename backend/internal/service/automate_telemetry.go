// automate_telemetry.go 负责自动化触发入口、缓存回源、时间条件判断、
// 场景执行编排和执行日志主链路，是遥测自动化的核心调度文件。
package service

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/common"

	"github.com/sirupsen/logrus"
)

const unsupportedActionMessage = "unsupported automate action"

type Automate struct {
	device  *model.Device
	formExt AutomateFromExt
	mu      sync.Mutex
	// Track per-uplink scene state so duplicate cache entries do not re-run side effects.
	attemptedSceneIDs map[string]bool
	executedSceneIDs  map[string]bool
}

var conditionAfterDecoration = []ConditionAfterFunc{
	ConditionAfterAlarm,
}

var actionAfterDecoration = []ActionAfterFunc{
	ActionAfterAlarm,
}

type (
	ConditionAfterFunc = func(ok bool, conditions initialize.DTConditions, deviceId string, contents []string) error
	ActionAfterFunc    = func(actions []model.ActionInfo, deviceId string, err error) error
)

type AutomateFromExt struct {
	TriggerParamType string
	TriggerParam     []string
	TriggerValues    map[string]interface{}
}

// conditionAfterDecorationRun triggers best-effort side effects such as alarm
// linkage after a condition group finishes evaluating.
func (a *Automate) conditionAfterDecorationRun(
	evaluation conditionGroupEvaluation,
	conditions initialize.DTConditions,
	deviceId string,
) {
	defer a.ErrorRecover()
	for _, fc := range conditionAfterDecoration {
		err := fc(evaluation.ok, conditions, deviceId, evaluation.contents)
		if err != nil {
			logrus.Error(err)
		}
	}
}

func (a *Automate) actionAfterDecorationRun(actions []model.ActionInfo, deviceId string, err error) {
	defer a.ErrorRecover()
	for _, fc := range actionAfterDecoration {
		err := fc(actions, deviceId, err)
		if err != nil {
			logrus.Error(err)
		}
	}
}

func (a *Automate) ErrorRecover() {
	if r := recover(); r != nil {
		logAutomationPanic("automation execution", r)
	}
}

func logAutomationPanic(scope string, recovered interface{}) {
	logrus.WithFields(logrus.Fields{
		"scope": scope,
		"panic": recovered,
		"stack": string(debug.Stack()),
	}).Error("automation panic recovered")
}

func (a *Automate) Execute(deviceInfo *model.Device, fromExt AutomateFromExt) error {
	defer a.ErrorRecover()
	if deviceInfo == nil {
		return errors.New("device info is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.device = deviceInfo
	a.formExt = fromExt
	a.resetExecutionState()

	// 模板级自动化与设备级自动化会各跑一遍，任何一条失败都会被汇总返回，
	// 这样既能覆盖模板默认规则，也不会吞掉设备私有规则的执行错误。
	var configErr error
	if deviceInfo.DeviceConfigID != nil {
		deviceConfigId := *deviceInfo.DeviceConfigID
		if err := a.telExecute(deviceInfo.ID, deviceConfigId, fromExt); err != nil {
			configErr = err
			logrus.WithError(err).Error("device-config automation execution failed")
		}
	}

	deviceErr := a.telExecute(deviceInfo.ID, "", fromExt)
	switch {
	case configErr != nil && deviceErr != nil:
		return fmt.Errorf("device-config automation failed: %v; device automation failed: %w", configErr, deviceErr)
	case configErr != nil:
		return configErr
	default:
		return deviceErr
	}
}

func (a *Automate) AutomateConditionCheck(conditions initialize.DTConditions, deviceId string) bool {
	logrus.Trace("automation condition check started")
	// 组内是 AND，组间是 OR；任意一个分组整体命中就可以触发场景。
	return a.anyConditionGroupMatches(groupConditionsByGroupID(conditions), deviceId)
}

// @description Evaluate a time-based trigger condition against the current UTC time.
// @params cond model.DeviceTriggerCondition
// @return bool
func (*Automate) automateConditionCheckWithTime(cond model.DeviceTriggerCondition) bool {
	logrus.Trace("time-based condition check started, triggerValue:", cond.TriggerValue)
	nowTime := automateNow().UTC()
	parts, ok := parseTimeConditionParts(cond.TriggerValue)
	if !ok {
		return false
	}
	// 这里统一基于 UTC 解释触发窗口，避免服务端时区漂移造成时间条件误判。
	weekDay := common.GetWeekDay(nowTime)
	// 先过星期维度，再判断日内时间范围，减少无效解析和比较。
	if !timeConditionWeekdayMatches(parts.weekdays, weekDay) {
		return false
	}
	nowTimeNotDay, _ := time.Parse(timeOfDayLayout, nowTime.Format(timeOfDayLayout))
	startTime, ok := parseTimeConditionBoundary(parts.start, cond.TriggerValue)
	if !ok {
		return false
	}
	if timeOfDayBeforeConditionStart(nowTimeNotDay, startTime) {
		return false
	}
	endTime, ok := parseTimeConditionBoundary(parts.end, cond.TriggerValue)
	if !ok {
		return false
	}
	if !timeOfDayBeforeConditionEnd(nowTimeNotDay, startTime, endTime) {
		return false
	}
	logrus.Trace("time-based condition matched")
	return true
}

type timeConditionParts struct {
	weekdays string
	start    string
	end      string
}

func parseTimeConditionParts(triggerValue string) (timeConditionParts, bool) {
	if triggerValue == "" {
		return timeConditionParts{}, false
	}
	valParts := strings.Split(triggerValue, "|")
	if len(valParts) < 3 {
		return timeConditionParts{}, false
	}
	return timeConditionParts{
		weekdays: valParts[0],
		start:    valParts[1],
		end:      valParts[2],
	}, true
}

func timeConditionWeekdayMatches(weekdays string, weekDay int) bool {
	for _, char := range weekdays {
		num, _ := strconv.Atoi(string(char))
		if weekDay == num {
			return true
		}
	}
	return false
}

func parseTimeConditionBoundary(value, triggerValue string) (time.Time, bool) {
	conditionTime, err := time.Parse(timeOfDayLayout, value)
	if err != nil {
		logrus.Errorf("failed to parse time condition boundary from triggerValue=%s", triggerValue)
		return time.Time{}, false
	}
	return conditionTime, true
}

func timeOfDayBeforeConditionStart(nowTimeNotDay, startTime time.Time) bool {
	return startTime.After(nowTimeNotDay)
}

func timeOfDayBeforeConditionEnd(nowTimeNotDay, startTime, endTime time.Time) bool {
	// Support windows that cross midnight, such as 23:00 -> 02:00.
	if endTime.Before(startTime) {
		if nowTimeNotDay.Before(startTime) && nowTimeNotDay.After(endTime) {
			return false
		}
	} else if endTime.Before(nowTimeNotDay) {
		return false
	}
	return true
}
