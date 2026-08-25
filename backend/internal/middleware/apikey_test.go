// 文件用途：覆盖 API key claims 映射等仍在当前请求链中使用的中间件辅助逻辑。
// 核心逻辑：验证 openAPIKeyClaims 是否把租户、创建者和权限映射到统一 claims 结构，并覆盖 GOTP_OPENAPI_KEY_AUTHORITY 下调能力。
// 关键注意事项：open_api_keys 表没有独立权限字段，权限来自全局配置；测试需在结束后恢复环境变量，避免污染其他用例。
// 重构建议：如果后续为 API Key 增加独立 scope 字段，应改为按字段授权并删除全局配置回退。

package middleware

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/constant"

	"github.com/spf13/viper"
)

func TestOpenAPIKeyClaimsPreservesTenantCreatorAndDefaultTenantUserAuthority(t *testing.T) {
	claims := openAPIKeyClaims("tenant-from-key", "creator-from-key")

	if claims == nil {
		t.Fatal("openAPIKeyClaims returned nil")
	}
	if claims.TenantID != "tenant-from-key" {
		t.Fatalf("TenantID = %q, want tenant-from-key", claims.TenantID)
	}
	if claims.ID != "creator-from-key" {
		t.Fatalf("ID = %q, want creator-from-key", claims.ID)
	}
	if claims.Authority != constant.TENANT_USER {
		t.Fatalf("Authority = %q, want default %q", claims.Authority, constant.TENANT_USER)
	}
}

func TestOpenAPIKeyAuthorityDefaultsToTenantUserWithoutConfig(t *testing.T) {
	if got := openAPIKeyAuthority(); got != constant.TENANT_USER {
		t.Fatalf("openAPIKeyAuthority() = %q, want default %q", got, constant.TENANT_USER)
	}
}

func TestOpenAPIKeyAuthorityHonorsEnvOverride(t *testing.T) {
	setupOpenAPIKeyAuthorityEnv(t)
	t.Setenv("GOTP_OPENAPI_KEY_AUTHORITY", "TENANT_ADMIN")

	if got := openAPIKeyAuthority(); got != "TENANT_ADMIN" {
		t.Fatalf("openAPIKeyAuthority() = %q, want TENANT_ADMIN from GOTP_OPENAPI_KEY_AUTHORITY", got)
	}

	claims := openAPIKeyClaims("tenant-env", "creator-env")
	if claims.Authority != "TENANT_ADMIN" {
		t.Fatalf("openAPIKeyClaims Authority = %q, want TENANT_ADMIN", claims.Authority)
	}
}

func TestOpenAPIKeyAuthorityFallsBackWhenOverrideIsBlank(t *testing.T) {
	setupOpenAPIKeyAuthorityEnv(t)
	t.Setenv("GOTP_OPENAPI_KEY_AUTHORITY", "   ")

	if got := openAPIKeyAuthority(); got != constant.TENANT_USER {
		t.Fatalf("openAPIKeyAuthority() = %q, want fallback %q for blank override", got, constant.TENANT_USER)
	}
}

func setupOpenAPIKeyAuthorityEnv(t *testing.T) {
	t.Helper()

	viper.SetEnvPrefix("GOTP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
