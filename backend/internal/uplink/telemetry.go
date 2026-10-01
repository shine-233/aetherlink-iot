// telemetry.go is the telemetry kind of the shared uplink pipeline
// (pipeline.go). It defines how one device's telemetry is normalized, enqueued
// to storage, and fanned out to WebSocket, automation and rule chains.
//
// Ordering contract (differs from attribute/event on purpose): liveness runs
// first, before storage admission, and storage is the non-durable enqueue path.
// Empty payloads skip every side effect except liveness.
package uplink

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

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

var telemetryKindSpec = kindSpec{
	name:                    "TelemetryUplink",
	gatewayMsgType:          "gateway_telemetry",
	dataType:                processor.DataTypeTelemetry,
	recordDecodeDiagnostics: true,
	gatewayParseErrorLog:    "Failed to unmarshal gateway message",
}

// TelemetryUplink consumes telemetry uplink messages.
type TelemetryUplink struct {
	uplinkBase
	storageInput     storage.MessageEnqueuer // 存储输入通道。
	wsPublishQueue   chan telemetryWSPublishTask
	wsPublishWorkers int
	wsPublishDropped uint64
	// wsSubs caches ws:sub:<id> EXISTS results per device (telemetry_ws_subs.go).
	wsSubs *wsSubCache
	// wsSubExists performs the uncached EXISTS check. Replaceable in tests.
	wsSubExists func(ctx context.Context, deviceID string) (bool, error)
	// 自动化副作用并发闸门：防止突发洪峰下每条遥测裸起 goroutine 导致无界增长。
	automationDropped uint64
	// 规则链副作用并发闸门：与自动化同理，防止每条遥测裸起 goroutine 无界增长。
	ruleChainSem     chan struct{}
	ruleChainDropped uint64
	// sideEffects runs everything after liveness for one device. Replaceable in tests.
	sideEffects func(device *model.Device, points []storage.TelemetryDataPoint, triggerParam []string, triggerValues map[string]interface{}, timestamp int64)
}

// TelemetryUplinkConfig 定义遥测上行处理器的外部依赖。
type TelemetryUplinkConfig struct {
	Processor          processor.DataProcessor
	StorageInput       storage.MessageEnqueuer // 存储输入通道。
	HeartbeatService   *service.HeartbeatService
	Logger             *logrus.Logger
	WSPublishQueueSize int
	WSPublishWorkers   int
	// AutomationMaxConcurrency 限制同时执行的遥测规则链副作用数量；超限时丢弃并计数。
	// 场景自动化已改由 service 层按设备分片的有界工作池承载，不再受此参数约束。
	AutomationMaxConcurrency int
	// Shards is the number of per-device worker shards. 0 = GOMAXPROCS (capped
	// at 32); 1 = legacy single consumer. Same-device order is always kept.
	Shards int
}

type telemetryWSPublishTask struct {
	deviceID string
	tenantID string
	data     map[string]interface{}
}

// NewTelemetryUplink 创建遥测上行处理器。
func NewTelemetryUplink(config TelemetryUplinkConfig) *TelemetryUplink {
	if config.WSPublishQueueSize <= 0 {
		config.WSPublishQueueSize = defaultTelemetryWSPublishQueueSize
	}
	if config.WSPublishWorkers <= 0 {
		config.WSPublishWorkers = defaultTelemetryWSPublishWorkers
	}
	if config.AutomationMaxConcurrency <= 0 {
		config.AutomationMaxConcurrency = defaultTelemetryAutomationMaxConcurrency
	}

	f := &TelemetryUplink{
		uplinkBase:       newUplinkBase(telemetryKindSpec, config.Processor, config.HeartbeatService, config.Logger),
		storageInput:     config.StorageInput,
		wsPublishQueue:   make(chan telemetryWSPublishTask, config.WSPublishQueueSize),
		wsPublishWorkers: config.WSPublishWorkers,
		ruleChainSem:     make(chan struct{}, config.AutomationMaxConcurrency),
		wsSubs:           newWSSubCache(wsSubCacheTTL),
	}
	f.SetShards(config.Shards)
	f.sideEffects = f.processTelemetrySideEffects
	return f
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
	f.startWebSocketPublishWorkers()
	f.run(messageChan, f.processMessage)
}

