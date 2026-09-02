// 文件用途：覆盖设备预注册服务层的建档契约——自动生成、CSV 导入、租户守卫与跳过明细。
// 核心逻辑：sqlite 内存库跑通 service 全链路，断言落库状态（inactive/批次/凭证形态）与响应报告。
// 关键注意事项：voucher 仅创建响应明文一次；CSV 路径校验与表头契约必须在测试中锁定。
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDevicePreRegisterTestDB(t *testing.T) {
	t.Helper()

	oldDB := global.DB
	// 与 dal 层 harness 相同的理由：纯 :memory: DSN 每个池化连接各建一个库，
	// 用共享内存库并为每个测试单独命名，避免并发用例互相串表。
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
	if err := db.AutoMigrate(&model.Device{}, &model.Product{}, &model.Group{}, &model.RGroupDevice{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	// gen 模型无 VoucherHash 字段（凭证哈希 Phase 2b），与 dal/device_voucher_dual_mode_test.go
	// 相同：测试表补列以承接 createDevicesWithDefaultRootGroup 的同事务回写。
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
}

func seedPreRegisterProduct(t *testing.T, id, tenantID string) {
	t.Helper()
	product := &model.Product{
		ID:       id,
		Name:     "pre-register-product",
		TenantID: &tenantID,
	}
	if err := global.DB.Create(product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
}

func preRegisterClaims(tenantID string) *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", TenantID: tenantID}
}

func writePreRegisterImportCSV(t *testing.T, rows string) string {
	t.Helper()
	dir := filepath.Join("files", "upload", "importBatch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir importBatch dir: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("prereg-%d.csv", time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(path)
	})
	return filepath.ToSlash(path)
}

func TestCreateDevicePreRegisterAutoBuildsInactiveDevices(t *testing.T) {
	setupDevicePreRegisterTestDB(t)
	seedPreRegisterProduct(t, "product-auto", "tenant-a")

	count := 3
	req := model.CreateDevicePreRegisterReq{
		ProductID:      "product-auto",
		BatchNumber:    "batch202608",
		CurrentVersion: strPtr("v1.2.0"),
		DeviceCount:    &count,
		CreateType:     "1",
	}
	rsp, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a"))
	if err != nil {
		t.Fatalf("CreateDevicePreRegister auto: %v", err)
	}

	created := rsp["created_count"].(int)
	if created != 3 {
		t.Fatalf("created_count = %d, want 3", created)
	}
	devices := rsp["devices"].([]preRegisterCreatedDevice)
	for i, item := range devices {
		if !strings.HasPrefix(item.DeviceNumber, "PR-") || len(item.DeviceNumber) > 36 {
			t.Fatalf("device_number = %q, want PR-prefixed within 36 chars", item.DeviceNumber)
		}
		wantName := fmt.Sprintf("batch202608-%04d", i+1)
		if item.Name != wantName {
			t.Fatalf("name = %q, want %q", item.Name, wantName)
		}
		if !strings.Contains(item.Voucher, `"username":"`) {
			t.Fatalf("voucher = %q, want username-shaped one-time credential", item.Voucher)
		}
	}

	var stored model.Device
	if err := global.DB.Where("device_number = ?", devices[0].DeviceNumber).First(&stored).Error; err != nil {
		t.Fatalf("load stored device: %v", err)
	}
	if stored.ActivateFlag != "inactive" {
		t.Fatalf("activate_flag = %q, want inactive", stored.ActivateFlag)
	}
	if stored.BatchNumber == nil || *stored.BatchNumber != "batch202608" {
		t.Fatalf("batch_number = %v, want batch202608", stored.BatchNumber)
	}
	if stored.CurrentVersion == nil || *stored.CurrentVersion != "v1.2.0" {
		t.Fatalf("current_version = %v, want v1.2.0", stored.CurrentVersion)
	}
	if stored.TenantID != "tenant-a" {
		t.Fatalf("tenant_id = %q, want tenant-a", stored.TenantID)
	}
}

func TestCreateDevicePreRegisterAutoRequiresCount(t *testing.T) {
	setupDevicePreRegisterTestDB(t)
	seedPreRegisterProduct(t, "product-auto", "tenant-a")

	req := model.CreateDevicePreRegisterReq{
		ProductID:   "product-auto",
		BatchNumber: "batch202608",
		CreateType:  "1",
	}
	if _, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a")); err == nil {
		t.Fatal("auto create without device_count should fail")
	}
}

