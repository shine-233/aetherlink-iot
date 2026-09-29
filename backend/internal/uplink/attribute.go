// attribute.go is the attribute kind of the shared uplink pipeline
// (pipeline.go). It only defines how one device's attribute payload is parsed,
// durably persisted, and what side effects follow.
//
// Ordering contract: durable persist -> liveness -> automation. Nothing after
// persist runs when durable admission fails.
package uplink

import (
	"encoding/json"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/processor"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

var attributeKindSpec = kindSpec{
	name:                 "AttributeUplink",
	gatewayMsgType:       "gateway_attribute",
	dataType:             processor.DataTypeAttribute,
	gatewayParseErrorLog: "Failed to unmarshal gateway message",
}

// AttributeUplink consumes attribute uplink messages.
type AttributeUplink struct {
	uplinkBase
	durableStorageInput storage.DurableMessagePersister
	// runAutomation launches attribute automation (async). Replaceable in tests.
	runAutomation func(device *model.Device, triggerParam []string, triggerValues map[string]interface{})
}

// AttributeUplinkConfig configures the attribute uplink worker.
type AttributeUplinkConfig struct {
	Processor           processor.DataProcessor
	DurableStorageInput storage.DurableMessagePersister
	HeartbeatService    *service.HeartbeatService
	Logger              *logrus.Logger
}

// NewAttributeUplink creates the attribute uplink worker.
func NewAttributeUplink(config AttributeUplinkConfig) *AttributeUplink {
	f := &AttributeUplink{
		uplinkBase:          newUplinkBase(attributeKindSpec, config.Processor, config.HeartbeatService, config.Logger),
		durableStorageInput: config.DurableStorageInput,
	}
	f.runAutomation = f.executeAttributeAutomation
	return f
}

// Start launches the consume loop.
func (f *AttributeUplink) Start(messageChan <-chan *DeviceMessage) {
	f.run(messageChan, f.processMessage)
}

func (f *AttributeUplink) processMessage(msg *DeviceMessage) {
	processUplinkMessage[map[string]interface{}](&f.uplinkBase, f, msg)
}

// parseDirect implements kindHandler.
func (f *AttributeUplink) parseDirect(device *model.Device, payload []byte) map[string]interface{} {
	return f.decodeAttributeDataMap(device, payload)
}

// parseGateway implements kindHandler.
func (f *AttributeUplink) parseGateway(payload []byte) (*gatewayNode[map[string]interface{}], error) {
	var msg model.GatewayPublish
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return gatewayPublishNode(&msg), nil
}

// handleDevice implements kindHandler: one device's attribute data.
func (f *AttributeUplink) handleDevice(device *model.Device, dataMap map[string]interface{}, originalMsg *DeviceMessage) {
	points, triggerParam, triggerValues := buildAttributeDataPoints(dataMap)

	runAfterDurableAttributeEventPersist(
		func() bool {
			return f.persistAttributeStorage(device, points, originalMsg)
		},
		func() {
			f.liveness.touch(device)
		},
		func() {
			if f.runAutomation != nil {
				f.runAutomation(device, triggerParam, triggerValues)
			}
		},
	)
}

func (f *AttributeUplink) decodeAttributeDataMap(device *model.Device, payload []byte) map[string]interface{} {
	var dataMap map[string]interface{}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"payload":   string(payload),
			"error":     err,
		}).Warn("attribute payload is not a valid JSON object, wrapping as {\"_raw\": ...}")

		dataMap = map[string]interface{}{
			"_raw": parseRawJSONValue(payload),
		}
	}
	return dataMap
}

// buildAttributeDataPoints converts attributes into storage points and trigger
// inputs. Keys are sorted so the output is deterministic.
func buildAttributeDataPoints(dataMap map[string]interface{}) ([]storage.AttributeDataPoint, []string, map[string]interface{}) {
	keys := sortedKeys(dataMap)
	points := make([]storage.AttributeDataPoint, 0, len(keys))
	triggerParam := make([]string, 0, len(keys))
	triggerValues := make(map[string]interface{}, len(keys))

	for _, key := range keys {
		value := dataMap[key]
		points = append(points, storage.AttributeDataPoint{Key: key, Value: value})
		triggerParam = append(triggerParam, key)
		triggerValues[key] = value
	}
	return points, triggerParam, triggerValues
}

func (f *AttributeUplink) persistAttributeStorage(device *model.Device, points []storage.AttributeDataPoint, originalMsg *DeviceMessage) bool {
	return persistDurableAttributeEvent(f.context(), f.durableStorageInput, &storage.Message{
		SourceMessageID: resolveStorageSourceID(originalMsg),
		DeviceID:        device.ID,
		TenantID:        device.TenantID,
		DataType:        storage.DataTypeAttribute,
		Timestamp:       resolveStorageTimestamp(originalMsg),
		Data:            points,
	}, f.log())
}

func (f *AttributeUplink) executeAttributeAutomation(device *model.Device, triggerParam []string, triggerValues map[string]interface{}) {
	// Dispatch 非阻塞入队到按设备分片的自动化工作池；同步调用才能保住同设备的触发顺序。
	err := service.GroupApp.Dispatch(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_ATTR,
		TriggerParam:     triggerParam,
		TriggerValues:    triggerValues,
	})
	if err != nil {
		f.log().WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Warn("Automation dispatch failed")
	}
}

// parseRawJSONValue returns payload as a JSON value when it is valid JSON
// (e.g. a bare number or array), otherwise as a string.
func parseRawJSONValue(payload []byte) interface{} {
	var rawValue interface{}
	if err := json.Unmarshal(payload, &rawValue); err != nil {
		return string(payload)
	}
	return rawValue
}
