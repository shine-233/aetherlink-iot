package app

import (
	"os"
	"path/filepath"
	"testing"

	"aetherlink-iot/backend/pkg/secrets"

	"github.com/spf13/viper"
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

// TestWithConfigPropagatesEnvironmentOverridesForEmptyMapKeys 锁住一个曾经把
// Compose 定时泳道长期打红的契约缺口。
//
// 背景：conf.yml 把几组 fail-closed 的密钥族声明成**空 map**，例如
// `secrets.master_keys: {}` 与 `market.bundle_signing_keys: {}`。真实密钥只能来自
// GOTP_* 环境变量。但 WithConfig 是靠 config.AllKeys() 逐键 viper.Set 拷贝到全局
// 单例的，而 AllKeys() 永远不会展开空 map 下的嵌套键——于是这些键既没有 override，
// 全局单例又从未开启 AutomaticEnv（initialize.ViperInit 只被 cmd/gen 调用）。
// 结果：环境变量明明注入了容器，pkg/secrets 通过全局 viper 仍然读不到主密钥，
// 信封加密 fail closed，POST /api/v1/secrets 一律返回 100000。
//
// 因此这里断言的不只是"键能读到"，而是"真实读取方 pkg/secrets 能完成一次加解密闭环"。
func TestWithConfigPropagatesEnvironmentOverridesForEmptyMapKeys(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configPath := filepath.Join(t.TempDir(), "conf.yml")
	if err := os.WriteFile(configPath, []byte(`
secrets:
  active_key_id: ""
  master_keys: {}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 必须是标准 base64（含 padding）的 32 字节 AES-256 密钥。
	const masterKey = "OaSR2E32/uPUbfQ0zwfGh0WWlyuEXtqcKSdkmc3Vm04="
	t.Setenv("GOTP_SECRETS_ACTIVE_KEY_ID", "k1")
	t.Setenv("GOTP_SECRETS_MASTER_KEYS_K1", masterKey)

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}
	// 子实例本身读得到（环境变量映射是好的）；缺口只出在往全局单例的拷贝上。
	if got := cfg.GetString("secrets.master_keys.k1"); got != masterKey {
		t.Fatalf("child viper secrets.master_keys.k1 = %q, want %q", got, masterKey)
	}
	if err := WithConfig(cfg)(&Application{}); err != nil {
		t.Fatalf("WithConfig returned error: %v", err)
	}

	if got := viper.GetString("secrets.master_keys.k1"); got != masterKey {
		t.Fatalf("global viper secrets.master_keys.k1 = %q, want %q (empty-map nested keys are invisible to AllKeys)", got, masterKey)
	}

	id, err := secrets.ActiveKeyID()
	if err != nil || id != "k1" {
		t.Fatalf("secrets.ActiveKeyID() = %q, %v; want %q, nil", id, err, "k1")
	}

	sealed, err := secrets.Seal("regression-value", "tenant-regression")
	if err != nil {
		t.Fatalf("secrets.Seal returned error: %v", err)
	}
	plain, err := secrets.Open(sealed, "tenant-regression")
	if err != nil {
		t.Fatalf("secrets.Open returned error: %v", err)
	}
	if plain != "regression-value" {
		t.Fatalf("envelope round-trip = %q, want %q", plain, "regression-value")
	}
}
