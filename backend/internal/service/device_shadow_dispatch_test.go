package service

import (
	"errors"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestDispatchShadowMessageRejectsUnsupportedType(t *testing.T) {
	payload := `{"value":1}`
	for _, messageType := range []string{"property", "notification", ""} {
		err := dispatchShadowMessage("device-1", &model.DeviceShadowMessage{
			MessageType: messageType,
			Payload:     &payload,
		})
		if !errors.Is(err, errUnsupportedShadowMessageType) {
			t.Fatalf("message type %q error = %v, want unsupported type", messageType, err)
		}
	}
}
