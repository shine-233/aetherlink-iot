// 文件用途：承载设备上行 telemetry 消息处理逻辑。
// 核心逻辑：接收 MQTT 或内部总线消息，解析遥测、属性、事件、状态和响应，并分发到处理、存储、自动化或通知链路。
// 关键注意事项：上行链路包含 goroutine、缓存、数据库和外部服务调用，修改时需关注并发关闭、消息类型兼容和副作用顺序。
// 重构建议：建议拆分通用解析、自动化触发、状态更新和副作用发送逻辑，并补齐消息类型矩阵测试。
package uplink

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/processor"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

const (
	defaultTelemetryWSPublishQueueSize = 4096
	defaultTelemetryWSPublishWorkers   = 4
	// 遥测自动化副作用的最大并发数；与 wsPublish 消费者规模同量级，防止 goroutine 无界。
	defaultTelemetryAutomationMaxConcurrency = 64
)

// TelemetryUplink 负责消费遥测上行消息并写入存储链路。
type TelemetryUplink struct {
	// 核心依赖由构造函数注入，便于测试和替换。
	processor        processor.DataProcessor
	storageInput     storage.MessageEnqueuer // 存储输入通道。
	heartbeatService *service.HeartbeatService
	logger           *logrus.Logger
	wsPublishQueue   chan telemetryWSPublishTask
	wsPublishWorkers int
	wsPublishDropped uint64
	// 自动化副作用并发闸门：防止突发洪峰下每条遥测裸起 goroutine 导致无界增长。
	automationSem     chan struct{}
	automationDropped uint64

	// ctx/cancel 控制后台消费协程生命周期。
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// TelemetryUplinkConfig 定义遥测上行处理器的外部依赖。
type TelemetryUplinkConfig struct {
	Processor          processor.DataProcessor
	StorageInput       storage.MessageEnqueuer // 存储输入通道。
	HeartbeatService   *service.HeartbeatService
	Logger             *logrus.Logger
	WSPublishQueueSize int
	WSPublishWorkers   int
	// AutomationMaxConcurrency 限制同时执行的遥测自动化副作用数量；超限时丢弃并计数。
	AutomationMaxConcurrency int
}

type telemetryWSPublishTask struct {
	deviceID string
	tenantID string
	data     map[string]interface{}
}

// NewTelemetryUplink 创建遥测上行处理器。
func NewTelemetryUplink(config TelemetryUplinkConfig) *TelemetryUplink {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}
	if config.WSPublishQueueSize <= 0 {
		config.WSPublishQueueSize = defaultTelemetryWSPublishQueueSize
	}
	if config.WSPublishWorkers <= 0 {
		config.WSPublishWorkers = defaultTelemetryWSPublishWorkers
	}
	if config.AutomationMaxConcurrency <= 0 {
		config.AutomationMaxConcurrency = defaultTelemetryAutomationMaxConcurrency
	}

	return &TelemetryUplink{
		processor:        config.Processor,
		storageInput:     config.StorageInput,
		heartbeatService: config.HeartbeatService,
		logger:           config.Logger,
		wsPublishQueue:   make(chan telemetryWSPublishTask, config.WSPublishQueueSize),
		wsPublishWorkers: config.WSPublishWorkers,
		automationSem:    make(chan struct{}, config.AutomationMaxConcurrency),
		ctx:              ctx,
		cancel:           cancel,
		done:             make(chan struct{}),
	}
}

// DeviceMessage 表示设备上行消息，包含消息类型、设备标识、载荷和协议上下文。
type DeviceMessage struct {
	Type      string
	DeviceID  string
	TenantID  string
	Timestamp int64
	Payload   []byte
	Metadata  map[string]interface{}
}

// GetMetadata 从消息元数据中读取指定键。
func (m *DeviceMessage) GetMetadata(key string) (interface{}, bool) {
	if m.Metadata == nil {
		return nil, false
	}
	val, ok := m.Metadata[key]
	return val, ok
}

// Start 启动遥测上行消费循环。
func (f *TelemetryUplink) Start(messageChan <-chan *DeviceMessage) {
	f.logger.Info("TelemetryUplink started")
	f.startWebSocketPublishWorkers()

	go func() {
		defer close(f.done)
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					f.logger.Info("TelemetryUplink message channel closed")
					return
				}
				f.processMessage(msg)

			case <-f.ctx.Done():
				f.logger.Info("TelemetryUplink stopped")
				return
			}
		}
	}()
}

