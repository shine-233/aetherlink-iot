// attribute.go processes device attribute uplink payloads.
//
// It stores reported attribute state and coordinates side effects used by
// dashboards, device details, and automation logic.
// 文件用途：承载上行消息模块的 attribute 处理逻辑。
// 核心逻辑：从 MQTT 或内部总线接收遥测、属性、事件、状态和响应消息并分发到处理、存储或通知链路，主要围绕 type AttributeUplink、type AttributeUplinkConfig、func NewAttributeUplink、func (f *AttributeUplink) Start 等声明展开。
// 关键注意事项：上行链路包含 goroutine、缓存和外部服务调用，修改需关注并发关闭和消息类型兼容。
// 重构建议：后续可拆分通用解析、自动化触发和副作用发送逻辑，提升可测试性。

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

// AttributeUplink 负责消费设备属性上行消息并刷新设备状态。
type AttributeUplink struct {
	// 核心依赖由构造函数注入，便于测试和替换。
	processor           processor.DataProcessor
	durableStorageInput storage.DurableMessagePersister
	heartbeatService    *service.HeartbeatService
	logger              *logrus.Logger

	// 上下文用于控制属性上行消费协程。
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// AttributeUplinkConfig 定义属性上行处理器依赖。
type AttributeUplinkConfig struct {
	Processor           processor.DataProcessor
	DurableStorageInput storage.DurableMessagePersister
	HeartbeatService    *service.HeartbeatService
	Logger              *logrus.Logger
}

// NewAttributeUplink 创建属性上行处理器。
func NewAttributeUplink(config AttributeUplinkConfig) *AttributeUplink {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}

	return &AttributeUplink{
		processor:           config.Processor,
		durableStorageInput: config.DurableStorageInput,
		heartbeatService:    config.HeartbeatService,
		logger:              config.Logger,
		ctx:                 ctx,
		cancel:              cancel,
		done:                make(chan struct{}),
	}
}

// Start 启动属性上行消费循环。
func (f *AttributeUplink) Start(messageChan <-chan *DeviceMessage) {
	f.logger.Info("AttributeUplink started")

	go func() {
		defer close(f.done)
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					f.logger.Info("AttributeUplink message channel closed")
					return
				}
				f.processMessage(msg)

			case <-f.ctx.Done():
				f.logger.Info("AttributeUplink stopped")
				return
			}
		}
	}()
}

// Stop 停止属性上行消费循环。
func (f *AttributeUplink) Stop() {
	f.cancel()
}

func (f *AttributeUplink) Done() <-chan struct{} {
	return f.done
}

// processMessage 处理单条属性上行消息。
func (f *AttributeUplink) processMessage(msg *DeviceMessage) {
	device, ok := f.resolveMessageDevice(msg)
	if !ok {
		return
	}

	processedPayload, ok := f.decodeAttributePayload(device, msg)
	if !ok {
		return
	}

	f.dispatchAttributePayload(device, processedPayload, msg)
}

