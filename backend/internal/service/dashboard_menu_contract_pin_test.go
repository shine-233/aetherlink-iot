// 文件用途：仪表盘菜单服务错误/响应契约钉桩（kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言 nil claims、空白租户、空白/超量 dashboard_id、跨租户/不存在看板、
//
//	各操作 DB 故障（删表）的 errcode JSON（含 UseCustomMsg 与 operation/dashboard_id 数据键），
//	以及 Get/批量 Get 的返回形状（未配置为 nil、批量缺失键映射 nil）。
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
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDashboardMenuPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dmpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TenantDashboardMenu{}, &model.VisDashboard{}, &model.Board{}); err != nil {
		t.Fatal(err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	now := time.Now().UTC()
	ta, tb := "tenant-a", "tenant-b"
	visName := "vis-name"
	must := func(v interface{}) {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(&model.VisDashboard{ID: "vis-a", TenantID: &ta, DashboardName: &visName})
	must(&model.VisDashboard{ID: "vis-b", TenantID: &tb})
	must(&model.Board{ID: "board-a", Name: "board-name", TenantID: ta, HomeFlag: "N", CreatedAt: now, UpdatedAt: now})
	must(&model.TenantDashboardMenu{ID: "m-a", TenantID: ta, DashboardID: "vis-a", DashboardName: "vis-name", MenuName: "menu", ParentCode: "home", Sort: 3, Enabled: true, CreatedAt: now, UpdatedAt: now})
	return db
}

func TestDashboardMenuContractPins(t *testing.T) {
	svc := &DashboardMenu{}
	ca := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a"}
	blank := &utils.UserClaims{ID: "u-x", TenantID: "  "}
	noPerm := errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage dashboard menu")
	noTenant := errcode.NewWithMessage(errcode.CodeNoPermission, "tenant dashboard menu is only available for tenant users")
	idReq := errcode.NewWithMessage(errcode.CodeParamError, "dashboard_id is required")
	idsReq := errcode.NewWithMessage(errcode.CodeParamError, "dashboard_ids are required")
	tooMany := errcode.NewWithMessage(errcode.CodeParamError, "dashboard_ids cannot exceed 100")
	foreign := errcode.NewWithMessage(errcode.CodeNoPermission, "dashboard not found or no permission")
	req := func() *model.UpsertTenantDashboardMenuReq { return &model.UpsertTenantDashboardMenuReq{MenuName: "mm"} }

	get := func(c *utils.UserClaims, id string) error { _, err := svc.GetTenantDashboardMenu(c, id); return err }
	batch := func(c *utils.UserClaims, ids ...string) error { _, err := svc.GetTenantDashboardMenus(c, ids); return err }
	upsert := func(c *utils.UserClaims, id string) error { _, err := svc.UpsertTenantDashboardMenu(c, id, req()); return err }
	del := func(c *utils.UserClaims, id string) error { return svc.DeleteTenantDashboardMenu(c, id) }

	over := make([]string, 101)
	for i := range over {
		over[i] = fmt.Sprintf("d-%d", i)
	}

	t.Run("gates", func(t *testing.T) {
		setupDashboardMenuPinDB(t)
		cases := []struct {
			name string
			got  error
			want error
		}{
			{"get nil", get(nil, "vis-a"), noPerm},
			{"get blank tenant", get(blank, "vis-a"), noTenant},
			{"get blank id", get(ca, "  "), idReq},
			{"batch nil", batch(nil, "vis-a"), noPerm},
			{"batch blank tenant", batch(blank, "vis-a"), noTenant},
			{"batch empty", batch(ca), idsReq},
			{"batch blank id", batch(ca, "vis-a", " "), idReq},
			{"batch over", batch(ca, over...), tooMany},
			{"upsert nil", upsert(nil, "vis-a"), noPerm},
			{"upsert blank tenant", upsert(blank, "vis-a"), noTenant},
			{"upsert blank id", upsert(ca, ""), idReq},
			{"upsert foreign vis", upsert(ca, "vis-b"), foreign},
			{"upsert missing", upsert(ca, "nope"), foreign},
			{"delete nil", del(nil, "vis-a"), noPerm},
			{"delete blank tenant", del(blank, "vis-a"), noTenant},
			{"delete blank id", del(ca, " "), idReq},
			{"delete missing ok", del(ca, "nope"), nil},
		}
		for _, tc := range cases {
			if g, w := pinWire(tc.got), pinWire(tc.want); g != w {
				t.Errorf("%s: got %s want %s", tc.name, g, w)
			}
		}
	})

	t.Run("shapes", func(t *testing.T) {
		setupDashboardMenuPinDB(t)
		rsp, err := svc.GetTenantDashboardMenu(ca, " vis-a ")
		if err != nil || rsp == nil || rsp.MenuName != "menu" || rsp.DashboardID != "vis-a" {
			t.Fatalf("get: %+v %v", rsp, err)
		}
		if rsp, err := svc.GetTenantDashboardMenu(ca, "nope"); err != nil || rsp != nil {
			t.Fatalf("get missing: %+v %v", rsp, err)
		}
		m, err := svc.GetTenantDashboardMenus(ca, []string{"vis-a", "nope", "vis-a"})
		if err != nil || len(m) != 2 || m["vis-a"] == nil || m["nope"] != nil {
			t.Fatalf("batch: %+v %v", m, err)
		}
		b, _ := json.Marshal(map[string]interface{}{"nope": m["nope"]})
		if string(b) != `{"nope":null}` {
			t.Fatalf("batch nil json: %s", b)
		}
		up, err := svc.UpsertTenantDashboardMenu(ca, "vis-a", req())
		if err != nil || up.DashboardID != "vis-a" || up.DashboardName != "vis-name" || up.MenuName != "mm" || up.Sort != 1 || !up.Enabled || up.ParentCode != "home" {
			t.Fatalf("upsert existing vis: %+v %v", up, err)
		}
		var kept model.TenantDashboardMenu
		if err := global.DB.Where("dashboard_id = ?", "vis-a").First(&kept).Error; err != nil || kept.ID != "m-a" {
			t.Fatalf("upsert must keep existing id: %+v %v", kept, err)
		}
		up, err = svc.UpsertTenantDashboardMenu(ca, "board-a", req())
		if err != nil || up.DashboardID != "board-a" || up.DashboardName != "board-name" {
			t.Fatalf("upsert native board: %+v %v", up, err)
		}
	})

	t.Run("db faults", func(t *testing.T) {
		opErr := func(op string, cause error) error {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"operation": op, "error": cause.Error()})
		}
		db := setupDashboardMenuPinDB(t)
		if err := db.Migrator().DropTable(&model.VisDashboard{}); err != nil {
			t.Fatal(err)
		}
		_, visErr := query.VisDashboard.Where(query.VisDashboard.ID.Eq("vis-a")).First()
		wantVis := errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "get_vis_dashboard_before_menu", "dashboard_id": "vis-a", "error": visErr.Error(),
		})
		if g, w := pinWire(upsert(ca, "vis-a")), pinWire(wantVis); g != w {
			t.Errorf("upsert vis fault: got %s want %s", g, w)
		}
		if err := db.AutoMigrate(&model.VisDashboard{}); err != nil {
			t.Fatal(err)
		}
		if err := db.Migrator().DropTable(&model.Board{}); err != nil {
			t.Fatal(err)
		}
		_, boardErr := query.Board.Where(query.Board.ID.Eq("board-a")).First()
		wantBoard := errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "get_native_board_before_menu", "dashboard_id": "board-a", "error": boardErr.Error(),
		})
		if g, w := pinWire(upsert(ca, "board-a")), pinWire(wantBoard); g != w {
			t.Errorf("upsert board fault: got %s want %s", g, w)
		}

		ta := "tenant-a"
		if err := db.Create(&model.VisDashboard{ID: "vis-a", TenantID: &ta}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Migrator().DropTable(&model.TenantDashboardMenu{}); err != nil {
			t.Fatal(err)
		}
		menuErr := db.Where("tenant_id = ? AND dashboard_id = ?", "tenant-a", "vis-a").First(&model.TenantDashboardMenu{}).Error
		delErr := db.Where("tenant_id = ? AND dashboard_id = ?", "tenant-a", "vis-a").Delete(&model.TenantDashboardMenu{}).Error
		var menus []model.TenantDashboardMenu
		batchErr := db.Where("tenant_id = ? AND dashboard_id IN ?", "tenant-a", []string{"vis-a"}).Find(&menus).Error
		cases := []struct {
			name string
			got  error
			want error
		}{
			{"get", get(ca, "vis-a"), opErr("get_dashboard_menu", menuErr)},
			{"batch", batch(ca, "vis-a"), opErr("batch_get_dashboard_menu", batchErr)},
			{"upsert pre-read", upsert(ca, "vis-a"), opErr("get_dashboard_menu_before_upsert", menuErr)},
			{"delete", del(ca, "vis-a"), opErr("delete_dashboard_menu", delErr)},
		}
		for _, tc := range cases {
			if g, w := pinWire(tc.got), pinWire(tc.want); g != w {
				t.Errorf("%s: got %s want %s", tc.name, g, w)
			}
		}
	})
}