func (f *TelemetryUplink) startWebSocketPublishWorkers() {
	for i := 0; i < f.wsPublishWorkers; i++ {
		go func(workerID int) {
			for {
				select {
				case task := <-f.wsPublishQueue:
					f.checkAndPublishToWS(task.deviceID, task.tenantID, task.data)
				case <-f.ctx.Done():
					f.logger.WithField("worker_id", workerID).Debug("Telemetry WebSocket publish worker stopped")
					return
				}
			}
		}(i)
	}
}

// Stop 停止遥测上行消费循环。
func (f *TelemetryUplink) Stop() {
	f.cancel()
}

func (f *TelemetryUplink) Done() <-chan struct{} {
	return f.done
}

// processMessage 处理单条遥测上行消息。
func (f *TelemetryUplink) processMessage(msg *DeviceMessage) {
	device, err := f.loadTelemetryDevice(msg)
	if err != nil {
		return
	}

	processedPayload, ok := f.decodeTelemetryPayload(device, msg)
	if !ok {
		return
	}

	f.dispatchTelemetryPayload(device, processedPayload, msg)
}

func (f *TelemetryUplink) loadTelemetryDevice(msg *DeviceMessage) (*model.Device, error) {
	deviceID, ok := f.resolveTelemetryDeviceID(msg)
	if !ok {
		return nil, fmt.Errorf("telemetry device id is missing or invalid")
	}

	device, err := initialize.GetDeviceCacheById(deviceID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err,
		}).Error("Failed to get device from cache")
		return nil, err
	}
	return device, nil
}

func (f *TelemetryUplink) resolveTelemetryDeviceID(msg *DeviceMessage) (string, bool) {
	deviceIDObj, ok := msg.GetMetadata("device_id")
	if !ok {
		f.logger.Error("Device ID not found in message metadata")
		return "", false
	}

	deviceID, ok := deviceIDObj.(string)
	if !ok {
		f.logger.Error("Invalid device ID type in metadata")
		return "", false
	}

	return deviceID, true
}

func (f *TelemetryUplink) decodeTelemetryPayload(device *model.Device, msg *DeviceMessage) ([]byte, bool) {
	if device.DeviceConfigID == nil || *device.DeviceConfigID == "" {
		return msg.Payload, true
	}

	output, err := f.processor.Decode(f.ctx, &processor.DecodeInput{
		DeviceConfigID: *device.DeviceConfigID,
		Type:           processor.DataTypeTelemetry,
		RawData:        msg.Payload,
		Timestamp:      msg.Timestamp,
	})
	if err != nil {
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("processor failed: %v", err))
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     err,
		}).Error("Processor decode failed, terminate processing")
		return nil, false
	}

	if !output.Success {
		errMsg := "processor returned unsuccessful result"
		if output.Error != nil {
			errMsg = fmt.Sprintf("processor returned unsuccessful result: %v", output.Error)
		}
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, errMsg)
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"error":     output.Error,
		}).Error("Processor execution failed, terminate processing")
		return nil, false
	}

	return output.Data, true
}

func (f *TelemetryUplink) dispatchTelemetryPayload(device *model.Device, processedPayload []byte, msg *DeviceMessage) {
	if msg.Type == "gateway_telemetry" {
		f.processGatewayMessage(device, processedPayload, msg)
	} else {
		f.processDirectDeviceMessage(device, processedPayload, msg)
	}
}

// processGatewayMessage 处理网关上报的子设备遥测消息。
func (f *TelemetryUplink) processGatewayMessage(device *model.Device, payload []byte, originalMsg *DeviceMessage) {
	var gatewayMsg model.GatewayPublish
	if err := json.Unmarshal(payload, &gatewayMsg); err != nil {
		diagnostics.GetInstance().RecordUplinkFailed(device.ID, diagnostics.StageProcessor, fmt.Sprintf("gateway message json parse failed: %v", err))
		f.logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Failed to unmarshal gateway message")
		return
	}

	f.processGatewayTelemetryNode(device, &gatewayMsg, originalMsg, 1)
}

