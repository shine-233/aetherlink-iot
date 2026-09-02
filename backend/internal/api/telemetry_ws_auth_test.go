// 文件用途：覆盖遥测 WebSocket 认证中的 JWT 用户状态校验行为。
// 核心逻辑：复用 HTTP 链路的 ValidateJWTUserStatus，验证被禁用、删除或缺失的用户在 token 过期前即被拒绝。
// 关键注意事项：校验必须 fail-closed；invalidateToken 分支在 Redis 未初始化时只跳过清理，不得放行。
// 重构建议：若后续引入 miniredis 等依赖，可补齐 validateToken 全链路（会话续签与失效 token 清理）用例。

package api

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/middleware"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 回归锚点：WS 上的 OpenAPI Key claims 必须与 HTTP 侧同口径（middleware.OpenAPIKeyAuthority，
// 默认最小权限 TENANT_USER），不得再硬编码 TENANT_ADMIN。
func TestTelemetryAPIKeyClaimsUseSharedLeastPrivilegeAuthority(t *testing.T) {
	claims := telemetryAPIKeyClaims(middleware.OpenAPIKeyAuthority(), "ws-tenant", "ws-user")
	if claims.Authority != "TENANT_USER" {
		t.Fatalf("default WS API key authority = %q, want least-privilege TENANT_USER", claims.Authority)
	}
	if claims.TenantID != "ws-tenant" || claims.ID != "ws-user" {
		t.Fatalf("unexpected claims identity: tenant=%q id=%q", claims.TenantID, claims.ID)
	}
}

func TestCheckTelemetryJWTUserStatusRejectsDisabledOrMissingUsers(t *testing.T) {
	db := setupTelemetryWSUserStatusDB(t)
	seedTelemetryWSUser(t, db, "ws-active-user", stringPtrForWS("N"))
	seedTelemetryWSUser(t, db, "ws-frozen-user", stringPtrForWS("F"))
	seedTelemetryWSUser(t, db, "ws-nil-status-user", nil)

	tests := []struct {
		name    string
		claims  *utils.UserClaims
		wantErr bool
	}{
		{name: "active user passes", claims: &utils.UserClaims{ID: "ws-active-user"}},
		{name: "frozen user is rejected", claims: &utils.UserClaims{ID: "ws-frozen-user"}, wantErr: true},
		{name: "nil status user is rejected", claims: &utils.UserClaims{ID: "ws-nil-status-user"}, wantErr: true},
		{name: "deleted user is rejected", claims: &utils.UserClaims{ID: "ws-deleted-user"}, wantErr: true},
		{name: "missing claim id is rejected", claims: &utils.UserClaims{}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkTelemetryJWTUserStatus(context.Background(), "ws-token", tc.claims)

			if !tc.wantErr && err != nil {
				t.Fatalf("checkTelemetryJWTUserStatus(%v) returned error: %v", tc.claims, err)
			}
			if tc.wantErr && err == nil {
				t.Fatalf("checkTelemetryJWTUserStatus(%v) = nil, want no permission error", tc.claims)
			}
		})
	}
}

func TestCheckTelemetryJWTUserStatusFailsClosedWithoutDB(t *testing.T) {
	oldDB := global.DB
	global.DB = nil
	t.Cleanup(func() {
		global.DB = oldDB
	})

	if err := checkTelemetryJWTUserStatus(context.Background(), "ws-token", &utils.UserClaims{ID: "user-1"}); err == nil {
		t.Fatal("checkTelemetryJWTUserStatus without database should fail closed")
	}
}

func setupTelemetryWSUserStatusDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	oldRedis := global.REDIS
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	global.DB = db
	global.REDIS = nil
	t.Cleanup(func() {
		global.DB = oldDB
		global.REDIS = oldRedis
	})
	return db
}

func seedTelemetryWSUser(t *testing.T, db *gorm.DB, id string, status *string) {
	t.Helper()

	now := time.Now().UTC()
	name := id
	authority := "TENANT_USER"
	tenantID := "tenant-1"
	if err := db.Create(&model.User{
		ID:                  id,
		Name:                &name,
		PhoneNumber:         id + "-phone",
		Email:               id + "@example.com",
		Status:              status,
		Authority:           &authority,
		Password:            "hashed",
		TenantID:            &tenantID,
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}).Error; err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func stringPtrForWS(value string) *string {
	return &value
}
