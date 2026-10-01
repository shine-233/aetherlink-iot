package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
)

// withExecuteRunHooks 注入 ExecuteRun 链路上的全部外部依赖，并关闭跨上报幂等，
// 让用例只观察执行上下文本身的隔离性。
func withExecuteRunHooks(
	t *testing.T,
	condition func(a *automationExec, conditions initialize.DTConditions, deviceID string) bool,
	execute func(a *automationExec, sceneID string, deviceIDs []string, actions []model.ActionInfo) error,
) {
	t.Helper()
	prevLimiter, prevClosed := executeRunLimiterAllow, executeRunCheckSceneAutomationHasClose
	prevCond, prevExec, prevAfter := executeRunConditionCheck, executeRunSceneAutomateExecute, executeRunActionAfterDecoration
	restoreDedupe := withSceneTriggerStore(nil)
	t.Cleanup(func() {
		executeRunLimiterAllow, executeRunCheckSceneAutomationHasClose = prevLimiter, prevClosed
		executeRunConditionCheck, executeRunSceneAutomateExecute, executeRunActionAfterDecoration = prevCond, prevExec, prevAfter
		restoreDedupe()
	})
	executeRunLimiterAllow = func(*automationExec, string) bool { return true }
	executeRunCheckSceneAutomationHasClose = func(*automationExec, string) bool { return false }
	executeRunConditionCheck = condition
	executeRunSceneAutomateExecute = execute
	executeRunActionAfterDecoration = func(*automationExec, []model.ActionInfo, string, error) {}
}

// 旧实现把 device/formExt/去重集合放在共享的 Automate 上、靠全局锁串行。
// 去锁后每次触发必须看到且只看到自己的上下文：大量 goroutine 共用同一个 *Automate
// 并发执行，条件求值读到的触发值和设备必须与本次调用一致，去重集合互不串扰。
func TestAutomationExecIsolatedAcrossConcurrentTriggers(t *testing.T) {
	var mismatches, executions atomic.Int64
	withExecuteRunHooks(t,
		func(a *automationExec, _ initialize.DTConditions, deviceID string) bool {
			got, ok := a.getActualValueFromTriggerOverrides("owner")
			if !ok || got != a.device.ID || deviceID != a.device.ID {
				mismatches.Add(1)
			}
			time.Sleep(time.Microsecond) // 拉宽交错窗口
			return true
		},
		func(a *automationExec, _ string, deviceIDs []string, _ []model.ActionInfo) error {
			if len(deviceIDs) != 1 || deviceIDs[0] != a.device.ID {
				mismatches.Add(1)
			}
			executions.Add(1)
			return nil
		},
	)

	shared := &Automate{}
	const goroutines, rounds = 64, 25
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				id := fmt.Sprintf("device-%d-%d", g, r)
				exec := newAutomationExec(shared, &model.Device{ID: id}, AutomateFromExt{
					TriggerValues: map[string]interface{}{"owner": id},
				})
				// 每次上报两个场景，其中 scene-a 重复出现：只允许执行一次。
				err := exec.ExecuteRun(initialize.AutomateExecteParams{
					DeviceId: id,
					AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
						{SceneAutomationId: "scene-a"},
						{SceneAutomationId: "scene-b"},
						{SceneAutomationId: "scene-a"},
					},
				})
				if err != nil {
					mismatches.Add(1)
				}
				if len(exec.executedSceneIDs) != 2 || !exec.executedSceneIDs["scene-a"] || !exec.executedSceneIDs["scene-b"] {
					mismatches.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if n := mismatches.Load(); n != 0 {
		t.Fatalf("%d cross-trigger context leaks detected", n)
	}
	if got, want := executions.Load(), int64(goroutines*rounds*2); got != want {
		t.Fatalf("scene executions = %d, want %d (duplicate entry must run once per trigger)", got, want)
	}
}

// 回归：一个设备的慢自动化不得阻塞其它设备（旧全局锁会让第二个调用一直等）。
func TestAutomationExecDoesNotSerializeAcrossDevices(t *testing.T) {
	release := make(chan struct{})
	slowEntered := make(chan struct{})
	withExecuteRunHooks(t,
		func(*automationExec, initialize.DTConditions, string) bool { return true },
		func(a *automationExec, _ string, _ []string, _ []model.ActionInfo) error {
			if a.device.ID == "slow" {
				close(slowEntered)
				<-release
			}
			return nil
		},
	)
	shared := &Automate{}
	params := func(id string) initialize.AutomateExecteParams {
		return initialize.AutomateExecteParams{DeviceId: id, AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{{SceneAutomationId: "s"}}}
	}
	go func() {
		_ = newAutomationExec(shared, &model.Device{ID: "slow"}, AutomateFromExt{}).ExecuteRun(params("slow"))
	}()
	waitOrFail(t, slowEntered, "slow device execution")

	done := make(chan struct{})
	go func() {
		_ = newAutomationExec(shared, &model.Device{ID: "fast"}, AutomateFromExt{}).ExecuteRun(params("fast"))
		close(done)
	}()
	waitOrFail(t, done, "fast device while slow device holds its execution")
	close(release)
}

