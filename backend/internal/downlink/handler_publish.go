package downlink

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func (h *Handler) publishMessage(msg *Message, payload []byte) error {
	if h.publisher == nil {
		return fmt.Errorf("message publisher not initialized")
	}

	if err := h.publisher.PublishMessage(
		msg.DeviceNumber,
		msg.Type,
		msg.DeviceType,
		msg.TopicPrefix,
		msg.MessageID,
		1,
		payload,
	); err != nil {
		h.logger.WithFields(logrus.Fields{
			"device_number": msg.DeviceNumber,
			"device_type":   msg.DeviceType,
			"msg_type":      msg.Type,
			"message_id":    msg.MessageID,
			"topic_prefix":  msg.TopicPrefix,
			"payload":       string(payload),
			"error":         err.Error(),
		}).Error("message publish failed, may not be delivered to device")
		return err
	}

	return nil
}
