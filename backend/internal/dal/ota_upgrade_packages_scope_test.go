// 文件用途：验证 OTA 升级包列表读路径的 tenant scopes 三态契约（ROADMAP C2 自上而下）：
// 0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN（self∪子孙）；
// 含空租户 [""] 平台包、名称/设备配置过滤与 LeftJoin 设备配置名回填用例。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetOtaUpgradePackageListByPageScopes(t *testing.T) {
	db := setupOtaUpgradePackageScopeTestDB(t)
	now := time.Now().UTC()
	hqTenant := "tenant-hq"
	childTenant := "tenant-child"
	foreignTenant := "tenant-x"
	platformTenant := ""
	packages := []model.OtaUpgradePackage{
		{ID: "pkg-hq-1", Name: "hq fw", Version: "1.0.0", DeviceConfigID: "cfg-hq", PackageType: 2, TenantID: &hqTenant, CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "pkg-hq-2", Name: "hq diff", Version: "1.1.0", DeviceConfigID: "cfg-hq", PackageType: 1, TenantID: &hqTenant, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "pkg-child", Name: "child fw", Version: "2.0.0", DeviceConfigID: "cfg-child", PackageType: 2, TenantID: &childTenant, CreatedAt: now.Add(-time.Minute)},
		{ID: "pkg-foreign", Name: "foreign fw", Version: "3.0.0", DeviceConfigID: "cfg-x", PackageType: 2, TenantID: &foreignTenant, CreatedAt: now},
		{ID: "pkg-platform", Name: "platform fw", Version: "0.9.0", DeviceConfigID: "cfg-platform", PackageType: 2, TenantID: &platformTenant, CreatedAt: now.Add(time.Minute)},
	}
	if err := db.Create(&packages).Error; err != nil {
		t.Fatalf("create ota packages: %v", err)
	}
	if err := db.Create(&model.DeviceConfig{
		ID: "cfg-hq", Name: "hq config", DeviceType: "1", TenantID: hqTenant, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create device config: %v", err)
	}

	t.Run("parent scope returns self and descendants only", func(t *testing.T) {
		total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{hqTenant, childTenant})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows, ok := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		if !ok {
			t.Fatalf("list type = %T, want []model.GetOTAUpgradeTaskListByPageRsp", rawList)
		}
		if total != 3 || len(rows) != 3 {
			t.Fatalf("total = %d, rows = %#v, want 3 in-scope packages", total, rows)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			seen[row.ID] = true
			if row.TenantID == nil || (*row.TenantID != hqTenant && *row.TenantID != childTenant) {
				t.Fatalf("row %q escaped scope with tenant %v", row.ID, row.TenantID)
			}
		}
		if !seen["pkg-hq-1"] || !seen["pkg-hq-2"] || !seen["pkg-child"] {
			t.Fatalf("in-scope rows = %v, want pkg-hq-1, pkg-hq-2 and pkg-child", seen)
		}
	})

	t.Run("device config name join still populated within scope", func(t *testing.T) {
		_, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{hqTenant, childTenant})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		for _, row := range rows {
			if row.ID == "pkg-hq-1" && row.DeviceConfigName != "hq config" {
				t.Fatalf("device_config_name = %q, want hq config", row.DeviceConfigName)
			}
		}
	})

	t.Run("single scope keeps legacy tenant filter", func(t *testing.T) {
		total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{childTenant})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		if total != 1 || len(rows) != 1 || rows[0].ID != "pkg-child" {
			t.Fatalf("total = %d, rows = %#v, want only pkg-child", total, rows)
		}
	})

	t.Run("name and device config filters combine with scopes", func(t *testing.T) {
		total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{Name: "hq diff", PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{hqTenant, childTenant})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		if total != 1 || len(rows) != 1 || rows[0].ID != "pkg-hq-2" {
			t.Fatalf("name filter total = %d, rows = %#v, want only pkg-hq-2", total, rows)
		}

		total, rawList, err = GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{DeviceConfigID: "cfg-hq", PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{hqTenant, childTenant})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows = rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		if total != 2 || len(rows) != 2 {
			t.Fatalf("config filter total = %d, rows = %#v, want 2 hq packages", total, rows)
		}
	})

	t.Run("empty tenant scope maps platform rows", func(t *testing.T) {
		total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{""})
		if err != nil {
			t.Fatalf("GetOtaUpgradePackageListByPage(): %v", err)
		}
		rows := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
		if total != 1 || len(rows) != 1 || rows[0].ID != "pkg-platform" {
			t.Fatalf("total = %d, rows = %#v, want only pkg-platform", total, rows)
		}
	})

	t.Run("nil and empty scopes fail closed", func(t *testing.T) {
		for _, scopes := range [][]string{nil, []string{}} {
			total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, scopes)
			if err != nil {
				t.Fatalf("GetOtaUpgradePackageListByPage(scopes=%v): %v", scopes, err)
			}
			rows, ok := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
			if !ok {
				t.Fatalf("list type = %T, want []model.GetOTAUpgradeTaskListByPageRsp", rawList)
			}
			if total != 0 || rows == nil || len(rows) != 0 {
				t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, non-nil empty, nil)", total, rows, err)
			}
		}
	})
}

func setupOtaUpgradePackageScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open ota package scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OtaUpgradePackage{}, &model.DeviceConfig{}); err != nil {
		t.Fatalf("migrate ota package scope tables: %v", err)
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
