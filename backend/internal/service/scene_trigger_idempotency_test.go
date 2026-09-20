package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize"
)

// ---------------------------------------------------------------------------
// 存储层
// ---------------------------------------------------------------------------

func TestSceneTriggerStoreClaimsOncePerSecond(t *testing.T) {
	store := NewInMemorySceneTriggerStore(0)
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)

	first, err := store.Claim(context.Background(), "k1", now)
	if err != nil {
		t.Fatalf("first claim err = %v", err)
	}
	if !first {
		t.Fatal("first claim = false, want true")
	}

	// 同一秒内的亚秒抖动：键已截断到秒，必须被判为重复。
	second, err := store.Claim(context.Background(), "k1", now.Add(300*time.Millisecond))
	if err != nil {
		t.Fatalf("second claim err = %v", err)
	}
	if second {
		t.Fatal("second claim = true, want false (sub-second jitter must be absorbed)")
	}
}

func TestSceneTriggerStoreExpiresAndReclaims(t *testing.T) {
	store := NewInMemorySceneTriggerStore(1 * time.Second)
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)

	if claimed, _ := store.Claim(context.Background(), "k1", now); !claimed {
		t.Fatal("first claim = false, want true")
	}
	// 未过期：仍判重复。
	if claimed, _ := store.Claim(context.Background(), "k1", now.Add(500*time.Millisecond)); claimed {
		t.Fatal("claim before expiry = true, want false")
	}
	// 过期后：必须能重新认领，否则同一场景在 TTL 之后再也不会执行。
	if claimed, _ := store.Claim(context.Background(), "k1", now.Add(2*time.Second)); !claimed {
		t.Fatal("claim after expiry = false, want true")
	}
}

func TestSceneTriggerStoreSweepsExpiredEntries(t *testing.T) {
	store := NewInMemorySceneTriggerStore(1 * time.Second)
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	_, _ = store.Claim(context.Background(), "k1", now)
	if len(store.seen) != 1 {
		t.Fatalf("seen size = %d, want 1", len(store.seen))
	}
	_, _ = store.Claim(context.Background(), "k2", now.Add(3*time.Second))
	if len(store.seen) != 1 {
		t.Fatalf("seen size after sweep = %d, want 1 (expired entry must be dropped)", len(store.seen))
	}
}

func TestSceneTriggerNilStoreIsUnavailable(t *testing.T) {
	var store *InMemorySceneTriggerStore
	claimed, err := store.Claim(context.Background(), "k1", time.Now())
	if err == nil {
		t.Fatal("nil store claim err = nil, want error")
	}
	if claimed {
		t.Fatal("nil store claim = true, want false")
	}
}

// ---------------------------------------------------------------------------
// 检查层（fail open / 接线）
// ---------------------------------------------------------------------------

// withSceneTriggerStore 替换全局幂等存储，返回还原函数。
func withSceneTriggerStore(store sceneTriggerStore) func() {
	prev := sceneTriggerDedupe
	sceneTriggerDedupe = store
	return func() { sceneTriggerDedupe = prev }
}

// withFreshSceneTriggerStore 为每个用例装一个**全新**的进程内存储。
//
// 为什么必须有它：幂等存储按设计是全局共享的——它要吸收的是"两次独立上报落在同一秒"，
// 若每个 Automate 实例各存一份就永远去重不了。但全局状态会跨用例泄漏：
// 本包多个用例都用 scene-1/device-1 调 ExecuteRun，且都在同一秒内跑完，
// 先跑的用例会把键吃掉，后面的用例就被误判成重复触发而拿不到预期的限流/执行次数。
// 这不是被测代码的问题，是共享可变全局的固有代价，故由用例显式隔离。
func withFreshSceneTriggerStore() func() {
	return withSceneTriggerStore(NewInMemorySceneTriggerStore(0))
}

