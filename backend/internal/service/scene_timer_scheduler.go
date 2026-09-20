// File purpose: P0.4 durable scene timer scheduling - compute the next fire time and
// drive one tick of the trigger loop.
// Core logic: NextSceneTimerRun is a pure cron/timezone computation; runSceneTimerTick
// claims due timers, triggers them, and only advances the clock after a real success.
// Key notes: the clock is NEVER advanced before a successful trigger. Advancing first is an
// optimistic success assumption, and if the trigger then fails that occurrence is gone
// forever - which is exactly the "lost schedule" this P0 item exists to prevent.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

// ErrInvalidSceneTimerTimezone 时区非法。与执行窗口同样采取 fail closed：
// 静默按 UTC 兜底会让触发时刻整体偏移，等于伪造了一份调度计划。
var ErrInvalidSceneTimerTimezone = errors.New("scene timer timezone is invalid")

// NextSceneTimerRun 计算严格晚于 from 的下一个触发时刻。
func NextSceneTimerRun(cronExpr, timezone string, from time.Time) (time.Time, error) {
	zone := timezone
	if zone == "" {
		zone = "UTC"
	}
	// 时区先单独校验，只为给出精确的错误哨兵（fail closed，不静默按 UTC 兜底）。
	if _, err := time.LoadLocation(zone); err != nil {
		return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidSceneTimerTimezone, timezone)
	}
	// 复用项目既有的 cron 解析（5 段走 ParseStandard、6 段走 Parse），
	// 避免这里长出第二套字段语义。
	location, schedule, err := parseReportSchedule(cronExpr, zone)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(from.In(location)), nil
}

// RunSceneTimerTick 执行一轮定时触发扫描，供 app 层的 worker 调用。
// 触发动作固定为"执行该场景自动化"；其余副作用仍走可注入的 sceneTimerOps，
// 便于在测试中替换。
func RunSceneTimerTick(ctx context.Context, owner string, limit int, lease time.Duration) (SceneTimerTickResult, error) {
	ops := sceneTimerOps
	ops.trigger = triggerSceneAutomationTimer
	return runSceneTimerTick(ctx, owner, limit, lease, ops)
}

func triggerSceneAutomationTimer(_ context.Context, timer model.SceneAutomationTimer) error {
	return GroupApp.Automate.ActiveSceneExecute(timer.SceneAutomationID, timer.TenantID)
}

// SceneTimerTickResult 一轮 tick 的结果，便于观测与测试。
type SceneTimerTickResult struct {
	Claimed    int `json:"claimed"`
	Triggered  int `json:"triggered"`
	Failed     int `json:"failed"`
	Stalled    int `json:"stalled"`
	Unparsable int `json:"unparsable"`
}

// sceneTimerOperations tick 的副作用集合，可注入以便无数据库验证。
type sceneTimerOperations struct {
	claim    func(context.Context, string, int, time.Duration, time.Time) ([]model.SceneAutomationTimer, error)
	trigger  func(context.Context, model.SceneAutomationTimer) error
	complete func(context.Context, string, time.Time, time.Time) error
	fail     func(context.Context, string, string, time.Time) error
	release  func(context.Context, string, time.Time) error
	now      func() time.Time
}

var sceneTimerOps = sceneTimerOperations{
	claim:    dal.ClaimDueSceneTimers,
	complete: dal.CompleteSceneTimerRun,
	fail:     dal.FailSceneTimerRun,
	release:  dal.ReleaseSceneTimerLease,
	now:      time.Now,
}

// runSceneTimerTick 执行一轮定时触发扫描。
// 逐个处理而非批量：一个定时器失败不应影响同批其他定时器，
// 且失败要能单独落到该行的 consecutive_failures 上。
func runSceneTimerTick(ctx context.Context, owner string, limit int, lease time.Duration, ops sceneTimerOperations) (SceneTimerTickResult, error) {
	now := ops.now().UTC()
	result := SceneTimerTickResult{}

	timers, err := ops.claim(ctx, owner, limit, lease, now)
	if err != nil {
		return result, err
	}
	result.Claimed = len(timers)

	for _, timer := range timers {
		// 连续失败已达上限：停摆等待人工介入，不再触发。
		// 继续重试一个必然失败的触发只会打爆下游并淹没真正的告警。
		if timer.IsStalled() {
			result.Stalled++
			if releaseErr := ops.release(ctx, timer.ID, now); releaseErr != nil {
				return result, releaseErr
			}
			continue
		}

		nextRun, nextErr := NextSceneTimerRun(timer.CronExpr, timer.Timezone, now)
		if nextErr != nil {
			// 表达式或时区坏了，靠重试不会变好：记为失败让它停摆，
			// 而不是每轮都领走、每轮都失败。
			result.Unparsable++
			if failErr := ops.fail(ctx, timer.ID, nextErr.Error(), now); failErr != nil {
				return result, failErr
			}
			continue
		}

		if triggerErr := ops.trigger(ctx, timer); triggerErr != nil {
			result.Failed++
			if failErr := ops.fail(ctx, timer.ID, triggerErr.Error(), now); failErr != nil {
				return result, failErr
			}
			continue
		}

		// 只有真的触发成功才推进时钟。
		if completeErr := ops.complete(ctx, timer.ID, nextRun, now); completeErr != nil {
			return result, completeErr
		}
		result.Triggered++
	}
	return result, nil
}
