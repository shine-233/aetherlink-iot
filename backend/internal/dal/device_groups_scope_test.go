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

func setupDeviceGroupScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Group{}, &model.Device{}, &model.RGroupDevice{}); err != nil {
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

func seedDeviceGroup(t *testing.T, db *gorm.DB, id, name, tenant, parentID string, ownerUserID *string) {
	t.Helper()
	now := time.Now().UTC()
	parent := parentID
	g := model.Group{
		ID:          id,
		ParentID:    &parent,
		Tier:        1,
		Name:        name,
		TenantID:    tenant,
		OwnerUserID: ownerUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("seed group %s: %v", id, err)
	}
}

func seedOwnerDeviceInGroup(t *testing.T, db *gorm.DB, deviceID, groupID, tenant, ownerUserID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.Device{
		ID:           deviceID,
		TenantID:     tenant,
		OwnerUserID:  &ownerUserID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		DeviceNumber: deviceID,
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("seed device %s: %v", deviceID, err)
	}
	if err := db.Create(&model.RGroupDevice{
		GroupID:  groupID,
		DeviceID: deviceID,
		TenantID: tenant,
	}).Error; err != nil {
		t.Fatalf("seed r_group_device %s/%s: %v", groupID, deviceID, err)
	}
}

// TestGetDeviceGroupListByPageScopeDown 自上而下作用域真实结果集：
// 总部作用域 [hq, child] 含自身+子租户分组；单元素作用域与旧单租户行为等价；
// 空作用域 fail-closed 返回空；名称/父级过滤与作用域正确叠加。
func TestGetDeviceGroupListByPageScopeDown(t *testing.T) {
	db := setupDeviceGroupScopeTestDB(t)
	hqRoot := "grp-hq-root"
	seedDeviceGroup(t, db, hqRoot, "HQ Root", "hq", "0", nil)
	seedDeviceGroup(t, db, "grp-hq-a", "HQ A", "hq", hqRoot, nil)
	seedDeviceGroup(t, db, "grp-child-root", "Child Root", "child", "0", nil)
	seedDeviceGroup(t, db, "grp-x-root", "X Root", "tenant-x", "0", nil)

	req := model.GetDeviceGroupsListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}
	idsOf := func(list interface{}) map[string]bool {
		m := map[string]bool{}
		if rows, ok := list.([]*model.Group); ok {
			for _, r := range rows {
				m[r.ID] = true
			}
		}
		return m
	}

	// 总部（hq 为根、child 为子孙）：只返回 self∪child 的分组。
	count, list, err := GetDeviceGroupListByPage(req, []string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("scope [hq child]: %v", err)
	}
	got := idsOf(list)
	if count != 3 || !got["grp-hq-root"] || !got["grp-hq-a"] || !got["grp-child-root"] || got["grp-x-root"] {
		t.Fatalf("scope [hq child] count=%d ids=%#v", count, got)
	}

	// hq 单独作用域看不到 child 分组。
	count, list, err = GetDeviceGroupListByPage(req, []string{"hq"}, nil)
	if err != nil {
		t.Fatalf("scope [hq]: %v", err)
	}
	got = idsOf(list)
	if count != 2 || got["grp-child-root"] || got["grp-x-root"] {
		t.Fatalf("scope [hq] count=%d ids=%#v", count, got)
	}

	// 单元素作用域 == 旧单租户过滤。
	count, list, err = GetDeviceGroupListByPage(req, []string{"tenant-x"}, nil)
	if err != nil {
		t.Fatalf("single scope: %v", err)
	}
	got = idsOf(list)
	if count != 1 || !got["grp-x-root"] {
		t.Fatalf("single scope count=%d ids=%#v", count, got)
	}

	// 空作用域 fail-closed。
	count, list, err = GetDeviceGroupListByPage(req, nil, nil)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if count != 0 {
		t.Fatalf("empty scope count=%d", count)
	}
	if rows, ok := list.([]*model.Group); !ok || len(rows) != 0 {
		t.Fatalf("empty scope list must be empty, got %#v", list)
	}

	// 名称过滤叠加作用域。
	name := "Child"
	req2 := model.GetDeviceGroupsListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}, Name: &name}
	count, list, err = GetDeviceGroupListByPage(req2, []string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("name filter: %v", err)
	}
	got = idsOf(list)
	if count != 1 || !got["grp-child-root"] {
		t.Fatalf("name filter count=%d ids=%#v", count, got)
	}
}

