// 文件用途：托管 SW3 MQTT 会话撤销 outbox 重试与 broker ACK 消费生命周期。
// 核心逻辑：启动时先确认 ACK 订阅，再立即及定时 claim 到期事件，并在同一循环持久化 ACK。
// 关键注意事项：停止时必须同时取消 drain、关闭 PubSub，并在旧 goroutine 退出前阻止重复启动。
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	defaultMQTTSessionRevocationWorkerInterval        = 10 * time.Second
	defaultMQTTSessionRevocationWorkerLimit           = 20
	defaultMQTTSessionRevocationWorkerShutdownTimeout = 3 * time.Second
	mqttSessionRevocationAckChannel                   = "aetherlink:mqtt:device-session:terminate:ack"
	mqttSessionRevocationAckSubscribeTimeout          = 3 * time.Second
)

// MQTTSessionRevocationOutboxWorker owns one cancellable polling loop for
// durable MQTT session-revocation delivery.
type MQTTSessionRevocationOutboxWorker struct {
	interval        time.Duration
	limit           int
	shutdownTimeout time.Duration

	cancel context.CancelFunc
	done   chan struct{}
	ackSub *redis.PubSub
	mu     sync.Mutex
}

// NewMQTTSessionRevocationOutboxWorker creates a worker from the current
// configuration snapshot and applies bounded defaults for missing values.
func NewMQTTSessionRevocationOutboxWorker() *MQTTSessionRevocationOutboxWorker {
	return &MQTTSessionRevocationOutboxWorker{
		interval:        mqttSessionRevocationWorkerInterval(),
		limit:           mqttSessionRevocationWorkerLimit(),
		shutdownTimeout: mqttSessionRevocationWorkerShutdownTimeout(),
	}
}

// Name returns the application lifecycle service identifier.
func (w *MQTTSessionRevocationOutboxWorker) Name() string {
	return "mqtt-session-revocation-outbox-worker"
}

// Start launches one worker loop. Repeated calls remain idempotent, including
// while a previously canceled loop is still finishing its database call.
func (w *MQTTSessionRevocationOutboxWorker) Start() error {
	if !mqttSessionRevocationWorkerEnabled() {
		logrus.Info("mqtt session revocation outbox worker disabled")
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		select {
		case <-w.done:
			if w.ackSub != nil {
				_ = w.ackSub.Close()
			}
			w.cancel = nil
			w.done = nil
			w.ackSub = nil
		default:
			return nil
		}
	}
	backfilled, err := service.PrepareMQTTSessionRevocationOutboxForWorker(context.Background())
	if err != nil {
		return fmt.Errorf("prepare mqtt session revocation outbox: %w", err)
	}
	if backfilled > 0 {
		logrus.WithField("rows", backfilled).Info("mqtt session revocation broker policy backfilled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	ackSub, err := subscribeMQTTSessionRevocationAcks(ctx)
	if err != nil {
		cancel()
		return err
	}
	w.cancel = cancel
	w.done = make(chan struct{})
	w.ackSub = ackSub
	go w.run(ctx, w.done, ackSub.Channel())
	logrus.WithFields(logrus.Fields{
		"interval": w.interval.String(),
		"limit":    w.limit,
	}).Info("mqtt session revocation outbox worker started")
	return nil
}

// Stop cancels the active loop and waits up to the configured shutdown budget.
// A timed-out loop remains registered until a later Stop observes its exit, so
// Start cannot create a second worker over an unfinished drain.
func (w *MQTTSessionRevocationOutboxWorker) Stop() error {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	ackSub := w.ackSub
	w.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	var closeErr error
	if ackSub != nil {
		closeErr = ackSub.Close()
	}
	timeout := w.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultMQTTSessionRevocationWorkerShutdownTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
			w.done = nil
			w.ackSub = nil
		}
		w.mu.Unlock()
		logrus.Info("mqtt session revocation outbox worker stopped")
	case <-timer.C:
		logrus.Warn("mqtt session revocation outbox worker stop timed out; processing leases will be retried")
	}
	return closeErr
}

func (w *MQTTSessionRevocationOutboxWorker) run(
	ctx context.Context,
	done chan<- struct{},
	ackMessages <-chan *redis.Message,
) {
	defer close(done)
	w.drainOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.drainOnce(ctx)
		case message, ok := <-ackMessages:
			if !ok {
				if ctx.Err() == nil {
					logrus.Warn("mqtt session revocation acknowledgement channel closed")
				}
				return
			}
			if message == nil {
				continue
			}
			if err := service.AcknowledgeMQTTSessionRevocation(ctx, message.Payload); err != nil && ctx.Err() == nil {
				logrus.WithError(err).Warn("mqtt session revocation acknowledgement rejected")
			}
		}
	}
}

func subscribeMQTTSessionRevocationAcks(ctx context.Context) (*redis.PubSub, error) {
	if global.REDIS == nil {
		return nil, fmt.Errorf("redis is not initialized for mqtt session revocation acknowledgements")
	}
	pubsub := global.REDIS.Subscribe(ctx, mqttSessionRevocationAckChannel)
	confirmCtx, cancel := context.WithTimeout(ctx, mqttSessionRevocationAckSubscribeTimeout)
	defer cancel()
	confirmation, err := pubsub.Receive(confirmCtx)
	if err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("subscribe mqtt session revocation acknowledgements: %w", err)
	}
	subscription, ok := confirmation.(*redis.Subscription)
	if !ok || subscription.Kind != "subscribe" || subscription.Channel != mqttSessionRevocationAckChannel {
		_ = pubsub.Close()
		return nil, fmt.Errorf("unexpected mqtt session revocation acknowledgement subscription confirmation: %T", confirmation)
	}
	return pubsub, nil
}

func (w *MQTTSessionRevocationOutboxWorker) drainOnce(ctx context.Context) {
	result, err := service.DrainMQTTSessionRevocationOutboxForWorker(ctx, w.limit)
	if err != nil {
		if ctx.Err() == nil {
			logrus.WithError(err).Warn("mqtt session revocation outbox drain failed")
		}
		return
	}
	if result == nil || result.Claimed == 0 {
		return
	}
	logrus.WithFields(logrus.Fields{
		"claimed":   result.Claimed,
		"published": result.Published,
		"retried":   result.Retried,
	}).Info("mqtt session revocation outbox drain completed")
}

func mqttSessionRevocationWorkerEnabled() bool {
	if !viper.IsSet("mqtt_session_revocations.worker.enabled") {
		return true
	}
	return viper.GetBool("mqtt_session_revocations.worker.enabled")
}

func mqttSessionRevocationWorkerInterval() time.Duration {
	if value := viper.GetDuration("mqtt_session_revocations.worker.interval"); value > 0 {
		return value
	}
	return defaultMQTTSessionRevocationWorkerInterval
}

func mqttSessionRevocationWorkerLimit() int {
	limit := viper.GetInt("mqtt_session_revocations.worker.limit")
	if limit < 1 {
		return defaultMQTTSessionRevocationWorkerLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func mqttSessionRevocationWorkerShutdownTimeout() time.Duration {
	if value := viper.GetDuration("mqtt_session_revocations.worker.shutdown_timeout"); value > 0 {
		return value
	}
	return defaultMQTTSessionRevocationWorkerShutdownTimeout
}

// WithMQTTSessionRevocationOutboxWorker registers the durable revocation
// retry worker with the application lifecycle.
func WithMQTTSessionRevocationOutboxWorker() Option {
	return func(app *Application) error {
		app.RegisterService(NewMQTTSessionRevocationOutboxWorker())
		return nil
	}
}
