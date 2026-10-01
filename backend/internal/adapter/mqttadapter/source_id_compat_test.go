package mqttadapter

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// legacyMQTTUplinkSourceID is the original implementation; the source id is a
// persisted idempotency key, so the optimized version must match it exactly.
func legacyMQTTUplinkSourceID(tenantID, deviceID, dataType, messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ""
	}
	material, _ := json.Marshal(struct {
		Version   int    `json:"version"`
		TenantID  string `json:"tenant_id"`
		DeviceID  string `json:"device_id"`
		DataType  string `json:"data_type"`
		MessageID string `json:"message_id"`
	}{1, tenantID, deviceID, dataType, messageID})
	sum := sha256.Sum256(material)
	return fmt.Sprintf("%x", sum[:])
}

func TestMQTTUplinkSourceIDMatchesLegacyEncoding(t *testing.T) {
	odd := []string{"", "plain", "a\"b", `back\slash`, "<tag>&", "中文", "tab\tnl\n", "\xff", strings.Repeat("x", 300)}
	for _, tenant := range odd {
		for _, device := range odd {
			for _, msg := range []string{"m-1", " m-2 ", "", "q\"x", "中"} {
				got := mqttUplinkSourceID(tenant, device, "telemetry", msg)
				want := legacyMQTTUplinkSourceID(tenant, device, "telemetry", msg)
				if got != want {
					t.Fatalf("(%q,%q,%q): got %s want %s", tenant, device, msg, got, want)
				}
			}
		}
	}
}

func TestParseAttributeOrEventTopicSegments(t *testing.T) {
	a := benchAdapter()
	cases := map[string]string{
		"devices/attributes/msg-1":       "msg-1",
		"devices/event/msg-2/extra/more": "msg-2",
		"gateway/attributes/m":           "m",
	}
	for topic, want := range cases {
		got, err := a.parseAttributeOrEventTopic(topic)
		if err != nil || got != want {
			t.Fatalf("%q: got %q, %v; want %q", topic, got, err, want)
		}
	}
	for _, topic := range []string{"devices/attributes", "devices", "", "devices/attributes/", "devices/attributes//x"} {
		if _, err := a.parseAttributeOrEventTopic(topic); err == nil {
			t.Fatalf("%q: expected error", topic)
		}
	}
}
