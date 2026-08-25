// 文件用途：计算字段引擎的应用层包装——把 uplink.Bus 与 storage 写回输入装配为可托管的 Service。
// 核心逻辑：CalcFieldServiceWrapper 提供 Name/Start/Stop 生命周期；calcFieldStorageAdapter 把
// calcfield.StorageEnqueuer seam 适配到 storage.DurableMessageInput（正常存储链路的统一入口）。
// 关键注意事项：适配器把派生遥测转成 storage.Message{DataType:telemetry, Data:[]TelemetryDataPoint}，
// 与 TelemetryUplink 的写回形态一致；队列满/未就绪返回 false，由引擎侧丢弃计数。
// 重构建议：若未来需要按租户限流派生写入，在适配器处加配额判断，不要改引擎求值逻辑。
package app

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/calcfield"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/internal/uplink"

	"github.com/sirupsen/logrus"
)

// CalcFieldServiceWrapper 托管 calcfield.Engine 的生命周期。
type CalcFieldServiceWrapper struct {
	engine *calcfield.Engine
	logger *logrus.Logger
}

// Name 返回服务名称。
func (w *CalcFieldServiceWrapper) Name() string { return "计算字段引擎" }

// Start 启动引擎消费循环。
func (w *CalcFieldServiceWrapper) Start() error {
	if w.engine == nil {
		return nil
	}
	if err := w.engine.Start(); err != nil {
		return err
	}
	w.logger.Info("Calcfield engine service started")
	return nil
}

// Stop 停止引擎并等待消费协程退出。
func (w *CalcFieldServiceWrapper) Stop() error {
	if w.engine == nil {
		return nil
	}
	if err := w.engine.Stop(); err != nil {
		return err
	}
	w.logger.Info("Calcfield engine service stopped")
	return nil
}

// GetEngine 暴露引擎实例供诊断读取丢弃计数等指标。
func (w *CalcFieldServiceWrapper) GetEngine() *calcfield.Engine { return w.engine }

// calcFieldStorageAdapter 把 calcfield seam 适配到 storage.DurableMessageInput。
type calcFieldStorageAdapter struct {
	input storage.DurableMessageInput
}

// EnqueueDerivedTelemetry 将派生遥测消息转换成 storage.Message 并入队正常存储链路。
func (a calcFieldStorageAdapter) EnqueueDerivedTelemetry(ctx context.Context, msg *uplink.DeviceMessage) bool {
	if a.input == nil || msg == nil || ctx.Err() != nil {
		return false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || len(payload) == 0 {
		return false
	}
	points := make([]storage.TelemetryDataPoint, 0, len(payload))
	for key, value := range payload {
		points = append(points, storage.TelemetryDataPoint{Key: key, Value: value})
	}

	timestamp := msg.Timestamp
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	return a.input.Enqueue(ctx, &storage.Message{
		DeviceID:  msg.DeviceID,
		TenantID:  msg.TenantID,
		DataType:  storage.DataTypeTelemetry,
		Timestamp: timestamp,
		Data:      points,
	}) == nil
}

// newCalcFieldEngine 构造接入生产存储输入的计算字段引擎。
func newCalcFieldEngine(bus *uplink.Bus, storageInput storage.DurableMessageInput, logger *logrus.Logger) *calcfield.Engine {
	return calcfield.NewEngine(bus, calcFieldStorageAdapter{input: storageInput}, logger)
}
