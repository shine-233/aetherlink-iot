package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetOtaUpgradePackageListReturnsEmptyArrayShape(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:ota_package_empty_list?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open ota package sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OtaUpgradePackage{}, &model.DeviceConfig{}); err != nil {
		t.Fatalf("migrate ota package tables: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	total, rawList, err := GetOtaUpgradePackageListByPage(&model.GetOTAUpgradePackageLisyByPageReq{}, []string{"tenant-empty"})
	if err != nil {
		t.Fatalf("list ota packages: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	rows, ok := rawList.([]model.GetOTAUpgradeTaskListByPageRsp)
	if !ok {
		t.Fatalf("list type = %T, want []model.GetOTAUpgradeTaskListByPageRsp", rawList)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("empty list = %#v, want a non-nil empty slice", rows)
	}
}
