// File purpose: P0.4 worker that drives durable scene timer triggers.
// Core logic: tick on an interval, claim due timers with a lease, trigger them, advance the
// clock only after success.
// Key notes on "no lost schedule after restart": there is no in-memory timer state at all.
// The next fire time lives in scene_automation_timers.next_run_at, so a restart simply
// resumes from the database. Recovery from a crash *mid-trigger* comes from the lease:
// the claim query treats timers whose lease_until has passed as claimable, so a timer
// grabbed by a replica that died is picked up again instead of hanging forever.

package app

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	defaultSceneTimerTickInterval = 15 * time.Second
	defaultSceneTimerLease        = 2 * time.Minute
	defaultSceneTimerBatch        = 50
)

// SceneTimerWorker 驱动场景定时触发的后台服务。
type SceneTimerWorker struct {
	owner     string
	interval  time.Duration
	lease     time.Duration
	batch     int
	cancel    context.CancelFunc
	stopped   chan struct{}
}

// NewSceneTimerWorker 构造场景定时器 worker。
// owner 用进程级唯一 id：租约靠它区分副本，重启后换一个 owner 也不会
// 误认自己持有旧租约。
func NewSceneTimerWorker() *SceneTimerWorker {
	return &SceneTimerWorker{
		owner:    "scene-timer-" + uuid.NewString(),
		interval: defaultSceneTimerTickInterval,
		lease:    defaultSceneTimerLease,
		batch:    defaultSceneTimerBatch,
		stopped:  make(chan struct{}),
	}
}

func (w *SceneTimerWorker) Name() string { return "场景定时触发 worker" }

func (w *SceneTimerWorker) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.loop(ctx)
	logrus.Infof("scene timer worker started: owner=%s interval=%s lease=%s",
		w.owner, w.interval, w.lease)
	return nil
}

func (w *SceneTimerWorker) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	// 等待循环退出，避免在触发执行中途把进程带走。
	select {
	case <-w.stopped:
	case <-time.After(5 * time.Second):
		logrus.Warn("scene timer worker stop timed out; in-flight trigger may be abandoned to lease expiry")
	}
	return nil
}

func (w *SceneTimerWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	defer close(w.stopped)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *SceneTimerWorker) tick(ctx context.Context) {
	if global.DB == nil {
		return
	}
	result, err := service.RunSceneTimerTick(ctx, w.owner, w.batch, w.lease)
	if err != nil {
		// 单轮失败不致命：下一轮会重试。租约到期后这些行会重新变为可领。
		logrus.WithError(err).Warn("scene timer tick failed")
		return
	}
	if result.Claimed == 0 && result.Triggered == 0 && result.Failed == 0 {
		return
	}
	logrus.Infof("scene timer tick: claimed=%d triggered=%d failed=%d stalled=%d unparsable=%d",
		result.Claimed, result.Triggered, result.Failed, result.Stalled, result.Unparsable)
}

// WithSceneTimerWorker 注册场景定时触发 worker。
func WithSceneTimerWorker() Option {
	return func(application *Application) error {
		application.RegisterService(NewSceneTimerWorker())
		return nil
	}
}
