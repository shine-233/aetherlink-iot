package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `device_type: direct
mqtt:
  broker: 127.0.0.1:1883
  client_id: file-client
  username: file-user
  password: file-password
device:
  device_id: file-device
  device_number: file-number
database:
  host: 127.0.0.1
  port: 5432
  dbname: file-db
  username: file-db-user
  password: file-db-password
  sslmode: disable
api:
  base_url: http://127.0.0.1:9999
  api_key: file-api-key
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}

func TestLoadAppliesLocalIntegrationEnvironmentOverrides(t *testing.T) {
	t.Setenv("AUTOTEST_MQTT_SERVER", "127.0.0.1")
	t.Setenv("AUTOTEST_MQTT_PORT", "1884")
	t.Setenv("AUTOTEST_MQTT_CLIENT_ID", "env-client")
	t.Setenv("AUTOTEST_MQTT_USERNAME", "env-user")
	t.Setenv("AUTOTEST_MQTT_PASSWORD", "env-password")
	t.Setenv("AUTOTEST_DEVICE_ID", "env-device")
	t.Setenv("AUTOTEST_DEVICE_NUMBER", "env-number")
	t.Setenv("AUTOTEST_API_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("AUTOTEST_API_KEY", "env-api-key")
	t.Setenv("AUTOTEST_COMMAND_FAILURE_IDENTIFY", "e2e_forced_failure")
	t.Setenv("AUTOTEST_DATABASE_PORT", "5433")
	t.Setenv("AUTOTEST_DATABASE_PASSWORD", "env-db-password")

	cfg, err := Load(writeConfigFixture(t))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MQTT.Broker != "127.0.0.1:1884" || cfg.MQTT.ClientID != "env-client" {
		t.Fatalf("MQTT overrides not applied: %#v", cfg.MQTT)
	}
	if cfg.MQTT.Username != "env-user" || cfg.MQTT.Password != "env-password" {
		t.Fatalf("MQTT credentials not applied: %#v", cfg.MQTT)
	}
	if cfg.Device.DeviceID != "env-device" || cfg.Device.DeviceNumber != "env-number" {
		t.Fatalf("device overrides not applied: %#v", cfg.Device)
	}
	if cfg.Database.Port != 5433 || cfg.Database.Password != "env-db-password" {
		t.Fatalf("database overrides not applied: %#v", cfg.Database)
	}
	if cfg.API.APIKey != "env-api-key" {
		t.Fatalf("API key override not applied: %#v", cfg.API)
	}
	if cfg.Test.CommandFailureIdentify != "e2e_forced_failure" {
		t.Fatalf("command failure identify override not applied: %#v", cfg.Test.CommandFailureIdentify)
	}
}

func TestLoadRejectsInvalidEnvironmentPort(t *testing.T) {
	t.Setenv("AUTOTEST_MQTT_SERVER", "127.0.0.1")
	t.Setenv("AUTOTEST_MQTT_PORT", "not-a-port")

	if _, err := Load(writeConfigFixture(t)); err == nil {
		t.Fatal("expected invalid MQTT environment port to fail")
	}
}
