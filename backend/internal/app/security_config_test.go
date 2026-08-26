// 文件用途：覆盖启动期安全关键配置校验的行为契约。
// 核心逻辑：验证 JWT 密钥的空值、已知占位符、弱长度与合法值四类路径，以及 NewApplication 装配层的 fail-fast 行为。
// 关键注意事项：仓库自带的 configs/conf.yml、conf-dev.yml 出厂值均为占位符，必须被拒绝；Compose 部署依赖 GOTP_JWT_KEY 环境注入。
// 重构建议：新增安全关键配置项（数据库口令等）时在本文件补对应表驱动用例。
package app

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateJWTSigningKeyAcceptsStrongKey(t *testing.T) {
	if err := validateJWTSigningKey("openssl-rand-base64-sample-key-with-48-bytes!!"); err != nil {
		t.Fatalf("validateJWTSigningKey returned error for strong key: %v", err)
	}
}

func TestValidateJWTSigningKeyTrimsSurroundingWhitespace(t *testing.T) {
	key := "  " + strings.Repeat("k", minJWTKeyLength) + "  "
	if err := validateJWTSigningKey(key); err != nil {
		t.Fatalf("validateJWTSigningKey should trim whitespace before validation: %v", err)
	}
}

func TestValidateJWTSigningKeyRejectsEmptyAndBlankKeys(t *testing.T) {
	for name, key := range map[string]string{
		"empty":     "",
		"blank":     "   ",
		"env-empty": " ",
	} {
		t.Run(name, func(t *testing.T) {
			err := validateJWTSigningKey(key)
			if err == nil {
				t.Fatal("expected error for empty JWT signing key")
			}
			if !strings.Contains(err.Error(), "GOTP_JWT_KEY") {
				t.Fatalf("error should contain the fix hint, got: %v", err)
			}
		})
	}
}

func TestValidateJWTSigningKeyRejectsShippedPlaceholders(t *testing.T) {
	// 与 backend/configs/conf.yml、conf.example.yml、conf-dev.yml 的出厂值保持同步。
	for _, key := range []string{
		"CHANGE_ME_JWT_SECRET",
		"CHANGE_ME_DEV_JWT_SECRET",
		"changeme",
		"a-strong-looking-key-with-CHANGE_ME-inside-0123456789",
	} {
		t.Run(key, func(t *testing.T) {
			if err := validateJWTSigningKey(key); err == nil {
				t.Fatalf("placeholder %q must be rejected at startup", key)
			}
		})
	}
}

func TestValidateJWTSigningKeyRejectsShortKeys(t *testing.T) {
	if err := validateJWTSigningKey(strings.Repeat("k", minJWTKeyLength-1)); err == nil {
		t.Fatal("short JWT signing keys must be rejected")
	}
	if err := validateJWTSigningKey(strings.Repeat("k", minJWTKeyLength)); err != nil {
		t.Fatalf("key at the length boundary should pass: %v", err)
	}
}

func TestValidateSecurityCriticalConfigReadsViperJWTKey(t *testing.T) {
	v := viper.New()
	v.Set("jwt.key", "CHANGE_ME_JWT_SECRET")
	if err := validateSecurityCriticalConfig(v); err == nil {
		t.Fatal("placeholder jwt.key in viper config must fail startup validation")
	}

	v.Set("jwt.key", strings.Repeat("s", 64))
	if err := validateSecurityCriticalConfig(v); err != nil {
		t.Fatalf("valid viper jwt.key should pass: %v", err)
	}
}

func TestNewApplicationRejectsPlaceholderJWTKeyAtAssembly(t *testing.T) {
	cfg := viper.New()
	cfg.Set("jwt.key", "CHANGE_ME_JWT_SECRET")
	if _, err := NewApplication(WithConfig(cfg)); err == nil {
		t.Fatal("NewApplication must refuse placeholder JWT signing keys (fail-fast)")
	}
}
