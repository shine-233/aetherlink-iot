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

func setupDeviceModelScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DeviceModelTelemetry{}, &model.DeviceModelCommand{}); err != nil {
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

func seedDeviceModelTelemetry(t *testing.T, db *gorm.DB, id, templateID, identifier, tenant string) {
	t.Helper()
	now := time.Now().UTC()
	row := model.DeviceModelTelemetry{
		ID:               id,
		DeviceTemplateID: templateID,
		DataIdentifier:   identifier,
		TenantID:         tenant,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed telemetry %s: %v", id, err)
	}
}

// TestGetDeviceModelTelemetryListByPageScopeDown 总部作用域 [hq, child] 可读取子租户模板的物模型；
// 单租户/空作用域保持旧隔离语义。
func TestGetDeviceModelTelemetryListByPageScopeDown(t *testing.T) {
	db := setupDeviceModelScopeTestDB(t)
	// tplH 属 hq；tplC 属 child（hq 的子租户）。
	seedDeviceModelTelemetry(t, db, "th-hq", "tplH", "temp_hq", "hq")
	seedDeviceModelTelemetry(t, db, "th-child", "tplC", "temp_child", "child")
	seedDeviceModelTelemetry(t, db, "th-x", "tplX", "temp_x", "tenant-x")

	list := func(templateID string, scopes []string) (int64, []*model.DeviceModelTelemetry, error) {
		req := model.GetDeviceModelListByPageReq{
			PageReq:          model.PageReq{Page: 1, PageSize: 20},
			DeviceTemplateId: templateID,
		}
		return GetDeviceModelTelemetryListByPage(req, scopes)
	}
	idsOf := func(rows []*model.DeviceModelTelemetry) map[string]bool {
		m := map[string]bool{}
		for _, r := range rows {
			m[r.ID] = true
		}
		return m
	}

	// HQ 打开子租户模板 tplC：scope=[hq child] → 仅返回 child 的物模型。
	count, rows, err := list("tplC", []string{"hq", "child"})
	if err != nil {
		t.Fatalf("scope [hq child]: %v", err)
	}
	got := idsOf(rows)
	if count != 1 || !got["th-child"] || got["th-hq"] || got["th-x"] {
		t.Fatalf("scope [hq child] template=tplC count=%d ids=%#v", count, got)
	}

	// 单元素作用域 == 旧单租户（叶子视角只看到自己模板的物模型）。
	count, rows, err = list("tplH", []string{"hq"})
	if err != nil {
		t.Fatalf("single scope: %v", err)
	}
	got = idsOf(rows)
	if count != 1 || !got["th-hq"] || got["th-child"] {
		t.Fatalf("single scope count=%d ids=%#v", count, got)
	}

	// 叶子租户（无子孙）不能借模板 id 越权读其它租户物模型：tenant-x 查 tplC → 空。
	count, rows, err = list("tplC", []string{"tenant-x"})
	if err != nil {
		t.Fatalf("leaf cross-tenant: %v", err)
	}
	if count != 0 || len(rows) != 0 {
		t.Fatalf("leaf cross-tenant must be empty, count=%d rows=%#v", count, rows)
	}

	// 空作用域 fail-closed。
	count, rows, err = list("tplC", nil)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if count != 0 || len(rows) != 0 {
		t.Fatalf("empty scope count=%d rows=%#v", count, rows)
	}
}
