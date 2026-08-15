package downlink

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/processor"
)

type Handler struct {
	publisher MessagePublisher
	processor processor.DataProcessor
	logger    *logrus.Logger
}

func NewHandler(publisher MessagePublisher, processor processor.DataProcessor, logger *logrus.Logger) *Handler {
	return &Handler{
		publisher: publisher,
		processor: processor,
		logger:    logger,
	}
}

func (h *Handler) HandleCommand(ctx context.Context, msg *Message) {
	h.handle(ctx, msg, processor.DataTypeCommand)
}

func (h *Handler) HandleAttributeSet(ctx context.Context, msg *Message) {
	h.handle(ctx, msg, processor.DataTypeAttributeSet)
}

func (h *Handler) HandleAttributeGet(ctx context.Context, msg *Message) {
	h.handle(ctx, msg, processor.DataTypeAttributeSet)
}

func (h *Handler) HandleTelemetry(ctx context.Context, msg *Message) {
	h.handle(ctx, msg, processor.DataTypeTelemetryControl)
}

func (h *Handler) handle(ctx context.Context, msg *Message, dataType processor.DataType) {
	start := time.Now()

	if msg != nil && msg.DeviceID != "" {
		diagnostics.GetInstance().RecordDownlinkTotal(msg.DeviceID)
	}

	if !h.validateMessage(msg) {
		if msg != nil && msg.MessageID != "" {
			h.updateLogStatus(msg.MessageID, msg.DeviceID, "2", "invalid message parameters", msg.Type)
		}
		return
	}

	encodedData, ok := h.encodeMessagePayload(ctx, msg, dataType)
	if !ok {
		return
	}

	if err := h.publishMessage(msg, encodedData); err != nil {
		if msg.DeviceID != "" {
			diagnostics.GetInstance().RecordDownlinkFailed(
				msg.DeviceID,
				diagnostics.StagePublish,
				fmt.Sprintf("MQTT发布失败：%v", err),
			)
		}
		h.logger.WithFields(logrus.Fields{
			"module":        "downlink",
			"device_id":     msg.DeviceID,
			"device_number": msg.DeviceNumber,
			"device_type":   msg.DeviceType,
			"error":         err,
			"duration_ms":   time.Since(start).Milliseconds(),
		}).Error("message publish failed")

		h.updateLogStatus(msg.MessageID, msg.DeviceID, "2", fmt.Sprintf("publish failed: %v", err), msg.Type)
		return
	}

	h.updateLogStatus(msg.MessageID, msg.DeviceID, "1", "", msg.Type)
	h.logger.WithFields(logrus.Fields{
		"module":      "downlink",
		"device_id":   msg.DeviceID,
		"type":        dataType,
		"duration_ms": time.Since(start).Milliseconds(),
	}).Info("message published successfully")
}

func (h *Handler) validateMessage(msg *Message) bool {
	if msg != nil && msg.DeviceNumber != "" && len(msg.Data) > 0 {
		return true
	}

	h.logger.WithFields(logrus.Fields{
		"module": "downlink",
		"error":  ErrInvalidMessage,
	}).Error("invalid message")
	return false
}
