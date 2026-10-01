// 文件用途：租户 CRUD 服务的错误/响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：对 data_converter / integration / widget_bundle 的 Get/Update/Delete/List，
//
//	在 sqlite 内存库上逐一断言：未登录/空租户、跨租户、不存在、DB 故障（删表）
//	四类分支的 errcode JSON（含 UseCustomMsg），以及列表 map 的值类型（int64 / []*model.X）。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit，避免钉桩随实现漂移。
package service

import (
	"context"
	"encoding/json"
	"errors"
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

// pinWire 把错误渲染成响应层可见的形态：errcode JSON + 是否自定义消息；非 errcode 记 raw。
func pinWire(err error) string {
	if err == nil {
		return "<nil>"
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		return "raw:" + err.Error()
	}
	b, _ := json.Marshal(e)
	return fmt.Sprintf("%s|%v", b, e.UseCustomMsg)
}

func setupCRUDPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.DataConverter{}, &model.Integration{}, &model.WidgetBundle{}); err != nil {
		t.Fatal(err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	now := time.Now().UTC()
	must := func(v interface{}) {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(&model.DataConverter{ID: "dc-a", Name: "dc", Type: "UPLINK", ConverterMode: "JSON_PATH", TenantID: "tenant-a", Configuration: "{}", CreatedAt: &now, UpdatedAt: &now})
	must(&model.Integration{ID: "in-a", Name: "in", TenantID: "tenant-a", ConnectorType: "opcua", Config: "{}", Enabled: true, CreatedAt: &now, UpdatedAt: &now})
	must(&model.WidgetBundle{ID: "wb-a", Name: "wb", TenantID: "tenant-a", Widgets: "[]", Version: "1.0.0", CreatedAt: &now, UpdatedAt: &now})
	return db
}

// crudPinOps 抽象一个服务的四个入口，便于三服务共享同一张钉桩表。
type crudPinOps struct {
	table, ownID, notFoundMsg string
	get                       func(id string, c *utils.UserClaims) error
	update                    func(id string, c *utils.UserClaims) error
	del                       func(id string, c *utils.UserClaims) error
	list                      func(c *utils.UserClaims) (map[string]interface{}, error)
	listType                  string // fmt %T of res["list"]
	emptyTenantDenied         bool
}

func crudPinServices() []crudPinOps {
	ctx := context.Background()
	dc, in, wb := &DataConverterService{}, &IntegrationService{}, &WidgetBundleService{}
	return []crudPinOps{
		{
			table: "data_converters", ownID: "dc-a", notFoundMsg: "data converter not found", listType: "[]*model.DataConverter",
			get: func(id string, c *utils.UserClaims) error { _, err := dc.GetDataConverterByID(ctx, id, c); return err },
			update: func(id string, c *utils.UserClaims) error {
				_, err := dc.UpdateDataConverter(ctx, &model.UpdateDataConverterReq{ID: id}, c)
				return err
			},
			del: func(id string, c *utils.UserClaims) error { return dc.DeleteDataConverter(ctx, id, c) },
			list: func(c *utils.UserClaims) (map[string]interface{}, error) {
				return dc.ListDataConverters(ctx, &model.GetDataConverterListReq{}, c)
			},
		},
		{
			table: "integrations", ownID: "in-a", notFoundMsg: "integration not found", listType: "[]*model.Integration",
			get: func(id string, c *utils.UserClaims) error { _, err := in.GetIntegrationByID(ctx, id, c); return err },
			update: func(id string, c *utils.UserClaims) error {
				_, err := in.UpdateIntegration(ctx, &model.UpdateIntegrationReq{ID: id}, c)
				return err
			},
			del: func(id string, c *utils.UserClaims) error { return in.DeleteIntegration(ctx, id, c) },
			list: func(c *utils.UserClaims) (map[string]interface{}, error) {
				return in.ListIntegrations(ctx, &model.GetIntegrationListReq{}, c)
			},
		},
		{
			table: "widget_bundles", ownID: "wb-a", notFoundMsg: "widget bundle not found", listType: "[]*model.WidgetBundle",
			emptyTenantDenied: true,
			get:               func(id string, c *utils.UserClaims) error { _, err := wb.GetWidgetBundleByID(ctx, id, c); return err },
			update: func(id string, c *utils.UserClaims) error {
				_, err := wb.UpdateWidgetBundle(ctx, &model.UpdateWidgetBundleReq{ID: id}, c)
				return err
			},
			del: func(id string, c *utils.UserClaims) error { return wb.DeleteWidgetBundle(ctx, id, c) },
			list: func(c *utils.UserClaims) (map[string]interface{}, error) {
				return wb.ListWidgetBundles(ctx, &model.GetWidgetBundleListReq{}, c)
			},
		},
	}
}
func TestCRUDContractPins(t *testing.T) {
	ownA := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	ownB := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	noTenant := &utils.UserClaims{ID: "u-sys", Authority: "SYS_ADMIN"}
	deny := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "claims required"))

	for _, svc := range crudPinServices() {
		svc := svc
		t.Run(svc.table, func(t *testing.T) {
			setupCRUDPinDB(t)
			notFound := pinWire(errcode.NewWithMessage(errcode.CodeNotFound, svc.notFoundMsg))
			ops := map[string]func(string, *utils.UserClaims) error{"get": svc.get, "update": svc.update, "delete": svc.del}
			for _, op := range []string{"get", "update"} {
				fn := ops[op]
				cases := []struct {
					name string
					id   string
					c    *utils.UserClaims
					want string
				}{
					{"nil claims", svc.ownID, nil, deny},
					{"cross tenant", svc.ownID, ownB, notFound},
					{"missing", "nope", ownA, notFound},
					{"own", svc.ownID, ownA, "<nil>"},
				}
				// 空租户：widget_bundle 拒绝；其余服务 DAL 跳过租户过滤，可见全部租户（历史行为）。
				if svc.emptyTenantDenied {
					cases = append(cases, struct {
						name string
						id   string
						c    *utils.UserClaims
						want string
					}{"empty tenant", svc.ownID, noTenant, deny})
				} else {
					cases = append(cases, struct {
						name string
						id   string
						c    *utils.UserClaims
						want string
					}{"empty tenant", svc.ownID, noTenant, "<nil>"})
				}
				for _, tc := range cases {
					if got := pinWire(fn(tc.id, tc.c)); got != tc.want {
						t.Errorf("%s/%s: got %s want %s", op, tc.name, got, tc.want)
					}
				}
			}

			// list：值类型钉死（mobile 适配器与既有测试做类型断言）。
			res, err := svc.list(ownA)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if total, ok := res["total"].(int64); !ok || total != 1 {
				t.Errorf("list total: %#v", res["total"])
			}
			if got := fmt.Sprintf("%T", res["list"]); got != svc.listType {
				t.Errorf("list type: %s want %s", got, svc.listType)
			}
			if len(res) != 2 {
				t.Errorf("list keys: %v", res)
			}
			if _, err := svc.list(nil); pinWire(err) != deny {
				t.Errorf("list nil claims: %s", pinWire(err))
			}

			// delete：未登录/跨租户/不存在先于删除生效；本租户删除成功。
			for _, tc := range []struct {
				name string
				id   string
				c    *utils.UserClaims
				want string
			}{
				{"nil claims", svc.ownID, nil, deny},
				{"cross tenant", svc.ownID, ownB, notFound},
				{"missing", "nope", ownA, notFound},
				{"own", svc.ownID, ownA, "<nil>"},
				{"already gone", svc.ownID, ownA, notFound},
			} {
				if got := pinWire(svc.del(tc.id, tc.c)); got != tc.want {
					t.Errorf("delete/%s: got %s want %s", tc.name, got, tc.want)
				}
			}

			// DB 故障：加载类错误一律掩码为 not found；列表错误走 {"error": msg}。
			if err := global.DB.Migrator().DropTable(svc.table); err != nil {
				t.Fatal(err)
			}
			if got := pinWire(svc.get(svc.ownID, ownA)); got != notFound {
				t.Errorf("get db fault: %s", got)
			}
			_, lerr := svc.list(ownA)
			var ec *errcode.Error
			if !errors.As(lerr, &ec) || ec.Code != errcode.CodeDBError || ec.UseCustomMsg {
				t.Fatalf("list db fault: %s", pinWire(lerr))
			}
			data, _ := ec.Data.(map[string]interface{})
			if msg, ok := data["error"].(string); !ok || len(data) != 1 || !strings.Contains(msg, svc.table) {
				t.Errorf("list db fault data: %#v", ec.Data)
			}
		})
	}
}
