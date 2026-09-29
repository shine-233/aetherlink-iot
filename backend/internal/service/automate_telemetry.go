// automate_telemetry.go 负责自动化触发入口、缓存回源、时间条件判断、
// 场景执行编排和执行日志主链路，是遥测自动化的核心调度文件。
package service

import (
	"errors"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/common"

	"github.com/sirupsen/logrus"
)

const unsupportedActionMessage = "unsupported automate action"

// Automate 是自动化引擎的无状态服务入口（挂在 GroupApp 上）。
// 它不持有任何每次触发的状态：一次触发的设备、触发值与场景去重集合都放在
// automationExec 里逐调用创建、沿调用链传递，因此任意多个触发可以并发执行，
// 不再需要进程级互斥锁把全部设备的自动化串行化。
type Automate struct{}

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

// Execute 同步执行一次设备触发的全部自动化（模板级 + 设备级）。
// 可被并发调用：每次调用拥有独立的 automationExec。需要限流/保序的调用方
// 应改走 Dispatch（按设备分片的有界工作池），而不是自己起 goroutine。
func (a *Automate) Execute(deviceInfo *model.Device, fromExt AutomateFromExt) error {
	defer a.ErrorRecover()
	if deviceInfo == nil {
		return errors.New("device info is required")
	}
	return newAutomationExec(a, deviceInfo, fromExt).run()
}

func (a *automationExec) AutomateConditionCheck(conditions initialize.DTConditions, deviceId string) bool {
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
