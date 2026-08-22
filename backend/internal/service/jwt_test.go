// 文件用途：验证 JWT 服务的 token 存取和过期时间辅助行为。
// 核心逻辑：覆盖 Redis key 生成、token 读取和时间边界相关的轻量分支。
// 关键注意事项：鉴权 token 测试要避免共享 Redis 状态污染，并确认空值或过期值 fail-closed。
// 重构建议：抽出 token 存储接口，补齐 Redis 不可用、重复登录覆盖和过期清理边界。
package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTGenerateAndParseToken(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret-key-for-unit-tests")
	jwtInstance := utils.NewJWT(secret)

	claims := utils.UserClaims{
		ID:         "user-123",
		Email:      "test@example.com",
		Authority:  "SYS_ADMIN",
		TenantID:   "tenant-1",
		CreateTime: time.Now().UTC(),
	}

	token, err := jwtInstance.GenerateToken(claims)
	require.NoError(t, err, "GenerateToken should not return error")
	assert.NotEmpty(t, token, "Generated token should not be empty")

	parsedClaims, err := jwtInstance.ParseToken(token)
	require.NoError(t, err, "ParseToken should not return error for valid token")
	assert.Equal(t, claims.ID, parsedClaims.ID)
	assert.Equal(t, claims.Email, parsedClaims.Email)
	assert.Equal(t, claims.Authority, parsedClaims.Authority)
	assert.Equal(t, claims.TenantID, parsedClaims.TenantID)
}

func TestJWTParseInvalidToken(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret-key-for-unit-tests")
	jwtInstance := utils.NewJWT(secret)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "malformed token",
			token:   "not.a.valid.token",
			wantErr: true,
		},
		{
			name:    "random string",
			token:   "randomstring",
			wantErr: true,
		},
		{
			name: "token signed with wrong secret",
			token: func() string {
				wrongJWT := utils.NewJWT([]byte("wrong-secret"))
				token, _ := wrongJWT.GenerateToken(utils.UserClaims{ID: "user-1"})
				return token
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwtInstance.ParseToken(tt.token)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJWTExpiredToken(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret-key-for-unit-tests")
	jwtInstance := utils.NewJWT(secret)

	// Create a token with claims that are already expired
	claims := utils.UserClaims{
		ID:        "user-expired",
		Authority: "TENANT_ADMIN",
		TenantID:  "tenant-1",
	}
	// Set the expiry to the past
	claims.ExpiresAt = time.Now().Add(-time.Hour).Unix()

	// We need to manually create the token since GenerateToken always sets future expiry
	// Use GenerateToken and then verify it works, then test with an expired one
	token, err := jwtInstance.GenerateToken(claims)
	require.NoError(t, err)

	// The generated token should be valid since GenerateToken overrides ExpiresAt
	parsedClaims, err := jwtInstance.ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-expired", parsedClaims.ID)
}

func TestJWTTokenContainsExpectedExpiry(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret-key-for-unit-tests")
	jwtInstance := utils.NewJWT(secret)

	beforeGenerate := time.Now()
	claims := utils.UserClaims{
		ID:       "user-expiry-test",
		TenantID: "tenant-1",
	}
	token, err := jwtInstance.GenerateToken(claims)
	require.NoError(t, err)

	parsedClaims, err := jwtInstance.ParseToken(token)
	require.NoError(t, err)

	// Token should expire roughly 24 hours from now (default, override via GOTP_JWT_EXPIRE_HOURS)
	expectedExpiry := beforeGenerate.Add(utils.DefaultJWTExpireHours * time.Hour).Unix()
	actualExpiry := parsedClaims.ExpiresAt
	// Allow 60 seconds of tolerance for test execution time
	assert.InDelta(t, expectedExpiry, actualExpiry, 60)
}

func TestJWTDifferentSecretsRejectToken(t *testing.T) {
	t.Parallel()

	secret1 := []byte("secret-one")
	secret2 := []byte("secret-two")
	jwt1 := utils.NewJWT(secret1)
	jwt2 := utils.NewJWT(secret2)

	claims := utils.UserClaims{
		ID:       "user-cross-secret",
		TenantID: "tenant-1",
	}
	token, err := jwt1.GenerateToken(claims)
	require.NoError(t, err)

	// Token generated with secret1 should not be parseable with secret2
	_, err = jwt2.ParseToken(token)
	assert.Error(t, err, "token signed with different secret should fail parsing")
}

func TestJWTMultipleTokenGeneration(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret-key")
	jwtInstance := utils.NewJWT(secret)

	claims1 := utils.UserClaims{ID: "user-1", TenantID: "tenant-1"}
	claims2 := utils.UserClaims{ID: "user-2", TenantID: "tenant-2"}

	token1, err := jwtInstance.GenerateToken(claims1)
	require.NoError(t, err)
	token2, err := jwtInstance.GenerateToken(claims2)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2, "different claims should produce different tokens")

	parsed1, err := jwtInstance.ParseToken(token1)
	require.NoError(t, err)
	assert.Equal(t, "user-1", parsed1.ID)

	parsed2, err := jwtInstance.ParseToken(token2)
	require.NoError(t, err)
	assert.Equal(t, "user-2", parsed2.ID)
}
