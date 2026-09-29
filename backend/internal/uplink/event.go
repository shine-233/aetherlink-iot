// event.go is the event kind of the shared uplink pipeline (pipeline.go). It
// only defines how one device's event is parsed, durably persisted, and which
// side effects (RDI alarm/unbind, automation, OTA progress) follow.
//
// Ordering contract: marshal params -> durable persist -> liveness -> side
// effects. Nothing after persist runs when durable admission fails.
package uplink

import (
	"encoding/json"
	"fmt"

	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/processor"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

var eventKindSpec = kindSpec{
	name:                 "EventUplink",
	gatewayMsgType:       "gateway_event",
	dataType:             processor.DataTypeEvent,
	gatewayParseErrorLog: "Failed to unmarshal gateway event message",
}

// EventUplink consumes event uplink messages.
type EventUplink struct {
	uplinkBase
	durableStorageInput storage.DurableMessagePersister
	// launchSideEffects runs the post-persist event effects. Replaceable in tests.
	launchSideEffects func(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte)
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
	f := &EventUplink{
		uplinkBase:          newUplinkBase(eventKindSpec, config.Processor, config.HeartbeatService, config.Logger),
		durableStorageInput: config.DurableStorageInput,
	}
	f.launchSideEffects = f.launchEventSideEffects
	return f
}

// Start launches the consume loop.
func (f *EventUplink) Start(messageChan <-chan *DeviceMessage) {
	f.run(messageChan, f.processMessage)
}

func (f *EventUplink) processMessage(msg *DeviceMessage) {
	processUplinkMessage[model.EventInfo](&f.uplinkBase, f, msg)
}

// parseDirect implements kindHandler.
func (f *EventUplink) parseDirect(device *model.Device, payload []byte) model.EventInfo {
	return f.parseDirectEventInfo(device, payload)
}

// parseGateway implements kindHandler.
func (f *EventUplink) parseGateway(payload []byte) (*gatewayNode[model.EventInfo], error) {
	var msg model.GatewayCommandPulish
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return gatewayEventNode(&msg), nil
}

// handleDevice implements kindHandler: one device's event.
func (f *EventUplink) handleDevice(device *model.Device, eventInfo model.EventInfo, originalMsg *DeviceMessage) {
	info := &eventInfo
	paramsJSON, ok := f.marshalEventParams(device, info)
	if !ok {
		return
	}

	runAfterDurableAttributeEventPersist(
		func() bool {
			return f.persistEventStorage(device, info, paramsJSON, originalMsg)
		},
		func() {
			f.liveness.touch(device)
		},
		func() {
			if f.launchSideEffects != nil {
				f.launchSideEffects(device, info, paramsJSON)
			}
		},
	)
}

func (f *EventUplink) parseDirectEventInfo(device *model.Device, payload []byte) model.EventInfo {
	var eventInfo model.EventInfo
	if err := json.Unmarshal(payload, &eventInfo); err != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"payload":   string(payload),
			"error":     err,
		}).Warn("event payload is not valid EventInfo, wrapping as raw event")

		return model.EventInfo{
			Method: "_raw",
			Params: map[string]interface{}{
				"value": parseRawJSONValue(payload),
			},
		}
	}
	return eventInfo
}

func (f *EventUplink) marshalEventParams(device *model.Device, eventInfo *model.EventInfo) ([]byte, bool) {
	paramsJSON, err := json.Marshal(eventInfo.Params)
	if err == nil {
		return paramsJSON, true
	}

	diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("failed to marshal event params: %v", err))
	f.log().WithFields(logrus.Fields{
		"device_id": device.ID,
		"error":     err,
	}).Error("Failed to marshal event params")
	return nil, false
}

func (f *EventUplink) persistEventStorage(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte, originalMsg *DeviceMessage) bool {
	return persistDurableAttributeEvent(f.context(), f.durableStorageInput, &storage.Message{
		SourceMessageID: resolveStorageSourceID(originalMsg),
		DeviceID:        device.ID,
		TenantID:        device.TenantID,
		DataType:        storage.DataTypeEvent,
		Timestamp:       resolveStorageTimestamp(originalMsg),
		Data: storage.EventData{
			Identify: eventInfo.Method,
			Data:     paramsJSON,
		},
	}, f.log())
}

func (f *EventUplink) launchEventSideEffects(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte) {
	go f.notifyRDIAlarmEvent(device, eventInfo)
	f.executeEventAutomation(device, eventInfo, paramsJSON)
	go f.handleRDIPhysicalUnbindEvent(device, eventInfo)

	if eventInfo.Method == "ota_progress" {
		go f.recordOTAProgress(device, eventInfo)
	}
}

func (f *EventUplink) notifyRDIAlarmEvent(device *model.Device, eventInfo *model.EventInfo) {
	defer f.recoverEventSideEffect(device.ID, "NotifyAlarmEvent")

	if err := service.GroupApp.RDI.NotifyAlarmEvent(device, eventInfo); err != nil {
		f.log().WithFields(logrus.Fields{
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
		f.log().WithFields(logrus.Fields{
			"device_id": deviceID,
			"method":    method,
			"error":     err,
		}).Warn("RDI physical unbind event failed")
	}
}

func (f *EventUplink) executeEventAutomation(device *model.Device, eventInfo *model.EventInfo, paramsJSON []byte) {
	defer f.recoverEventSideEffect(device.ID, "Automation execute")

	err := service.GroupApp.Dispatch(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_EVT,
		TriggerParam:     []string{eventInfo.Method},
		TriggerValues: map[string]interface{}{
			eventInfo.Method: string(paramsJSON),
		},
	})
	if err != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Warn("Automation dispatch failed")
	}
}

func (f *EventUplink) recordOTAProgress(device *model.Device, eventInfo *model.EventInfo) {
	defer f.recoverEventSideEffect(device.ID, "RecordOTAProgress")

	if err := service.GroupApp.OTA.RecordOTAProgress(device.ID, eventInfo.Params); err != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"method":    eventInfo.Method,
			"error":     err,
		}).Warn("Record OTA progress failed")
	}
}

func (f *EventUplink) recoverEventSideEffect(deviceID string, name string) {
	if r := recover(); r != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": deviceID,
			"panic":     r,
		}).Error(name + " goroutine panic")
	}
}