func (f *TelemetryUplink) processGatewayTelemetryNode(gatewayDevice *model.Device, gatewayMsg *model.GatewayPublish, originalMsg *DeviceMessage, childGatewayDepth int) {
	if gatewayMsg == nil {
		return
	}

	// Keep gateway fan-out order stable: gateway itself, direct subdevices, then nested gateways.
	if gatewayMsg.GatewayData != nil {
		gatewayData, _ := json.Marshal(gatewayMsg.GatewayData)
		f.processDirectDeviceMessage(gatewayDevice, gatewayData, originalMsg)
	}

	if gatewayMsg.SubDeviceData != nil {
		f.processSubDevices(gatewayDevice.ID, *gatewayMsg.SubDeviceData, originalMsg)
	}

	if gatewayMsg.SubGatewayData != nil {
		f.processSubGateways(gatewayDevice.ID, *gatewayMsg.SubGatewayData, originalMsg, childGatewayDepth)
	}
}

func sortedSubDeviceAddresses(subDeviceData map[string]map[string]interface{}) []string {
	addrs := make([]string, 0, len(subDeviceData))
	for addr := range subDeviceData {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return addrs
}

func sortedSubGatewayAddresses(subGatewayData map[string]*model.GatewayPublish) []string {
	addrs := make([]string, 0, len(subGatewayData))
	for addr := range subGatewayData {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return addrs
}

// processSubDevices 处理网关直连子设备遥测数据，subDeviceData 的键为子设备地址。
func (f *TelemetryUplink) processSubDevices(parentID string, subDeviceData map[string]map[string]interface{}, originalMsg *DeviceMessage) {
	if len(subDeviceData) == 0 {
		return
	}

	subDeviceAddrs := sortedSubDeviceAddresses(subDeviceData)

	subDevices, err := dal.GetDeviceBySubDeviceAddress(subDeviceAddrs, parentID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub devices")
		return
	}

	// 逐个子设备转成普通直连遥测流程。
	for _, addr := range subDeviceAddrs {
		data := subDeviceData[addr]
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

// processSubGateways 递归处理多级子网关遥测数据。
func (f *TelemetryUplink) processSubGateways(parentID string, subGatewayData map[string]*model.GatewayPublish, originalMsg *DeviceMessage, depth int) {
	if depth > 5 {
		f.logger.Warn("Maximum gateway depth (5) exceeded")
		return
	}

	if len(subGatewayData) == 0 {
		return
	}

	subGatewayAddrs := sortedSubGatewayAddresses(subGatewayData)

	// 按父网关和地址批量查询子网关。
	subGateways, err := dal.GetDeviceBySubDeviceAddress(subGatewayAddrs, parentID)
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"parent_id": parentID,
			"error":     err,
		}).Error("Failed to get sub gateways")
		return
	}

	// 逐个子网关分派自身数据、子设备数据和下级网关数据。
	for _, addr := range subGatewayAddrs {
		gatewayMsg := subGatewayData[addr]
		subGateway, ok := subGateways[addr]
		if !ok {
			f.logger.WithFields(logrus.Fields{
				"parent_id":    parentID,
				"gateway_addr": addr,
			}).Warn("Sub gateway not found")
			continue
		}

		f.processGatewayTelemetryNode(subGateway, gatewayMsg, originalMsg, depth+1)
	}
}

// updateDeviceStateForTelemetry refreshes device heartbeat/status/cache state before telemetry side effects.
func (f *TelemetryUplink) updateDeviceStateForTelemetry(device *model.Device) {
	f.refreshHeartbeat(device)
}

func (f *TelemetryUplink) recordTelemetryDiagnostics(deviceID string, pointCount int) {
	if inst := diagnostics.GetInstance(); inst != nil && inst.IsEnabled() {
		for i := 0; i < pointCount; i++ {
			inst.RecordUplinkTotal(deviceID)
		}
	}
}

func (f *TelemetryUplink) enqueueTelemetryStorage(device *model.Device, telemetryPoints []storage.TelemetryDataPoint, timestamp int64) bool {
	return enqueueStorageMessage(f.ctx, f.storageInput, &storage.Message{
		DeviceID:  device.ID,
		TenantID:  device.TenantID,
		DataType:  storage.DataTypeTelemetry,
		Timestamp: timestamp,
		Data:      telemetryPoints,
	}, f.logger)
}

func (f *TelemetryUplink) publishTelemetryWebSocket(device *model.Device, triggerValues map[string]interface{}) {
	if f.wsPublishQueue == nil {
		f.logger.WithField("device_id", device.ID).Warn("Telemetry WebSocket publish queue is not initialized, dropping event")
		return
	}

	select {
	case <-f.ctx.Done():
		return
	default:
	}

	task := telemetryWSPublishTask{
		deviceID: device.ID,
		tenantID: device.TenantID,
		data:     triggerValues,
	}
	select {
	case f.wsPublishQueue <- task:
	case <-f.ctx.Done():
	default:
		dropped := atomic.AddUint64(&f.wsPublishDropped, 1)
		if dropped == 1 || dropped%1000 == 0 {
			f.logger.WithFields(logrus.Fields{
				"device_id": device.ID,
				"queue_len": len(f.wsPublishQueue),
				"queue_cap": cap(f.wsPublishQueue),
				"dropped":   dropped,
			}).Warn("Telemetry WebSocket publish queue full, dropping event")
		}
	}
}

func (f *TelemetryUplink) executeTelemetryAutomation(device *model.Device, triggerParam []string, triggerValues map[string]interface{}) {
	select {
	case <-f.ctx.Done():
		return
	default:
	}
	select {
	case f.automationSem <- struct{}{}:
	case <-f.ctx.Done():
		return
	default:
		dropped := atomic.AddUint64(&f.automationDropped, 1)
		if dropped == 1 || dropped%1000 == 0 {
			f.logger.WithFields(logrus.Fields{
				"device_id":    device.ID,
				"dropped":      dropped,
				"max_inflight": cap(f.automationSem),
			}).Warn("Telemetry automation concurrency limit reached, dropping automation execution")
		}
		return
	}

	go func() {
		defer func() { <-f.automationSem }()
		err := service.GroupApp.Execute(device, service.AutomateFromExt{
			TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
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

// processDirectDeviceMessage handles one decoded telemetry payload for one device.
func (f *TelemetryUplink) processDirectDeviceMessage(device *model.Device, payload []byte, originalMsg *DeviceMessage) {
	// 有效遥测上行会刷新设备在线状态，再进入存储、推送和自动化副作用。
	f.updateDeviceStateForTelemetry(device)

	telemetryPoints, triggerParam, triggerValues, _ := f.convertToTelemetryPoints(payload, device)
	f.processTelemetrySideEffects(device, telemetryPoints, triggerParam, triggerValues, resolveStorageTimestamp(originalMsg))
}

func telemetrySideEffectsEnabled(telemetryPoints []storage.TelemetryDataPoint) bool {
	return len(telemetryPoints) > 0
}
func (f *TelemetryUplink) processTelemetrySideEffects(device *model.Device, telemetryPoints []storage.TelemetryDataPoint, triggerParam []string, triggerValues map[string]interface{}, timestamp int64) {
	if !telemetrySideEffectsEnabled(telemetryPoints) {
		f.logger.WithField("device_id", device.ID).Debug("Telemetry payload produced no data points; side effects skipped")
		return
	}
	f.recordTelemetryDiagnostics(device.ID, len(telemetryPoints))
	if !f.enqueueTelemetryStorage(device, telemetryPoints, timestamp) {
		return
	}
	f.publishTelemetryWebSocket(device, triggerValues)
	f.executeTelemetryAutomation(device, triggerParam, triggerValues)
}

// convertToTelemetryPoints converts telemetry payloads into storage points and automation trigger inputs.
func (f *TelemetryUplink) convertToTelemetryPoints(payload []byte, device *model.Device) ([]storage.TelemetryDataPoint, []string, map[string]interface{}, error) {
	dataMap := f.normalizeTelemetryPayloadShape(payload, device)
	points, triggerParam, triggerValues := convertTelemetryMapToPoints(dataMap)

	return points, triggerParam, triggerValues, nil
}

func (f *TelemetryUplink) normalizeTelemetryPayloadShape(payload []byte, device *model.Device) map[string]interface{} {
	var dataMap map[string]interface{}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		f.logger.WithFields(logrus.Fields{
			"device_id": device.ID,
			"payload":   string(payload),
			"error":     err,
		}).Warn("payload is not valid JSON object, wrapping as {\"_raw\": ...}")

		dataMap = map[string]interface{}{
			"_raw": normalizeRawTelemetryValue(payload),
		}
	}

	return normalizeLegacyRDITelemetryAliases(dataMap)
}

var legacyRDITelemetryAliases = map[string][]string{
	"temperature_1":      {"T1"},
	"temperature_2":      {"T2"},
	"switch_1":           {"NC_INPUT_1_LEVEL", "NC_INPUT_1_Level"},
	"switch_2":           {"NC_INPUT_2_LEVEL", "NC_INPUT_2_Level"},
	"dry_contact_output": {"NO_LEVEL", "NO_Level"},
}

func normalizeLegacyRDITelemetryAliases(dataMap map[string]interface{}) map[string]interface{} {
	if len(dataMap) == 0 {
		return dataMap
	}

	for targetKey, legacyKeys := range legacyRDITelemetryAliases {
		if _, hasTarget := dataMap[targetKey]; hasTarget {
			continue
		}
		for _, legacyKey := range legacyKeys {
			if value, ok := dataMap[legacyKey]; ok {
				dataMap[targetKey] = value
				break
			}
		}
	}

	return dataMap
}

func normalizeRawTelemetryValue(payload []byte) interface{} {
	var rawValue interface{}
	if jsonErr := json.Unmarshal(payload, &rawValue); jsonErr != nil {
		return string(payload)
	}
	return rawValue
}

func convertTelemetryMapToPoints(dataMap map[string]interface{}) ([]storage.TelemetryDataPoint, []string, map[string]interface{}) {
	keys := make([]string, 0, len(dataMap))
	for key := range dataMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	points := make([]storage.TelemetryDataPoint, 0, len(keys))
	triggerParam := make([]string, 0, len(keys))
	triggerValues := make(map[string]interface{}, len(keys))

	for _, key := range keys {
		value := dataMap[key]
		points = append(points, storage.TelemetryDataPoint{
			Key:   key,
			Value: value,
		})

		triggerParam = append(triggerParam, key)
		triggerValues[key] = value
	}

	return points, triggerParam, triggerValues
}

func (f *TelemetryUplink) refreshHeartbeat(device *model.Device) {
	// 未启用心跳服务时跳过在线状态刷新。
	if f.heartbeatService == nil {
		return
	}

	// 获取当前设备的心跳配置。
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

// checkAndPublishToWS 检查设备是否有 WebSocket 订阅，并在有订阅时推送遥测事件。
func (f *TelemetryUplink) checkAndPublishToWS(deviceID, tenantID string, data map[string]interface{}) {
	// 先检查订阅标记，避免对无人订阅的设备发送无效消息。
	ctx := context.Background()
	exists, err := global.REDIS.Exists(ctx, "ws:sub:"+deviceID).Result()
	if err != nil {
		f.logger.WithError(err).WithField("device_id", deviceID).Debug("Failed to check WebSocket subscription")
		return
	}

	if exists == 0 {
		// 无订阅者时直接跳过。
		return
	}

	// 构造 WebSocket 设备事件。
	event := global.WSEvent{
		DeviceID:  deviceID,
		TenantID:  tenantID,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}

	// 序列化事件载荷。
	jsonData, err := json.Marshal(event)
	if err != nil {
		f.logger.WithError(err).WithField("device_id", deviceID).Error("Failed to marshal WebSocket event")
		return
	}

	// 通过 Redis Pub/Sub 广播给 WebSocket 网关。
	if err := global.REDIS.Publish(ctx, "ws:device:"+deviceID, jsonData).Err(); err != nil {
		f.logger.WithError(err).WithField("device_id", deviceID).Debug("WS event publish failed")
		return
	}

	f.logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"data_keys": len(data),
	}).Debug("WebSocket event published to Redis")
}

// notifyDeviceOnline 通知前端设备已上线，并同步期望数据状态。
func (f *TelemetryUplink) notifyDeviceOnline(device *model.Device) {
	notifyDeviceOnlineAndExpectedData(f.logger, device)
}
