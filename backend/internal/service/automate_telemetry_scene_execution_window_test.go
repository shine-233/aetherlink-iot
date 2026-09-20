package service

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
)

func automateExecteParamsFixture(sceneIDs ...string) initialize.AutomateExecteParams {
	infos := make([]initialize.AutomateExecteSceneInfo, 0, len(sceneIDs))
	for _, id := range sceneIDs {
		infos = append(infos, initialize.AutomateExecteSceneInfo{SceneAutomationId: id})
	}
	return initialize.AutomateExecteParams{DeviceId: "device-1", AutomateExecteSceeInfos: infos}
}

func sceneExecutionCheckName(check sceneExecutionCheck) string {
	pc := runtime.FuncForPC(reflect.ValueOf(check).Pointer())
	if pc == nil {
		return ""
	}
	name := pc.Name()
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	return name
}

// P0.4 接线证据：ExecutionWindow / CanRun 此前已实现但零生产调用点，
// 等于"写了但没接上"。本文件钉住窗口门禁真正接入执行链后的行为。
//
// 断言直接针对 sceneWithinExecutionWindow，因为它是链上的实际守卫；
// 若有人把它从 sceneExecutionChecks 中摘掉，下面的窗口行为将不再有任何约束。

func sceneWindowCandidate() sceneExecutionCandidate {
	return sceneExecutionCandidate{sceneAutomationID: "scene-1", deviceID: "device-1"}
}

func automateWithTenant(tenantID string) *Automate {
	return &Automate{device: &model.Device{TenantID: tenantID}}
}

func TestSceneWithinExecutionWindowAllowsUnboundedScene(t *testing.T) {
	automate := &Automate{}
	// 未配置窗口：window 为 nil，必须放行。
	if !automate.sceneWithinExecutionWindow(sceneWindowCandidate()) {
		t.Fatal("scene with no window must run; nil window means unbounded")
	}
	// 有 window 对象但没有任何边界：同样必须放行。
	candidate := sceneWindowCandidate()
	candidate.window = &model.SceneAutomationWindow{SceneAutomationID: "scene-1"}
	if !automate.sceneWithinExecutionWindow(candidate) {
		t.Fatal("scene with an empty window must run; no bounds means unbounded")
	}
}

func TestSceneWithinExecutionWindowHonoursBounds(t *testing.T) {
	now := time.Now()
	start := now.Add(-time.Hour)
	expiry := now.Add(time.Hour)
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	cases := []struct {
		name      string
		startsAt  *time.Time
		expiresAt *time.Time
		want      bool
	}{
		{name: "inside window runs", startsAt: &start, expiresAt: &expiry, want: true},
		{name: "before start is skipped", startsAt: &future, expiresAt: nil, want: false},
		{name: "at or after expiry is skipped", startsAt: nil, expiresAt: &past, want: false},
		{name: "only lower bound and already started runs", startsAt: &start, expiresAt: nil, want: true},
		{name: "only upper bound and not yet expired runs", startsAt: nil, expiresAt: &expiry, want: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := sceneWindowCandidate()
			candidate.window = &model.SceneAutomationWindow{
				SceneAutomationID: "scene-1",
				StartsAt:          testCase.startsAt,
				ExpiresAt:         testCase.expiresAt,
			}
			if got := (&Automate{}).sceneWithinExecutionWindow(candidate); got != testCase.want {
				t.Fatalf("sceneWithinExecutionWindow = %v, want %v", got, testCase.want)
			}
		})
	}
}

// 非法时区若被静默按 UTC 兜底，窗口边界会整体偏移，凭空制造出一段"可执行"时间。
// 这里钉住 fail closed：宁可不执行，也不能算错边界。
func TestSceneWithinExecutionWindowFailsClosedOnInvalidTimezone(t *testing.T) {
	now := time.Now()
	start := now.Add(-time.Hour)
	expiry := now.Add(time.Hour)
	candidate := sceneWindowCandidate()
	candidate.window = &model.SceneAutomationWindow{
		SceneAutomationID: "scene-1",
		StartsAt:          &start,
		ExpiresAt:         &expiry,
		Timezone:          "Not/ARealTimezone",
	}
	if (&Automate{}).sceneWithinExecutionWindow(candidate) {
		t.Fatal("invalid timezone must refuse to run, not silently fall back to UTC")
	}
}