// TestGetDeviceGroupAllScopeDown 树/下拉全量同样按作用域过滤。
func TestGetDeviceGroupAllScopeDown(t *testing.T) {
	db := setupDeviceGroupScopeTestDB(t)
	seedDeviceGroup(t, db, "grp-hq-root", "HQ Root", "hq", "0", nil)
	seedDeviceGroup(t, db, "grp-child-root", "Child Root", "child", "0", nil)
	seedDeviceGroup(t, db, "grp-x-root", "X Root", "tenant-x", "0", nil)

	rows, err := GetDeviceGroupAll([]string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("all scope err: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("all scope [hq child] len=%d", len(rows))
	}

	rows, err = GetDeviceGroupAll([]string{"hq"}, nil)
	if err != nil {
		t.Fatalf("all scope [hq] err: %v", err)
	}
	if len(rows) != 1 || rows[0].TenantID != "hq" {
		t.Fatalf("all scope [hq] len=%d rows=%#v", len(rows), rows)
	}

	rows, err = GetDeviceGroupAll(nil, nil)
	if err != nil {
		t.Fatalf("all empty scope err: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("all empty scope len=%d", len(rows))
	}
}

// TestGetVisibleGroupIDsForOwnerScopeDown owner 可见集合按作用域收口：
// u1 在 hq 的设备所在分组与其祖先可见；child/tenant-x 分组、他租户 owner 的分组不泄漏；
// 空作用域/空 owner fail-closed。
func TestGetVisibleGroupIDsForOwnerScopeDown(t *testing.T) {
	db := setupDeviceGroupScopeTestDB(t)
	seedDeviceGroup(t, db, "grp-hq-root", "HQ Root", "hq", "0", nil)
	seedDeviceGroup(t, db, "grp-hq-a", "HQ A", "hq", "grp-hq-root", nil)
	seedDeviceGroup(t, db, "grp-hq-owned", "HQ Owned", "hq", "grp-hq-root", stringPtr("u1"))
	seedDeviceGroup(t, db, "grp-child-root", "Child Root", "child", "0", stringPtr("u2"))
	seedDeviceGroup(t, db, "grp-x-root", "X Root", "tenant-x", "0", nil)
	seedOwnerDeviceInGroup(t, db, "dev-u1", "grp-hq-a", "hq", "u1")

	visible, err := GetVisibleGroupIDsForOwner([]string{"hq"}, "u1")
	if err != nil {
		t.Fatalf("visible [hq]: %v", err)
	}
	want := map[string]bool{"grp-hq-a": true, "grp-hq-root": true, "grp-hq-owned": true}
	for _, id := range visible {
		if !want[id] {
			t.Fatalf("visible [hq] leaked %q (all=%#v)", id, visible)
		}
	}
	if len(visible) != len(want) {
		t.Fatalf("visible [hq] len=%d want %d got %#v", len(visible), len(want), visible)
	}

	// 扩大作用域到 child：u1 在 child 无资源，结果集不变；child 自有 owner 分组不出现。
	visible, err = GetVisibleGroupIDsForOwner([]string{"hq", "child"}, "u1")
	if err != nil {
		t.Fatalf("visible [hq child]: %v", err)
	}
	if len(visible) != len(want) {
		t.Fatalf("visible [hq child] len=%d want %d got %#v", len(visible), len(want), visible)
	}

	// 空作用域与空 owner fail-closed。
	empty, err := GetVisibleGroupIDsForOwner(nil, "u1")
	if err != nil || len(empty) != 0 {
		t.Fatalf("visible empty scope err=%v len=%d", err, len(empty))
	}
	empty, err = GetVisibleGroupIDsForOwner([]string{"hq"}, "")
	if err != nil || len(empty) != 0 {
		t.Fatalf("visible empty owner err=%v len=%d", err, len(empty))
	}
}

func stringPtr(s string) *string {
	return &s
}
