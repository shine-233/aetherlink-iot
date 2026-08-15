// 文件用途：承载上行消息模块的 device online 处理逻辑。
// 核心逻辑：从 MQTT 或内部总线接收遥测、属性、事件、状态和响应消息并分发到处理、存储或通知链路，主要围绕 const expectedDataOnlineDelay、func notifyDeviceOnlineAndExpectedData、func broadcastDeviceOnline、func triggerOnlineAutomation 等声明展开。
// 关键注意事项：上行链路包含 goroutine、缓存和外部服务调用，修改需关注并发关闭和消息类型兼容。
// 重构建议：后续可拆分通用解析、自动化触发和副作用发送逻辑，提升可测试性。

package uplink

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

const expectedDataOnlineDelay = 3 * time.Second

// notifyDeviceOnlineAndExpectedData keeps the three data uplinks aligned:
// telemetry, attribute, and event messages all imply that the device is online,
// so they must emit the same SSE payload, trigger the same status automation,
// and then ask the expected-data service to send any pending downlink data.
func notifyDeviceOnlineAndExpectedData(logger *logrus.Logger, device *model.Device) {
	defer func() {
		if r := recover(); r != nil {
			logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"panic":     r,
			}).Error("notifyDeviceOnline goroutine panic")
		}
	}()

	broadcastDeviceOnline(logger, device)
	triggerOnlineAutomation(logger, device)
	sendExpectedDataAfterOnlineDelay(logger, device)
}

func onlineDeviceSnapshot(device *model.Device) *model.Device {
	if device == nil {
		return nil
	}
	snapshot := *device
	snapshot.IsOnline = 1
	return &snapshot
}

func broadcastDeviceOnline(logger *logrus.Logger, device *model.Device) {
	deviceName := device.DeviceNumber
	if device.Name != nil {
		deviceName = *device.Name
	}

	messageData := map[string]interface{}{
		"device_id":   device.DeviceNumber,
		"device_name": deviceName,
		"is_online":   true,
	}

	jsonBytes, _ := json.Marshal(messageData)
	sseEvent := global.SSEEvent{
		Type:     "device_online",
		TenantID: device.TenantID,
		Message:  string(jsonBytes),
	}
	if err := global.BroadcastSSEEventToTenant(device.TenantID, sseEvent); err != nil {
		logger.WithError(err).WithField("device_id", device.ID).Debug("SSE device online event publish failed")
	}
}

func triggerOnlineAutomation(logger *logrus.Logger, device *model.Device) {
	err := service.GroupApp.Execute(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_STATUS,
		TriggerParam:     []string{},
		TriggerValues: map[string]interface{}{
			"login": "ON-LINE",
		},
	})
	if err != nil {
		logger.WithError(err).WithField("device_id", device.ID).Warn("Automation execution failed")
	}
}

func sendExpectedDataAfterOnlineDelay(logger *logrus.Logger, device *model.Device) {
	time.Sleep(expectedDataOnlineDelay)
	err := service.GroupApp.ExpectedData.Send(context.Background(), device.ID)
	if err != nil {
		logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to send expected data")
	}
}