// withAutomationPool 为用例装一个独立的全局工作池，并注入执行函数。
func withAutomationPool(t *testing.T, cfg AutomationPoolConfig, exec func(*Automate, *model.Device, AutomateFromExt) error) {
	t.Helper()
	automationPoolMu.Lock()
	prevPool := automationPool
	automationPool = nil
	automationPoolMu.Unlock()
	prevExec := automationPoolExecute
	automationPoolExecute = exec
	if !StartAutomationPool(cfg) {
		t.Fatal("StartAutomationPool returned false on a fresh pool")
	}
	t.Cleanup(func() {
		_ = StopAutomationPool(context.Background())
		automationPoolMu.Lock()
		automationPool = prevPool
		automationPoolMu.Unlock()
		automationPoolExecute = prevExec
	})
}

// Dispatch 端到端：四类触发混合并发投递，同设备按投递顺序执行，停机排空后计数闭合。
func TestDispatchPreservesPerDeviceOrderAndDrainsOnStop(t *testing.T) {
	var mu sync.Mutex
	order := make(map[string][]int)
	withAutomationPool(t, AutomationPoolConfig{Workers: 8, QueueSize: 4096},
		func(_ *Automate, device *model.Device, fromExt AutomateFromExt) error {
			mu.Lock()
			order[device.ID] = append(order[device.ID], fromExt.TriggerValues["seq"].(int))
			mu.Unlock()
			return nil
		})
	if StartAutomationPool(AutomationPoolConfig{}) {
		t.Fatal("StartAutomationPool must be a no-op while running")
	}

	types := []string{model.TRIGGER_PARAM_TYPE_TEL, model.TRIGGER_PARAM_TYPE_ATTR, model.TRIGGER_PARAM_TYPE_EVT, model.TRIGGER_PARAM_TYPE_STATUS}
	const devices, perDevice = 30, 100
	a := &Automate{}
	var wg sync.WaitGroup
	for d := 0; d < devices; d++ {
		device := &model.Device{ID: fmt.Sprintf("dev-%d", d)}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perDevice; i++ {
				if err := a.Dispatch(device, AutomateFromExt{
					TriggerParamType: types[i%len(types)],
					TriggerValues:    map[string]interface{}{"seq": i},
				}); err != nil {
					t.Errorf("dispatch: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	if err := StopAutomationPool(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	for id, seqs := range order {
		if len(seqs) != perDevice {
			t.Fatalf("%s ran %d, want %d", id, len(seqs), perDevice)
		}
		for i, s := range seqs {
			if s != i {
				t.Fatalf("%s out of order at %d: %d", id, i, s)
			}
		}
	}
	if len(order) != devices {
		t.Fatalf("devices = %d, want %d", len(order), devices)
	}
	stats := AutomationPoolSnapshot()
	if stats.Submitted != devices*perDevice || stats.Completed != stats.Submitted || stats.Dropped != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if err := a.Dispatch(&model.Device{ID: "late"}, AutomateFromExt{}); !errors.Is(err, ErrAutomationPoolStopped) {
		t.Fatalf("dispatch after stop err = %v, want ErrAutomationPoolStopped", err)
	}
	// 停止后可重新启动（进程内重启 Application 的场景）。
	if !StartAutomationPool(AutomationPoolConfig{Workers: 1, QueueSize: 1}) {
		t.Fatal("StartAutomationPool after stop must start a new pool")
	}
}

func TestDispatchRejectsNilDevice(t *testing.T) {
	if err := (&Automate{}).Dispatch(nil, AutomateFromExt{}); err == nil {
		t.Fatal("Dispatch(nil) err = nil, want error")
	}
}

// 执行期错误与 panic 都留在池内：不影响投递方，也不杀死 worker。
func TestDispatchContainsExecutionErrorsAndPanics(t *testing.T) {
	var calls atomic.Int32
	done := make(chan struct{})
	withAutomationPool(t, AutomationPoolConfig{Workers: 1, QueueSize: 8},
		func(_ *Automate, device *model.Device, _ AutomateFromExt) error {
			switch calls.Add(1) {
			case 1:
				return errors.New("boom")
			case 2:
				panic("kaboom")
			default:
				close(done)
				return nil
			}
		})
	a := &Automate{}
	for i := 0; i < 3; i++ {
		if err := a.Dispatch(&model.Device{ID: "d"}, AutomateFromExt{}); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	waitOrFail(t, done, "third dispatch after error and panic")
	if stats := AutomationPoolSnapshot(); stats.Panics != 1 {
		t.Fatalf("panics = %d, want 1", stats.Panics)
	}
}
