package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// mobilePGTenant 本测试专用租户，便于清理。
const mobilePGTenant = "pg-mobile-verify"

// setupMobilePostgres 连接真实 PostgreSQL 并插入两台归属不同的设备。
func setupMobilePostgres(t *testing.T, ownedBy, otherOwner string) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; mobile PostgreSQL verification skipped")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	prev := global.DB
	global.DB = db
	// 设备列表读模型用 gen 单例（query.Device）拼字段表达式，未 SetDefault 时该单例为 nil，
	// 一进去就 panic。生产由 internal/app 装配，测试必须自己补上。
	query.SetDefault(db.Session(&gorm.Session{NewDB: true}))
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM devices WHERE tenant_id = ?", mobilePGTenant).Error
		global.DB = prev
	})

	// 默认设备列表只返回 activate_flag=active 的设备；两台都置为 active，
	// 否则"看不到"可能只是激活状态不匹配，掩盖了归属过滤是否真的生效。
	for _, d := range []struct{ id, name, owner string }{
		{"pg-mobile-dev-a", "A 的设备", ownedBy},
		{"pg-mobile-dev-b", "B 的设备", otherOwner},
	} {
		owner := &d.owner
		if strings.TrimSpace(d.owner) == "" {
			owner = nil
		}
		device := &model.Device{
			ID:           d.id,
			Name:         &d.name,
			Voucher:      "pg-mobile-voucher-" + d.id,
			TenantID:     mobilePGTenant,
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			DeviceNumber: d.id,
			IsOnline:     1,
			OwnerUserID:  owner,
		}
		if err := db.Create(device).Error; err != nil {
			t.Fatalf("insert device %s: %v", d.id, err)
		}
	}
	return db
}

// TestMobileDeviceListOwnershipFilterOnPostgres 在真实 PostgreSQL 上验证归属过滤。
//
// 为什么必须真库：设备列表的归属过滤最终落到 SQL 的 owner_user_id 条件与租户
// IN (scopes) 条件上。SQLite 内存库即便能跑同义 SQL，也证明不了真实库上的
// 类型/索引/约束下这个条件仍然成立——本项目已经踩过"用 SQLite 冒充 PG 证据"的坑。
func TestMobileDeviceListOwnershipFilterOnPostgres(t *testing.T) {
	setupMobilePostgres(t, "uA", "uB")
	ctx := context.Background()
	lister := NewMobileDeviceLister()

	ids := func(claims *utils.UserClaims) []string {
		t.Helper()
		list, _, err := lister.List(ctx, claims, "", 1, 50)
		if err != nil {
			t.Fatalf("List as %s/%s: %v", claims.Authority, claims.ID, err)
		}
		out := make([]string, 0, len(list))
		for _, d := range list {
			out = append(out, d.DeviceID)
		}
		return out
	}

	// 普通用户 A：只看自己名下的设备。
	gotA := ids(&utils.UserClaims{ID: "uA", TenantID: mobilePGTenant, Authority: "TENANT_USER"})
	if len(gotA) != 1 || gotA[0] != "pg-mobile-dev-a" {
		t.Fatalf("TENANT_USER uA saw %v, want only [pg-mobile-dev-a]", gotA)
	}

	// 普通用户 B：同理。这一条是防"过滤写死成某个用户"的关键负向对照。
	gotB := ids(&utils.UserClaims{ID: "uB", TenantID: mobilePGTenant, Authority: "TENANT_USER"})
	if len(gotB) != 1 || gotB[0] != "pg-mobile-dev-b" {
		t.Fatalf("TENANT_USER uB saw %v, want only [pg-mobile-dev-b]", gotB)
	}

	// 租户管理员：看全租户两台。这一条防"过滤过严把管理员也挡住"。
	gotAdmin := ids(&utils.UserClaims{ID: "uAdmin", TenantID: mobilePGTenant, Authority: "TENANT_ADMIN"})
	if len(gotAdmin) != 2 {
		t.Fatalf("TENANT_ADMIN saw %v, want both devices", gotAdmin)
	}

	// 其它租户：一台都看不到。
	gotOther := ids(&utils.UserClaims{ID: "uX", TenantID: "pg-mobile-other-tenant", Authority: "TENANT_ADMIN"})
	if len(gotOther) != 0 {
		t.Fatalf("other-tenant admin saw %v, want none", gotOther)
	}
}

