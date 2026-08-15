package uplink

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"
)

func (f *StatusUplink) notifyClients(device *model.Device, status int16) {
	defer func() {
		if r := recover(); r != nil {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"panic":     r,
			}).Error("notifyClients goroutine panic")
		}
	}()

	deviceName := device.DeviceNumber
	if device.Name != nil {
		deviceName = *device.Name
	}

	messageData := map[string]interface{}{
		"device_id":   device.DeviceNumber,
		"device_name": deviceName,
		"is_online":   status == 1,
	}

	jsonBytes, err := json.Marshal(messageData)
	if err != nil {
		f.logger.WithError(err).Error("Failed to marshal SSE message")
		return
	}

	sseEvent := global.SSEEvent{
		Type:     "device_online",
		TenantID: device.TenantID,
		Message:  string(jsonBytes),
	}

	if err := global.BroadcastSSEEventToTenant(device.TenantID, sseEvent); err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Debug("SSE device status event publish failed")
	}
}

func (f *StatusUplink) triggerAutomation(device *model.Device, status int16) {
	defer func() {
		if r := recover(); r != nil {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"panic":     r,
			}).Error("triggerAutomation goroutine panic")
		}
	}()

	loginStatus := "OFF-LINE"
	if status == 1 {
		loginStatus = "ON-LINE"
	}

	err := service.GroupApp.Execute(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_STATUS,
		TriggerParam:     []string{},
		TriggerValues: map[string]interface{}{
			"login": loginStatus,
		},
	})

	if err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Warn("Automation execution failed")
	} else {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"status":    loginStatus,
		}).Debug("Automation triggered")
	}
}

func (f *StatusUplink) sendExpectedData(device *model.Device) {
	defer func() {
		if r := recover(); r != nil {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"panic":     r,
			}).Error("sendExpectedData goroutine panic")
		}
	}()

	time.Sleep(3 * time.Second)

	err := service.GroupApp.ExpectedData.Send(context.Background(), device.ID)
	if err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to send expected data")
	} else {
		f.logger.WithField("device_id", device.ID).Debug("Expected data sent")
	}
}

func (f *StatusUplink) publishToRedis(device *model.Device, status int16, metadata map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"panic":     r,
			}).Error("publishToRedis goroutine panic")
		}
	}()

	deviceName := device.DeviceNumber
	if device.Name != nil {
		deviceName = *device.Name
	}

	source := statusMetadataSource(metadata)

	messageData := map[string]interface{}{
		"is_online": int(status),
	}

	jsonBytes, err := json.Marshal(messageData)
	if err != nil {
		f.logger.WithError(err).Error("Failed to marshal Redis message")
		return
	}

	channel := fmt.Sprintf("device:%s:status", device.ID)
	if err := global.REDIS.Publish(f.ctx, channel, string(jsonBytes)).Err(); err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"device_id": device.ID,
		}).Debug("Status publish failed")
		return
	}

	f.logger.WithFields(logrus.Fields{
		"device_id":   device.ID,
		"device_name": deviceName,
		"channel":     channel,
		"status":      status,
		"source":      source,
	}).Debug("Status published to Redis")
}
