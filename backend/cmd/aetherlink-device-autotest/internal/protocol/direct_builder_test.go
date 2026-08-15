package protocol

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestWrapResponseEnvelopeUsesBackendWireContract(t *testing.T) {
	inner := []byte(`{"result":0,"message":"success","method":"RestartDevice"}`)
	payload, err := WrapResponseEnvelope("device-uuid", inner)
	if err != nil {
		t.Fatalf("WrapResponseEnvelope returned error: %v", err)
	}

	var envelope struct {
		DeviceID string `json:"device_id"`
		Values   string `json:"values"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("response envelope is not JSON: %v", err)
	}
	if envelope.DeviceID != "device-uuid" {
		t.Fatalf("device_id = %q, want device-uuid", envelope.DeviceID)
	}
	decoded, err := base64.StdEncoding.DecodeString(envelope.Values)
	if err != nil {
		t.Fatalf("values is not base64: %v", err)
	}
	if string(decoded) != string(inner) {
		t.Fatalf("decoded values = %q, want %q", decoded, inner)
	}
}

func TestWrapResponseEnvelopeRejectsIncompletePayload(t *testing.T) {
	if _, err := WrapResponseEnvelope("", []byte(`{}`)); err == nil {
		t.Fatal("expected empty device id to be rejected")
	}
	if _, err := WrapResponseEnvelope("device-uuid", nil); err == nil {
		t.Fatal("expected empty response values to be rejected")
	}
}
