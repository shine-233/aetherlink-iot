// 文件用途：Product 服务错误/响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：在 sqlite 内存库上断言 Get/Update/Delete/List 的空租户、跨租户、不存在、
//
//	DB 故障（删表）分支的 errcode JSON（含 UseCustomMsg），以及列表 map 的值类型与 JSON。
//
// 关键注意事项：期望值一律用 errcode/authz 原始构造器手写，不经 kit。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/authz"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupProductPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:productpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE products (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT, product_type TEXT, product_key TEXT,
			product_model TEXT, image_url TEXT, created_at DATETIME NOT NULL, remark TEXT, additional_info TEXT, tenant_id TEXT, device_config_id TEXT)`,
		`CREATE TABLE device_configs (id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE devices (id TEXT PRIMARY KEY, product_id TEXT, tenant_id TEXT)`,
		`INSERT INTO products (id, name, created_at, tenant_id) VALUES ('p-a', 'prod', '2026-01-02 03:04:05', 'tenant-a')`,
		`INSERT INTO products (id, name, created_at, tenant_id) VALUES ('p-used', 'used', '2026-01-01 03:04:05', 'tenant-a')`,
		`INSERT INTO devices (id, product_id, tenant_id) VALUES ('d1', 'p-used', 'tenant-a')`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func TestProductContractPins(t *testing.T) {
	svc := &Product{}
	own := &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	other := &utils.UserClaims{ID: "u2", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	blank := &utils.UserClaims{ID: "u3", Authority: "SYS_ADMIN"}
	nf := errcode.New(errcode.CodeNotFound)
	unauth := errcode.New(errcode.CodeUnauthorized)
	newName := "renamed"

	type op struct {
		name           string
		call           func(id string, c *utils.UserClaims) error
		nilErr, blankE error
	}
	ops := []op{
		{"get", func(id string, c *utils.UserClaims) error { _, err := svc.GetProductByID(id, c); return err }, unauth, unauth},
		{"update", func(id string, c *utils.UserClaims) error {
			_, err := svc.UpdateProduct(&model.UpdateProductReq{Id: id, Name: &newName}, c)
			return err
		}, authz.NoPermission("no permission to update product"), authz.NoPermission("complete tenant initialization before update product")},
		{"delete", func(id string, c *utils.UserClaims) error { return svc.DeleteProduct(id, c) },
			authz.NoPermission("no permission to delete product"), authz.NoPermission("complete tenant initialization before delete product")},
	}
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			db := setupProductPinDB(t)
			cases := []struct {
				name string
				id   string
				c    *utils.UserClaims
				want error
			}{
				{"nil claims", "p-a", nil, o.nilErr},
				{"blank tenant", "p-a", blank, o.blankE},
				{"cross tenant", "p-a", other, nf},
				{"missing", "nope", own, nf},
			}
			for _, tc := range cases {
				if g, w := pinWire(o.call(tc.id, tc.c)), pinWire(tc.want); g != w {
					t.Errorf("%s: got %s want %s", tc.name, g, w)
				}
			}
			if err := db.Exec(`DROP TABLE products`).Error; err != nil {
				t.Fatal(err)
			}
			if g, w := pinWire(o.call("p-a", own)), pinWire(nf); g != w {
				t.Errorf("db fault masked as not found: got %s want %s", g, w)
			}
		})
	}

	t.Run("delete referenced and happy", func(t *testing.T) {
		setupProductPinDB(t)
		err := svc.DeleteProduct("p-used", own)
		want := errcode.NewWithMessage(errcode.CodeParamError, "cannot delete product: devices are still referencing this product")
		if g, w := pinWire(err), pinWire(want); g != w {
			t.Errorf("referenced: got %s want %s", g, w)
		}
		if err := svc.DeleteProduct("p-a", own); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := svc.GetProductByID("p-a", own); pinWire(err) != pinWire(nf) {
			t.Fatalf("after delete: %v", err)
		}
	})

	t.Run("update happy", func(t *testing.T) {
		setupProductPinDB(t)
		got, err := svc.UpdateProduct(&model.UpdateProductReq{Id: "p-a", Name: &newName}, own)
		if err != nil || got.Name != "renamed" {
			t.Fatalf("update: %+v %v", got, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		db := setupProductPinDB(t)
		for _, c := range []*utils.UserClaims{nil, blank} { // nil：迁移前为解引用 panic，迁移后经 kit.TenantUnauthorized 报 401
			if _, err := svc.GetProductList(&model.GetProductListByPageReq{}, c); pinWire(err) != pinWire(unauth) {
				t.Errorf("gate %v: %s", c, pinWire(err))
			}
		}
		res, err := svc.GetProductList(&model.GetProductListByPageReq{}, own)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := res["total"].(int64); !ok {
			t.Errorf("total type %T", res["total"])
		}
		if _, ok := res["list"].([]model.ProductList); !ok {
			t.Errorf("list type %T", res["list"])
		}
		b, _ := json.Marshal(res)
		if !strings.HasPrefix(string(b), `{"list":[{"id":"p-a","name":"prod",`) || !strings.HasSuffix(string(b), `"total":2}`) {
			t.Errorf("list json %s", b)
		}
		empty, err := svc.GetProductList(&model.GetProductListByPageReq{}, other)
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := json.Marshal(empty); string(b) != `{"list":[],"total":0}` {
			t.Errorf("empty json %s", b)
		}
		if err := db.Exec(`DROP TABLE products`).Error; err != nil {
			t.Fatal(err)
		}
		_, err = svc.GetProductList(&model.GetProductListByPageReq{}, own)
		if !strings.Contains(pinWire(err), `"sql_error"`) || !strings.Contains(pinWire(err), fmt.Sprintf(`"code":%d`, errcode.CodeDBError)) {
			t.Errorf("list db fault: %s", pinWire(err))
		}
	})
}
