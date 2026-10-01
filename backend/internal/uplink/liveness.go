// liveness.go is the shared device liveness component: any valid business
// uplink (telemetry, attribute, event) marks the device online, notifies the
// frontend once on an offline->online transition, and refreshes the heartbeat
// key when the device has a heartbeat rule.
package uplink

import (
	"reflect"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

// heartbeatRefresher is the subset of *service.HeartbeatService liveness uses.
type heartbeatRefresher interface {
	GetConfig(device *model.Device) (*service.HeartbeatConfig, error)
	RefreshHeartbeat(device *model.Device, config *service.HeartbeatConfig) error
}

type deviceLiveness struct {
	heartbeat heartbeatRefresher
	logger    *logrus.Logger
	// setOnline flips the stored online flag; returns whether it changed.
	setOnline func(deviceID string) (bool, error)
	// notifyOnline runs asynchronously after an offline->online transition.
	notifyOnline func(logger *logrus.Logger, device *model.Device)
	// throttle coalesces heartbeat-key SETs per device (liveness_throttle.go).
	// nil disables throttling.
	throttle *heartbeatThrottle
}

func newDeviceLiveness(heartbeat heartbeatRefresher, logger *logrus.Logger) *deviceLiveness {
	if isNilHeartbeat(heartbeat) {
		heartbeat = nil
	}
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &deviceLiveness{
		heartbeat: heartbeat,
		logger:    logger,
		setOnline: func(deviceID string) (bool, error) {
			return dal.UpdateDeviceStatus(deviceID, 1)
		},
		notifyOnline: notifyDeviceOnlineAndExpectedData,
		throttle:     newHeartbeatThrottle(),
	}
}

func isNilHeartbeat(h heartbeatRefresher) bool {
	if h == nil {
		return true
	}
	v := reflect.ValueOf(h)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// touch records business traffic from device. It is a no-op when the heartbeat
// service is disabled (historical behavior of all three uplinks).
func (l *deviceLiveness) touch(device *model.Device) {
	if l == nil || l.heartbeat == nil || device == nil {
		return
	}

	if device.IsOnline != 1 {
		// An offline device must get a fresh key now, whatever was stamped earlier.
		l.throttle.forget(device.ID)
		statusChanged, err := l.setOnline(device.ID)
		if err != nil {
			l.logger.WithError(err).WithField("device_id", device.ID).Error("Failed to auto online device")
			return
		}
		if statusChanged {
			l.logger.WithField("device_id", device.ID).Info("Device auto online by business message")
			go l.notifyOnline(l.logger, onlineDeviceSnapshot(device))
		}
	}

	config, err := l.heartbeat.GetConfig(device)
	if err != nil {
		l.logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to get heartbeat config")
		return
	}
	if config == nil {
		return
	}
	ttl := heartbeatTTLSeconds(config)
	if l.throttle.fresh(device.ID, ttl) {
		return
	}
	if err := l.heartbeat.RefreshHeartbeat(device, config); err != nil {
		l.throttle.forget(device.ID)
		l.logger.WithError(err).WithField("device_id", device.ID).Error("Failed to refresh heartbeat")
		return
	}
	if ttl > 0 {
		l.throttle.record(device.ID, ttl)
	}
}
