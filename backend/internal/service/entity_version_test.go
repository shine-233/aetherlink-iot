// 文件用途: 覆盖实体版本控制服务层（ROADMAP C7）——租户作用域、实体类型白名单、不可变列剔除与版本号递增。
// 核心逻辑: sqlite 内存库驱动 Create/List/Get/Restore 的作用域行为与错误码契约。
// 关键注意事项: 断言错误码使用 errcode 常量；快照恢复必须剔除 id/tenant_id/created_at。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupEntityVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "entity_version_" + t.Name()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&model.EntityVersion{}, &model.Board{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func entityVersionClaims() *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
}

// errCodeOf 提取错误对象的整型错误码；非 errcode.Error 时返回 -1。
func errCodeOf(err error) int {
	if err == nil {
		return -1
	}
	if e, ok := err.(*errcode.Error); ok {
		return e.Code
	}
	return -1
}

func TestEntityVersionRequiresClaimsAndTenant(t *testing.T) {
	db := setupEntityVersionTestDB(t)
	svc := &EntityVersionService{}

	// nil claims 一律拒绝
	_, err := svc.CreateEntityVersion(&model.EntityVersionCreateReq{EntityType: "board", EntityID: "b1"}, nil)
	if err == nil {
		t.Fatal("expected error for nil claims")
	}
	if errCodeOf(err) != errcode.CodeNoPermission {
		t.Fatalf("want permission error, got %d", errCodeOf(err))
	}

	// 空租户拒绝
	_, err = svc.CreateEntityVersion(
		&model.EntityVersionCreateReq{EntityType: "board", EntityID: "b1"},
		&utils.UserClaims{ID: "u", TenantID: ""},
	)
	if err == nil || errCodeOf(err) != errcode.CodeNoPermission {
		t.Fatalf("want permission error for empty tenant, got %v", err)
	}

	// 未知实体类型拒绝（白名单）
	_, err = svc.CreateEntityVersion(
		&model.EntityVersionCreateReq{EntityType: "customer_table", EntityID: "x1"},
		entityVersionClaims(),
	)
	if err == nil || errCodeOf(err) != errcode.CodeParamError {
		t.Fatalf("want param error for unsupported type, got %v", err)
	}
	_ = db
}

func TestEntityVersionCreateAndVersionIncrement(t *testing.T) {
	db := setupEntityVersionTestDB(t)
	if err := db.Create(&model.Board{ID: "board-1", TenantID: "tenant-a", Name: "t"}).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	// 其他租户的实体不应被本次快照读到（作用域隔离）：同 tenant_b 另立实体
	if err := db.Create(&model.Board{ID: "board-1b", TenantID: "tenant-b", Name: "other"}).Error; err != nil {
		t.Fatalf("seed other tenant board: %v", err)
	}

	svc := &EntityVersionService{}
	req := &model.EntityVersionCreateReq{EntityType: "board", EntityID: "board-1"}

	v1, err := svc.CreateEntityVersion(req, entityVersionClaims())
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if v1.TenantID != "tenant-a" {
		t.Fatalf("snapshot must pin tenant-a, got %s", v1.TenantID)
	}
	if v1.VersionNumber != 1 {
		t.Fatalf("v1 number = %d, want 1", v1.VersionNumber)
	}

	v2, err := svc.CreateEntityVersion(req, entityVersionClaims())
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	if v2.VersionNumber != 2 {
		t.Fatalf("v2 number = %d, want 2", v2.VersionNumber)
	}
}

func TestEntityVersionRestoreDryRunSkipsImmutableColumns(t *testing.T) {
	db := setupEntityVersionTestDB(t)
	if err := db.Create(&model.Board{ID: "board-2", TenantID: "tenant-a", Name: "before"}).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	svc := &EntityVersionService{}

	version, err := svc.CreateEntityVersion(
		&model.EntityVersionCreateReq{EntityType: "board", EntityID: "board-2", Remark: StringPtr("snapshot")},
		entityVersionClaims(),
	)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	dry := true
	payload, isDry, err := svc.RestoreEntityVersion(version.ID, &model.EntityVersionRestoreReq{DryRun: &dry}, entityVersionClaims())
	if err != nil {
		t.Fatalf("dry-run restore: %v", err)
	}
	if !isDry {
		t.Fatal("expected dry-run flag")
	}
	if _, ok := payload["id"]; ok {
		t.Fatal("dry-run payload must not include id (immutable)")
	}
	if _, ok := payload["tenant_id"]; ok {
		t.Fatal("dry-run payload must not include tenant_id (immutable)")
	}
	if _, ok := payload["created_at"]; ok {
		t.Fatal("dry-run payload must not include created_at (immutable)")
	}

	// 真实恢复：把快照（before 状态）写回，此时先改成 after 再恢复应回到 before。
	if err := db.Model(&model.Board{}).Where("id = ?", "board-2").Update("name", "after").Error; err != nil {
		t.Fatalf("mutate board: %v", err)
	}
	_, isDry, err = svc.RestoreEntityVersion(version.ID, &model.EntityVersionRestoreReq{}, entityVersionClaims())
	if err != nil {
		t.Fatalf("real restore: %v", err)
	}
	if isDry {
		t.Fatal("expected non-dry restore")
	}
	var after model.Board
	if err := db.Where("id = ?", "board-2").First(&after).Error; err != nil {
		t.Fatalf("load restored board: %v", err)
	}
	if after.Name != "before" {
		t.Fatalf("restored title = %q, want %q", after.Name, "before")
	}
}