func TestCreateDevicePreRegisterFileModeImportsSkipsAndReports(t *testing.T) {
	setupDevicePreRegisterTestDB(t)
	seedPreRegisterProduct(t, "product-file", "tenant-a")

	seeded := &model.Device{
		ID:           "dev-existing",
		Name:         strPtr("existing"),
		DeviceNumber: "EXIST-1",
		TenantID:     "tenant-a",
		ActivateFlag: "active",
	}
	if err := global.DB.Create(seeded).Error; err != nil {
		t.Fatalf("seed existing device: %v", err)
	}

	csvPath := writePreRegisterImportCSV(t, "device_number,name\nNEW-1,新设备一号\nEXIST-1,重复占用\nNEW-1,文件内重复\n")
	req := model.CreateDevicePreRegisterReq{
		ProductID:   "product-file",
		BatchNumber: "batch-csv",
		CreateType:  "2",
		BatchFile:   strPtr(csvPath),
	}
	rsp, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a"))
	if err != nil {
		t.Fatalf("CreateDevicePreRegister file: %v", err)
	}

	if got := rsp["created_count"].(int); got != 1 {
		t.Fatalf("created_count = %d, want 1", got)
	}
	existingSkips := rsp["skipped_existing"].([]string)
	if len(existingSkips) != 1 || existingSkips[0] != "EXIST-1" {
		t.Fatalf("skipped_existing = %v, want [EXIST-1]", existingSkips)
	}
	duplicateSkips := rsp["skipped_duplicate_rows"].([]string)
	if len(duplicateSkips) != 1 || duplicateSkips[0] != "NEW-1" {
		t.Fatalf("skipped_duplicate_rows = %v, want [NEW-1]", duplicateSkips)
	}

	var stored model.Device
	if err := global.DB.Where("device_number = ?", "NEW-1").First(&stored).Error; err != nil {
		t.Fatalf("load imported device: %v", err)
	}
	if stored.ActivateFlag != "inactive" || stored.BatchNumber == nil || *stored.BatchNumber != "batch-csv" {
		t.Fatalf("imported device state wrong: flag=%s batch=%v", stored.ActivateFlag, stored.BatchNumber)
	}
}

func TestCreateDevicePreRegisterFileRejectsBadHeaderAndMissingField(t *testing.T) {
	setupDevicePreRegisterTestDB(t)
	seedPreRegisterProduct(t, "product-file", "tenant-a")

	badHeader := writePreRegisterImportCSV(t, "number,name\nNEW-1,x\n")
	req := model.CreateDevicePreRegisterReq{
		ProductID:   "product-file",
		BatchNumber: "batch-csv",
		CreateType:  "2",
		BatchFile:   strPtr(badHeader),
	}
	if _, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a")); err == nil {
		t.Fatal("bad csv header should fail")
	}

	missingName := writePreRegisterImportCSV(t, "device_number,name\nNEW-1,\n")
	req.BatchFile = strPtr(missingName)
	if _, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a")); err == nil {
		t.Fatal("row without name should fail")
	}
}

func TestCreateDevicePreRegisterGuardsEmptyTenantAndForeignProduct(t *testing.T) {
	setupDevicePreRegisterTestDB(t)
	seedPreRegisterProduct(t, "product-b", "tenant-b")

	count := 1
	req := model.CreateDevicePreRegisterReq{
		ProductID:   "product-b",
		BatchNumber: "batch-x",
		DeviceCount: &count,
		CreateType:  "1",
	}
	if _, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("")); err == nil {
		t.Fatal("empty tenant claims should fail")
	}
	if _, err := (&Device{}).CreateDevicePreRegister(req, preRegisterClaims("tenant-a")); err == nil {
		t.Fatal("cross-tenant product should fail")
	}
}