// TestMobileDeviceListHidesUnownedDevicesFromTenantUser 归属为空的设备对普通用户不可见。
// 这条容易被忽略：把"没配归属"当成"人人可见"会让新接入的设备对所有人敞开。
func TestMobileDeviceListHidesUnownedDevicesFromTenantUser(t *testing.T) {
	setupMobilePostgres(t, "uA", "") // 第二台无归属
	ctx := context.Background()
	lister := NewMobileDeviceLister()

	list, _, err := lister.List(ctx, &utils.UserClaims{ID: "uA", TenantID: mobilePGTenant, Authority: "TENANT_USER"}, "", 1, 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].DeviceID != "pg-mobile-dev-a" {
		t.Fatalf("TENANT_USER saw %d devices, want only the owned one", len(list))
	}

	// 管理员仍应看到全部两台（含无归属那台）。
	adminList, _, err := lister.List(ctx, &utils.UserClaims{ID: "uAdmin", TenantID: mobilePGTenant, Authority: "TENANT_ADMIN"}, "", 1, 50)
	if err != nil {
		t.Fatalf("List as admin: %v", err)
	}
	if len(adminList) != 2 {
		t.Fatalf("TENANT_ADMIN saw %d devices, want 2 (including the unowned one)", len(adminList))
	}
}

// ---------------------------------------------------------------------------
// OTA 与看板
// ---------------------------------------------------------------------------

// insertOTAChain 插入一条完整的 OTA 链（包 → 任务 → 明细），用于验证租户过滤。
func insertOTAChain(t *testing.T, db *gorm.DB, tenantID, deviceID string, status int16) {
	t.Helper()
	pkgTenant := tenantID
	now := time.Now().UTC()
	pkg := &model.OtaUpgradePackage{
		ID:        "pg-ota-pkg-" + tenantID,
		Name:      "pkg",
		TenantID:  &pkgTenant,
		CreatedAt: now,
	}
	if err := db.Create(pkg).Error; err != nil {
		t.Fatalf("insert ota package: %v", err)
	}
	task := &model.OtaUpgradeTask{
		ID:                   "pg-ota-task-" + tenantID,
		Name:                 "task",
		OtaUpgradePackageID:  pkg.ID,
		CreatedAt:            now,
		Status:               "running",
		TargetMode:           "explicit",
		TimeoutSeconds:       3600,
		RolloutRatePerMinute: 60,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("insert ota task: %v", err)
	}
	detail := &model.OtaUpgradeTaskDetail{
		ID:               "pg-ota-detail-" + tenantID,
		OtaUpgradeTaskID: task.ID,
		DeviceID:         deviceID,
		Status:           status,
		UpdatedAt:        &now,
	}
	if err := db.Create(detail).Error; err != nil {
		t.Fatalf("insert ota detail: %v", err)
	}
}

