// 文件用途：预注册清理执行面的运行期证据（ROADMAP P0.5）。
// 覆盖：真实删除只作用于未激活设备、已激活设备保留并在结果中回传、
// 重复清理幂等（返回 0 不报错）、跨租户设备不受影响。
// 说明：需要真实 PostgreSQL（devices 表）。缺 DSN 或缺表一律 Skip，不得把 Skip 当通过。
// 与 device_preregister_cleanup_test.go 的区别：那份用注入的 ops 验证语义，
// 本份走真实 load/classify/remove，验证删除真的按分流结果执行。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
	"github.com/go-basic/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openPreregisterPostgres 打开数据库并校验 devices 表存在。
func openPreregisterPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; pre-register cleanup tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	// gorm gen 的 query.Device 等全局查询对象必须通过 SetDefault 绑定连接，
	// 只设 global.DB 不够——否则 gen 对象仍是 nil，一调用就空指针 panic。
	query.SetDefault(db)

	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='devices')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe devices table: %v", err)
	}
	if !exists {
		t.Skip("devices missing; apply migrations first")
	}
	return db
}

// seedPreregisterProduct 写入一条产品记录。
// devices.product_id 有外键 fk_product_id，直接插设备会因 23503 失败。
func seedPreregisterProduct(t *testing.T, db *gorm.DB, productID, name string) {
	t.Helper()
	if err := db.Exec(
		"INSERT INTO products (id, name, created_at) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING",
		productID, name, time.Now().UTC()).Error; err != nil {
		t.Fatalf("insert product: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM products WHERE id = ?", productID).Error
	})
}

// seedPreregisterDevice 写入一台预注册设备；activateFlag 决定它是否属于可清理范围。
func seedPreregisterDevice(t *testing.T, db *gorm.DB, tenantID, productID, number, activateFlag string) *model.Device {
	t.Helper()
	now := time.Now().UTC()
	name := "evidence-" + number
	row := &model.Device{
		ID:           uuid.New(),
		TenantID:     tenantID,
		DeviceNumber: number,
		Name:         &name,
		Voucher:      `{"username":"` + uuid.New() + `"}`,
		ProductID:    &productID,
		IsOnline:     0,
		ActivateFlag: activateFlag,
		CreatedAt:    &now,
		UpdateAt:     &now,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM devices WHERE id = ?", row.ID).Error
	})
	return row
}

func deviceExists(t *testing.T, db *gorm.DB, id string) bool {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM devices WHERE id = ?", id).Scan(&count).Error; err != nil {
		t.Fatalf("count device: %v", err)
	}
	return count > 0
}

// TestCleanupDeletesOnlyInactiveDevices 核心：清理只删未激活设备，
// 已激活的必须留在库里并出现在 blocked_activated 中。
func TestCleanupDeletesOnlyInactiveDevices(t *testing.T) {
	db := openPreregisterPostgres(t)
	tenant := "tenant-cleanup-evidence"
	product := "product-cleanup-evidence"
	seedPreregisterProduct(t, db, product, "product-cleanup-evidence")

	inactiveOne := seedPreregisterDevice(t, db, tenant, product, "CL-INACTIVE-1", "inactive")
	inactiveTwo := seedPreregisterDevice(t, db, tenant, product, "CL-INACTIVE-2", "inactive")
	activeOne := seedPreregisterDevice(t, db, tenant, product, "CL-ACTIVE-1", "active")

	productID := product
	req := model.ExportPreRegisterReq{ProductID: productID}
	claims := &utils.UserClaims{TenantID: tenant}

	result, err := (&Device{}).CleanupDevicePreRegister(req, claims)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2", result.Deleted)
	}
	if len(result.BlockedActivated) != 1 || result.BlockedActivated[0] != activeOne.DeviceNumber {
		t.Fatalf("blocked_activated = %v, want [%s]", result.BlockedActivated, activeOne.DeviceNumber)
	}

	// 已激活设备必须还在——清批次绝不能杀掉在运设备。
	if !deviceExists(t, db, activeOne.ID) {
		t.Fatalf("activated device must survive cleanup")
	}
	if deviceExists(t, db, inactiveOne.ID) || deviceExists(t, db, inactiveTwo.ID) {
		t.Fatalf("inactive devices must be removed")
	}
}

// TestCleanupIsIdempotent 重复清理同一批次返回 0 且不报错。
func TestCleanupIsIdempotent(t *testing.T) {
	db := openPreregisterPostgres(t)
	tenant := "tenant-cleanup-idem"
	product := "product-cleanup-idem"
	seedPreregisterProduct(t, db, product, "product-cleanup-idem")
	seedPreregisterDevice(t, db, tenant, product, "CL-IDEM-1", "inactive")

	req := model.ExportPreRegisterReq{ProductID: product}
	claims := &utils.UserClaims{TenantID: tenant}

	first, err := (&Device{}).CleanupDevicePreRegister(req, claims)
	if err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	if first.Deleted != 1 {
		t.Fatalf("first deleted = %d, want 1", first.Deleted)
	}

	second, err := (&Device{}).CleanupDevicePreRegister(req, claims)
	if err != nil {
		t.Fatalf("second cleanup must not error: %v", err)
	}
	if second.Deleted != 0 {
		t.Fatalf("second deleted = %d, want 0", second.Deleted)
	}
}

// TestCleanupLeavesOtherTenantsUntouched 别的租户的同产品设备不受影响。
func TestCleanupLeavesOtherTenantsUntouched(t *testing.T) {
	db := openPreregisterPostgres(t)
	tenant := "tenant-cleanup-owner"
	other := "tenant-cleanup-other"
	product := "product-cleanup-cross"
	seedPreregisterProduct(t, db, product, "product-cleanup-cross")

	mine := seedPreregisterDevice(t, db, tenant, product, "CL-CROSS-MINE", "inactive")
	theirs := seedPreregisterDevice(t, db, other, product, "CL-CROSS-THEIRS", "inactive")

	req := model.ExportPreRegisterReq{ProductID: product}
	result, err := (&Device{}).CleanupDevicePreRegister(req, &utils.UserClaims{TenantID: tenant})
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (own tenant only)", result.Deleted)
	}
	if !deviceExists(t, db, theirs.ID) {
		t.Fatalf("other tenant's device must survive")
	}
	if deviceExists(t, db, mine.ID) {
		t.Fatalf("own inactive device must be removed")
	}
	_ = query.Device
}
