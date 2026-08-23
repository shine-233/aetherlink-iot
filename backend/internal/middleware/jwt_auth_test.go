// 文件用途：覆盖 HTTP 中间件 jwt auth 行为的 Go 测试。
// 核心逻辑：通过请求上下文、响应状态和边界输入验证认证、跨域、日志或安全处理逻辑，主要围绕 func TestValidateJWTUserStatusRequiresExistingNormalUser、func TestValidateJWTUserStatusFailsClosedWithoutDeletingTokenWhenDBUnavailable、func setupJWTAuthUserStatusDB、func seedJWTAuthUser 等声明展开。
// 关键注意事项：中间件测试需保持状态码、上下文键和错误响应格式与客户端契约一致。
// 重构建议：后续可统一测试路由和上下文构造，减少重复的请求搭建代码。

package middleware

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidateJWTUserStatusRequiresExistingNormalUser(t *testing.T) {
	db := setupJWTAuthUserStatusDB(t)
	seedJWTAuthUser(t, db, "active-user", stringPtr("N"))
	seedJWTAuthUser(t, db, "frozen-user", stringPtr("F"))
	seedJWTAuthUser(t, db, "nil-status-user", nil)

	tests := []struct {
		name                string
		claims              *utils.UserClaims
		wantActive          bool
		wantInvalidateToken bool
	}{
		{
			name:       "active user stays authorized",
			claims:     &utils.UserClaims{ID: "active-user"},
			wantActive: true,
		},
		{
			name:                "frozen user invalidates old token",
			claims:              &utils.UserClaims{ID: "frozen-user"},
			wantInvalidateToken: true,
		},
		{
			name:                "nil status invalidates old token",
			claims:              &utils.UserClaims{ID: "nil-status-user"},
			wantInvalidateToken: true,
		},
		{
			name:                "deleted user invalidates old token",
			claims:              &utils.UserClaims{ID: "deleted-user"},
			wantInvalidateToken: true,
		},
		{
			name:                "missing claim id invalidates old token",
			claims:              &utils.UserClaims{},
			wantInvalidateToken: true,
		},
		{
			name:                "nil claims invalidates old token",
			claims:              nil,
			wantInvalidateToken: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			active, invalidateToken := ValidateJWTUserStatus(context.Background(), tc.claims)

			if active != tc.wantActive || invalidateToken != tc.wantInvalidateToken {
				t.Fatalf("ValidateJWTUserStatus() = (%v, %v), want (%v, %v)",
					active, invalidateToken, tc.wantActive, tc.wantInvalidateToken)
			}
		})
	}
}

func TestValidateJWTUserStatusFailsClosedWithoutDeletingTokenWhenDBUnavailable(t *testing.T) {
	oldDB := global.DB
	global.DB = nil
	t.Cleanup(func() {
		global.DB = oldDB
	})

	active, invalidateToken := ValidateJWTUserStatus(context.Background(), &utils.UserClaims{ID: "user-1"})
	if active || invalidateToken {
		t.Fatalf("ValidateJWTUserStatus() = (%v, %v), want (false, false)", active, invalidateToken)
	}
}

func resetJWTUserStatusCache() {
	jwtUserStatusCache.Range(func(key, _ interface{}) bool {
		jwtUserStatusCache.Delete(key)
		return true
	})
}

func TestCachedJWTUserStatusServesRepeatRequestsFromProcessCache(t *testing.T) {
	db := setupJWTAuthUserStatusDB(t)
	seedJWTAuthUser(t, db, "cache-active-user", stringPtr("N"))
	t.Cleanup(resetJWTUserStatusCache)

	claims := &utils.UserClaims{ID: "cache-active-user"}

	active, invalidateToken := cachedJWTUserStatus(context.Background(), claims)
	if !active || invalidateToken {
		t.Fatalf("first lookup = (%v, %v), want (true, false)", active, invalidateToken)
	}

	// 移除数据库后再次查询：命中进程内缓存，热路径不再依赖 DB。
	oldDB := global.DB
	global.DB = nil
	t.Cleanup(func() { global.DB = oldDB })

	active, invalidateToken = cachedJWTUserStatus(context.Background(), claims)
	if !active || invalidateToken {
		t.Fatalf("cached lookup = (%v, %v), want (true, false)", active, invalidateToken)
	}
}

func TestCachedJWTUserStatusDoesNotCacheTransientDBFailures(t *testing.T) {
	setupJWTAuthUserStatusDB(t)
	resetJWTUserStatusCache()
	t.Cleanup(resetJWTUserStatusCache)

	oldDB := global.DB
	global.DB = nil
	active, invalidateToken := cachedJWTUserStatus(context.Background(), &utils.UserClaims{ID: "transient-user"})
	global.DB = oldDB

	if active || invalidateToken {
		t.Fatalf("db failure = (%v, %v), want (false, false)", active, invalidateToken)
	}
	if _, cached := jwtUserStatusCache.Load("transient-user"); cached {
		t.Fatal("transient DB failure must not be cached")
	}

	// 数据库恢复后同一用户应立即按真实状态放行（未被失败结果污染）。
	seedJWTAuthUser(t, dbOf(t), "transient-user", stringPtr("N"))
	active, invalidateToken = cachedJWTUserStatus(context.Background(), &utils.UserClaims{ID: "transient-user"})
	if !active || invalidateToken {
		t.Fatalf("post-recovery lookup = (%v, %v), want (true, false)", active, invalidateToken)
	}
}

func dbOf(t *testing.T) *gorm.DB {
	t.Helper()
	if global.DB == nil {
		t.Fatal("global.DB is not initialized for test")
	}
	return global.DB
}

func setupJWTAuthUserStatusDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}

func seedJWTAuthUser(t *testing.T, db *gorm.DB, id string, status *string) {
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

func stringPtr(value string) *string {
	return &value
}
