package downlink

import (
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/dal"
)

// updateLogStatus updates the delivery log after encode/publish outcomes.
// status: 0=pending, 1=sent, 2=failed.
func (h *Handler) updateLogStatus(messageID, deviceID, status, errorMsg string, msgType MessageType) {
	if messageID == "" || deviceID == "" {
		return
	}

	switch msgType {
	case MessageTypeCommand:
		h.updateCommandLogStatus(messageID, deviceID, status, errorMsg)
	case MessageTypeAttributeSet:
		h.updateAttributeSetLogStatus(messageID, deviceID, status, errorMsg)
	case MessageTypeTelemetry:
		h.updateTelemetryLogStatus(messageID, status, errorMsg)
	default:
		h.logger.WithFields(logrus.Fields{
			"message_id": messageID,
			"msg_type":   msgType,
		}).Warn("Unknown message type for log update")
	}
}

func (h *Handler) updateCommandLogStatus(messageID, deviceID, status, errorMsg string) {
	updated, err := dal.UpdateCommandSetLogDeliveryStatus(messageID, deviceID, status, errorMsg)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"message_id": messageID,
			"device_id":  deviceID,
		}).Error("Failed to update command log delivery status")
		return
	}
	if !updated {
		h.logger.WithFields(logrus.Fields{
			"message_id":     messageID,
			"device_id":      deviceID,
			"ignored_status": status,
		}).Debug("Command delivery status update skipped because the log is missing or already has a device response")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"message_id": messageID,
		"device_id":  deviceID,
		"status":     status,
		"type":       "command",
	}).Debug("Command log status updated")
}

func (h *Handler) updateAttributeSetLogStatus(messageID, deviceID, status, errorMsg string) {
	log, err := dal.GetAttributeSetLogByMessageID(messageID, deviceID)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"message_id": messageID,
			"device_id":  deviceID,
		}).Warn("Failed to find attribute log")
		return
	}

	log.Status = &status
	if errorMsg != "" {
		log.ErrorMessage = &errorMsg
	}

	if err := dal.UpdateAttributeSetLog(log); err != nil {
		h.logger.WithError(err).WithField("message_id", messageID).Error("Failed to update attribute log status")
	} else {
		h.logger.WithFields(logrus.Fields{
			"message_id": messageID,
			"device_id":  deviceID,
			"status":     status,
			"type":       "attribute_set",
		}).Debug("Attribute log status updated")
	}
}

func (h *Handler) updateTelemetryLogStatus(messageID, status, errorMsg string) {
	log, err := dal.GetTelemetrySetLogByID(messageID)
	if err != nil {
		h.logger.WithError(err).WithField("log_id", messageID).Warn("Failed to find telemetry log")
		return
	}

	log.Status = &status
	if errorMsg != "" {
		log.ErrorMessage = &errorMsg
	}

	if err := dal.UpdateTelemetrySetLog(log); err != nil {
		h.logger.WithError(err).WithField("log_id", messageID).Error("Failed to update telemetry log status")
	} else {
		h.logger.WithFields(logrus.Fields{
			"log_id": messageID,
			"status": status,
			"type":   "telemetry",
		}).Debug("Telemetry log status updated")
	}
}
