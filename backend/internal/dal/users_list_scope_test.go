// 文件用途：验证用户目录列表读路径的 tenant scopes 三态契约（ROADMAP C2 自上而下）：
// TENANT_ADMIN → users.tenant_id IN (self∪子孙) 且仅成员用户；TENANT_USER → 单租户
// self-only 与旧行为等价；SYS_ADMIN → 平台级管理员目录无租户过滤；空 scopes 显式拒绝。
package dal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetUserListByPageScopes(t *testing.T) {
	db := setupUserListScopeTestDB(t)
	now := time.Now().UTC()
	newUser := func(id, name, authority, tenantID string) model.User {
		userName := name
		status := "N"
		tenant := tenantID
		auth := authority
		return model.User{
			ID: id, Name: &userName, PhoneNumber: "13800000000", Email: id + "@test.local", Status: &status,
			Authority: &auth, Password: "not-a-real-hash", TenantID: &tenant, CreatedAt: &now,
		}
	}
	users := []model.User{
		newUser("u-hq-1", "hq member one", TENANT_USER, "tenant-hq"),
		newUser("u-hq-2", "hq member two", TENANT_USER, "tenant-hq"),
		newUser("u-child-1", "child member", TENANT_USER, "tenant-child"),
		newUser("u-hq-admin", "hq admin", TENANT_ADMIN, "tenant-hq"),
		newUser("u-child-admin", "child admin", TENANT_ADMIN, "tenant-child"),
		newUser("u-x-1", "foreign member", TENANT_USER, "tenant-x"),
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}

	memberIDs := func(t *testing.T, total int64, rawList interface{}) map[string]bool {
		t.Helper()
		rows, ok := rawList.([]map[string]interface{})
		if !ok {
			t.Fatalf("list type = %T, want []map[string]interface{}", rawList)
		}
		if int64(len(rows)) != total {
			t.Fatalf("total = %d, rows = %d, want equal", total, len(rows))
		}
		ids := make(map[string]bool, len(rows))
		for _, row := range rows {
			id, _ := row["id"].(string)
			ids[id] = true
		}
		return ids
	}

	t.Run("tenant admin sees member users of self and descendants", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "tenant-hq", Authority: TENANT_ADMIN}
		total, rawList, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetUserListByPage(): %v", err)
		}
		ids := memberIDs(t, total, rawList)
		want := map[string]bool{"u-hq-1": true, "u-hq-2": true, "u-child-1": true}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("member ids = %v, want %v", ids, want)
		}
	})

	t.Run("single scope keeps legacy tenant filter", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "tenant-hq", Authority: TENANT_ADMIN}
		total, rawList, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetUserListByPage(): %v", err)
		}
		ids := memberIDs(t, total, rawList)
		want := map[string]bool{"u-hq-1": true, "u-hq-2": true}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("member ids = %v, want %v", ids, want)
		}
	})

	t.Run("tenant user keeps self only regardless of scopes", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "tenant-hq", Authority: TENANT_USER}
		total, rawList, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetUserListByPage(): %v", err)
		}
		ids := memberIDs(t, total, rawList)
		want := map[string]bool{"u-hq-1": true, "u-hq-2": true}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("member ids = %v, want %v", ids, want)
		}
	})

	t.Run("empty scopes for tenant roles fail closed", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "tenant-hq", Authority: TENANT_ADMIN}
		total, rawList, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, nil)
		if err == nil {
			t.Fatalf("GetUserListByPage(nil scopes) = (%d, %#v, nil), want explicit error", total, rawList)
		}
		if !strings.Contains(err.Error(), "empty tenant scope") {
			t.Fatalf("error = %v, want empty tenant scope rejection", err)
		}
	})

	t.Run("sys admin keeps platform admin directory", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "", Authority: SYS_ADMIN}
		total, rawList, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, nil)
		if err != nil {
			t.Fatalf("GetUserListByPage(): %v", err)
		}
		ids := memberIDs(t, total, rawList)
		want := map[string]bool{"u-hq-admin": true, "u-child-admin": true}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("admin ids = %v, want %v", ids, want)
		}
	})

	t.Run("unknown authority fails closed", func(t *testing.T) {
		claims := &utils.UserClaims{TenantID: "tenant-hq", Authority: "UNEXPECTED"}
		_, _, err := GetUserListByPage(&model.UserListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, claims, []string{"tenant-hq"})
		if err == nil {
			t.Fatal("expected authority exception, got nil")
		}
	})
}

func setupUserListScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open user list scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserAddress{}); err != nil {
		t.Fatalf("migrate user list scope tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}
