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

func setupDeviceTemplateScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DeviceTemplate{}, &model.Device{}, &model.DeviceConfig{}); err != nil {
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

func seedDeviceTemplate(t *testing.T, db *gorm.DB, id, name, tenant string) {
	t.Helper()
	now := time.Now().UTC()
	tpl := model.DeviceTemplate{
		ID:        id,
		Name:      name,
		TenantID:  tenant,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("seed template %s: %v", id, err)
	}
}

// TestGetDeviceTemplateListByPageScopeDown 总部作用域 [hq, child] 可见本租户+子租户模板；
// 单元素作用域与旧单租户行为等价；空作用域 fail-closed 返回空。
func TestGetDeviceTemplateListByPageScopeDown(t *testing.T) {
	db := setupDeviceTemplateScopeTestDB(t)
	seedDeviceTemplate(t, db, "tpl-hq", "Template HQ", "hq")
	seedDeviceTemplate(t, db, "tpl-child", "Template Child", "child")
	seedDeviceTemplate(t, db, "tpl-x", "Template X", "tenant-x")

	paged := func(scopes []string, name *string) (int64, interface{}, error) {
		req := &model.GetDeviceTemplateListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}, Name: name}
		return GetDeviceTemplateListByPage(req, scopes)
	}
	idsOf := func(list interface{}) map[string]bool {
		m := map[string]bool{}
		if rows, ok := list.([]*model.DeviceTemplate); ok {
			for _, r := range rows {
				m[r.ID] = true
			}
		}
		return m
	}

	// 总部（hq 为根、child 为子孙）：scope 含二者。
	count, list, err := paged([]string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("scope [hq child]: %v", err)
	}
	got := idsOf(list)
	if count != 2 || !got["tpl-hq"] || !got["tpl-child"] || got["tpl-x"] {
		t.Fatalf("scope [hq child] count=%d ids=%#v", count, got)
	}

	// 单元素作用域 == 旧单租户过滤（叶子租户不受层级影响）。
	count, list, err = paged([]string{"tenant-x"}, nil)
	if err != nil {
		t.Fatalf("single scope: %v", err)
	}
	got = idsOf(list)
	if count != 1 || !got["tpl-x"] {
		t.Fatalf("single scope count=%d ids=%#v", count, got)
	}

	// hq 未含 child 的作用域（如无子孙链）不应看到 child 模板。
	count, list, err = paged([]string{"hq"}, nil)
	if err != nil {
		t.Fatalf("scope [hq]: %v", err)
	}
	got = idsOf(list)
	if count != 1 || got["tpl-child"] || got["tpl-x"] {
		t.Fatalf("scope [hq] count=%d ids=%#v", count, got)
	}

	// 空作用域 fail-closed。
	count, list, err = paged(nil, nil)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if count != 0 || list != nil {
		t.Fatalf("empty scope count=%d list=%#v", count, list)
	}

	// 名称过滤叠加作用域。
	name := "Child"
	count, list, err = paged([]string{"hq", "child"}, &name)
	if err != nil {
		t.Fatalf("name filter: %v", err)
	}
	got = idsOf(list)
	if count != 1 || !got["tpl-child"] {
		t.Fatalf("name filter count=%d ids=%#v", count, got)
	}
}

// TestGetDeviceTemplateMenuScopeDown 下拉菜单同样按作用域过滤。
func TestGetDeviceTemplateMenuScopeDown(t *testing.T) {
	db := setupDeviceTemplateScopeTestDB(t)
	seedDeviceTemplate(t, db, "tpl-hq", "Template HQ", "hq")
	seedDeviceTemplate(t, db, "tpl-child", "Template Child", "child")

	data, err := GetDeviceTemplateMenu(&model.GetDeviceTemplateMenuReq{}, []string{"hq", "child"})
	if err != nil {
		t.Fatalf("menu scope err: %v", err)
	}
	rows, ok := data.([]map[string]interface{})
	if !ok || len(rows) != 2 {
		t.Fatalf("menu scope len=%d data=%#v", len(rows), data)
	}

	data, err = GetDeviceTemplateMenu(&model.GetDeviceTemplateMenuReq{}, []string{"hq"})
	if err != nil {
		t.Fatalf("menu single err: %v", err)
	}
	rows, ok = data.([]map[string]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("menu single len=%d data=%#v", len(rows), data)
	}
}

// TestGetDeviceTemplateStatsTenantGated 统计的模板归属必须落在作用域内：作用域外/空作用域一律失败。
func TestGetDeviceTemplateStatsTenantGated(t *testing.T) {
	db := setupDeviceTemplateScopeTestDB(t)
	seedDeviceTemplate(t, db, "tpl-child", "Template Child", "child")

	if _, err := GetDeviceTemplateStats("tpl-child", []string{"child"}); err != nil {
		t.Fatalf("in-scope stats err: %v", err)
	}
	if _, err := GetDeviceTemplateStats("tpl-child", []string{"hq"}); err == nil {
		t.Fatalf("out-of-scope stats must fail")
	}
	if _, err := GetDeviceTemplateStats("tpl-child", nil); err == nil {
		t.Fatalf("empty-scope stats must fail")
	}
}