// withPassThroughSceneChecks 让除幂等以外的守卫全部放行，便于单独观察幂等行为。
func withPassThroughSceneChecks() func() {
	prevClosed := executeRunCheckSceneAutomationHasClose
	prevCondition := executeRunConditionCheck
	prevLimiter := executeRunLimiterAllow
	executeRunCheckSceneAutomationHasClose = func(*Automate, string) bool { return false }
	executeRunConditionCheck = func(*Automate, initialize.DTConditions, string) bool { return true }
	executeRunLimiterAllow = func(*Automate, string) bool { return true }
	return func() {
		executeRunCheckSceneAutomationHasClose = prevClosed
		executeRunConditionCheck = prevCondition
		executeRunLimiterAllow = prevLimiter
	}
}

func TestSceneTriggerIsNewFailsOpenWhenStoreMissing(t *testing.T) {
	defer withSceneTriggerStore(nil)()
	a := &Automate{}
	candidate := sceneExecutionCandidate{sceneAutomationID: "s1", deviceID: "d1"}
	if !a.sceneTriggerIsNew(candidate) {
		t.Fatal("sceneTriggerIsNew with nil store = false, want true (fail open)")
	}
}

// errStore 用于验证"存储故障不得让场景停摆"。
type errStore struct{}

func (errStore) Claim(context.Context, string, time.Time) (bool, error) {
	return false, errors.New("store offline")
}

func TestSceneTriggerIsNewFailsOpenOnStoreError(t *testing.T) {
	defer withSceneTriggerStore(errStore{})()
	a := &Automate{}
	candidate := sceneExecutionCandidate{sceneAutomationID: "s1", deviceID: "d1"}
	if !a.sceneTriggerIsNew(candidate) {
		t.Fatal("sceneTriggerIsNew on store error = false, want true (fail open)")
	}
}

// TestPrepareSceneExecutionSuppressesSameSecondDuplicate 端到端：
// 同一次定时器触发在亚秒抖动下重复到达，只允许执行一次。
func TestPrepareSceneExecutionSuppressesSameSecondDuplicate(t *testing.T) {
	defer withSceneTriggerStore(NewInMemorySceneTriggerStore(0))()
	defer withPassThroughSceneChecks()()

	a := &Automate{}
	candidate := sceneExecutionCandidate{sceneAutomationID: "s1", deviceID: "d1"}

	if !a.prepareSceneExecution(candidate) {
		t.Fatal("first prepare = false, want true")
	}
	// 同一实例内第二次：canAttemptScene 会先拦（已 attempted），
	// 这里换成新实例以单独考察跨上报的幂等，而不是实例内去重。
	b := &Automate{}
	if b.prepareSceneExecution(candidate) {
		t.Fatal("second prepare = true, want false (same-second duplicate must be suppressed)")
	}
}

// TestPrepareSceneExecutionAllowsDifferentSecond 防止过度去重：
// 幂等只收敛"同一秒"，跨秒的正常触发必须照常执行，否则场景会被永久饿死。
func TestPrepareSceneExecutionAllowsDifferentSecond(t *testing.T) {
	store := NewInMemorySceneTriggerStore(1 * time.Second)
	defer withSceneTriggerStore(store)()
	defer withPassThroughSceneChecks()()

	// 直接对存储打点：模拟上一秒已执行过。
	lastSecond := time.Now().Add(-1 * time.Second)
	if _, err := store.Claim(context.Background(),
		FlowTriggerKey(FlowTriggerIdentity{FlowID: "s1", DeviceID: "d1", TriggerAt: lastSecond}), lastSecond); err != nil {
		t.Fatalf("seed claim err = %v", err)
	}

	a := &Automate{}
	candidate := sceneExecutionCandidate{sceneAutomationID: "s1", deviceID: "d1"}
	if !a.prepareSceneExecution(candidate) {
		t.Fatal("prepare in a later second = false, want true (must not starve the scene)")
	}
}
