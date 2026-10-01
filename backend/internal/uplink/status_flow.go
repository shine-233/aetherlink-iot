package uplink

import (
	"fmt"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

type statusMessageContext struct {
	message         *DeviceMessage
	device          *model.Device
	heartbeatConfig *service.HeartbeatConfig
	status          int16
	source          string
}

func (f *StatusUplink) processMessage(msg *DeviceMessage) {
	ctx, ok := f.buildStatusMessageContext(msg)
	if !ok {
		return
	}

	if !f.allowHeartbeatManagedStatus(ctx) {
		return
	}

	f.refreshStatusOnlineTimeout(ctx)

	if !f.persistStatusChange(ctx) {
		return
	}

	f.dispatchStatusChangedSideEffects(ctx)
}

func (f *StatusUplink) parseStatus(payload []byte) (int16, error) {
	str := string(payload)
	switch str {
	case "0":
		return 0, nil
	case "1":
		return 1, nil
	default:
		return 0, fmt.Errorf("invalid status value: %s (expected 0 or 1)", str)
	}
}

func statusMetadataSource(metadata map[string]interface{}) string {
	source, _ := metadata["source"].(string)
	if source == "" {
		return "unknown"
	}

	return source
}

func isHeartbeatExpiredStatusSource(source string) bool {
	return source == "heartbeat_expired"
}

func (f *StatusUplink) buildStatusMessageContext(msg *DeviceMessage) (*statusMessageContext, bool) {
	status, err := f.parseStatus(msg.Payload)
	if err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"device_id": msg.DeviceID,
			"payload":   string(msg.Payload),
		}).Error("Invalid status value")
		return nil, false
	}

	f.logger.WithFields(logrus.Fields{
		"device_id": msg.DeviceID,
		"status":    status,
	}).Debug("Parsed device online status")

	device, err := initialize.GetDeviceCacheById(msg.DeviceID)
	if err != nil {
		f.logger.WithError(err).WithField("device_id", msg.DeviceID).Error("Device not found")
		return nil, false
	}

	var config *service.HeartbeatConfig
	if f.heartbeatService != nil {
		config, err = f.heartbeatService.GetConfig(device)
		if err != nil {
			f.logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to get heartbeat config")
		}
	}

	return &statusMessageContext{
		message:         msg,
		device:          device,
		heartbeatConfig: config,
		status:          status,
		source:          statusMetadataSource(msg.Metadata),
	}, true
}

func (f *StatusUplink) allowHeartbeatManagedStatus(ctx *statusMessageContext) bool {
	config := ctx.heartbeatConfig
	if config == nil || config.Heartbeat <= 0 {
		return true
	}

	f.logger.WithFields(logrus.Fields{
		"device_id": ctx.device.ID,
		"heartbeat": config.Heartbeat,
		"source":    ctx.source,
		"status":    ctx.status,
	}).Debug("Device in heartbeat mode")

	if isHeartbeatExpiredStatusSource(ctx.source) {
		return true
	}

	f.logger.Debug("Ignoring status message from device (heartbeat mode)")
	return false
}

func (f *StatusUplink) refreshStatusOnlineTimeout(ctx *statusMessageContext) {
	config := ctx.heartbeatConfig
	if config == nil || config.OnlineTimeout <= 0 || ctx.status != 1 || f.heartbeatService == nil {
		return
	}

	if err := f.heartbeatService.SetTimeout(ctx.device.ID, config.OnlineTimeout); err != nil {
		f.logger.WithError(err).Error("Failed to set timeout key")
	}
}

func (f *StatusUplink) persistStatusChange(ctx *statusMessageContext) bool {
	statusChanged, err := dal.UpdateDeviceStatus(ctx.device.ID, ctx.status)
	if err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"device_id": ctx.device.ID,
			"status":    ctx.status,
		}).Error("Failed to update device status")
		return false
	}

	if !statusChanged {
		f.logger.WithFields(logrus.Fields{
			"device_id": ctx.device.ID,
			"status":    ctx.status,
			"source":    ctx.source,
		}).Debug("Device status unchanged, skipping notification")
		return false
	}

	f.logger.WithFields(logrus.Fields{
		"device_id": ctx.device.ID,
		"status":    ctx.status,
		"source":    ctx.source,
	}).Debug("Device status updated")

	return true
}

func (f *StatusUplink) dispatchStatusChangedSideEffects(ctx *statusMessageContext) {
	initialize.DelDeviceCache(ctx.device.ID)

	// Both fan-outs are fire-and-forget network writes with their own panic
	// guards; one goroutine per status change instead of two.
	device, status, metadata := ctx.device, ctx.status, ctx.message.Metadata
	go func() {
		f.publishToRedis(device, status, metadata)
		f.notifyClients(device, status)
	}()
	f.triggerAutomation(ctx.device, ctx.status)

	if ctx.status == 1 {
		f.scheduleOnlineDeliveries(ctx.device, statusOnlineDeliveryDelay, f.runOnlineDeliveries)
		f.triggerRuleChainsOnline(ctx.device)
	}
}

// statusOnlineDeliveryDelay gives a device that just came online time to finish
// its session (subscribe to command topics) before we push to it.
const statusOnlineDeliveryDelay = 3 * time.Second

// scheduleOnlineDeliveries replaces two goroutines that each slept 3s with one
// timer: after the delay, expected data is sent first and then pending shadow
// messages, in that order. Deliveries are skipped once the uplink is stopped;
// both queues are persistent and are retried on the next online transition.
func (f *StatusUplink) scheduleOnlineDeliveries(device *model.Device, delay time.Duration, deliver func(*model.Device)) *time.Timer {
	return time.AfterFunc(delay, func() {
		if f.ctx != nil && f.ctx.Err() != nil {
			return
		}
		deliver(device)
	})
}

func (f *StatusUplink) runOnlineDeliveries(device *model.Device) {
	f.sendExpectedData(device)
	f.sendPendingShadowMessages(device)
}

// triggerRuleChainsOnline 设备上线触发规则链（ROADMAP B2）：异步、错误不阻断状态流。
func (f *StatusUplink) triggerRuleChainsOnline(device *model.Device) {
	if device == nil {
		return
	}
	deviceCopy := *device
	go service.GroupApp.RuleChain.OnDeviceOnline(deviceCopy)
}
