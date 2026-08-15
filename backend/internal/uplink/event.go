// event.go processes device event uplink payloads.
//
// It stores event data, feeds automation/event-condition logic, and supports
// diagnostics. Payload-shape changes affect MQTT ingestion, API history, and
// automation triggers.
// 文件用途：承载事件上行消息的解码、入库和业务副作用调度。
// 核心逻辑：从设备元数据解析设备，按直连/网关事件分派，并触发 RDI、自动化和 OTA 进度处理。
// 关键注意事项：事件上行链路包含 goroutine 副作用，修改时要保持失败隔离和网关子设备兼容。
// 重构建议：优先继续拆分网关递归、心跳刷新和事件副作用的可测边界。
package uplink

import (
	"context"
	"encoding/json"
	"fmt"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/processor"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

// EventUplink 负责消费设备事件上行消息。
type EventUplink struct {
	// 核心依赖由构造函数注入，便于测试和替换。
	processor           processor.DataProcessor
	durableStorageInput storage.DurableMessagePersister
	heartbeatService    *service.HeartbeatService
	logger              *logrus.Logger

	// ctx/cancel 控制后台消费协程生命周期。
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// EventUplinkConfig configures the event uplink worker.
type EventUplinkConfig struct {
	Processor           processor.DataProcessor
	DurableStorageInput storage.DurableMessagePersister
	HeartbeatService    *service.HeartbeatService
	Logger              *logrus.Logger
}

// NewEventUplink creates the event uplink worker.
func NewEventUplink(config EventUplinkConfig) *EventUplink {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}

	return &EventUplink{
		processor:           config.Processor,
		durableStorageInput: config.DurableStorageInput,
		heartbeatService:    config.HeartbeatService,
		logger:              config.Logger,
		ctx:                 ctx,
		cancel:              cancel,
		done:                make(chan struct{}),
	}
}

// Start 启动事件上行消费循环。
func (f *EventUplink) Start(messageChan <-chan *DeviceMessage) {
	f.logger.Info("EventUplink started")

	go func() {
		defer close(f.done)
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					f.logger.Info("EventUplink message channel closed")
					return
				}
				f.processMessage(msg)

			case <-f.ctx.Done():
				f.logger.Info("EventUplink stopped")
				return
			}
		}
	}()
}

// Stop 停止事件上行消费循环。
func (f *EventUplink) Stop() {
	f.cancel()
}

func (f *EventUplink) Done() <-chan struct{} {
	return f.done
}

// processMessage 处理单条事件上行消息。
func (f *EventUplink) processMessage(msg *DeviceMessage) {
	device, ok := f.resolveEventDevice(msg)
	if !ok {
		return
	}

	processedPayload, ok := f.decodeEventPayload(device, msg)
	if !ok {
		return
	}

	if msg.Type == "gateway_event" {
		f.processGatewayMessage(device, processedPayload, msg)
		return
	}

	eventInfo := f.parseDirectEventInfo(device, processedPayload)
	f.processDirectDeviceEvent(device, &eventInfo, msg)
}

func (f *EventUplink) resolveEventDevice(msg *DeviceMessage) (*model.Device, bool) {
	deviceIDObj, ok := msg.GetMetadata("device_id")
	if !ok {
		f.logger.Error("Device ID not found in message metadata")
		return nil, false
	}

	deviceID, ok := deviceIDObj.(string)
	if !ok {
		f.logger.Error("Invalid device ID type in metadata")
		return nil, false
	}

	device, err := initialize.GetDeviceCacheById(deviceID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err,
		}).Error("Failed to get device from cache")
		return nil, false
	}

	return device, true
}

func (f *EventUplink) decodeEventPayload(device *model.Device, msg *DeviceMessage) ([]byte, bool) {
	if device.DeviceConfigID == nil || *device.DeviceConfigID == "" {
		return msg.Payload, true
	}

	output, err := f.processor.Decode(f.ctx, &processor.DecodeInput{
		DeviceConfigID: *device.DeviceConfigID,
		Type:           processor.DataTypeEvent,
		RawData:        msg.Payload,
		Timestamp:      msg.Timestamp,
	})
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Processor decode failed, terminate processing")
		return nil, false
	}

	if !output.Success {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     output.Error,
		}).Error("Processor execution failed, terminate processing")
		return nil, false
	}

	return output.Data, true
}

func (f *EventUplink) parseDirectEventInfo(device *model.Device, payload []byte) model.EventInfo {
	var eventInfo model.EventInfo
	if err := json.Unmarshal(payload, &eventInfo); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"payload":   string(payload),
			"error":     err,
		}).Warn("event payload is not valid EventInfo, wrapping as raw event")

		return model.EventInfo{
			Method: "_raw",
			Params: map[string]interface{}{
				"value": parseRawEventValue(payload),
			},
		}
	}
	return eventInfo
}

