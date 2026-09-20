package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func sceneTimerOpsSpy(timers []model.SceneAutomationTimer, triggerErr error) (sceneTimerOperations, *[]string) {
	events := &[]string{}
	return sceneTimerOperations{
		claim: func(context.Context, string, int, time.Duration, time.Time) ([]model.SceneAutomationTimer, error) {
			return timers, nil
		},
		trigger: func(_ context.Context, timer model.SceneAutomationTimer) error {
			*events = append(*events, "trigger:"+timer.ID)
			return triggerErr
		},
		complete: func(_ context.Context, id string, nextRunAt time.Time, _ time.Time) error {
			*events = append(*events, "complete:"+id+"@"+nextRunAt.UTC().Format(time.RFC3339))
			return nil
		},
		fail: func(_ context.Context, id string, message string, _ time.Time) error {
			*events = append(*events, "fail:"+id+":"+message)
			return nil
		},
		release: func(_ context.Context, id string, _ time.Time) error {
			*events = append(*events, "release:"+id)
			return nil
		},
		now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}, events
}

func sceneTimerFixture(id string) model.SceneAutomationTimer {
	return model.SceneAutomationTimer{
		ID:                id,
		TenantID:          "tenant-hq",
		SceneAutomationID: "scene-1",
		CronExpr:          "0 * * * *",
		Timezone:          "UTC",
		Enabled:           true,
		NextRunAt:         time.Unix(1699999999, 0).UTC(),
	}
}

func TestNextSceneTimerRunAdvancesStrictlyForward(t *testing.T) {
	from := time.Date(2026, 9, 11, 10, 30, 0, 0, time.UTC)
	next, err := NextSceneTimerRun("0 * * * *", "UTC", from)
	if err != nil {
		t.Fatalf("NextSceneTimerRun error = %v", err)
	}
	if !next.After(from) {
		t.Fatalf("next = %s, want strictly after %s", next, from)
	}
	if next.Minute() != 0 || next.Second() != 0 {
		t.Fatalf("hourly cron should land on the hour; got %s", next)
	}
}

// 非法时区必须 fail closed：静默按 UTC 兜底会让触发时刻整体偏移，
// 等于伪造了一份调度计划。
func TestNextSceneTimerRunFailsClosedOnInvalidTimezone(t *testing.T) {
	if _, err := NextSceneTimerRun("0 * * * *", "Not/AZone", time.Now()); err == nil {
		t.Fatal("invalid timezone must fail, not silently fall back to UTC")
	} else if !errors.Is(err, ErrInvalidSceneTimerTimezone) {
		t.Fatalf("error = %v, want ErrInvalidSceneTimerTimezone", err)
	}
}

func TestNextSceneTimerRunRejectsInvalidCron(t *testing.T) {
	if _, err := NextSceneTimerRun("not a cron", "UTC", time.Now()); err == nil {
		t.Fatal("invalid cron expression must fail")
	}
}

// P0.4 的核心：时钟只在触发成功后推进。
// 先推进后执行等于乐观假设成功——执行一旦失败，这次触发就永久消失了。
func TestSceneTimerTickAdvancesClockOnlyAfterSuccess(t *testing.T) {
	timer := sceneTimerFixture("timer-ok")
	ops, events := sceneTimerOpsSpy([]model.SceneAutomationTimer{timer}, nil)
	result, err := runSceneTimerTick(context.Background(), "replica-1", 10, time.Minute, ops)
	if err != nil {
		t.Fatalf("tick error = %v", err)
	}
	if result.Triggered != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want triggered=1 failed=0", result)
	}
	if len(*events) != 2 {
		t.Fatalf("events = %v, want trigger then complete", *events)
	}
	if (*events)[0] != "trigger:timer-ok" {
		t.Fatalf("first event = %q, want trigger first", (*events)[0])
	}
	if got := (*events)[1]; got[:len("complete:")] != "complete:" {
		t.Fatalf("second event = %q, want complete after successful trigger", got)
	}
}

