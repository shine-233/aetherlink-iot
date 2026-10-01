// 文件用途：资产服务错误/响应契约钉桩（kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库（含最小 tenants 父子链）上断言 Create/Update/Delete/Get/List/Tree 的
//
//	平台级/空租户拒绝、不存在/跨租户（裸 CodeNotFound）、父租户可读不可写子租户资产、
//	存在子节点拒删、DB 故障（裸 CodeDBError，不带 data）各分支，以及空作用域列表/树的形状。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit；复用 crud_contract_pin_test 的 pinWire。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAssetPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:assetpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Asset{}); err != nil {
		t.Fatal(err)
	}
	// 最小租户父子链：tenant-p 为 tenant-c 的父租户（自上而下可读）。
	for _, q := range []string{
		"CREATE TABLE tenants (id text primary key, parent_tenant_id text)",
		"INSERT INTO tenants (id, parent_tenant_id) VALUES ('tenant-p', NULL), ('tenant-c', 'tenant-p'), ('tenant-x', NULL)",
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	now := time.Now().UTC()
	for _, a := range []*model.Asset{
		{ID: "a-root", TenantID: "tenant-c", Name: "root", AssetType: "site", CreatedAt: &now, UpdatedAt: &now},
		{ID: "a-leaf", TenantID: "tenant-c", ParentID: "a-root", Name: "leaf", AssetType: "device", CreatedAt: &now, UpdatedAt: &now},
	} {
		if err := db.Create(a).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestAssetContractPins(t *testing.T) {
	svc := &Asset{}
	child := &utils.UserClaims{ID: "u-c", TenantID: "tenant-c", Authority: "TENANT_ADMIN"}
	parent := &utils.UserClaims{ID: "u-p", TenantID: "tenant-p", Authority: "TENANT_ADMIN"}
	other := &utils.UserClaims{ID: "u-x", TenantID: "tenant-x", Authority: "TENANT_ADMIN"}
	platform := &utils.UserClaims{ID: "u-s", Authority: "SYS_ADMIN"}
	nf := errcode.New(errcode.CodeNotFound)
	dbErr := errcode.New(errcode.CodeDBError)
	createGate := errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产；请切换至租户")
	writeGate := errcode.NewWithMessage(errcode.CodeParamError, "平台级（无租户）暂不支持资产")

	get := func(c *utils.UserClaims, id string) error { _, err := svc.Get(c, id); return err }
	update := func(c *utils.UserClaims, id string) error {
		_, err := svc.Update(c, &AssetReq{ID: id, Name: "renamed"})
		return err
	}
	del := func(c *utils.UserClaims, id string) error { return svc.Delete(c, id) }

	type pin struct {
		name string
		got  error
		want error
	}
	check := func(t *testing.T, cases []pin) {
		t.Helper()
		for _, tc := range cases {
			if g, w := pinWire(tc.got), pinWire(tc.want); g != w {
				t.Errorf("%s: got %s want %s", tc.name, g, w)
			}
		}
	}

	t.Run("gates and not found", func(t *testing.T) {
		setupAssetPinDB(t)
		_, createErr := svc.Create(platform, &AssetReq{Name: "x"})
		_, createNil := svc.Create(nil, &AssetReq{Name: "x"})
		_, missingID := svc.Update(child, &AssetReq{ID: " ", Name: "x"})
		check(t, []pin{
			{"create platform", createErr, createGate},
			{"create nil", createNil, createGate},
			{"update platform", update(platform, "a-leaf"), writeGate},
			{"update nil", update(nil, "a-leaf"), writeGate},
			{"update missing id", missingID, errcode.NewWithMessage(errcode.CodeParamError, "缺少资产 ID")},
			{"delete platform", del(platform, "a-leaf"), writeGate},
			{"get nil", get(nil, "a-leaf"), nf},
			{"get platform", get(platform, "a-leaf"), nf},
			{"get missing", get(child, "nope"), nf},
			{"get cross tenant", get(other, "a-leaf"), nf},
			{"get parent reads child", get(parent, "a-leaf"), nil},
			{"update missing", update(child, "nope"), nf},
			{"update cross tenant", update(other, "a-leaf"), nf},
			{"update parent cannot write child", update(parent, "a-leaf"), nf},
			{"delete missing", del(child, "nope"), nf},
			{"delete cross tenant", del(other, "a-leaf"), nf},
			{"delete parent cannot write child", del(parent, "a-leaf"), nf},
			{"delete with children", del(child, "a-root"), errcode.NewWithMessage(errcode.CodeParamError, "存在子节点，请先删除或迁移子树")},
			{"delete leaf ok", del(child, "a-leaf"), nil},
			{"get after delete", get(child, "a-leaf"), nf},
		})
	})

	t.Run("shapes", func(t *testing.T) {
		setupAssetPinDB(t)
		list, total, err := svc.List(nil, "", "", 1, 10)
		if b, _ := json.Marshal(list); err != nil || total != 0 || string(b) != "[]" {
			t.Fatalf("nil list: %s %d %v", b, total, err)
		}
		tree, err := svc.Tree(platform)
		if b, _ := json.Marshal(tree); err != nil || string(b) != "[]" {
			t.Fatalf("platform tree: %s %v", b, err)
		}
		got, err := svc.Update(child, &AssetReq{ID: "a-leaf", ParentID: "a-root", Name: " renamed "})
		if err != nil || got.Name != "renamed" || got.AssetType != "device" || got.TenantID != "tenant-c" {
			t.Fatalf("update: %+v %v", got, err)
		}
	})

	t.Run("db faults", func(t *testing.T) {
		db := setupAssetPinDB(t)
		if err := db.Migrator().DropTable(&model.Asset{}); err != nil {
			t.Fatal(err)
		}
		check(t, []pin{
			{"get", get(child, "a-leaf"), dbErr},
			{"update", update(child, "a-leaf"), dbErr},
			{"delete", del(child, "a-leaf"), dbErr},
		})
	})
}
