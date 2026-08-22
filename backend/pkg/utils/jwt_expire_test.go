// 文件用途：覆盖 GenerateToken 有效期的默认值与环境变量覆盖行为。
// 核心逻辑：默认 24 小时；GOTP_JWT_EXPIRE_HOURS（viper 键 jwt.expire_hours）可上调或下调，非法值回落默认。
// 关键注意事项：t.Setenv 与 viper 全局状态不能用于并行测试，本文件用例保持串行。
// 重构建议：若有效期配置项继续增多，抽统一的配置读取 helper 并集中测试。

package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func setupJWTExpireEnvLookup(t *testing.T) {
	t.Helper()

	viper.SetEnvPrefix("GOTP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func TestGenerateTokenDefaultsTo24HourExpiry(t *testing.T) {
	generatedAt := time.Now()
	j := NewJWT([]byte("unit-test-secret"))
	token, err := j.GenerateToken(UserClaims{ID: "expiry-default-user"})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	parsed, err := j.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}

	expectedExpiry := generatedAt.Add(DefaultJWTExpireHours * time.Hour).Unix()
	if diff := parsed.ExpiresAt - expectedExpiry; diff > 60 || diff < -60 {
		t.Fatalf("ExpiresAt = %d, want within 60s of %d (default %dh)", parsed.ExpiresAt, expectedExpiry, DefaultJWTExpireHours)
	}
}

func TestGenerateTokenHonorsExpireHoursOverride(t *testing.T) {
	setupJWTExpireEnvLookup(t)
	t.Setenv("GOTP_JWT_EXPIRE_HOURS", "48")

	generatedAt := time.Now()
	j := NewJWT([]byte("unit-test-secret"))
	token, err := j.GenerateToken(UserClaims{ID: "expiry-override-user"})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	parsed, err := j.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}

	expectedExpiry := generatedAt.Add(48 * time.Hour).Unix()
	if diff := parsed.ExpiresAt - expectedExpiry; diff > 60 || diff < -60 {
		t.Fatalf("ExpiresAt = %d, want within 60s of %d (48h override)", parsed.ExpiresAt, expectedExpiry)
	}
}

func TestGenerateTokenFallsBackToDefaultOnInvalidOverride(t *testing.T) {
	setupJWTExpireEnvLookup(t)
	t.Setenv("GOTP_JWT_EXPIRE_HOURS", "not-a-number")

	generatedAt := time.Now()
	j := NewJWT([]byte("unit-test-secret"))
	token, err := j.GenerateToken(UserClaims{ID: "expiry-invalid-user"})
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	parsed, err := j.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}

	expectedExpiry := generatedAt.Add(DefaultJWTExpireHours * time.Hour).Unix()
	if diff := parsed.ExpiresAt - expectedExpiry; diff > 60 || diff < -60 {
		t.Fatalf("ExpiresAt = %d, want default %d expiry for invalid override", parsed.ExpiresAt, DefaultJWTExpireHours)
	}
}
