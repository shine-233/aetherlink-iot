// 文件用途：覆盖认证共享密钥唯一规范化入口（authkeys）的契约测试。
// 核心逻辑：验证 secret 文件常见的尾随换行/CRLF/空格被统一去除，空值 fail-closed，
// 且运行期 viper.Set 立即生效（不缓存）。
// 关键注意事项：签发、HTTP/WS 校验、启动检查都依赖这里的规则；改动规则必须同步本测试。

package authkeys

import (
	"errors"
	"testing"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/spf13/viper"
)

const strongKey = "openssl-rand-base64-sample-key-with-48-bytes!!xy"

func setKey(t *testing.T, key, value string) {
	t.Helper()
	old := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() { viper.Set(key, old) })
}

func TestNormalizeStripsSecretFileWhitespace(t *testing.T) {
	for _, raw := range []string{strongKey, strongKey + "\n", strongKey + "\r\n", "  " + strongKey + "\t ", "\n" + strongKey} {
		if got := Normalize(raw); got != strongKey {
			t.Fatalf("Normalize(%q) = %q, want %q", raw, got, strongKey)
		}
	}
	// 内部空白属于密钥本身，不得改动。
	if got := Normalize(" a b\n"); got != "a b" {
		t.Fatalf("Normalize must keep inner whitespace, got %q", got)
	}
}

func TestJWTSigningKeyIsNormalizedAndFailsClosedWhenEmpty(t *testing.T) {
	setKey(t, JWTKeyConfigKey, strongKey+"\n")
	key, err := JWTSigningKey()
	if err != nil || string(key) != strongKey {
		t.Fatalf("JWTSigningKey() = %q, %v; want normalized key", key, err)
	}

	for _, empty := range []string{"", " \n\t"} {
		viper.Set(JWTKeyConfigKey, empty)
		if _, err := JWTSigningKey(); !errors.Is(err, ErrJWTKeyEmpty) {
			t.Fatalf("JWTSigningKey() with %q: err = %v, want ErrJWTKeyEmpty", empty, err)
		}
		if j, err := JWT(); j != nil || !errors.Is(err, ErrJWTKeyEmpty) {
			t.Fatalf("JWT() with %q must fail closed, got (%v, %v)", empty, j, err)
		}
	}
}

// 回归锚点：同一配置值无论尾随空白如何，签出的 token 都能被同一入口构造的校验器验证。
func TestJWTRoundTripWithTrailingNewlineKey(t *testing.T) {
	setKey(t, JWTKeyConfigKey, strongKey+"\r\n")
	signer, err := JWT()
	if err != nil {
		t.Fatalf("JWT(): %v", err)
	}
	token, err := signer.GenerateToken(utils.UserClaims{ID: "u1", Email: "u1@example.com", TenantID: "t1"})
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	verifier, err := JWT()
	if err != nil {
		t.Fatalf("JWT(): %v", err)
	}
	claims, err := verifier.ParseToken(token)
	if err != nil || claims.ID != "u1" {
		t.Fatalf("ParseToken = (%v, %v), want claims for u1", claims, err)
	}
	// 与规范化后的明文密钥构造的校验器等价（证明签名密钥就是 TrimSpace 值）。
	if _, err := utils.NewJWT([]byte(strongKey)).ParseToken(token); err != nil {
		t.Fatalf("token must be signed with the normalized key: %v", err)
	}
}

func TestRuntimeViperChangesTakeEffectImmediately(t *testing.T) {
	setKey(t, JWTKeyConfigKey, strongKey)
	first, _ := JWTSigningKey()
	viper.Set(JWTKeyConfigKey, strongKey+"-rotated")
	second, _ := JWTSigningKey()
	if string(first) == string(second) {
		t.Fatal("JWTSigningKey must re-read config on every call (no stale cache)")
	}
}

func TestPluginServiceKeyIsNormalized(t *testing.T) {
	setKey(t, PluginServiceKeyConfigKey, "plugin-secret\n")
	if got := PluginServiceKey(); got != "plugin-secret" {
		t.Fatalf("PluginServiceKey() = %q, want trimmed value", got)
	}
	viper.Set(PluginServiceKeyConfigKey, " \n")
	if got := PluginServiceKey(); got != "" {
		t.Fatalf("whitespace-only plugin key must read as unset, got %q", got)
	}
}

type mapGetter map[string]string

func (m mapGetter) GetString(key string) string { return m[key] }

func TestFromHelpersUseInjectedSourceAndTolerateNil(t *testing.T) {
	src := mapGetter{JWTKeyConfigKey: " k1 \n", PluginServiceKeyConfigKey: "\tk2\n"}
	if got := JWTKeyFrom(src); got != "k1" {
		t.Fatalf("JWTKeyFrom = %q", got)
	}
	if got := PluginServiceKeyFrom(src); got != "k2" {
		t.Fatalf("PluginServiceKeyFrom = %q", got)
	}
	if JWTKeyFrom(nil) != "" || PluginServiceKeyFrom(nil) != "" {
		t.Fatal("nil source must yield empty key")
	}
}
