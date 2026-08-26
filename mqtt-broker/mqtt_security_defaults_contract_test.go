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

// TestDefaultBrokerConfigKeepsMemoryPersistenceAsFileDefault 锁定
// default_config.yml 的持久化文件默认值为 memory：裸机/本地开发无需 Redis 即可启动；
// Compose 部署通过 GMQTT_PERSISTENCE_* 环境变量显式切换到 Redis 后端（见
// cmd/gmqttd/command/persistence_env.go 与 docker-compose.yml）。
// 若要把文件默认改为 redis，必须同步提供静态可用的地址与口令方案，并更新本测试。
func TestDefaultBrokerConfigKeepsMemoryPersistenceAsFileDefault(t *testing.T) {
	parsed, err := brokerconfig.ParseConfig("cmd/gmqttd/default_config.yml")
	if err != nil {
		t.Fatalf("parse default broker config: %v", err)
	}
	if parsed.Persistence.Type != brokerconfig.PersistenceTypeMemory {
		t.Errorf("file-default persistence.type = %q, want %q (deployment opts into redis via GMQTT_PERSISTENCE_TYPE env)",
			parsed.Persistence.Type, brokerconfig.PersistenceTypeMemory)
	}
}
