package downlink

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/processor"
)

func (h *Handler) encodeMessagePayload(ctx context.Context, msg *Message, dataType processor.DataType) ([]byte, bool) {
	if msg.DeviceConfigID == "" {
		h.logger.WithFields(logrus.Fields{
			"module":    "downlink",
			"device_id": msg.DeviceID,
			"type":      dataType,
		}).Debug("no device config, using raw data")
		return msg.Data, true
	}

	encodeOutput, err := h.processor.Encode(ctx, &processor.EncodeInput{
		DeviceConfigID: msg.DeviceConfigID,
		Type:           dataType,
		Data:           msg.Data,
		Timestamp:      time.Now().UnixMilli(),
	})
	if err != nil {
		h.recordEncodeFailure(msg, dataType, fmt.Sprintf("encode failed: %v", err), fmt.Sprintf("编码失败：%v", err), err)
		return nil, false
	}

	if !encodeOutput.Success {
		errMsg := "脚本执行失败"
		if encodeOutput.Error != nil {
			errMsg = fmt.Sprintf("脚本执行失败：%v", encodeOutput.Error)
		}
		h.recordEncodeFailure(
			msg,
			dataType,
			fmt.Sprintf("script execution failed: %v", encodeOutput.Error),
			errMsg,
			encodeOutput.Error,
		)
		return nil, false
	}

	return encodeOutput.EncodedData, true
}

func (h *Handler) recordEncodeFailure(
	msg *Message,
	dataType processor.DataType,
	logError string,
	diagnosticError string,
	logFieldError interface{},
) {
	if msg.DeviceID != "" {
		diagnostics.GetInstance().RecordDownlinkFailed(
			msg.DeviceID,
			diagnostics.StageEncode,
			diagnosticError,
		)
	}
	h.logger.WithFields(logrus.Fields{
		"module":           "downlink",
		"device_config_id": msg.DeviceConfigID,
		"type":             dataType,
		"error":            logFieldError,
	}).Error("encode failed")

	h.updateLogStatus(msg.MessageID, msg.DeviceID, "2", logError, msg.Type)
}
