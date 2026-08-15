package device

import (
	"testing"

	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

func TestCommandMessageDetailsExtractsMessageIDAndMethod(t *testing.T) {
	id, method, ok := commandMessageDetails(
		"devices/command/device-number/message-123",
		[]byte(`{"method":"RestartDevice","params":{"mode":"safe"}}`),
	)
	if !ok || id != "message-123" || method != "RestartDevice" {
		t.Fatalf("commandMessageDetails = %q, %q, %v", id, method, ok)
	}
}

func TestCommandMessageDetailsRejectsMalformedCommand(t *testing.T) {
	cases := []struct {
		name    string
		topic   string
		payload []byte
	}{
		{name: "wrong topic shape", topic: "devices/command/device-number", payload: []byte(`{"method":"RestartDevice"}`)},
		{name: "missing method", topic: "devices/command/device-number/message-123", payload: []byte(`{"params":{}}`)},
		{name: "invalid json", topic: "devices/command/device-number/message-123", payload: []byte("not-json")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := commandMessageDetails(tc.topic, tc.payload); ok {
				t.Fatal("expected malformed command to be rejected")
			}
		})
	}
}

func TestStatusTopicUsesDeviceIDAndNotDeviceNumber(t *testing.T) {
	topics := utils.NewMQTTTopics("device-number")
	if got := topics.Status("device-uuid"); got != "devices/status/device-uuid" {
		t.Fatalf("Status topic = %q, want devices/status/device-uuid", got)
	}
}

func TestCommandResponseSuccessSupportsNamedFailureIdentify(t *testing.T) {
	tests := []struct {
		name            string
		defaultSuccess  bool
		method          string
		failureIdentify string
		want            bool
	}{
		{name: "default success", defaultSuccess: true, method: "test_dry_contact", want: true},
		{name: "matching identify fails", defaultSuccess: true, method: "e2e_forced_failure", failureIdentify: "e2e_forced_failure", want: false},
		{name: "different identify stays successful", defaultSuccess: true, method: "test_dry_contact", failureIdentify: "e2e_forced_failure", want: true},
		{name: "default failure remains failure", defaultSuccess: false, method: "test_dry_contact", failureIdentify: "e2e_forced_failure", want: false},
		{name: "whitespace is normalized", defaultSuccess: true, method: " e2e_forced_failure ", failureIdentify: "e2e_forced_failure", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandResponseSuccess(tt.defaultSuccess, tt.method, tt.failureIdentify); got != tt.want {
				t.Fatalf("commandResponseSuccess(%v, %q, %q) = %v, want %v", tt.defaultSuccess, tt.method, tt.failureIdentify, got, tt.want)
			}
		})
	}
}