// 窗口读取失败时若当作"有窗口并拒绝"，一次存储抖动就会静默停掉全部场景，
// 比窗口本身失效危险得多。这里钉住退化为无界。
func TestExecutionWindowsTreatsLoadFailureAsUnbounded(t *testing.T) {
	original := loadSceneExecutionWindows
	defer func() { loadSceneExecutionWindows = original }()
	loadSceneExecutionWindows = func(context.Context, string, []string) (map[string]model.SceneAutomationWindow, error) {
		return nil, errors.New("storage unavailable")
	}
	automate := automateWithTenant("tenant-hq")
	params := automateExecteParamsFixture("scene-1")
	if windows := automate.executionWindows(params); windows != nil {
		t.Fatalf("executionWindows on failure = %v, want nil (unbounded)", windows)
	}
}

func TestExecutionWindowsLoadsPersistedWindowForCandidate(t *testing.T) {
	original := loadSceneExecutionWindows
	defer func() { loadSceneExecutionWindows = original }()
	start := time.Now().Add(-time.Hour)
	var seenTenant string
	loadSceneExecutionWindows = func(_ context.Context, tenantID string, ids []string) (map[string]model.SceneAutomationWindow, error) {
		seenTenant = tenantID
		windows := make(map[string]model.SceneAutomationWindow, len(ids))
		for _, id := range ids {
			windows[id] = model.SceneAutomationWindow{SceneAutomationID: id, StartsAt: &start}
		}
		return windows, nil
	}
	automate := automateWithTenant("tenant-hq")
	windows := automate.executionWindows(automateExecteParamsFixture("scene-1", "scene-2"))
	if len(windows) != 2 {
		t.Fatalf("executionWindows loaded %d windows, want 2", len(windows))
	}
	if seenTenant != "tenant-hq" {
		t.Fatalf("window lookup used tenant %q, want %q", seenTenant, "tenant-hq")
	}
	candidate := sceneWindowCandidate()
	if window, ok := windows["scene-1"]; !ok {
		t.Fatal("scene-1 window missing")
	} else {
		candidate.window = &window
	}
	if !automate.sceneWithinExecutionWindow(candidate) {
		t.Fatal("scene-1 started an hour ago and must run")
	}
}

// 没有租户上下文时必须放弃查询，而不是退化成"不带 tenant_id 的全表查"——
// 后者会把别家租户的窗口配置套用到本场景，属于跨租户泄漏。
func TestExecutionWindowsSkipsLookupWhenTenantUnknown(t *testing.T) {
	original := loadSceneExecutionWindows
	defer func() { loadSceneExecutionWindows = original }()
	called := false
	loadSceneExecutionWindows = func(context.Context, string, []string) (map[string]model.SceneAutomationWindow, error) {
		called = true
		return nil, nil
	}
	automate := &Automate{} // 无 device，无法判定租户
	if windows := automate.executionWindows(automateExecteParamsFixture("scene-1")); windows != nil {
		t.Fatalf("executionWindows without tenant = %v, want nil", windows)
	}
	if called {
		t.Fatal("window lookup must not run when the tenant is unknown")
	}
}

// 守卫必须真的挂在链上，否则上面的行为测试全部落空。
func TestSceneExecutionChecksIncludeExecutionWindow(t *testing.T) {
	found := false
	for _, check := range sceneExecutionChecks {
		if name := sceneExecutionCheckName(check); name != "" {
			if name == "sceneWithinExecutionWindow" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("sceneWithinExecutionWindow is not wired into sceneExecutionChecks; the window gate is dead code")
	}
}
