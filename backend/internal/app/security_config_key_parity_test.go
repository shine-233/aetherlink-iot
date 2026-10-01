// 文件用途：启动期 JWT 密钥校验与运行期签发/校验密钥同源的契约测试。
// 核心逻辑：启动检查看到的密钥必须与 authkeys.JWTSigningKey 运行期使用的字节完全一致，
// 否则会出现"启动检查通过、运行期全量 invalid token"的分叉。

package app

import (
	"testing"

	"aetherlink-iot/backend/pkg/authkeys"

	"github.com/spf13/viper"
)

func TestSecurityConfigJWTKeyMatchesRuntimeSigningKey(t *testing.T) {
	const strong = "openssl-rand-base64-sample-key-with-48-bytes!!xy"
	old := viper.Get(authkeys.JWTKeyConfigKey)
	t.Cleanup(func() { viper.Set(authkeys.JWTKeyConfigKey, old) })

	for _, raw := range []string{strong + "\n", strong + "\r\n", " " + strong + "\t"} {
		v := viper.New()
		v.Set(authkeys.JWTKeyConfigKey, raw)
		if err := validateSecurityCriticalConfig(v); err != nil {
			t.Fatalf("startup check rejected %q: %v", raw, err)
		}
		// 启动检查读取的规范化值 == 运行期签名密钥。
		viper.Set(authkeys.JWTKeyConfigKey, raw)
		runtimeKey, err := authkeys.JWTSigningKey()
		if err != nil {
			t.Fatalf("runtime key for %q: %v", raw, err)
		}
		if got := authkeys.JWTKeyFrom(v); got != string(runtimeKey) || got != strong {
			t.Fatalf("startup key %q != runtime key %q", got, runtimeKey)
		}
	}

	// 仅空白的密钥：启动检查与运行期都必须判定为空。
	v := viper.New()
	v.Set(authkeys.JWTKeyConfigKey, " \n")
	if err := validateSecurityCriticalConfig(v); err == nil {
		t.Fatal("whitespace-only jwt.key must fail startup validation")
	}
}
