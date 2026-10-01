// 文件用途：自动化单次触发的执行上下文。
// 核心逻辑：automationExec 承载“一次上报”范围内的全部可变状态（触发设备、触发值、
// 已尝试/已执行场景集合），由 Automate.Execute 逐调用创建并沿条件求值与场景执行链传递。
// 关键注意事项：automationExec 只属于创建它的 goroutine，不得跨 goroutine 共享；
// 进程级共享的只有无状态的 Automate、线程安全的缓存/限流器/幂等存储。
package service

import (
	"fmt"

	"aetherlink-iot/backend/initialize/automatecache"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

type automationExec struct {
	*Automate
	device  *model.Device
	formExt AutomateFromExt
	// 同一条上报命中重复缓存项时，去重集合保证场景副作用只跑一次。
	attemptedSceneIDs map[string]bool
	executedSceneIDs  map[string]bool
}

func newAutomationExec(a *Automate, device *model.Device, fromExt AutomateFromExt) *automationExec {
	if a == nil {
		a = &Automate{}
	}
	return &automationExec{
		Automate:          a,
		device:            device,
		formExt:           fromExt,
		attemptedSceneIDs: make(map[string]bool),
		executedSceneIDs:  make(map[string]bool),
	}
}

// run 模板级自动化与设备级自动化各跑一遍，任何一条失败都会被汇总返回，
// 这样既能覆盖模板默认规则，也不会吞掉设备私有规则的执行错误。
func (a *automationExec) run() error {
	var configErr error
	if a.device.DeviceConfigID != nil {
		if err := a.telExecute(a.device.ID, *a.device.DeviceConfigID, a.formExt); err != nil {
			configErr = err
			logrus.WithError(err).Error("device-config automation execution failed")
		}
	}

	deviceErr := a.telExecute(a.device.ID, "", a.formExt)
	switch {
	case configErr != nil && deviceErr != nil:
		return fmt.Errorf("device-config automation failed: %v; device automation failed: %w", configErr, deviceErr)
	case configErr != nil:
		return configErr
	default:
		return deviceErr
	}
}

// automateCache 是自动化引擎对触发缓存的依赖面；实现为 automatecache.Cache（无状态、并发安全）。
type automateCache interface {
	GetCacheByDeviceId(deviceID, deviceConfigID string) (automatecache.AutomateExecteParams, int, error)
	SetCacheByDeviceId(deviceID, deviceConfigID string, conditions []model.DeviceTriggerCondition, actions []model.ActionInfo) error
	SetCacheByDeviceIdWithNoTask(deviceID, deviceConfigID string) error
	SetCacheBySceneAutomationId(sceneAutomationID string, conditions []model.DeviceTriggerCondition, actions []model.ActionInfo) error
	DeleteCacheBySceneAutomationId(sceneAutomationID string) error
}

// automateCacheProvider 可注入，测试可换成内存实现而无需 Redis。
var automateCacheProvider = func() automateCache { return automatecache.Default() }
