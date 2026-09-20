// 文件用途：存量明文凭证清理（P0.5 收尾）的定向证据。
// 最要紧的一条：**没有 voucher_hash 的行绝不能被清理**——那是凭证的唯一副本，
// 清掉等于让设备永久失联。其余覆盖幂等、上限与已清理行不重复入选。
package dal

import (
	"os"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupVoucherPurgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.Device{}); err != nil {
		t.Fatalf("migrate devices: %v", err)
	}
	// gen/AutoMigrate 模型没有 voucher_hash 字段（生成文件不手改），测试里补列。
	if err := db.Exec(`ALTER TABLE devices ADD COLUMN voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("add voucher_hash column: %v", err)
	}
	return db
}

func seedVoucherPurgeRow(t *testing.T, db *gorm.DB, id, voucher string, hash *string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO devices (id, voucher, tenant_id, device_number, is_enabled, activate_flag, is_online, voucher_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, voucher, "tenant-purge", id, "enabled", "active", 0, hash,
	).Error; err != nil {
		t.Fatalf("seed device %s: %v", id, err)
	}
}

func voucherPurgeStoredPlaintext(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	var voucher string
	if err := db.Raw(`SELECT voucher FROM devices WHERE id = ?`, id).Scan(&voucher).Error; err != nil {
		t.Fatalf("read voucher for %s: %v", id, err)
	}
	return voucher
}

// TestPurgeDeviceVoucherPlaintextKeepsRowsWithoutHash 锁定最关键的安全约束：
// 缺 voucher_hash 的行必须原样保留。宁可留着明文，也不能删掉唯一副本。
func TestPurgeDeviceVoucherPlaintextKeepsRowsWithoutHash(t *testing.T) {
	db := setupVoucherPurgeTestDB(t)
	plain := `{"username":"purge-user"}`
	hash := utils.VoucherStorageHash(plain)

	seedVoucherPurgeRow(t, db, "dev-with-hash", plain, &hash)
	seedVoucherPurgeRow(t, db, "dev-no-hash", plain, nil)

	purged, err := PurgeDeviceVoucherPlaintext(db, 0)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged = %d, want 1 (only the row that has a hash)", purged)
	}
	if got := voucherPurgeStoredPlaintext(t, db, "dev-with-hash"); got != "" {
		t.Fatalf("row with hash must be purged, voucher still %q", got)
	}
	if got := voucherPurgeStoredPlaintext(t, db, "dev-no-hash"); got != plain {
		t.Fatalf("row WITHOUT hash must keep its plaintext (the only copy), got %q", got)
	}
}

func TestPurgeDeviceVoucherPlaintextIdempotent(t *testing.T) {
	db := setupVoucherPurgeTestDB(t)
	plain := `{"username":"purge-twice"}`
	hash := utils.VoucherStorageHash(plain)
	seedVoucherPurgeRow(t, db, "dev-1", plain, &hash)
	seedVoucherPurgeRow(t, db, "dev-blank", "", &hash)

	if purged, err := PurgeDeviceVoucherPlaintext(db, 0); err != nil || purged != 1 {
		t.Fatalf("first purge = (%d, %v), want (1, nil)", purged, err)
	}
	if purged, err := PurgeDeviceVoucherPlaintext(db, 0); err != nil || purged != 0 {
		t.Fatalf("second purge = (%d, %v), want (0, nil): must be idempotent", purged, err)
	}
}

func TestPurgeDeviceVoucherPlaintextRespectsMaxRows(t *testing.T) {
	db := setupVoucherPurgeTestDB(t)
	plain := `{"username":"purge-limit"}`
	hash := utils.VoucherStorageHash(plain)
	for _, id := range []string{"dev-l1", "dev-l2", "dev-l3"} {
		seedVoucherPurgeRow(t, db, id, plain+id, &hash)
	}

	purged, err := PurgeDeviceVoucherPlaintext(db, 2)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 2 {
		t.Fatalf("purged = %d, want 2 (maxRows cap)", purged)
	}
}

func TestPurgeDeviceVoucherPlaintextRejectsNilDB(t *testing.T) {
	if _, err := PurgeDeviceVoucherPlaintext(nil, 0); err == nil {
		t.Fatal("nil db must be rejected instead of silently doing nothing")
	}
}

// TestPurgeDeviceVoucherPlaintextPostgres 在真实 PostgreSQL 上复验清理的安全约束。
// 整个用例跑在一个事务里并回滚，因此不会在验证库留下任何残留行。
func TestPurgeDeviceVoucherPlaintextPostgres(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; plaintext purge regression requires PostgreSQL")
	}
	pg, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	tx := pg.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	defer tx.Rollback()

	plain := `{"username":"pg-purge-user"}`
	hash := utils.VoucherStorageHash(plain)

	for _, row := range []struct {
		id   string
		hash *string
	}{
		{id: "purge-pg-with-hash", hash: &hash},
		{id: "purge-pg-no-hash", hash: nil},
	} {
		if err := tx.Exec(
			`INSERT INTO devices (id, voucher, tenant_id, device_number, is_enabled, activate_flag, is_online, voucher_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (id) DO UPDATE SET voucher = EXCLUDED.voucher, voucher_hash = EXCLUDED.voucher_hash`,
			row.id, plain+row.id, "tenant-purge-pg", row.id, "enabled", "active", 0, row.hash,
		).Error; err != nil {
			t.Fatalf("seed %s: %v", row.id, err)
		}
	}

	purged, err := PurgeDeviceVoucherPlaintext(tx, 10)
	if err != nil {
		t.Fatalf("purge on postgres: %v", err)
	}
	if purged == 0 {
		t.Fatal("expected at least one row purged on postgres")
	}

	var kept, cleared string
	if err := tx.Raw(`SELECT voucher FROM devices WHERE id = ?`, "purge-pg-with-hash").Scan(&cleared).Error; err != nil {
		t.Fatalf("read purged row: %v", err)
	}
	if err := tx.Raw(`SELECT voucher FROM devices WHERE id = ?`, "purge-pg-no-hash").Scan(&kept).Error; err != nil {
		t.Fatalf("read retained row: %v", err)
	}
	if cleared != "" {
		t.Fatalf("row with hash must be purged on postgres, voucher still %q", cleared)
	}
	if kept == "" {
		t.Fatal("row WITHOUT hash must keep its plaintext on postgres")
	}
}
