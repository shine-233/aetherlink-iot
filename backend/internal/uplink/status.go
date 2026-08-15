package uplink

import (
	"context"

	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/service"
)

// StatusUplink consumes device online/offline status messages from the bus.
// The entry file owns lifecycle only; decision and fan-out details live in
// status_flow.go and status_notifications.go.
type StatusUplink struct {
	heartbeatService *service.HeartbeatService
	logger           *logrus.Logger

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

type StatusUplinkConfig struct {
	HeartbeatService *service.HeartbeatService
	Logger           *logrus.Logger
}

func NewStatusUplink(config StatusUplinkConfig) *StatusUplink {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}

	return &StatusUplink{
		heartbeatService: config.HeartbeatService,
		logger:           config.Logger,
		ctx:              ctx,
		cancel:           cancel,
		done:             make(chan struct{}),
	}
}

func (f *StatusUplink) Start(input <-chan *DeviceMessage) error {
	f.logger.Info("StatusUplink starting...")

	go func() {
		defer close(f.done)
		f.logger.Info("StatusUplink message loop started")
		for {
			select {
			case <-f.ctx.Done():
				f.logger.Info("StatusUplink stopped")
				return
			case msg, ok := <-input:
				if !ok {
					f.logger.Info("StatusUplink message channel closed")
					return
				}
				if msg == nil {
					f.logger.Warn("Received nil message, skipping")
					continue
				}
				f.logger.WithField("device_id", msg.DeviceID).Debug("StatusUplink received message from channel")
				f.processMessage(msg)
			}
		}
	}()

	f.logger.Info("StatusUplink started successfully")
	return nil
}

func (f *StatusUplink) Stop() error {
	f.cancel()
	return nil
}

func (f *StatusUplink) Done() <-chan struct{} {
	return f.done
}