func (f *AttributeUplink) resolveMessageDevice(msg *DeviceMessage) (*model.Device, bool) {
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

func (f *AttributeUplink) decodeAttributePayload(device *model.Device, msg *DeviceMessage) ([]byte, bool) {
	processedPayload := msg.Payload
	if device.DeviceConfigID == nil || *device.DeviceConfigID == "" {
		return processedPayload, true
	}

	output, err := f.processor.Decode(f.ctx, &processor.DecodeInput{
		DeviceConfigID: *device.DeviceConfigID,
		Type:           processor.DataTypeAttribute,
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

func (f *AttributeUplink) dispatchAttributePayload(device *model.Device, payload []byte, msg *DeviceMessage) {
	if msg.Type == "gateway_attribute" {
		f.processGatewayMessage(device, payload, msg)
		return
	}

	f.processDirectDeviceMessage(device, payload, msg)
}

// processGatewayMessage 处理网关上报的子设备属性消息。
func (f *AttributeUplink) processGatewayMessage(device *model.Device, payload []byte, originalMsg *DeviceMessage) {
	var gatewayMsg model.GatewayPublish
	if err := json.Unmarshal(payload, &gatewayMsg); err != nil {
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("gateway message json parse failed: %v", err))
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Failed to unmarshal gateway message")
		return
	}

	// Attributes share the same payload path as telemetry publishing.
	if gatewayMsg.GatewayData != nil {
		gatewayData, _ := json.Marshal(gatewayMsg.GatewayData)
		f.processDirectDeviceMessage(device, gatewayData, originalMsg)
	}

	// 处理网关直连子设备属性数据。
	if gatewayMsg.SubDeviceData != nil {
		f.processSubDevices(device.ID, *gatewayMsg.SubDeviceData, originalMsg)
	}

	// 处理网关下挂子网关属性数据。
	if gatewayMsg.SubGatewayData != nil {
		f.processSubGateways(device.ID, *gatewayMsg.SubGatewayData, originalMsg, 1)
	}
}

// processSubDevices 处理子设备属性数据，subDeviceData 的 key 为子设备地址。
func (f *AttributeUplink) processSubDevices(parentID string, subDeviceData map[string]map[string]interface{}, originalMsg *DeviceMessage) {
	if len(subDeviceData) == 0 {
		return
	}

	// 对子设备地址排序，保证批量处理顺序稳定。
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

	// Forward each sub-device attribute payload through the same parser path.
	for addr, data := range subDeviceData {
		subDevice, ok := subDevices[addr]
		if !ok {
			f.logger.WithFields(logrus.Fields{
				"parent_id":   parentID,
				"device_addr": addr,
			}).Warn("Sub device not found")
			continue
		}

		subDeviceData, _ := json.Marshal(data)
		f.processDirectDeviceMessage(subDevice, subDeviceData, originalMsg)
	}
}

// processSubGateways 递归处理子网关及其下挂设备属性数据。
func (f *AttributeUplink) processSubGateways(parentID string, subGatewayData map[string]*model.GatewayPublish, originalMsg *DeviceMessage, depth int) {
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

	// 查询当前父设备下的子网关。
	subGateways, err := dal.GetDeviceBySubDeviceAddress(subGatewayAddrs, parentID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub gateways")
		return
	}

	// 逐个处理子网关载荷。
	for addr, gatewayMsg := range subGatewayData {
		subGateway, ok := subGateways[addr]
		if !ok {
			f.logger.WithFields(logrus.Fields{
				"parent_id":    parentID,
				"gateway_addr": addr,
			}).Warn("Sub gateway not found")
			continue
		}

		// 处理子网关自身属性数据。
		if gatewayMsg.GatewayData != nil {
			gatewayData, _ := json.Marshal(gatewayMsg.GatewayData)
			f.processDirectDeviceMessage(subGateway, gatewayData, originalMsg)
		}

		// 处理子网关直连子设备属性数据。
		if gatewayMsg.SubDeviceData != nil {
			f.processSubDevices(subGateway.ID, *gatewayMsg.SubDeviceData, originalMsg)
		}

		// 递归处理更深层子网关。
		if gatewayMsg.SubGatewayData != nil {
			f.processSubGateways(subGateway.ID, *gatewayMsg.SubGatewayData, originalMsg, depth+1)
		}
	}
}

// processDirectDeviceMessage 处理单个设备的属性数据。
func (f *AttributeUplink) processDirectDeviceMessage(device *model.Device, payload []byte, originalMsg *DeviceMessage) {
	dataMap := f.decodeAttributeDataMap(device, payload)
	points, triggerParam, triggerValues := buildAttributeDataPoints(dataMap)

	runAfterDurableAttributeEventPersist(
		func() bool {
			return f.persistAttributeStorage(device, points, originalMsg)
		},
		func() {
			f.refreshHeartbeat(device)
		},
		func() {
			f.executeAttributeAutomation(device, triggerParam, triggerValues)
		},
	)
}

func (f *AttributeUplink) decodeAttributeDataMap(device *model.Device, payload []byte) map[string]interface{} {
	var dataMap map[string]interface{}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"payload":   string(payload),
			"error":     err,
		}).Warn("attribute payload is not a valid JSON object, wrapping as {\"_raw\": ...}")

		var rawValue interface{}
		if jsonErr := json.Unmarshal(payload, &rawValue); jsonErr != nil {
			rawValue = string(payload)
		}

		dataMap = map[string]interface{}{
			"_raw": rawValue,
		}
	}

	return dataMap
}

func buildAttributeDataPoints(dataMap map[string]interface{}) ([]storage.AttributeDataPoint, []string, map[string]interface{}) {
	var points []storage.AttributeDataPoint
	var triggerParam []string
	triggerValues := make(map[string]interface{})

	for key, value := range dataMap {
		points = append(points, storage.AttributeDataPoint{
			Key:   key,
			Value: value,
		})

		triggerParam = append(triggerParam, key)
		triggerValues[key] = value
	}

	return points, triggerParam, triggerValues
}

func (f *AttributeUplink) persistAttributeStorage(device *model.Device, points []storage.AttributeDataPoint, originalMsg *DeviceMessage) bool {
	return persistDurableAttributeEvent(f.ctx, f.durableStorageInput, &storage.Message{
		SourceMessageID: resolveStorageSourceID(originalMsg),
		DeviceID:        device.ID,
		TenantID:        device.TenantID,
		DataType:        storage.DataTypeAttribute,
		Timestamp:       resolveStorageTimestamp(originalMsg),
		Data:            points,
	}, f.logger)
}

func (f *AttributeUplink) executeAttributeAutomation(device *model.Device, triggerParam []string, triggerValues map[string]interface{}) {
	go func() {
		err := service.GroupApp.Execute(device, service.AutomateFromExt{
			TriggerParamType: model.TRIGGER_PARAM_TYPE_ATTR,
			TriggerParam:     triggerParam,
			TriggerValues:    triggerValues,
		})
		if err != nil {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"error":     err,
			}).Error("Automation execute failed")
		}
	}()
}

// refreshHeartbeat 根据属性上行刷新设备心跳和在线状态。
func (f *AttributeUplink) refreshHeartbeat(device *model.Device) {
	// 未启用心跳服务时跳过在线状态刷新。
	if f.heartbeatService == nil {
		return
	}

	// 获取设备心跳配置，缺失配置时不改变状态。
	config, err := f.heartbeatService.GetConfig(device)
	if err != nil {
		f.logger.WithError(err).WithField("device_id", device.ID).Debug("Failed to get heartbeat config")
		return
	}

	// 未配置心跳规则时直接返回。
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

// notifyDeviceOnline 推送设备上线通知，并刷新前端期望数据。
func (f *AttributeUplink) notifyDeviceOnline(device *model.Device) {
	notifyDeviceOnlineAndExpectedData(f.logger, device)
}