func TestSceneTimerTickDoesNotAdvanceClockOnFailure(t *testing.T) {
	timer := sceneTimerFixture("timer-fail")
	ops, events := sceneTimerOpsSpy([]model.SceneAutomationTimer{timer}, errors.New("downstream unavailable"))
	result, err := runSceneTimerTick(context.Background(), "replica-1", 10, time.Minute, ops)
	if err != nil {
		t.Fatalf("tick error = %v", err)
	}
	if result.Failed != 1 || result.Triggered != 0 {
		t.Fatalf("result = %+v, want failed=1 triggered=0", result)
	}
	for _, event := range *events {
		if len(event) >= 8 && event[:8] == "complete" {
			t.Fatalf("clock must not advance after a failed trigger; events=%v", *events)
		}
	}
}

// 停摆的定时器必须被释放而不是继续触发：
// 无限重试一个必然失败的触发只会打爆下游并淹没真正的告警。
func TestSceneTimerTickReleasesStalledTimers(t *testing.T) {
	timer := sceneTimerFixture("timer-stalled")
	timer.ConsecutiveFailures = model.SceneAutomationTimerMaxConsecutiveFailures
	ops, events := sceneTimerOpsSpy([]model.SceneAutomationTimer{timer}, nil)
	result, err := runSceneTimerTick(context.Background(), "replica-1", 10, time.Minute, ops)
	if err != nil {
		t.Fatalf("tick error = %v", err)
	}
	if result.Stalled != 1 || result.Triggered != 0 {
		t.Fatalf("result = %+v, want stalled=1 triggered=0", result)
	}
	if len(*events) != 1 || (*events)[0] != "release:timer-stalled" {
		t.Fatalf("events = %v, want only a lease release", *events)
	}
}

// 表达式/时区坏了，靠重试不会变好：记失败让它停摆，而不是每轮都领走又失败。
func TestSceneTimerTickFailsUnparsableTimers(t *testing.T) {
	timer := sceneTimerFixture("timer-bad")
	timer.CronExpr = "definitely not cron"
	ops, events := sceneTimerOpsSpy([]model.SceneAutomationTimer{timer}, nil)
	result, err := runSceneTimerTick(context.Background(), "replica-1", 10, time.Minute, ops)
	if err != nil {
		t.Fatalf("tick error = %v", err)
	}
	if result.Unparsable != 1 || result.Triggered != 0 {
		t.Fatalf("result = %+v, want unparsable=1 triggered=0", result)
	}
	if len(*events) != 1 || (*events)[0][:5] != "fail:" {
		t.Fatalf("events = %v, want a single failure record", *events)
	}
}

// 一个定时器失败不应拖垮同批其他定时器。
func TestSceneTimerTickIsolatesFailuresPerTimer(t *testing.T) {
	bad := sceneTimerFixture("timer-bad")
	good := sceneTimerFixture("timer-good")
	ops, _ := sceneTimerOpsSpy([]model.SceneAutomationTimer{bad, good}, nil)
	ops.trigger = func(_ context.Context, timer model.SceneAutomationTimer) error {
		if timer.ID == "timer-bad" {
			return errors.New("boom")
		}
		return nil
	}
	result, err := runSceneTimerTick(context.Background(), "replica-1", 10, time.Minute, ops)
	if err != nil {
		t.Fatalf("tick error = %v", err)
	}
	if result.Failed != 1 || result.Triggered != 1 {
		t.Fatalf("result = %+v, want failed=1 triggered=1", result)
	}
}

func TestSceneAutomationTimerLeaseActiveAndStalled(t *testing.T) {
	now := time.Now()
	timer := model.SceneAutomationTimer{}
	if timer.LeaseActive(now) {
		t.Fatal("no lease must not be active")
	}
	expired := now.Add(-time.Minute)
	timer.LeaseUntil = &expired
	if timer.LeaseActive(now) {
		t.Fatal("expired lease must be reclaimable, otherwise the schedule is lost")
	}
	future := now.Add(time.Minute)
	timer.LeaseUntil = &future
	if !timer.LeaseActive(now) {
		t.Fatal("unexpired lease must stay active")
	}
	if (model.SceneAutomationTimer{ConsecutiveFailures: 1}).IsStalled() {
		t.Fatal("one failure must not stall a timer")
	}
	if !(model.SceneAutomationTimer{ConsecutiveFailures: model.SceneAutomationTimerMaxConsecutiveFailures}).IsStalled() {
		t.Fatal("reaching the failure cap must stall a timer")
	}
}
