package uplink

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

type capturedStorageInput struct {
	message    *storage.Message
	receipt    storage.DurabilityReceipt
	persistErr error
}

func (c *capturedStorageInput) Enqueue(_ context.Context, message *storage.Message) error {
	c.message = message
	return nil
}

func (c *capturedStorageInput) PersistDurably(_ context.Context, message *storage.Message) (storage.DurabilityReceipt, error) {
	c.message = message
	if c.persistErr != nil {
		return storage.DurabilityReceipt{}, c.persistErr
	}
	if c.receipt.MessageID == "" {
		c.receipt = storage.DurabilityReceipt{MessageID: "11111111-1111-5111-8111-111111111111", Tier: storage.DurabilityTierPrimary}
	}
	return c.receipt, nil
}

type typedNilDurableStorageInput struct{}

func (*typedNilDurableStorageInput) PersistDurably(context.Context, *storage.Message) (storage.DurabilityReceipt, error) {
	panic("PersistDurably must not be called through a typed-nil input")
}

func TestStorageAdmissionPreservesOriginalUplinkTimestamp(t *testing.T) {
	const timestamp int64 = 1_784_400_123_456
	device := &model.Device{ID: "device-1", TenantID: "tenant-1"}
	logger := logrus.New()

	tests := []struct {
		name     string
		dataType storage.DataType
		admit    func(*capturedStorageInput) bool
	}{
		{
			name:     "telemetry",
			dataType: storage.DataTypeTelemetry,
			admit: func(input *capturedStorageInput) bool {
				uplink := &TelemetryUplink{storageInput: input, ctx: context.Background(), logger: logger}
				return uplink.enqueueTelemetryStorage(device, []storage.TelemetryDataPoint{{Key: "temperature", Value: 21.5}}, timestamp)
			},
		},
		{
			name:     "attribute",
			dataType: storage.DataTypeAttribute,
			admit: func(input *capturedStorageInput) bool {
				uplink := &AttributeUplink{durableStorageInput: input, ctx: context.Background(), logger: logger}
				return uplink.persistAttributeStorage(device, []storage.AttributeDataPoint{{Key: "mode", Value: "auto"}}, &DeviceMessage{Timestamp: timestamp})
			},
		},
		{
			name:     "event",
			dataType: storage.DataTypeEvent,
			admit: func(input *capturedStorageInput) bool {
				uplink := &EventUplink{durableStorageInput: input, ctx: context.Background(), logger: logger}
				return uplink.persistEventStorage(device, &model.EventInfo{Method: "alarm"}, []byte(`{"level":"high"}`), &DeviceMessage{Timestamp: timestamp})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &capturedStorageInput{}
			if !tt.admit(input) {
				t.Fatal("storage admission returned false")
			}
			if input.message == nil {
				t.Fatal("storage message was not captured")
			}
			if input.message.Timestamp != timestamp {
				t.Fatalf("timestamp = %d, want %d", input.message.Timestamp, timestamp)
			}
			if input.message.DataType != tt.dataType {
				t.Fatalf("data type = %q, want %q", input.message.DataType, tt.dataType)
			}
		})
	}
}

func TestDurableAttributeEventCarriesProtocolSourceIdentity(t *testing.T) {
	input := &capturedStorageInput{}
	device := &model.Device{ID: "child-device", TenantID: "tenant-1"}
	original := &DeviceMessage{
		Timestamp: 1000,
		Metadata:  map[string]interface{}{"source_id": "opaque-hashed-mqtt-source"},
	}
	uplink := &EventUplink{durableStorageInput: input, ctx: context.Background(), logger: logrus.New()}
	if !uplink.persistEventStorage(device, &model.EventInfo{Method: "alarm"}, []byte(`{}`), original) {
		t.Fatal("event durable persistence returned false")
	}
	if input.message == nil || input.message.SourceMessageID != "opaque-hashed-mqtt-source" {
		t.Fatalf("storage source identity = %#v", input.message)
	}
}

func TestResolveStorageTimestampFallsBackOnlyWhenMissing(t *testing.T) {
	if got := resolveStorageTimestamp(&DeviceMessage{Timestamp: 1234}); got != 1234 {
		t.Fatalf("timestamp = %d, want 1234", got)
	}
	if got := resolveStorageTimestamp(nil); got <= 0 {
		t.Fatalf("fallback timestamp = %d, want positive", got)
	}
}

func TestPersistDurableAttributeEventRejectsNilAndTypedNilInputs(t *testing.T) {
	logger := logrus.New()
	logs := &strings.Builder{}
	logger.SetOutput(logs)
	message := &storage.Message{
		DeviceID: "device-1",
		DataType: storage.DataTypeAttribute,
	}

	tests := []struct {
		name  string
		input storage.DurableMessagePersister
	}{
		{name: "nil interface"},
		{name: "typed nil", input: (*typedNilDurableStorageInput)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs.Reset()
			if persistDurableAttributeEvent(context.Background(), tt.input, message, logger) {
				t.Fatal("durable persist returned true for an unavailable input")
			}
			if !strings.Contains(logs.String(), "Failed to durably persist attribute/event storage message") {
				t.Fatalf("durable failure log = %q, want durable persist diagnostic", logs.String())
			}
		})
	}
}

func TestPersistDurableAttributeEventPropagatesFailureWithoutReceipt(t *testing.T) {
	wantErr := errors.New("primary and spool unavailable")
	input := &capturedStorageInput{persistErr: wantErr}
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	message := &storage.Message{DeviceID: "device-1", DataType: storage.DataTypeEvent}

	if persistDurableAttributeEvent(context.Background(), input, message, logger) {
		t.Fatal("durable persist returned true after input failure")
	}
	if input.message != message {
		t.Fatal("durable input did not receive the original storage message")
	}
}

func TestRunAfterDurableAttributeEventPersistOrdersHeartbeatBeforeSideEffects(t *testing.T) {
	var order []string
	accepted := runAfterDurableAttributeEventPersist(
		func() bool {
			order = append(order, "persist")
			return true
		},
		func() {
			order = append(order, "heartbeat")
		},
		func() {
			order = append(order, "side-effects")
		},
	)

	if !accepted {
		t.Fatal("sequence returned false after durable accept")
	}
	want := []string{"persist", "heartbeat", "side-effects"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("sequence order = %v, want %v", order, want)
	}
}

func TestRunAfterDurableAttributeEventPersistStopsAllEffectsOnFailure(t *testing.T) {
	var order []string
	accepted := runAfterDurableAttributeEventPersist(
		func() bool {
			order = append(order, "persist")
			return false
		},
		func() {
			order = append(order, "heartbeat")
		},
		func() {
			order = append(order, "side-effects")
		},
	)

	if accepted {
		t.Fatal("sequence returned true after durable accept failure")
	}
	want := []string{"persist"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("failure sequence order = %v, want %v", order, want)
	}
}

func TestAttributeAndEventConstructorsWireDurableStorageInput(t *testing.T) {
	input := &capturedStorageInput{}
	attribute := NewAttributeUplink(AttributeUplinkConfig{DurableStorageInput: input})
	event := NewEventUplink(EventUplinkConfig{DurableStorageInput: input})

	if attribute.durableStorageInput != input {
		t.Fatalf("attribute durable input = %T, want captured input", attribute.durableStorageInput)
	}
	if event.durableStorageInput != input {
		t.Fatalf("event durable input = %T, want captured input", event.durableStorageInput)
	}
}
