package app

import (
	"context"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	defaultDeadLetterWorkerInterval        = 30 * time.Second
	defaultDeadLetterWorkerLimit           = 20
	defaultDeadLetterWorkerShutdownTimeout = 5 * time.Second
)

type TelemetryDeadLetterWorker struct {
	interval        time.Duration
	limit           int
	shutdownTimeout time.Duration
	drainReady      telemetryDeadLetterDrainer

	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

type telemetryDeadLetterDrainer func(ctx context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error)

func NewTelemetryDeadLetterWorker() *TelemetryDeadLetterWorker {
	return &TelemetryDeadLetterWorker{
		interval:        telemetryDeadLetterWorkerInterval(),
		limit:           telemetryDeadLetterWorkerLimit(),
		shutdownTimeout: telemetryDeadLetterWorkerShutdownTimeout(),
		drainReady:      defaultTelemetryDeadLetterDrainer,
	}
}

func (w *TelemetryDeadLetterWorker) Name() string {
	return "telemetry-dead-letter-worker"
}

func (w *TelemetryDeadLetterWorker) Start() error {
	if !telemetryDeadLetterWorkerEnabled() {
		logrus.Info("telemetry dead-letter worker disabled")
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	done := w.done

	go w.run(ctx, done)
	logrus.WithFields(logrus.Fields{
		"interval": w.interval.String(),
		"limit":    w.limit,
	}).Info("telemetry dead-letter worker started")
	return nil
}

func (w *TelemetryDeadLetterWorker) Stop() error {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()

	if cancel == nil || done == nil {
		return nil
	}

	shutdownTimeout := w.shutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultDeadLetterWorkerShutdownTimeout
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	cancel()
	select {
	case <-done:
		logrus.Info("telemetry dead-letter worker loop stopped")
	case <-shutdownCtx.Done():
		logrus.Warn("telemetry dead-letter worker stop timed out before final drain")
		return nil
	}

	if err := w.drainOnce(shutdownCtx); err != nil {
		if shutdownCtx.Err() != nil {
			logrus.Warn("telemetry dead-letter worker shutdown final drain timed out")
		}
		return nil
	}
	if shutdownCtx.Err() != nil {
		logrus.Warn("telemetry dead-letter worker shutdown final drain timed out")
		return nil
	}
	logrus.Info("telemetry dead-letter worker stopped after final drain")
	return nil
}

func (w *TelemetryDeadLetterWorker) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	_ = w.drainOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.drainOnce(ctx)
		}
	}
}

func (w *TelemetryDeadLetterWorker) drainOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	drainReady := w.drainReady
	if drainReady == nil {
		drainReady = defaultTelemetryDeadLetterDrainer
	}

	result, err := drainReady(ctx, w.limit)
	if err != nil {
		if ctx.Err() == nil {
			logrus.WithError(err).Warn("telemetry dead-letter worker drain failed")
		}
		return err
	}
	if result == nil {
		logrus.Warn("telemetry dead-letter worker drain returned nil result")
		return nil
	}
	if result.Attempted == 0 && result.TotalReady == 0 {
		return nil
	}
	logrus.WithFields(logrus.Fields{
		"total_ready": result.TotalReady,
		"attempted":   result.Attempted,
		"replayed":    result.Replayed,
		"failed":      result.Failed,
	}).Info("telemetry dead-letter worker drain completed")
	return nil
}

func defaultTelemetryDeadLetterDrainer(ctx context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
	return service.GroupApp.TelemetryData.DrainReadyTelemetryDeadLettersForWorker(ctx, limit)
}

func telemetryDeadLetterWorkerEnabled() bool {
	if !viper.IsSet("telemetry_dead_letters.worker.enabled") {
		return true
	}
	return viper.GetBool("telemetry_dead_letters.worker.enabled")
}

func telemetryDeadLetterWorkerInterval() time.Duration {
	if value := viper.GetDuration("telemetry_dead_letters.worker.interval"); value > 0 {
		return value
	}
	return defaultDeadLetterWorkerInterval
}

func telemetryDeadLetterWorkerLimit() int {
	limit := viper.GetInt("telemetry_dead_letters.worker.limit")
	if limit < 1 {
		return defaultDeadLetterWorkerLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func telemetryDeadLetterWorkerShutdownTimeout() time.Duration {
	if value := viper.GetDuration("telemetry_dead_letters.worker.shutdown_timeout"); value > 0 {
		return value
	}
	return defaultDeadLetterWorkerShutdownTimeout
}

func WithTelemetryDeadLetterWorker() Option {
	return func(app *Application) error {
		app.RegisterService(NewTelemetryDeadLetterWorker())
		return nil
	}
}