// TestMobileOTAStatusIsTenantScopedOnPostgres 在真实 PostgreSQL 上验证 OTA 状态查询的租户边界。
//
// 为什么必须专门验：`ota_upgrade_task_details` 自己没有 tenant_id 列，
// 租户信息要经过 task → package 两级关联才拿得到。只按 device_id 查会跨租户泄漏升级进度。
func TestMobileOTAStatusIsTenantScopedOnPostgres(t *testing.T) {
	db := setupMobilePostgres(t, "uA", "uB")

	// 本租户那台设备：一条 upgrading 明细。
	insertOTAChain(t, db, mobilePGTenant, "pg-mobile-dev-a", 3)
	// 另一个租户的同名设备：一条 succeeded 明细。若过滤漏了，这里会被读出来。
	otherTenant := "pg-mobile-ota-other"
	insertOTAChain(t, db, otherTenant, "pg-mobile-dev-a", 4)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM "+model.TableNameOtaUpgradeTaskDetail+" WHERE device_id = ?", "pg-mobile-dev-a").Error
		_ = db.Exec("DELETE FROM " + model.TableNameOtaUpgradeTask + " WHERE id LIKE 'pg-ota-task-%'").Error
		_ = db.Exec("DELETE FROM " + model.TableNameOtaUpgradePackage + " WHERE id LIKE 'pg-ota-pkg-%'").Error
	})

	reader := NewMobileOTAStatusReader()
	claims := &utils.UserClaims{ID: "uA", TenantID: mobilePGTenant, Authority: "TENANT_USER"}

	got, err := reader.Status(context.Background(), claims, "pg-mobile-dev-a")
	if err != nil {
		t.Fatalf("OTAStatus: %v", err)
	}
	if got != "upgrading" {
		t.Fatalf("status = %q, want upgrading (the other tenant's succeeded row must not leak in)", got)
	}

	// 没有升级记录的设备返回 none，而不是报错。
	none, err := reader.Status(context.Background(), &utils.UserClaims{ID: "uB", TenantID: mobilePGTenant, Authority: "TENANT_USER"}, "pg-mobile-dev-b")
	if err != nil {
		t.Fatalf("OTAStatus for device without records: %v", err)
	}
	if none != OTAStatusNone {
		t.Fatalf("status = %q, want %q", none, OTAStatusNone)
	}
}

// TestMobileOTAStatusHonoursDeviceOwnership OTA 状态必须过设备归属这一道闸。
// 只做租户过滤不够：同租户的普通用户不该读到别人名下设备的升级进度。
func TestMobileOTAStatusHonoursDeviceOwnership(t *testing.T) {
	setupMobilePostgres(t, "uA", "uB")
	reader := NewMobileOTAStatusReader()

	// uB 读 uA 名下的设备：必须被拒。
	if _, err := reader.Status(context.Background(),
		&utils.UserClaims{ID: "uB", TenantID: mobilePGTenant, Authority: "TENANT_USER"},
		"pg-mobile-dev-a"); err == nil {
		t.Fatal("TENANT_USER must not read another user's device OTA status")
	}
}

// TestMobileDashboardsAreTenantScoped 看板是租户级共享资产（无归属列），
// 由既有 `resolveBoardListTenant` 按角色裁决：TENANT_ADMIN 只见本租户，其余角色被拒。
func TestMobileDashboardsAreTenantScoped(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; mobile PostgreSQL verification skipped")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	prev := global.DB
	global.DB = db
	query.SetDefault(db.Session(&gorm.Session{NewDB: true}))
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM boards WHERE tenant_id IN (?, ?)", mobilePGTenant, "pg-mobile-board-other").Error
		global.DB = prev
	})

	now := time.Now().UTC()
	_ = now
	for _, b := range []struct{ id, name, tenant string }{
		{"pg-mobile-board-1", "本租户看板", mobilePGTenant},
		{"pg-mobile-board-2", "别租户看板", "pg-mobile-board-other"},
	} {
		name := b.name
		board := &model.Board{
			ID:        b.id,
			Name:      name,
			TenantID:  b.tenant,
			CreatedAt: now,
			UpdatedAt: now,
			HomeFlag:  "N",
		}
		if err := db.Create(board).Error; err != nil {
			t.Fatalf("insert board %s: %v", b.id, err)
		}
	}

	lister := NewMobileDashboardReader()
	list, err := lister.List(context.Background(), &utils.UserClaims{ID: "uAdmin", TenantID: mobilePGTenant, Authority: "TENANT_ADMIN"})
	if err != nil {
		t.Fatalf("ListDashboards: %v", err)
	}
	ids := make([]string, 0, len(list))
	for _, d := range list {
		ids = append(ids, d.ID)
	}
	if len(ids) != 1 || ids[0] != "pg-mobile-board-1" {
		t.Fatalf("TENANT_ADMIN saw %v, want only the own-tenant board", ids)
	}

	// 普通用户被既有实现拒绝（看板没有归属列，不做逐用户裁剪）。
	if _, err := lister.List(context.Background(), &utils.UserClaims{ID: "uA", TenantID: mobilePGTenant, Authority: "TENANT_USER"}); err == nil {
		t.Fatal("TENANT_USER must be rejected by the existing board access rule")
	}
}
