// 文件用途: 覆盖 DAL 层手写查询、缓存或聚合逻辑的回归测试，验证数据访问边界不会漂移。
// 核心逻辑: 构造最小依赖场景并断言查询条件、缓存键、事务副作用或租户过滤结果。
// 关键注意事项: 测试应显式覆盖租户隔离、权限前置假设和事务失败路径，避免只验证成功路径。
// 重构建议: 随 DAL 查询拆分同步拆小测试夹具，并优先补齐跨租户、空依赖和半提交风险用例。

package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDeviceConfigDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	// 与 setupDeviceDALTestDB 相同的理由：纯 :memory: DSN 每个池化连接各建一个库，
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
	if err := db.AutoMigrate(&model.DeviceConfig{}, &model.Device{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
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

func TestDeviceConfigCRUDLifecycle(t *testing.T) {
	db := setupDeviceConfigDALTestDB(t)
	now := time.Now().UTC()
	templateID := "template-lifecycle"

	config := &model.DeviceConfig{
		ID:               "config-lifecycle",
		Name:             "lifecycle-config",
		DeviceTemplateID: &templateID,
		DeviceType:       "1",
		TenantID:         "tenant-a",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := CreateDeviceConfig(config); err != nil {
		t.Fatalf("CreateDeviceConfig: %v", err)
	}

	loaded, err := GetDeviceConfigByID("config-lifecycle")
	if err != nil {
		t.Fatalf("GetDeviceConfigByID after create: %v", err)
	}
	if loaded.Name != "lifecycle-config" || loaded.TenantID != "tenant-a" || loaded.DeviceTemplateID == nil || *loaded.DeviceTemplateID != templateID {
		t.Fatalf("loaded config = %+v, want created row", loaded)
	}

	newTemplate := ""
	if err := UpdateDeviceConfigTemplateID("config-lifecycle", &newTemplate); err != nil {
		t.Fatalf("UpdateDeviceConfigTemplateID: %v", err)
	}
	if err := UpdateDeviceConfig("config-lifecycle", map[string]interface{}{
		"name": "lifecycle-renamed",
	}); err != nil {
		t.Fatalf("UpdateDeviceConfig: %v", err)
	}
	renamed, err := GetDeviceConfigByID("config-lifecycle")
	if err != nil {
		t.Fatalf("GetDeviceConfigByID after update: %v", err)
	}
	if renamed.Name != "lifecycle-renamed" {
		t.Fatalf("renamed.Name = %q, want lifecycle-renamed", renamed.Name)
	}
	if renamed.DeviceTemplateID == nil || *renamed.DeviceTemplateID != "" {
		t.Fatalf("template id = %v, want explicit empty string written", renamed.DeviceTemplateID)
	}

	if err := DeleteDeviceConfig("config-lifecycle"); err != nil {
		t.Fatalf("DeleteDeviceConfig existing row: %v", err)
	}
	var remaining int64
	db.Model(&model.DeviceConfig{}).Where("id = ?", "config-lifecycle").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("delete reported success but row remains (count=%d)", remaining)
	}
	if err := DeleteDeviceConfig("config-lifecycle"); err == nil {
		t.Fatal("second DeleteDeviceConfig should fail with no rows affected, got nil error")
	}
}

func TestUpdateDeviceConfigMissingRowFails(t *testing.T) {
	setupDeviceConfigDALTestDB(t)

	err := UpdateDeviceConfig("config-missing", map[string]interface{}{"name": "ghost"})
	if err == nil {
		t.Fatal("UpdateDeviceConfig on missing id should fail with no rows affected, got nil error")
	}
	if !strings.Contains(err.Error(), "no rows affected") {
		t.Fatalf("error = %v, want no-rows-affected guard message", err)
	}
}

func TestGetDeviceConfigForTenantRejectsForeignTenant(t *testing.T) {
	db := setupDeviceConfigDALTestDB(t)
	now := time.Now().UTC()
	rows := []model.DeviceConfig{
		{ID: "config-a", Name: "a", DeviceType: "1", TenantID: "tenant-a", CreatedAt: now, UpdatedAt: now},
		{ID: "config-b", Name: "b", DeviceType: "1", TenantID: "tenant-b", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed configs: %v", err)
	}

	owned, err := GetDeviceConfigForTenant("config-b", "tenant-b")
	if err != nil {
		t.Fatalf("GetDeviceConfigForTenant same tenant: %v", err)
	}
	if owned.ID != "config-b" {
		t.Fatalf("owned.ID = %q, want config-b", owned.ID)
	}

	if _, err := GetDeviceConfigForTenant("config-b", "tenant-a"); err == nil {
		t.Fatal("cross-tenant read must be rejected, got nil error")
	} else if !strings.Contains(err.Error(), "tenant-b") && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("cross-tenant error = %v, want tenant-scoped not-found", err)
	}

	if _, err := GetDeviceConfigForTenant("config-missing", "tenant-a"); err == nil {
		t.Fatal("missing id must return error even inside own tenant")
	}
}

func TestGetDeviceConfigListByPageFiltersByTenant(t *testing.T) {
	db := setupDeviceConfigDALTestDB(t)
	now := time.Now().UTC()
	rows := []model.DeviceConfig{
		{ID: "config-a1", Name: "alpha", DeviceType: "1", TenantID: "tenant-a", CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now},
		{ID: "config-a2", Name: "beta", DeviceType: "1", TenantID: "tenant-a", CreatedAt: now.Add(time.Minute), UpdatedAt: now},
		{ID: "config-b1", Name: "foreign", DeviceType: "1", TenantID: "tenant-b", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed configs: %v", err)
	}

	count, data, err := GetDeviceConfigListByPage(
		&model.GetDeviceConfigListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 10}},
		&utils.UserClaims{TenantID: "tenant-a"},
	)
	if err != nil {
		t.Fatalf("GetDeviceConfigListByPage: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (tenant-a rows only)", count)
	}
	list, ok := data.([]model.DeviceConfigRsp)
	if !ok {
		t.Fatalf("data type = %T, want []model.DeviceConfigRsp", data)
	}
	for _, item := range list {
		if item.DeviceConfig == nil || item.DeviceConfig.TenantID != "tenant-a" {
			t.Fatalf("list leaked foreign tenant row: %+v", item)
		}
	}
	if len(list) != 0 && list[0].DeviceConfig.ID != "config-a1" {
		t.Fatalf("first row = %q, want newest-first config-a1", list[0].DeviceConfig.ID)
	}

	if _, _, err := GetDeviceConfigListByPage(
		&model.GetDeviceConfigListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 10}},
		&utils.UserClaims{TenantID: "   "},
	); err == nil {
		t.Fatal("blank TenantID in claims must be rejected, got nil error")
	}
}
