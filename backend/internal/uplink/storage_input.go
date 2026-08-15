package uplink

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"aetherlink-iot/backend/internal/diagnostics"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

var errStorageInputUnavailable = errors.New("storage input is unavailable")
var errDurableMessagePersisterUnavailable = errors.New("durable attribute/event storage persister is unavailable")

func resolveStorageTimestamp(msg *DeviceMessage) int64 {
	if msg != nil && msg.Timestamp > 0 {
		return msg.Timestamp
	}
	return time.Now().UnixMilli()
}

func resolveStorageSourceID(msg *DeviceMessage) string {
	if msg == nil {
		return ""
	}
	value, ok := msg.GetMetadata("source_id")
	if !ok {
		return ""
	}
	sourceID, _ := value.(string)
	return sourceID
}

func enqueueStorageMessage(ctx context.Context, input storage.MessageEnqueuer, msg *storage.Message, logger *logrus.Logger) bool {
	var err error
	if input == nil {
		err = errStorageInputUnavailable
	} else {
		err = input.Enqueue(ctx, msg)
	}
	if err == nil {
		return true
	}

	deviceID := ""
	dataType := storage.DataType("")
	if msg != nil {
		deviceID = msg.DeviceID
		dataType = msg.DataType
	}
	diagnostics.GetInstance().RecordStorageFailed(deviceID, fmt.Sprintf("storage enqueue failed: %v", err))
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"data_type": dataType,
	}).WithError(err).Error("Failed to enqueue storage message")
	return false
}

// persistDurableAttributeEvent synchronously obtains a storage-owned durability
// receipt. A true result means the primary table, PostgreSQL dead-letter, or
// independent file spool retained the canonical envelope; it does not describe
// MQTT or uplink-bus acknowledgement.
func persistDurableAttributeEvent(
	ctx context.Context,
	input storage.DurableMessagePersister,
	msg *storage.Message,
	logger *logrus.Logger,
) bool {
	var err error
	var receipt storage.DurabilityReceipt
	deviceID := ""
	dataType := storage.DataType("")
	if msg != nil {
		deviceID = msg.DeviceID
		dataType = msg.DataType
	}
	if isNilDurableMessagePersister(input) {
		err = errDurableMessagePersisterUnavailable
	} else {
		receipt, err = input.PersistDurably(ctx, msg)
	}
	if err == nil {
		if receipt.MessageID == "" {
			err = errors.New("durable storage returned an empty receipt")
		} else {
			switch receipt.Tier {
			case storage.DurabilityTierPrimary:
				return true
			case storage.DurabilityTierPostgresDeadLetter, storage.DurabilityTierFileSpool:
				if logger == nil {
					logger = logrus.StandardLogger()
				}
				logger.WithFields(logrus.Fields{
					"device_id":  deviceID,
					"data_type":  dataType,
					"message_id": receipt.MessageID,
					"tier":       receipt.Tier,
				}).Warn("Attribute/event accepted by deferred durable storage tier")
				return true
			default:
				err = fmt.Errorf("durable storage returned unknown receipt tier %q", receipt.Tier)
			}
		}
	}

	diagnostics.GetInstance().RecordStorageFailed(
		deviceID,
		fmt.Sprintf("durable attribute/event storage persist failed: %v", err),
	)
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"data_type": dataType,
	}).WithError(err).Error("Failed to durably persist attribute/event storage message")
	return false
}

func isNilDurableMessagePersister(input storage.DurableMessagePersister) bool {
	if input == nil {
		return true
	}

	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// runAfterDurableAttributeEventPersist centralizes the business ordering shared
// by attribute and event uplinks. Heartbeat and downstream effects must not run
// until durable admission has succeeded.
func runAfterDurableAttributeEventPersist(
	persist func() bool,
	refreshHeartbeat func(),
	launchSideEffects func(),
) bool {
	if persist == nil || !persist() {
		return false
	}
	if refreshHeartbeat != nil {
		refreshHeartbeat()
	}
	if launchSideEffects != nil {
		launchSideEffects()
	}
	return true
}
