// 文件用途：提供遥测、属性或事件存储模块的 direct writer 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type DirectWriter、func NewDirectWriter、func (w *DirectWriter) WriteAttributeData、func (w *DirectWriter) WriteEventData 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// DirectWriter 属性和事件直接写入器（导出接口）
type DirectWriter struct {
	db      *gorm.DB
	logger  Logger
	metrics *metricsCollector
}

// NewDirectWriter 创建直接写入器
func NewDirectWriter(db *gorm.DB, logger Logger) *DirectWriter {
	return &DirectWriter{
		db:      db,
		logger:  logger,
		metrics: newMetricsCollector(),
	}
}

// WriteAttributeData 直接写入属性数据
func (w *DirectWriter) WriteAttributeData(ctx context.Context, data *AttributeData) error {
	result := w.db.WithContext(ctx).Clauses(AttributeCurrentUpsertClause()).Create(data)
	if result.Error != nil {
		err := result.Error
		w.logger.Errorf("insert attribute failed: %v", err)
		if w.metrics != nil {
			w.metrics.incAttributeFailed()
		}
		return err
	}

	if result.RowsAffected > 0 && w.metrics != nil {
		w.metrics.incAttributeWritten()
	}
	return nil
}

// WriteEventData 直接写入事件数据
func (w *DirectWriter) WriteEventData(ctx context.Context, data *EventDataModel) error {
	if err := w.db.WithContext(ctx).Create(data).Error; err != nil {
		w.logger.Errorf("insert event failed: %v", err)
		if w.metrics != nil {
			w.metrics.incEventFailed()
		}
		return err
	}

	if w.metrics != nil {
		w.metrics.incEventWritten()
	}
	return nil
}

// directWriter 属性和事件直接写入器（内部使用）。写入器只返回错误并更新逐条指标，应用日志由 storage 边界统一记录。
type directWriter struct {
	db      *gorm.DB
	metrics *metricsCollector
}

func newDirectWriter(db *gorm.DB, metrics *metricsCollector) *directWriter {
	return &directWriter{
		db:      db,
		metrics: metrics,
	}
}

func (w *directWriter) writeAttribute(msg *Message) error {
	points, ok := msg.Data.([]AttributeDataPoint)
	if !ok {
		if dataSlice, ok := msg.Data.([]interface{}); ok {
			points = make([]AttributeDataPoint, 0, len(dataSlice))
			for _, item := range dataSlice {
				if point, ok := item.(AttributeDataPoint); ok {
					points = append(points, point)
				}
			}
		}
		if len(points) == 0 {
			return fmt.Errorf("invalid attribute data format")
		}
	}

	writeErrors := make([]error, 0, len(points))
	for index, point := range points {
		stored, err := w.insertAttribute(msg, point)
		if err != nil {
			w.metrics.incAttributeFailed()
			writeErrors = append(writeErrors, fmt.Errorf("attribute point %d (%q): %w", index, point.Key, err))
		} else if stored {
			w.metrics.incAttributeWritten()
		}
	}

	return errors.Join(writeErrors...)
}

func (w *directWriter) insertAttribute(msg *Message, point AttributeDataPoint) (bool, error) {
	boolV, numberV, stringV := convertValue(point.Value)

	data := AttributeData{
		ID:       uuid.New(),
		DeviceID: msg.DeviceID,
		Key:      point.Key,
		TS:       time.UnixMilli(msg.Timestamp),
		BoolV:    boolV,
		NumberV:  numberV,
		StringV:  stringV,
		TenantID: msg.TenantID,
	}

	result := w.db.Clauses(AttributeCurrentUpsertClause()).Create(&data)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (w *directWriter) writeEvent(msg *Message) error {
	eventData, ok := msg.Data.(EventData)
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	data := EventDataModel{
		ID:       uuid.New(),
		DeviceID: msg.DeviceID,
		Identify: eventData.Identify,
		TS:       time.UnixMilli(msg.Timestamp),
		Data:     eventData.Data,
		TenantID: msg.TenantID,
	}

	if err := w.db.Create(&data).Error; err != nil {
		w.metrics.incEventFailed()
		return err
	}

	w.metrics.incEventWritten()
	return nil
}
