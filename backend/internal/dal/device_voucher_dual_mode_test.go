// 文件用途：凭证哈希存储 Phase 1（references/backend-hardening-plan.md 车道1）dual-mode 回归。
// 核心逻辑：读侧 GetDeviceByVoucher / CheckVoucherExists 必须 hash 优先、明文兜底；
// 写侧创建路径二段式（gen 插入后同事务补 UPDATE voucher_hash）；BackfillDeviceVoucherHash
// 幂等可重入且跳过空 voucher 行。
// 关键注意事项：sqlite 用例默认运行；PostgreSQL 用例照 devices_isolation_test.go 约定以
// AETHERLINK_TEST_PSQL_DSN 门控，未设置时跳过。

package dal

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupDeviceVoucherDualModeTestDB 提供带 voucher_hash 列的内存库
// （列由 50.sql 迁移负责，gen 模型无该字段，须按生产 schema 手工补列）。
func setupDeviceVoucherDualModeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
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
	if err := db.AutoMigrate(&model.Device{}, &model.Group{}, &model.RGroupDevice{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	if err := db.Exec(`ALTER TABLE devices ADD COLUMN voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("add voucher_hash column: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

// seedDualModeDevice 走真实创建路径落库后，再按用例覆写 voucher_hash
// （model.Device 无该字段，只能 raw UPDATE）。
func seedDualModeDevice(t *testing.T, id string, voucher string, voucherHash *string) {
	t.Helper()

	device := &model.Device{
		ID:           id,
		Voucher:      voucher,
		TenantID:     "tenant-voucher-dual",
		DeviceNumber: id,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		IsOnline:     0,
	}
	if err := CreateDevice(device); err != nil {
		t.Fatalf("create device %s: %v", id, err)
	}
	hashArg := interface{}(nil)
	if voucherHash != nil {
		hashArg = *voucherHash
	}
	if err := global.DB.Exec(`UPDATE devices SET voucher = ?, voucher_hash = ? WHERE id = ?`,
		voucher, hashArg, id).Error; err != nil {
		t.Fatalf("override voucher columns for %s: %v", id, err)
	}
}

func storedVoucherHash(t *testing.T, deviceID string) *string {
	t.Helper()

	var got *string
	if err := global.DB.Raw(`SELECT voucher_hash FROM devices WHERE id = ?`, deviceID).Scan(&got).Error; err != nil {
		t.Fatalf("read voucher_hash for %s: %v", deviceID, err)
	}
	return got
}

// TestCreateDevicesPersistVoucherHash 锁定写入侧二段式契约：
// 单个与批量创建都必须在同事务内补齐 voucher_hash。
func TestCreateDevicesPersistVoucherHash(t *testing.T) {
	setupDeviceVoucherDualModeTestDB(t)

	single := &model.Device{
		ID:           "vh-create-single",
		Voucher:      `{"username":"single-user"}`,
		TenantID:     "tenant-voucher-dual",
		DeviceNumber: "vh-create-single",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
	}
	if err := CreateDevice(single); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	wantSingle := utils.VoucherStorageHash(single.Voucher)
	if got := storedVoucherHash(t, single.ID); got == nil || *got != wantSingle {
		t.Fatalf("single create voucher_hash = %v, want %q", got, wantSingle)
	}

	batch := []*model.Device{
		{ID: "vh-create-batch-a", Voucher: `{"username":"batch-a"}`, TenantID: "tenant-voucher-dual", DeviceNumber: "vh-batch-a", IsEnabled: "enabled", ActivateFlag: "active"},
		{ID: "vh-create-batch-b", Voucher: `{"username":"batch-b"}`, TenantID: "tenant-voucher-dual", DeviceNumber: "vh-batch-b", IsEnabled: "enabled", ActivateFlag: "active"},
	}
	if err := CreateDeviceBatch(batch); err != nil {
		t.Fatalf("CreateDeviceBatch: %v", err)
	}
	for _, device := range batch {
		want := utils.VoucherStorageHash(device.Voucher)
		if got := storedVoucherHash(t, device.ID); got == nil || *got != want {
			t.Fatalf("batch create %s voucher_hash = %v, want %q", device.ID, got, want)
		}
	}
}

// TestGetDeviceByVoucherPrefersHashColumnThenPlaintextFallback 锁定读侧匹配顺序：
// 1) hash 列优先命中（明文不同也以 hash 为准）；2) hash 缺失回落明文兜底；
// 3) 两列均未命中 → 包装后的 gorm.ErrRecordNotFound，且错误不回显 voucher 明文。
func TestGetDeviceByVoucherPrefersHashColumnThenPlaintextFallback(t *testing.T) {
	setupDeviceVoucherDualModeTestDB(t)

	hashHit := utils.VoucherStorageHash("fresh-voucher")
	seedDualModeDevice(t, "vh-read-hash-hit", "stale-plaintext", &hashHit)
	seedDualModeDevice(t, "vh-read-fallback", "legacy-plaintext", nil)

	device, err := GetDeviceByVoucher("fresh-voucher")
	if err != nil || device == nil || device.ID != "vh-read-hash-hit" {
		t.Fatalf("hash-first lookup = (%v, %v), want vh-read-hash-hit", device, err)
	}

	device, err = GetDeviceByVoucher("legacy-plaintext")
	if err != nil || device == nil || device.ID != "vh-read-fallback" {
		t.Fatalf("plaintext fallback lookup = (%v, %v), want vh-read-fallback", device, err)
	}

	unknown := `{"username":"no-such-device"}`
	_, err = GetDeviceByVoucher(unknown)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown voucher error = %v, want wrapped gorm.ErrRecordNotFound", err)
	}
	if strings.Contains(err.Error(), unknown) {
		t.Fatalf("voucher leaked in not-found error: %s", err)
	}
}

// TestCheckVoucherExistsDualMode 锁定唯一性预检双模式：hash 列与明文列命中任一即冲突，
// 排除自身语义不变。
func TestCheckVoucherExistsDualMode(t *testing.T) {
	setupDeviceVoucherDualModeTestDB(t)

	hashed := utils.VoucherStorageHash("voucher-hashed")
	seedDualModeDevice(t, "vh-exists-hashed", "voucher-hashed", &hashed)
	seedDualModeDevice(t, "vh-exists-legacy", "voucher-legacy", nil)

	cases := []struct {
		name    string
		voucher string
		exclude string
		want    bool
	}{
		{name: "hash column hit", voucher: "voucher-hashed", exclude: "other-device", want: true},
		{name: "hash hit excluded by self", voucher: "voucher-hashed", exclude: "vh-exists-hashed", want: false},
		{name: "legacy plaintext hit", voucher: "voucher-legacy", exclude: "other-device", want: true},
		{name: "legacy plaintext excluded by self", voucher: "voucher-legacy", exclude: "vh-exists-legacy", want: false},
		{name: "missing voucher", voucher: "voucher-missing", exclude: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckVoucherExists(tc.voucher, tc.exclude)
			if err != nil || got != tc.want {
				t.Fatalf("CheckVoucherExists(%q, %q) = (%v, %v), want (%v, nil)", tc.voucher, tc.exclude, got, err, tc.want)
			}
		})
	}
}

// TestBackfillDeviceVoucherHashIdempotentAndReentrant 锁定回填契约：
// 只补 voucher_hash IS NULL 且 voucher<>” 的行；重复执行零漂移。
func TestBackfillDeviceVoucherHashIdempotentAndReentrant(t *testing.T) {
	db := setupDeviceVoucherDualModeTestDB(t)

	seedDualModeDevice(t, "vh-backfill-pending", `{"username":"pending"}`, nil)
	seedDualModeDevice(t, "vh-backfill-empty", "", nil)
	preset := utils.VoucherStorageHash(`{"username":"preset"}`)
	seedDualModeDevice(t, "vh-backfill-preset", `{"username":"preset"}`, &preset)

	if err := BackfillDeviceVoucherHash(db); err != nil {
		t.Fatalf("first backfill: %v", err)
	}

	pendingWant := utils.VoucherStorageHash(`{"username":"pending"}`)
	if got := storedVoucherHash(t, "vh-backfill-pending"); got == nil || *got != pendingWant {
		t.Fatalf("pending row voucher_hash = %v, want %q", got, pendingWant)
	}
	// 空 voucher 行不属于匹配面，保持 NULL。
	if got := storedVoucherHash(t, "vh-backfill-empty"); got != nil {
		t.Fatalf("empty voucher row must stay NULL, got %v", got)
	}
	if got := storedVoucherHash(t, "vh-backfill-preset"); got == nil || *got != preset {
		t.Fatalf("preset row must stay untouched, got %v want %q", got, preset)
	}

	// 幂等重入：再次执行无错误、结果不变。
	if err := BackfillDeviceVoucherHash(db); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if got := storedVoucherHash(t, "vh-backfill-pending"); got == nil || *got != pendingWant {
		t.Fatalf("pending row drifted after reentrant run: %v", got)
	}
}

// TestDeviceVoucherDualModeAgainstPostgres 以真实 PostgreSQL 复验双模式匹配顺序与
// 回填幂等（含 idx_devices_voucher_hash 索引存在性），照 devices_isolation_test.go 门控。
func TestDeviceVoucherDualModeAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; voucher dual-mode regression requires PostgreSQL")
	}

	pg, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	// 列/索引由 50.sql 迁移负责；这里 IF NOT EXISTS 自建保证测试对旧库也可独立运行。
	if err := pg.Exec(`ALTER TABLE devices ADD COLUMN IF NOT EXISTS voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("ensure voucher_hash column: %v", err)
	}
	var indexCount int64
	if err := pg.Raw(
		`SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'devices' AND indexname = 'idx_devices_voucher_hash'`,
	).Scan(&indexCount).Error; err != nil {
		t.Fatalf("check index: %v", err)
	}
	if indexCount != 1 {
		if err := pg.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_voucher_hash ON devices (voucher_hash)`).Error; err != nil {
			t.Fatalf("ensure voucher_hash index: %v", err)
		}
	}

	oldDB := global.DB
	global.DB = pg
	query.SetDefault(pg)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	const (
		hashRowID   = "11111111-2222-3333-4444-00000000vh01"
		fallbackID  = "11111111-2222-3333-4444-00000000vh02"
		dualTenant  = "tenant-vh-pg-regression"
		hashPlain   = `{"username":"pg-hash-user","password":"pw1"}`
		fallbackLex = `{"password":"pw2","username":"pg-fb-user"}`
		presented   = `{"username":"pg-fb-user","password":"pw2"}`
	)
	rows := []struct {
		id      string
		voucher string
		hash    *string
	}{
		{id: hashRowID, voucher: "sentinel-not-real", hash: strPtr(utils.VoucherStorageHash(hashPlain))},
		{id: fallbackID, voucher: fallbackLex, hash: nil},
	}
	for _, row := range rows {
		if err := pg.Exec(
			`INSERT INTO devices (id, voucher, tenant_id, device_number, is_enabled, activate_flag, is_online, voucher_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (id) DO UPDATE SET voucher = EXCLUDED.voucher, voucher_hash = EXCLUDED.voucher_hash`,
			row.id, row.voucher, dualTenant, row.id, "enabled", "active", 0, row.hash,
		).Error; err != nil {
			t.Fatalf("seed %s: %v", row.id, err)
		}
		defer pg.Exec(`DELETE FROM devices WHERE id = ?`, row.id)
	}

	device, err := GetDeviceByVoucher(hashPlain)
	if err != nil || device == nil || device.ID != hashRowID {
		t.Fatalf("postgres hash-first lookup = (%v, %v), want %s", device, err, hashRowID)
	}
	device, err = GetDeviceByVoucher(presented)
	if err != nil || device == nil || device.ID != fallbackID {
		t.Fatalf("postgres plaintext fallback lookup = (%v, %v), want %s", device, err, fallbackID)
	}
	if _, err := GetDeviceByVoucher(`{"username":"pg-no-such-device"}`); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("postgres unknown voucher error = %v, want record not found", err)
	}
}

func strPtr(value string) *string {
	return &value
}
