package mqttdebug

import (
	"errors"
	"testing"
)

func TestAuthorizeTopicKeepsDebugInsideDeviceScope(t *testing.T) {
	scope := Scope{TenantID: "tenant-1", UserID: "user-1", DeviceID: "device-1", DeviceNumber: "D-001"}

	if topic, payloadScoped, err := authorizeTopic(scope, "devices/command/D-001/request-1", false); err != nil || topic == "" || payloadScoped {
		t.Fatalf("device publish topic = %q payloadScoped=%t err=%v", topic, payloadScoped, err)
	}
	if topic, payloadScoped, err := authorizeTopic(scope, "devices/telemetry", true); err != nil || topic == "" || !payloadScoped {
		t.Fatalf("shared telemetry subscription = %q payloadScoped=%t err=%v", topic, payloadScoped, err)
	}
	if _, _, err := authorizeTopic(scope, "devices/telemetry", false); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("shared telemetry publish err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "devices/command/OTHER/request-1", false); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("other-device publish err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "devices/attributes/D-001", false); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("shared uplink with matching message id err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "devices/command/+/D-001", true); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("identity outside canonical position err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "arbitrary/D-001/topic", true); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("arbitrary device-looking topic err=%v, want ErrTopicDenied", err)
	}
	if topic, payloadScoped, err := authorizeTopic(scope, "devices/status/device-1", true); err != nil || topic == "" || payloadScoped {
		t.Fatalf("device status subscription = %q payloadScoped=%t err=%v", topic, payloadScoped, err)
	}
	if _, _, err := authorizeTopic(scope, "devices/status/device-1", false); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("device status publish err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "devices/command/device-1/request-1", false); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("downlink using device id err=%v, want ErrTopicDenied", err)
	}
	if topic, payloadScoped, err := authorizeTopic(scope, "devices/attributes/message-1", true); err != nil || topic == "" || !payloadScoped {
		t.Fatalf("specific shared uplink = %q payloadScoped=%t err=%v", topic, payloadScoped, err)
	}
	if _, _, err := authorizeTopic(scope, "$SYS/broker/clients/#", true); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("system topic err=%v, want ErrTopicDenied", err)
	}
	if _, _, err := authorizeTopic(scope, "devices/+suffix", true); !errors.Is(err, ErrInvalidTopic) {
		t.Fatalf("partial wildcard err=%v, want ErrInvalidTopic", err)
	}
}

func TestMQTTTopicFilterMatchesOnlyCanonicalSegments(t *testing.T) {
	if !mqttTopicFilterMatches("devices/attributes/+", "devices/attributes/message-1") {
		t.Fatal("single-level shared uplink filter should match")
	}
	if mqttTopicFilterMatches("devices/attributes/+", "devices/attributes/message-1/extra") {
		t.Fatal("different topic depth must not match")
	}
	if mqttTopicFilterMatches("devices/attributes/+", "devices/event/message-1") {
		t.Fatal("different topic prefix must not match")
	}
}
