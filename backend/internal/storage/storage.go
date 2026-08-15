// 文件用途：提供遥测、属性或事件存储模块的 storage 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type Storage、type storage、func New、func (s *storage) Start 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Storage 存储层接口
type Storage interface {
	Start(ctx context.Context, inputChan <-chan *Message) error
	Stop(timeout time.Duration) error
	GetMetrics() Metrics
}

// storage 存储层实现
type storage struct {
	db      *gorm.DB
	logger  Logger
	config  Config
	metrics *metricsCollector

	telemetryWriter *telemetryWriter
	directWriter    *directWriter
	attributeEvent  *attributeEventIngress

	inputChan <-chan *Message
	stopCh    chan struct{}
	doneCh    chan struct{}
	stopOnce  sync.Once
	doneOnce  sync.Once
}

// New 创建存储层实例
func New(db *gorm.DB, logger Logger, config Config) DurableStorage {
	metrics := newMetricsCollector(config.EnableMetrics)

	return &storage{
		db:              db,
		logger:          logger,
		config:          config,
		metrics:         metrics,
		telemetryWriter: newTelemetryWriter(db, logger, config, metrics),
		directWriter:    newDirectWriter(db, metrics),
		attributeEvent:  newAttributeEventIngress(db, logger, config, metrics),
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
}

// PersistDurably is the synchronous attribute/event durability boundary. A
// nil result means the complete frozen envelope is confirmed by at least one
// durable tier. It never accepts telemetry.
func (s *storage) PersistDurably(ctx context.Context, msg *Message) (DurabilityReceipt, error) {
	if s == nil || s.attributeEvent == nil {
		return DurabilityReceipt{}, fmt.Errorf("attribute/event durable input is unavailable")
	}
	return s.attributeEvent.accept(ctx, msg)
}

// Start 启动存储服务
func (s *storage) Start(ctx context.Context, inputChan <-chan *Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if inputChan == nil {
		if s.telemetryWriter != nil {
			s.telemetryWriter.finish()
		}
		if s.attributeEvent != nil {
			s.attributeEvent.requestStop()
		}
		s.finish()
		return fmt.Errorf("input channel is nil")
	}

	s.inputChan = inputChan
	if err := s.attributeEvent.start(ctx); err != nil {
		if s.telemetryWriter != nil {
			s.telemetryWriter.finish()
		}
		s.attributeEvent.requestStop()
		s.finish()
		return fmt.Errorf("start attribute/event durable input: %w", err)
	}

	if err := s.telemetryWriter.start(ctx); err != nil {
		stopTimeout := s.config.AttributeEventSpoolReplayTimeout
		if stopTimeout <= 0 {
			stopTimeout = time.Second
		}
		_ = s.attributeEvent.stop(stopTimeout)
		s.finish()
		return fmt.Errorf("start telemetry writer: %w", err)
	}

	go s.run(ctx)

	s.logger.Info("storage service started")
	return nil
}

// Stop 停止存储服务
func (s *storage) Stop(timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("storage stop timeout must be positive")
	}
	deadline := time.Now().Add(timeout)
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})

	if !waitForStorageDone(s.doneCh, time.Until(deadline)) {
		// Keep the writer available until the main loop really finishes draining.
		// The caller receives a timeout instead of a false successful shutdown.
		go func() {
			<-s.doneCh
			_ = s.telemetryWriter.stop(timeout)
			_ = s.attributeEvent.stop(timeout)
		}()
		return fmt.Errorf("storage main loop stop timeout")
	}
	s.logger.Info("storage main loop stopped")

	telemetryErr := s.telemetryWriter.stop(time.Until(deadline))
	attributeEventErr := s.attributeEvent.stop(time.Until(deadline))
	if err := errors.Join(telemetryErr, attributeEventErr); err != nil {
		return err
	}

	s.logger.Info("storage service stopped")
	return nil
}

func waitForStorageDone(done <-chan struct{}, timeout time.Duration) bool {
	if timeout <= 0 {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// GetMetrics 获取监控指标
func (s *storage) GetMetrics() Metrics {
	return s.metrics.GetMetrics()
}

// run 主循环
func (s *storage) run(ctx context.Context) {
	defer func() {
		// The main loop owns the writer's input lifetime. Request writer shutdown
		// on every exit path (explicit Stop, context cancellation, or independently
		// closed input) so a missing outer Stop cannot leave its tickers/goroutine
		// alive. All buffered input has been handled before this defer runs.
		s.telemetryWriter.requestStop()
		s.attributeEvent.requestStop()
		s.finish()
	}()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("storage context cancelled")
			s.drainBufferedInput()
			return
		case <-s.stopCh:
			s.logger.Info("storage stopped")
			s.drainBufferedInput()
			return
		case msg, ok := <-s.inputChan:
			if !ok {
				s.logger.Info("input channel closed")
				return
			}
			s.processMessage(msg)
		}
	}
}

func (s *storage) finish() {
	if s == nil {
		return
	}
	s.doneOnce.Do(func() { close(s.doneCh) })
}

func (s *storage) drainBufferedInput() {
	for {
		select {
		case msg, ok := <-s.inputChan:
			if !ok {
				return
			}
			s.processMessage(msg)
		default:
			return
		}
	}
}

func (s *storage) processMessage(msg *Message) error {
	err := s.handleMessage(msg)
	if err == nil {
		return nil
	}

	deviceID := ""
	dataType := DataType("")
	if msg != nil {
		deviceID = msg.DeviceID
		dataType = msg.DataType
	}
	s.logger.Errorf("storage message handling failed: device_id=%s data_type=%s: %v", deviceID, dataType, err)
	return err
}

// handleMessage 处理消息并把写入错误返回给主循环。
func (s *storage) handleMessage(msg *Message) error {
	if msg == nil {
		s.logger.Warn("nil storage message")
		return nil
	}
	switch msg.DataType {
	case DataTypeTelemetry:
		s.metrics.incTelemetryReceived()
		if err := s.telemetryWriter.write(msg); err != nil {
			return fmt.Errorf("handle telemetry message: %w", err)
		}

	case DataTypeAttribute:
		if s.attributeEvent != nil {
			if _, err := s.PersistDurably(context.Background(), msg); err != nil {
				return fmt.Errorf("handle attribute message: %w", err)
			}
			break
		}
		if err := s.directWriter.writeAttribute(msg); err != nil {
			return fmt.Errorf("handle attribute message: %w", err)
		}

	case DataTypeEvent:
		if s.attributeEvent != nil {
			if _, err := s.PersistDurably(context.Background(), msg); err != nil {
				return fmt.Errorf("handle event message: %w", err)
			}
			break
		}
		if err := s.directWriter.writeEvent(msg); err != nil {
			return fmt.Errorf("handle event message: %w", err)
		}

	default:
		s.logger.Warnf("unknown data type: %s", msg.DataType)
	}
	return nil
}
