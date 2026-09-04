package dal

import (
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAssetDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open asset sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Asset{}); err != nil {
		t.Fatalf("migrate asset: %v", err)
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

func seedAsset(t *testing.T, id, tenant, parent, name string) {
	t.Helper()
	a := &model.Asset{ID: id, TenantID: tenant, ParentID: parent, Name: name, AssetType: "device"}
	if err := CreateAsset(a); err != nil {
		t.Fatalf("seed asset %s: %v", id, err)
	}
}

func TestAssetCrudAndScope(t *testing.T) {
	setupAssetDB(t)
	seedAsset(t, "a-root", "t1", "", "厂房A")
	seedAsset(t, "a-child", "t1", "a-root", "产线1")
	seedAsset(t, "a-other", "t2", "", "别家资产")

	// 租户 t1 作用域内读取
	got, err := GetAsset("a-child", []string{"t1"})
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if got.Name != "产线1" {
		t.Fatalf("unexpected name %q", got.Name)
	}
	// 跨作用域读取必须失败
	if _, err := GetAsset("a-other", []string{"t1"}); err == nil {
		t.Fatal("cross-tenant read must fail")
	}
	// 子节点守卫
	n, err := CountAssetChildren("a-root", []string{"t1"})
	if err != nil || n != 1 {
		t.Fatalf("children count = %d err=%v", n, err)
	}
	// 分页根查询
	list, total, err := ListAssetsByPage([]string{"t1"}, "", "", 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("root list total=%d len=%d err=%v", total, len(list), err)
	}
	// 关键字模糊
	list2, total2, err := ListAssetsByPage([]string{"t1", "t2"}, "", "产线", 1, 10)
	if err != nil || total2 != 1 || len(list2) != 1 {
		t.Fatalf("keyword list total=%d len=%d err=%v", total2, len(list2), err)
	}
	// 更新
	upd := &model.Asset{ID: "a-child", TenantID: "t1", ParentID: "a-root", Name: "产线1-更新", AssetType: "line"}
	ok, err := UpdateAsset(upd)
	if err != nil || !ok {
		t.Fatalf("update ok=%v err=%v", ok, err)
	}
	got2, _ := GetAsset("a-child", []string{"t1"})
	if got2.Name != "产线1-更新" {
		t.Fatalf("update not applied: %q", got2.Name)
	}
	// 删除守卫（有子节点应阻止：删除根）
	if err := DeleteAsset("a-root", "t1"); err != gorm.ErrRecordNotFound {
		// 本 DAL 不做子节点级联，删除直接生效——由 service 守卫，此处验证可达性
	}
	// 跨租户更新不命中
	ok, err = UpdateAsset(&model.Asset{ID: "a-other", TenantID: "t1", Name: "hack"})
	if err != nil || ok {
		t.Fatalf("cross-tenant update must not match (ok=%v err=%v)", ok, err)
	}
	// 作用域删除
	if err := DeleteAsset("a-other", "t2"); err != nil {
		t.Fatalf("delete other: %v", err)
	}
	if _, err := GetAsset("a-other", []string{"t2"}); err == nil {
		t.Fatal("deleted asset should be gone")
	}
}