func (f *TelemetryUplink) startWebSocketPublishWorkers() {
	for i := 0; i < f.wsPublishWorkers; i++ {
		go func(workerID int) {
			// Worker 0 also sweeps the subscription cache so it stays bounded by
			// the set of devices seen in the last sweep interval.
			var sweep <-chan time.Time
			if workerID == 0 && f.wsSubs != nil {
				ticker := time.NewTicker(wsSubCacheSweepEvery)
				defer ticker.Stop()
				sweep = ticker.C
			}
			for {
				select {
				case task := <-f.wsPublishQueue:
					f.checkAndPublishToWS(task.deviceID, task.tenantID, task.data)
				case <-sweep:
					f.wsSubs.sweep()
				case <-f.ctx.Done():
					f.log().WithField("worker_id", workerID).Debug("Telemetry WebSocket publish worker stopped")
					return
				}
			}
		}(i)
	}
}

func (f *TelemetryUplink) processMessage(msg *DeviceMessage) {
	processUplinkMessage[map[string]interface{}](&f.uplinkBase, f, msg)
}

// parseDirect implements kindHandler.
func (f *TelemetryUplink) parseDirect(device *model.Device, payload []byte) map[string]interface{} {
	return decodeJSONObjectOrRaw(f.log(), device.ID, payload, "payload is not valid JSON object, wrapping as {\"_raw\": ...}")
}

// parseGateway implements kindHandler.
func (f *TelemetryUplink) parseGateway(payload []byte) (*gatewayNode[map[string]interface{}], error) {
	var msg model.GatewayPublish
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return gatewayPublishNode(&msg), nil
}

// handleDevice implements kindHandler: one device's telemetry.
func (f *TelemetryUplink) handleDevice(device *model.Device, dataMap map[string]interface{}, originalMsg *DeviceMessage) {
	// 有效遥测上行会刷新设备在线状态，再进入存储、推送和自动化副作用。
	f.liveness.touch(device)

	// Gateway fan-out hands the envelope's own map to us; alias normalization
	// copies on write so the caller's data is never mutated.
	dataMap = normalizeLegacyRDITelemetryAliases(dataMap)
	points, triggerParam, triggerValues := convertTelemetryMapToPoints(dataMap)
	if f.sideEffects != nil {
		f.sideEffects(device, points, triggerParam, triggerValues, resolveStorageTimestamp(originalMsg))
	}
}

func (f *TelemetryUplink) recordTelemetryDiagnostics(deviceID string, pointCount int) {
	if inst := diagnostics.GetInstance(); inst != nil && inst.IsEnabled() {
		inst.RecordUplinkTotalN(deviceID, pointCount)
	}
}

func (f *TelemetryUplink) enqueueTelemetryStorage(device *model.Device, telemetryPoints []storage.TelemetryDataPoint, timestamp int64) bool {
	return enqueueStorageMessage(f.context(), f.storageInput, &storage.Message{
		DeviceID:  device.ID,
		TenantID:  device.TenantID,
		DataType:  storage.DataTypeTelemetry,
		Timestamp: timestamp,
		Data:      telemetryPoints,
	}, f.log())
}

