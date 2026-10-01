// 文件用途：RuleChain 服务 CRUD 错误/响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：在 sqlite 内存库上断言 Get/Export/Update/Delete 的 nil claims、空租户、跨租户、
//
//	不存在、DB 故障（删表）分支的 errcode JSON（含 UseCustomMsg），以及 Delete 的被引用拒绝。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit。
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRuleChainPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:rcpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.RuleChain{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE device_configs (id TEXT PRIMARY KEY, tenant_id TEXT, default_rule_chain_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, id := range []string{"rc-a", "rc-used"} {
		if err := db.Create(&model.RuleChain{ID: id, TenantID: "tenant-a", Name: id, Enabled: true, Graph: []byte(`{"nodes":[]}`), CreatedAt: &now, UpdatedAt: &now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO device_configs (id, tenant_id, default_rule_chain_id) VALUES ('dc1', 'tenant-a', 'rc-used')`).Error; err != nil {
		t.Fatal(err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func TestRuleChainContractPins(t *testing.T) {
	svc := &RuleChain{}
	own := &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	other := &utils.UserClaims{ID: "u2", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	blank := &utils.UserClaims{ID: "u3", TenantID: "  ", Authority: "TENANT_ADMIN"}

	bareDeny := errcode.New(errcode.CodeNoPermission)
	emptyTenant := errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant id in claims")
	nf := errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	bareNF := errcode.New(errcode.CodeNotFound)
	dbErr := func(table string) error {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": "SQL logic error: no such table: " + table + " (1)"})
	}
	updateBody := func(id string) []byte {
		return []byte(`{"id":"` + id + `","name":"renamed","enabled":true,"graph":` + simpleChainGraph + `}`)
	}

	type op struct {
		name                    string
		call                    func(id string, c *utils.UserClaims) error
		blankErr, nfErr, faultE error
		dropTable               string
	}
	ops := []op{
		{"get", func(id string, c *utils.UserClaims) error { _, err := svc.GetChain(id, c); return err },
			emptyTenant, nf, dbErr("rule_chains"), "rule_chains"},
		{"export", func(id string, c *utils.UserClaims) error { _, err := svc.ExportChain(id, c); return err },
			// ExportChain 不 trim 租户："  " 放行后按租户 "  " 查无此链
			bareNF, bareNF, dbErr("rule_chains"), "rule_chains"},
		{"update", func(id string, c *utils.UserClaims) error { _, err := svc.UpdateChain(updateBody(id), c); return err },
			emptyTenant, nf, dbErr("rule_chains"), "rule_chains"},
		{"delete", func(id string, c *utils.UserClaims) error { return svc.DeleteChain(id, c) },
			emptyTenant, nf, dbErr("device_configs"), "device_configs"},
	}
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			db := setupRuleChainPinDB(t)
			cases := []struct {
				name string
				id   string
				c    *utils.UserClaims
				want error
			}{
				{"nil claims", "rc-a", nil, bareDeny},
				{"blank tenant", "rc-a", blank, o.blankErr},
				{"cross tenant", "rc-a", other, o.nfErr},
				{"missing", "nope", own, o.nfErr},
			}
			for _, tc := range cases {
				if g, w := pinWire(o.call(tc.id, tc.c)), pinWire(tc.want); g != w {
					t.Errorf("%s: got %s want %s", tc.name, g, w)
				}
			}
			if err := db.Exec(`DROP TABLE ` + o.dropTable).Error; err != nil {
				t.Fatal(err)
			}
			if g, w := pinWire(o.call("rc-a", own)), pinWire(o.faultE); g != w {
				t.Errorf("db fault: got %s want %s", g, w)
			}
		})
	}

	t.Run("delete referenced, empty id and happy paths", func(t *testing.T) {
		setupRuleChainPinDB(t)
		want := errcode.NewWithMessage(errcode.CodeParamError,
			"rule chain is referenced by device profile default_rule_chain_id; unbind it first")
		if g, w := pinWire(svc.DeleteChain("rc-used", own)), pinWire(want); g != w {
			t.Errorf("referenced: got %s want %s", g, w)
		}
		if g, w := pinWire(svc.DeleteChain(" ", own)), pinWire(errcode.NewWithMessage(errcode.CodeParamError, "id is required")); g != w {
			t.Errorf("empty id: got %s want %s", g, w)
		}
		got, err := svc.UpdateChain(updateBody("rc-a"), own)
		if err != nil || got == nil || got.Name != "renamed" {
			t.Fatalf("update: %+v %v", got, err)
		}
		exp, err := svc.ExportChain("rc-a", own)
		if err != nil || exp.Name != "renamed" || len(exp.Graph) == 0 {
			t.Fatalf("export: %+v %v", exp, err)
		}
		if err := svc.DeleteChain("rc-a", own); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := svc.GetChain("rc-a", own); pinWire(err) != pinWire(nf) {
			t.Fatalf("after delete: %s", pinWire(err))
		}
	})
}
