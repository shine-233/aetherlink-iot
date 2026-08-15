package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileMapsHyphenatedKeysToUnderscoreEnvironmentNames(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "conf.yml")
	if err := os.WriteFile(configPath, []byte(`
classified-protect:
  login-max-fail-times: 3
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GOTP_CLASSIFIED_PROTECT_LOGIN_MAX_FAIL_TIMES", "7")

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}
	if got := cfg.GetInt("classified-protect.login-max-fail-times"); got != 7 {
		t.Fatalf("classified-protect.login-max-fail-times = %d, want environment override 7", got)
	}
}

func TestLoadConfigFileAllowsExplicitEmptyEnvironmentOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "conf.yml")
	if err := os.WriteFile(configPath, []byte(`
mqtt:
  pass: configured-placeholder
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GOTP_MQTT_PASS", "")

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}
	if got := cfg.GetString("mqtt.pass"); got != "" {
		t.Fatalf("mqtt.pass = %q, want explicit empty environment override", got)
	}
}