func (f *TelemetryUplink) publishTelemetryWebSocket(device *model.Device, triggerValues map[string]interface{}) {
	if f.wsPublishQueue == nil {
		f.log().WithField("device_id", device.ID).Warn("Telemetry WebSocket publish queue is not initialized, dropping event")
		return
	}

	ctx := f.context()
	select {
	case <-ctx.Done():
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
	case <-ctx.Done():
	default:
		dropped := atomic.AddUint64(&f.wsPublishDropped, 1)
		if dropped == 1 || dropped%1000 == 0 {
			f.log().WithFields(logrus.Fields{
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
	case <-f.context().Done():
		return
	default:
	}
	// 自动化统一走按设备分片的有界工作池：同设备保序、跨设备并发、队列满即丢弃并计数
	// （计数见 service.AutomationPoolSnapshot）。Dispatch 非阻塞，不会拖慢上行链路。
	if err := service.GroupApp.Dispatch(device, service.AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
		TriggerParam:     triggerParam,
		TriggerValues:    triggerValues,
	}); err != nil {
		if dropped := atomic.AddUint64(&f.automationDropped, 1); dropped == 1 || dropped%1000 == 0 {
			f.log().WithFields(logrus.Fields{
				"device_id": device.ID,
				"dropped":   dropped,
				"error":     err,
			}).Warn("Telemetry automation dispatch rejected")
		}
	}
}

func telemetrySideEffectsEnabled(telemetryPoints []storage.TelemetryDataPoint) bool {
	return len(telemetryPoints) > 0
}

func (f *TelemetryUplink) processTelemetrySideEffects(device *model.Device, telemetryPoints []storage.TelemetryDataPoint, triggerParam []string, triggerValues map[string]interface{}, timestamp int64) {
	if !telemetrySideEffectsEnabled(telemetryPoints) {
		f.log().WithField("device_id", device.ID).Debug("Telemetry payload produced no data points; side effects skipped")
		return
	}
	f.recordTelemetryDiagnostics(device.ID, len(telemetryPoints))
	if !f.enqueueTelemetryStorage(device, telemetryPoints, timestamp) {
		return
	}
	f.publishTelemetryWebSocket(device, triggerValues)
	f.executeTelemetryAutomation(device, triggerParam, triggerValues)
	f.executeRuleChains(device, triggerValues)
}

// executeRuleChains 规则链触发（ROADMAP B2）：异步执行，错误只记录不阻断上行。
func (f *TelemetryUplink) executeRuleChains(device *model.Device, values map[string]interface{}) {
	if len(values) == 0 {
		return
	}
	deviceCopy := *device
	valuesCopy := make(map[string]interface{}, len(values))
	for k, v := range values {
		valuesCopy[k] = v
	}
	f.runBoundedRuleChain(device.ID, func() {
		service.GroupApp.RuleChain.OnTelemetry(deviceCopy, valuesCopy)
	})
}

// runBoundedRuleChain 在规则链并发闸门内异步执行 run；闸门已满时丢弃并节流告警。
// ruleChainSem 为 nil（测试直接构造结构体）时保持历史的无闸门异步执行。
func (f *TelemetryUplink) runBoundedRuleChain(deviceID string, run func()) {
	if f.ruleChainSem == nil {
		go run()
		return
	}
	select {
	case f.ruleChainSem <- struct{}{}:
	default:
		dropped := atomic.AddUint64(&f.ruleChainDropped, 1)
		if dropped == 1 || dropped%1000 == 0 {
			f.log().WithFields(logrus.Fields{
				"device_id":    deviceID,
				"dropped":      dropped,
				"max_inflight": cap(f.ruleChainSem),
			}).Warn("Telemetry rule chain concurrency limit reached, dropping rule chain execution")
		}
		return
	}
	go func() {
		defer func() { <-f.ruleChainSem }()
		run()
	}()
}

// legacyRDITelemetryAliases maps a canonical key to its legacy RDI spellings;
// the first legacy key present wins. A slice (not a map) keeps the scan cheap
// and deterministic on every telemetry message.
var legacyRDITelemetryAliases = [...]struct {
	target string
	legacy []string
}{
	{"temperature_1", []string{"T1"}},
	{"temperature_2", []string{"T2"}},
	{"switch_1", []string{"NC_INPUT_1_LEVEL", "NC_INPUT_1_Level"}},
	{"switch_2", []string{"NC_INPUT_2_LEVEL", "NC_INPUT_2_Level"}},
	{"dry_contact_output", []string{"NO_LEVEL", "NO_Level"}},
}

// normalizeLegacyRDITelemetryAliases adds missing canonical keys for legacy RDI
// keys. It never mutates dataMap: the first alias that must be added triggers a
// single copy, and payloads without legacy keys are returned as-is.
func normalizeLegacyRDITelemetryAliases(dataMap map[string]interface{}) map[string]interface{} {
	if len(dataMap) == 0 {
		return dataMap
	}
	out := dataMap
	copied := false
	for _, alias := range legacyRDITelemetryAliases {
		if _, hasTarget := out[alias.target]; hasTarget {
			continue
		}
		for _, legacyKey := range alias.legacy {
			value, ok := dataMap[legacyKey]
			if !ok {
				continue
			}
			if !copied {
				out = make(map[string]interface{}, len(dataMap)+len(legacyRDITelemetryAliases))
				for k, v := range dataMap {
					out[k] = v
				}
				copied = true
			}
			out[alias.target] = value
			break
		}
	}
	return out
}

// convertTelemetryMapToPoints returns the storage points in sorted key order,
// the sorted keys as automation trigger params, and dataMap itself as the
// trigger values. Callers treat triggerParam and triggerValues as read-only
// (the rule chain path copies before going async), so no per-message copies.
func convertTelemetryMapToPoints(dataMap map[string]interface{}) ([]storage.TelemetryDataPoint, []string, map[string]interface{}) {
	keys := sortedKeys(dataMap)
	points := make([]storage.TelemetryDataPoint, len(keys))
	for i, key := range keys {
		points[i] = storage.TelemetryDataPoint{Key: key, Value: dataMap[key]}
	}
	return points, keys, dataMap
}

// checkAndPublishToWS 检查设备是否有 WebSocket 订阅，并在有订阅时推送遥测事件。
func (f *TelemetryUplink) checkAndPublishToWS(deviceID, tenantID string, data map[string]interface{}) {
	// 先检查订阅标记，避免对无人订阅的设备发送无效消息。
	ctx := context.Background()
	subscribed, err := f.hasWSSubscriber(ctx, deviceID)
	if err != nil {
		f.log().WithError(err).WithField("device_id", deviceID).Debug("Failed to check WebSocket subscription")
		return
	}
	if !subscribed {
		return
	}
	if global.REDIS == nil {
		return
	}

	event := global.WSEvent{
		DeviceID:  deviceID,
		TenantID:  tenantID,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}
	jsonData, err := json.Marshal(event)
	if err != nil {
		f.log().WithError(err).WithField("device_id", deviceID).Error("Failed to marshal WebSocket event")
		return
	}

	// 通过 Redis Pub/Sub 广播给 WebSocket 网关。
	if err := global.REDIS.Publish(ctx, "ws:device:"+deviceID, jsonData).Err(); err != nil {
		f.log().WithError(err).WithField("device_id", deviceID).Debug("WS event publish failed")
		return
	}

	f.log().WithFields(logrus.Fields{
		"device_id": deviceID,
		"data_keys": len(data),
	}).Debug("WebSocket event published to Redis")
}

// hasWSSubscriber answers "is ws:sub:<id> set" through the 1s per-device cache.
// Errors are returned uncached so a Redis blip never hides a live subscriber.
func (f *TelemetryUplink) hasWSSubscriber(ctx context.Context, deviceID string) (bool, error) {
	if exists, ok := f.wsSubs.lookup(deviceID); ok {
		return exists, nil
	}
	check := f.wsSubExists
	if check == nil {
		check = redisWSSubExists
	}
	exists, err := check(ctx, deviceID)
	if err != nil {
		return false, err
	}
	f.wsSubs.store(deviceID, exists)
	return exists, nil
}

func redisWSSubExists(ctx context.Context, deviceID string) (bool, error) {
	if global.REDIS == nil {
		return false, errRedisNotInitialized
	}
	n, err := global.REDIS.Exists(ctx, "ws:sub:"+deviceID).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
