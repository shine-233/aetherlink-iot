// 文件用途：行业方案（TB-19）读/删/安装入口的错误与列表响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上逐一断言 nil/空租户、跨租户、不存在、DB 故障分支的 errcode JSON，
//
//	以及列表 map 的值类型（int64 / []model.IndustrySolution）与详情 map 的键集。
//
// 关键注意事项：期望值一律用 errcode/authz 原始构造器手写，不经 kit，避免钉桩随实现漂移。
package service

import (
	"context"
	"errors"
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

func setupIndustrySolutionPinDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ispin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE industry_solutions (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
			description TEXT, resources BLOB NOT NULL, status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE industry_solution_installs (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, solution_id TEXT NOT NULL,
			solution_name TEXT NOT NULL, item_index INTEGER NOT NULL, resource_type TEXT NOT NULL, resource_id TEXT NOT NULL,
			target_id TEXT, status TEXT NOT NULL, error TEXT, created_at DATETIME)`,
		`INSERT INTO industry_solutions (id, tenant_id, name, resources, status, created_at, updated_at) VALUES ('is-a', 'tenant-a', 'sol', X'5B5D', 'active', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

func TestIndustrySolutionContractPins(t *testing.T) {
	setupIndustrySolutionPinDB(t)
	ctx := context.Background()
	svc := &IndustrySolutionService{}
	ownA := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	ownB := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	noTenant := &utils.UserClaims{ID: "u-sys", Authority: "SYS_ADMIN"}

	unauth := pinWire(errcode.New(errcode.CodeUnauthorized))
	notFound := pinWire(errcode.New(errcode.CodeNotFound))
	writeDeny := func(action string) string { return pinWire(authz.NoPermission("no permission to " + action)) }
	writeInit := func(action string) string {
		return pinWire(authz.NoPermission("complete tenant initialization before " + action))
	}

	get := func(id string, c *utils.UserClaims) error { _, err := svc.GetIndustrySolution(ctx, id, c); return err }
	del := func(id string, c *utils.UserClaims) error { return svc.DeleteIndustrySolution(ctx, id, c) }
	install := func(id string, c *utils.UserClaims) error {
		_, err := svc.InstallIndustrySolution(ctx, id, nil, c)
		return err
	}
	list := func(c *utils.UserClaims) (map[string]interface{}, error) {
		return svc.ListIndustrySolutions(ctx, 1, 20, c)
	}

	cases := []struct {
		name string
		fn   func(string, *utils.UserClaims) error
		id   string
		c    *utils.UserClaims
		want string
	}{
		{"get nil", get, "is-a", nil, unauth},
		{"get empty tenant", get, "is-a", noTenant, unauth},
		{"get cross tenant", get, "is-a", ownB, notFound},
		{"get missing", get, "nope", ownA, notFound},
		{"get own (id trimmed)", get, " is-a ", ownA, "<nil>"},
		{"install nil", install, "is-a", nil, writeDeny("install industry solution")},
		{"install empty tenant", install, "is-a", noTenant, writeInit("install industry solution")},
		{"install cross tenant", install, "is-a", ownB, notFound},
		{"install missing", install, "nope", ownA, notFound},
		// 资源清单为 '[]'：越过加载后命中清单校验。
		{"install empty refs", install, "is-a", ownA,
			pinWire(errcode.NewWithMessage(errcode.CodeParamError, "solution resource list is empty or corrupted"))},
		{"delete nil", del, "is-a", nil, writeDeny("delete industry solution")},
		{"delete empty tenant", del, "is-a", noTenant, writeInit("delete industry solution")},
		{"delete cross tenant", del, "is-a", ownB, notFound},
		{"delete missing", del, "nope", ownA, notFound},
	}
	for _, tc := range cases {
		if got := pinWire(tc.fn(tc.id, tc.c)); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}

	// 详情：键集固定为 solution/installs。
	detail, err := svc.GetIndustrySolution(ctx, "is-a", ownA)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := detail["solution"].(*model.IndustrySolution); !ok || len(detail) != 2 {
		t.Errorf("detail shape: %#v", detail)
	}
	if _, ok := detail["installs"].([]model.IndustrySolutionInstall); !ok {
		t.Errorf("detail installs type: %T", detail["installs"])
	}

	// 列表：值类型钉死。
	res, err := list(ownA)
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 1 {
		t.Errorf("list total: %#v", res["total"])
	}
	if got := fmt.Sprintf("%T", res["list"]); got != "[]model.IndustrySolution" || len(res) != 2 {
		t.Errorf("list shape: %s %v", got, res)
	}
	for _, c := range []*utils.UserClaims{nil, noTenant} {
		if _, err := list(c); pinWire(err) != unauth {
			t.Errorf("list deny: %s", pinWire(err))
		}
	}

	// 本租户删除成功；再删为 not found。
	if got := pinWire(del("is-a", ownA)); got != "<nil>" {
		t.Errorf("delete own: %s", got)
	}
	if got := pinWire(del("is-a", ownA)); got != notFound {
		t.Errorf("delete again: %s", got)
	}

	// DB 故障：加载/删除类错误一律掩码为 not found；列表为固定消息的 DBError（无 data）。
	if err := global.DB.Migrator().DropTable("industry_solutions"); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(string, *utils.UserClaims) error{"get": get, "install": install, "delete": del} {
		if got := pinWire(fn("is-a", ownA)); got != notFound {
			t.Errorf("%s db fault: %s", name, got)
		}
	}
	_, lerr := list(ownA)
	var ec *errcode.Error
	if !errors.As(lerr, &ec) || pinWire(lerr) != pinWire(errcode.NewWithMessage(errcode.CodeDBError, "list solutions failed")) {
		t.Errorf("list db fault: %s", pinWire(lerr))
	}
}
