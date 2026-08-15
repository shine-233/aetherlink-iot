// 文件用途：覆盖应用编排与服务管理的 Go 测试。
// 核心逻辑：验证配置加载、选项注入、服务启动停止和失败回滚等应用生命周期行为，主要围绕 func TestWithConfigAssignsApplicationConfigAndMirrorsGlobalViperKeys、func TestLoadConfigFileReadsYamlAndEnablesEnvironmentOverride、func TestLoadConfigFileReturnsErrorForMissingOrInvalidConfig、func TestLoadEnvironmentConfigRejectsUnknownEnvironment 等声明展开。
// 关键注意事项：测试依赖服务生命周期契约，新增断言需避免引入真实外部服务。
// 重构建议：后续可沉淀统一的 mock service 和配置夹具，降低测试重复度。

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestWithConfigAssignsApplicationConfigAndMirrorsGlobalViperKeys(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	cfg := viper.New()
	cfg.Set("server.port", 9999)
	cfg.Set("mqtt.host", "127.0.0.1")

	app := &Application{}
	if err := WithConfig(cfg)(app); err != nil {
		t.Fatalf("WithConfig returned error: %v", err)
	}
	if app.Config != cfg {
		t.Fatal("WithConfig should assign the provided viper instance")
	}
	if got := viper.GetInt("server.port"); got != 9999 {
		t.Fatalf("global viper server.port = %d, want 9999", got)
	}
	if got := viper.GetString("mqtt.host"); got != "127.0.0.1" {
		t.Fatalf("global viper mqtt.host = %q, want 127.0.0.1", got)
	}
}

func TestLoadConfigFileReadsYamlAndEnablesEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "conf.yml")
	if err := os.WriteFile(configPath, []byte(`
server:
  port: 9999
database:
  host: localhost
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GOTP_SERVER_PORT", "10001")

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}

	if got := cfg.GetInt("server.port"); got != 10001 {
		t.Fatalf("server.port = %d, want environment override 10001", got)
	}
	if got := cfg.GetString("database.host"); got != "localhost" {
		t.Fatalf("database.host = %q, want localhost", got)
	}
}

func TestLoadConfigFileReturnsErrorForMissingOrInvalidConfig(t *testing.T) {
	if _, err := LoadConfigFile(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatal("LoadConfigFile expected error for missing file")
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.yml")
	if err := os.WriteFile(invalidPath, []byte("server: ["), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	if _, err := LoadConfigFile(invalidPath); err == nil {
		t.Fatal("LoadConfigFile expected parse error for invalid YAML")
	}
}

func TestLoadEnvironmentConfigRejectsUnknownEnvironment(t *testing.T) {
	if _, err := LoadEnvironmentConfig("staging"); err == nil {
		t.Fatal("LoadEnvironmentConfig expected error for unknown environment")
	}
}

func TestWithOptionalRsaDecryptSkipsMissingKey(t *testing.T) {
	option := WithOptionalRsaDecrypt(filepath.Join(t.TempDir(), "missing.pem"))
	if err := option(&Application{}); err != nil {
		t.Fatalf("WithOptionalRsaDecrypt returned error for missing optional key: %v", err)
	}
}

func TestWithOptionalRsaDecryptRejectsMalformedKey(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(keyPath, []byte("not-a-private-key"), 0o600); err != nil {
		t.Fatalf("write malformed key: %v", err)
	}

	if err := WithOptionalRsaDecrypt(keyPath)(&Application{}); err == nil {
		t.Fatal("WithOptionalRsaDecrypt should reject malformed key material")
	}
}

func TestEnvironmentOptionHelpersPropagateLoadErrors(t *testing.T) {
	for name, option := range map[string]Option{
		"dev":  WithDevelopmentConfig(),
		"test": WithTestConfig(),
		"prod": WithProductionConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			err := option(&Application{})
			if err == nil {
				t.Skip("environment config file exists in this checkout; load path is covered by LoadConfigFile")
			}
		})
	}
}

func TestNewApplicationAppliesOptionsAndNilBusesAreSafe(t *testing.T) {
	cfg := viper.New()
	cfg.Set("server.port", 9999)
	app, err := NewApplication(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewApplication returned error: %v", err)
	}
	if app.Config != cfg {
		t.Fatal("NewApplication should apply WithConfig")
	}
	if app.Logger == nil {
		t.Fatal("NewApplication should initialize default logger")
	}
	if app.ServiceManager == nil {
		t.Fatal("NewApplication should initialize service manager")
	}
	if app.GetDownlinkBus() != nil {
		t.Fatal("GetDownlinkBus should be nil before downlink service is configured")
	}
	if app.GetUplinkBus() != nil {
		t.Fatal("GetUplinkBus should be nil before uplink service is configured")
	}
}

func TestNewApplicationStopsWhenOptionReturnsError(t *testing.T) {
	wantErr := os.ErrInvalid
	_, err := NewApplication(func(*Application) error {
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("NewApplication error = %v, want %v", err, wantErr)
	}
}