func parseRawEventValue(payload []byte) interface{} {
	var rawValue interface{}
	if err := json.Unmarshal(payload, &rawValue); err != nil {
		return string(payload)
	}
	return rawValue
}

// processGatewayMessage 处理网关上报的子设备事件消息。
func (f *EventUplink) processGatewayMessage(device *model.Device, payload []byte, originalMsg *DeviceMessage) {
	var gatewayMsg model.GatewayCommandPulish
	if err := json.Unmarshal(payload, &gatewayMsg); err != nil {
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("gateway message json parse failed: %v", err))
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Failed to unmarshal gateway event message")
		return
	}

	// 处理网关自身事件数据。
	if gatewayMsg.GatewayData != nil {
		f.processDirectDeviceEvent(device, gatewayMsg.GatewayData, originalMsg)
	}

	// 处理网关直连子设备事件数据。
	if gatewayMsg.SubDeviceData != nil {
		f.processSubDevices(device.ID, *gatewayMsg.SubDeviceData, originalMsg)
	}

	// 递归处理下级子网关事件数据。
	if gatewayMsg.SubGatewayData != nil {
		f.processSubGateways(device.ID, *gatewayMsg.SubGatewayData, originalMsg, 1)
	}
}

// processSubDevices 处理网关直连子设备事件数据。
func (f *EventUplink) processSubDevices(parentID string, subDeviceData map[string]model.EventInfo, originalMsg *DeviceMessage) {
	if len(subDeviceData) == 0 {
		return
	}

	// 收集子设备地址后批量查询设备。
	var subDeviceAddrs []string
	for addr := range subDeviceData {
		subDeviceAddrs = append(subDeviceAddrs, addr)
	}

	subDevices, err := dal.GetDeviceBySubDeviceAddress(subDeviceAddrs, parentID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub devices")
		return
	}

	// 逐个子设备转成普通直连事件流程。
	for addr, eventData := range subDeviceData {
		subDevice, ok := subDevices[addr]
		if !ok {
			f.logger.WithFields(logrus.Fields{
				"parent_id":   parentID,
				"device_addr": addr,
			}).Warn("Sub device not found")
			continue
		}

		f.processDirectDeviceEvent(subDevice, &eventData, originalMsg)
	}
}

// processSubGateways handles nested gateway event payloads.
func (f *EventUplink) processSubGateways(parentID string, subGatewayData map[string]*model.GatewayCommandPulish, originalMsg *DeviceMessage, depth int) {
	if depth > 5 {
		f.logger.Warn("Maximum gateway depth (5) exceeded")
		return
	}

	if len(subGatewayData) == 0 {
		return
	}

	// 对子设备地址排序，保证批量处理顺序稳定。
	var subGatewayAddrs []string
	for addr := range subGatewayData {
		subGatewayAddrs = append(subGatewayAddrs, addr)
	}

	// 按父网关和地址批量查询子网关。
	subGateways, err := dal.GetDeviceBySubDeviceAddress(subGatewayAddrs, parentID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub gateways")
		return
	}

	// 逐个子网关分派自身事件、子设备事件和下级网关事件。
	for addr, gatewayMsg := range subGatewayData {
		subGateway, ok := subGateways[addr]
		if !ok {
			f.logger.WithFields(logrus.Fields{
				"parent_id":    parentID,
				"gateway_addr": addr,
			}).Warn("Sub gateway not found")
			continue
		}

		// 子网关自身事件数据。
		if gatewayMsg.GatewayData != nil {
			f.processDirectDeviceEvent(subGateway, gatewayMsg.GatewayData, originalMsg)
		}

		// 子网关下挂设备事件数据。
		if gatewayMsg.SubDeviceData != nil {
			f.processSubDevices(subGateway.ID, *gatewayMsg.SubDeviceData, originalMsg)
		}

		// 继续递归处理更深层级的子网关。
		if gatewayMsg.SubGatewayData != nil {
			f.processSubGateways(subGateway.ID, *gatewayMsg.SubGatewayData, originalMsg, depth+1)
		}
	}
}

// processDirectDeviceEvent 处理单个设备的事件数据，并触发存储与业务副作用。
func (f *EventUplink) processDirectDeviceEvent(device *model.Device, eventInfo *model.EventInfo, originalMsg *DeviceMessage) {
	paramsJSON, ok := f.marshalEventParams(device, eventInfo)
	if !ok {
		return
	}

	runAfterDurableAttributeEventPersist(
		func() bool {
			return f.persistEventStorage(device, eventInfo, paramsJSON, originalMsg)
		},
		func() {
			f.refreshHeartbeat(device)
		},
		func() {
			f.launchEventSideEffects(device, eventInfo, paramsJSON)
		},
	)
}

