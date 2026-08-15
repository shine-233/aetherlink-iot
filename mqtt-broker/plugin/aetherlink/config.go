// 文件用途：定义 AetherLink broker 插件配置边界与会话撤销 broker 身份校验。
// 核心逻辑：对 aetherlink.yml 中显式配置的 broker_id 做规范化和 fail-fast 校验。
// 关键注意事项：broker_id 是处理 ACK 的稳定身份，不得回退到 hostname 或随机值。

package aetherlink

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	mqttSessionRevocationBrokerIDConfigKey = "mqtt_session_revocations.broker_id"
	mqttSessionRevocationBrokerIDMaxLength = 128
	postgresSSLModeConfigKey               = "db.psql.sslmode"
	defaultPostgresSSLMode                 = "disable"
)

var (
	mqttSessionRevocationBrokerIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
	postgresSSLModes                     = map[string]struct{}{
		"disable":     {},
		"allow":       {},
		"prefer":      {},
		"require":     {},
		"verify-ca":   {},
		"verify-full": {},
	}
)

// Config is the AetherLink IoT plugin configuration.
// Runtime settings are loaded from aetherlink.yml.
type Config struct {
}

// Validate accepts the daemon-level plugin block. Runtime settings, including
// the required MQTT session-revocation broker identity, live in aetherlink.yml
// and are validated after that file is loaded.
func (c *Config) Validate() error {
	return nil
}

// DefaultConfig is the plugin default configuration.
var DefaultConfig = Config{}

// UnmarshalYAML satisfies yaml.Unmarshaler for the daemon-level plugin block.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return nil
}

func normalizePostgresSSLMode(raw string) (string, error) {
	sslMode := strings.ToLower(strings.TrimSpace(raw))
	if sslMode == "" {
		return defaultPostgresSSLMode, nil
	}
	if _, ok := postgresSSLModes[sslMode]; !ok {
		return "", fmt.Errorf(
			"%s must be one of disable, allow, prefer, require, verify-ca, or verify-full",
			postgresSSLModeConfigKey,
		)
	}
	return sslMode, nil
}

func normalizeMQTTSessionRevocationBrokerID(raw string) (string, error) {
	brokerID := strings.TrimSpace(raw)
	if brokerID == "" {
		return "", fmt.Errorf("%s is required", mqttSessionRevocationBrokerIDConfigKey)
	}
	if len(brokerID) > mqttSessionRevocationBrokerIDMaxLength {
		return "", fmt.Errorf(
			"%s must be at most %d characters",
			mqttSessionRevocationBrokerIDConfigKey,
			mqttSessionRevocationBrokerIDMaxLength,
		)
	}
	if !mqttSessionRevocationBrokerIDPattern.MatchString(brokerID) {
		return "", fmt.Errorf(
			"%s may contain only letters, digits, dot, underscore, colon, and hyphen",
			mqttSessionRevocationBrokerIDConfigKey,
		)
	}
	return brokerID, nil
}
