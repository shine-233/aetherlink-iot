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

func setupCountAndFindGenPageDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SceneLog{}, &model.DeviceModelCustomControl{}); err != nil {
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

// TestCountAndFindGenPageSceneLog 计数为筛选后总数（不受分页影响），分页结果按 executed_at 倒序。
func TestCountAndFindGenPageSceneLog(t *testing.T) {
	db := setupCountAndFindGenPageDB(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		row := model.SceneLog{ID: fmt.Sprintf("l%d", i), SceneID: "s1", ExecutedAt: base.Add(time.Duration(i) * time.Hour), TenantID: "t1"}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := db.Create(&model.SceneLog{ID: "other", SceneID: "s2", ExecutedAt: base, TenantID: "t1"}).Error; err != nil {
		t.Fatalf("seed other: %v", err)
	}

	req := model.GetSceneLogListByPageReq{ID: "s1"}
	req.Page, req.PageSize = 2, 2
	count, rows, err := GetSceneLogByPage(req)
	if err != nil {
		t.Fatalf("GetSceneLogByPage: %v", err)
	}
	if count != 5 {
		t.Fatalf("count=%d want 5", count)
	}
	if len(rows) != 2 || rows[0].ID != "l2" || rows[1].ID != "l1" {
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		t.Fatalf("page 2 rows=%v want [l2 l1]", ids)
	}
}

// TestCountAndFindGenPageCustomControl 租户/模板/启用状态筛选 + created_at 倒序。
func TestCountAndFindGenPageCustomControl(t *testing.T) {
	db := setupCountAndFindGenPageDB(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seed := []model.DeviceModelCustomControl{
		{ID: "c1", DeviceTemplateID: "tpl", EnableStatus: "enable", TenantID: "t1", CreatedAt: base},
		{ID: "c2", DeviceTemplateID: "tpl", EnableStatus: "enable", TenantID: "t1", CreatedAt: base.Add(time.Hour)},
		{ID: "c3", DeviceTemplateID: "tpl", EnableStatus: "disable", TenantID: "t1", CreatedAt: base.Add(2 * time.Hour)},
		{ID: "c4", DeviceTemplateID: "tpl", EnableStatus: "enable", TenantID: "t2", CreatedAt: base},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	enable := "enable"
	req := model.GetDeviceModelListByPageReq{DeviceTemplateId: "tpl", EnableStatus: &enable}
	count, rows, err := GetDeviceModelCustomControlByPage(req, "t1")
	if err != nil {
		t.Fatalf("GetDeviceModelCustomControlByPage: %v", err)
	}
	if count != 2 || len(rows) != 2 || rows[0].ID != "c2" || rows[1].ID != "c1" {
		t.Fatalf("count=%d rows=%d (first=%v) want 2 rows [c2 c1]", count, len(rows), rows)
	}
}