func (f *EventUplink) marshalEventParams(device *model.Device, eventInfo *model.EventInfo) ([]byte, bool) {
	paramsJSON, err := json.Marshal(eventInfo.Params)
	if err == nil {
		return paramsJSON, true
	}

	diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("failed to marshal event params: %v", err))
	f.logger.WithFields(logrus.Fields{
		"device_id": device.ID,
		"error":     err,
	}).Error("Failed to marshal event params")
	return nil, false
}

func (f *EventUplink) persistEventStorage(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte, originalMsg *DeviceMessage) bool {
	return persistDurableAttributeEvent(f.ctx, f.durableStorageInput, &storage.Message{
		SourceMessageID: resolveStorageSourceID(originalMsg),
		DeviceID:        device.ID,
		TenantID:        device.TenantID,
		DataType:        storage.DataTypeEvent,
		Timestamp:       resolveStorageTimestamp(originalMsg),
		Data: storage.EventData{
			Identify: eventInfo.Method,
			Data:     paramsJSON,
		},
	}, f.logger)
}

func (f *EventUplink) launchEventSideEffects(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte) {
	go f.notifyRDIAlarmEvent(device, eventInfo)
	go f.executeEventAutomation(device, eventInfo, paramsJSON)
	go f.handleRDIPhysicalUnbindEvent(device, eventInfo)

	if eventInfo.Method == "ota_progress" {
		go f.recordOTAProgress(device, eventInfo)
	}
}

func (f *EventUplink) notifyRDIAlarmEvent(device *model.Device, eventInfo *model.EventInfo) {
	defer f.recoverEventSideEffect(device.ID, "NotifyAlarmEvent")

	if err := service.GroupApp.RDI.NotifyAlarmEvent(device, eventInfo); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"method":    eventInfo.Method,
			"error":     err,
		}).Warn("RDI alarm email notification failed")
	}
}

func (f *EventUplink) handleRDIPhysicalUnbindEvent(device *model.Device, eventInfo *model.EventInfo) {
	deviceID := ""
	method := ""
	if device != nil {
		deviceID = device.ID
	}
	if eventInfo != nil {
		method = eventInfo.Method
	}
	defer f.recoverEventSideEffect(deviceID, "RDI physical unbind")

	if err := service.GroupApp.RDI.HandlePhysicalUnbindEvent(device, eventInfo); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"method":    method,
			"error":     err,
		}).Warn("RDI physical unbind event failed")
	}
}

func (f *EventUplink) executeEventAutomation(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte) {
	defer f.recoverEventSideEffect(device.ID, "Automation execute")

	err := service.GroupApp.Execute(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_EVT,
		TriggerParam:     []string{eventInfo.Method},
		TriggerValues: map[string]interface{}{
			eventInfo.Method: string(paramsJSON),
		},
	})
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Automation execute failed")
	}
}

func (f *EventUplink) recordOTAProgress(device *model.Device, eventInfo *model.EventInfo) {
	defer f.recoverEventSideEffect(device.ID, "RecordOTAProgress")

	if err := service.GroupApp.OTA.RecordOTAProgress(device.ID, eventInfo.Params); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"method":    eventInfo.Method,
			"error":     err,
		}).Warn("Record OTA progress failed")
	}
}

func (f *EventUplink) recoverEventSideEffect(deviceID string, name string) {
	if r := recover(); r != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"panic":     r,
		}).Error(name + " goroutine panic")
	}
}

// refreshHeartbeat marks event traffic as a heartbeat signal.
func (f *EventUplink) refreshHeartbeat(device *model.Device) {
	if f.heartbeatService == nil {
		return
	}

	config, err := f.heartbeatService.GetConfig(device)
	if err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to get heartbeat config")
		return
	}

	// 未配置心跳时不刷新在线状态。
	if config == nil {
		return
	}

	// 任意有效业务上行都可将设备自动恢复为在线。
	if device.IsOnline != 1 {
		// 状态变化时再推送 SSE，避免重复通知前端。
		statusChanged, err := dal.UpdateDeviceStatus(device.ID, 1)
		if err != nil {
			f.logger.WithError(err).WithField("device_id", device.ID).Error("Failed to auto online device")
			return
		}

		if statusChanged {
			f.logger.WithField("device_id", device.ID).Info("Device auto online by business message")

			// 异步通知前端设备已上线。
			go f.notifyDeviceOnline(onlineDeviceSnapshot(device))
		}
	}

	// 刷新心跳 key，TTL 需要大于在线超时时间。
	if err := f.heartbeatService.RefreshHeartbeat(device, config); err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Error("Failed to refresh heartbeat")
	}
}

// notifyDeviceOnline 通知前端设备已上线，并同步期望数据状态。
func (f *EventUplink) notifyDeviceOnline(device *model.Device) {
	notifyDeviceOnlineAndExpectedData(f.logger, device)
}
