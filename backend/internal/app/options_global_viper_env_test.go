package app

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// 回归：secrets.master_keys.<id> / market.bundle_signing_keys.<id> 在 conf.yml 里是
// 空 map，没有任何叶子键，因此不会出现在 config.AllKeys() 里，也就无法被 WithConfig
// 的搬运循环带进全局 viper。而 pkg/secrets 与 service/device_template_market_integrity
// 读的正是全局 viper——必须让全局 viper 自己也具备环境变量解析能力，否则这两组密钥
// 恒为空，写入一律 fail closed（POST /api/v1/secrets → 100000；导出资源包 → 100002）。
func TestWithConfigPropagatesEnvOnlyNestedKeys(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	const masterKey = "t3Qz9EGGieVxP55ENLCoLCFd/GZss6RgoWCcLfi5ohI="
	const signingKey = "4HuZUgnjNn9OY2Gs2p/HYideeMxLRRQMen64LdPSyK0wCMr/HXl3alANaAJ3s1jl"

	t.Setenv("GOTP_SECRETS_ACTIVE_KEY_ID", "k1")
	t.Setenv("GOTP_SECRETS_MASTER_KEYS_K1", masterKey)
	t.Setenv("GOTP_MARKET_ACTIVE_BUNDLE_SIGNING_KEY_ID", "mk1")
	t.Setenv("GOTP_MARKET_BUNDLE_SIGNING_KEYS_MK1", signingKey)

	config, err := LoadConfigFile("../../configs/conf.yml")
	if err != nil {
		t.Fatalf("LoadConfigFile(conf.yml): %v", err)
	}

	// 前置条件：这两组键确实不在 AllKeys() 里（空 map 没有叶子键）。
	// 若哪天 conf.yml 把它们展开成了叶子键，这个测试就失去意义了。
	for _, key := range config.AllKeys() {
		if strings.HasPrefix(key, "secrets.master_keys") ||
			strings.HasPrefix(key, "market.bundle_signing_keys") {
			t.Fatalf("前置条件不成立：%q 出现在了 AllKeys() 里", key)
		}
	}

	if err := WithConfig(config)(&Application{}); err != nil {
		t.Fatalf("WithConfig: %v", err)
	}

	global := viper.GetViper()

	if got := global.GetString("secrets.active_key_id"); got != "k1" {
		t.Errorf("secrets.active_key_id = %q, want %q", got, "k1")
	}
	if got := global.GetString("secrets.master_keys.k1"); got != masterKey {
		t.Errorf("secrets.master_keys.k1 = %q, want the env-provided key", got)
	}
	if got := global.GetString("market.bundle_signing_keys.mk1"); got != signingKey {
		t.Errorf("market.bundle_signing_keys.mk1 = %q, want the env-provided key", got)
	}
}
