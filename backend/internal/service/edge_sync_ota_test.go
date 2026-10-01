// 文件用途：锁定 OTA 经边分发的目标设备解析语义——批量 IN 查询替代逐设备查询后，
// 仍须按请求顺序为租户内设备生成任务、跳过跨租户/不存在设备，且不因跳过而报错。
package service

import (
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupEdgeOTATestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := "edge_ota_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}, &model.OtaUpgradePackage{}, &model.EdgeSyncTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
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

func TestDistributeEdgeOTAResolvesTargetsInTenantAndOrder(t *testing.T) {
	db := setupEdgeOTATestDB(t)
	const tenantID = "tenant-edge"

	seedDevice := func(id, tenant string) {
		t.Helper()
		if err := db.Create(&model.Device{ID: id, DeviceNumber: "num-" + id, TenantID: tenant, Voucher: "{}"}).Error; err != nil {
			t.Fatalf("seed device %s: %v", id, err)
		}
	}
	seedDevice("gw-1", tenantID)
	seedDevice("dev-1", tenantID)
	seedDevice("dev-2", tenantID)
	seedDevice("dev-foreign", "tenant-other")
	pkgTenant := tenantID
	if err := db.Create(&model.OtaUpgradePackage{ID: "pkg-1", Name: "fw", Version: "1.0.0", TenantID: &pkgTenant}).Error; err != nil {
		t.Fatalf("seed package: %v", err)
	}

	tasks, err := EdgeSyncService{}.DistributeEdgeOTA(&model.EdgeOtaDistributeReq{
		GatewayDeviceID: "gw-1",
		PackageID:       "pkg-1",
		DeviceIDs:       []string{"dev-2", "dev-missing", "dev-foreign", "dev-1"},
	}, &utils.UserClaims{TenantID: tenantID})
	if err != nil {
		t.Fatalf("DistributeEdgeOTA: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2 (foreign/missing skipped)", len(tasks))
	}
	if tasks[0].ResourceID != "dev-2" || tasks[1].ResourceID != "dev-1" {
		t.Fatalf("task order = [%s %s], want [dev-2 dev-1]", tasks[0].ResourceID, tasks[1].ResourceID)
	}
	for _, task := range tasks {
		if task.GatewayDeviceID != "gw-1" || task.TenantID != tenantID {
			t.Fatalf("unexpected task scope: %+v", task)
		}
		if !strings.Contains(task.Payload, `"num-`+task.ResourceID+`"`) {
			t.Fatalf("payload missing target device number: %s", task.Payload)
		}
	}
	var persisted int64
	if err := db.Model(&model.EdgeSyncTask{}).Count(&persisted).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if persisted != 2 {
		t.Fatalf("persisted tasks = %d, want 2", persisted)
	}
}
