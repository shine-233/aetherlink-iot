package gmqtt

import (
	"testing"

	brokerconfig "github.com/DrmagicE/gmqtt/config"
)

const (
	defaultMaxMQTTPacketSize = 1024 * 1024
	defaultReceiveMaximum    = 100
	defaultMaxQueuedMessages = 10000
	defaultMaxInflight       = 100
)

// TestDefaultBrokerConfigBoundsUntrustedWork protects the deployed
// configuration from reverting to MQTT's protocol maximum or oversized
// per-client queues. These limits must remain compatible with the default
// Compose container memory budget; larger deployments can override them only
// after validating payload size, client count, and retained/inflight traffic.
func TestDefaultBrokerConfigBoundsUntrustedWork(t *testing.T) {
	parsed, err := brokerconfig.ParseConfig("cmd/gmqttd/default_config.yml")
	if err != nil {
		t.Fatalf("parse default broker config: %v", err)
	}

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"max_packet_size", int(parsed.MQTT.MaxPacketSize), defaultMaxMQTTPacketSize},
		{"server_receive_maximum", int(parsed.MQTT.ReceiveMax), defaultReceiveMaximum},
		{"max_queued_messages", parsed.MQTT.MaxQueuedMsg, defaultMaxQueuedMessages},
		{"max_inflight", int(parsed.MQTT.MaxInflight), defaultMaxInflight},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("default mqtt %s = %d, want %d", check.name, check.got, check.want)
		}
	}

	defaults := brokerconfig.DefaultMQTTConfig
	if defaults.MaxPacketSize != parsed.MQTT.MaxPacketSize ||
		defaults.ReceiveMax != parsed.MQTT.ReceiveMax ||
		defaults.MaxQueuedMsg != parsed.MQTT.MaxQueuedMsg ||
		defaults.MaxInflight != parsed.MQTT.MaxInflight {
		t.Error("code and deployed MQTT resource-budget defaults must remain aligned")
	}
}
