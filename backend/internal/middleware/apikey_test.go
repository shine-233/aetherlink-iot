// 文件用途：覆盖 API key claims 映射等仍在当前请求链中使用的中间件辅助逻辑。
// 核心逻辑：验证 openAPIKeyClaims 是否把租户、创建者和权限映射到统一 claims 结构。
// 关键注意事项：这里不再覆盖已移除的历史 APIKeyValidator middleware 壳层，避免测试继续锁定非生产链路。
// 重构建议：如果后续新增真实生产使用的 API key helper，再按当前实现补聚焦测试，而不是恢复旧壳测试。

package middleware

import (
	"testing"

	"aetherlink-iot/backend/pkg/constant"
)

func TestOpenAPIKeyClaimsPreservesTenantCreatorAndTenantAdminAuthority(t *testing.T) {
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
	if claims.Authority != constant.TENANT_ADMIN {
		t.Fatalf("Authority = %q, want %q", claims.Authority, constant.TENANT_ADMIN)
	}
}
