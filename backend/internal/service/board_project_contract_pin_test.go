// 文件用途：看板项目服务错误/响应契约钉桩（kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言读门禁（nil/空租户→"tenant context is required"）、写门禁、
//
//	项目不存在/跨租户（CodeParamError "board project not found"）、看板不在租户（任意加载失败掩码）、
//	DB 故障（sql_error 包装 / 看板加载故障仍掩码）各分支，以及列表空结果编码为 []。
//
// 关键注意事项：期望值一律用 errcode/authz 原始构造器手写，不经 kit；复用 crud_contract_pin_test 的 pinWire。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupBoardProjectPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:bppin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Board{}, &model.BoardProject{}, &model.BoardProjectMember{}); err != nil {
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
	must(&model.Board{ID: "b-a", Name: "a", TenantID: "tenant-a", HomeFlag: "N", CreatedAt: now, UpdatedAt: now})
	must(&model.Board{ID: "b-b", Name: "b", TenantID: "tenant-b", HomeFlag: "N", CreatedAt: now, UpdatedAt: now})
	must(&model.BoardProject{ID: "p-a", TenantID: "tenant-a", Name: "pa", CreatedAt: now, UpdatedAt: now})
	return db
}

func TestBoardProjectContractPins(t *testing.T) {
	svc := &BoardProjectService{}
	ca := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a"}
	cb := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b"}
	empty := &utils.UserClaims{ID: "u-x"}
	readGate := errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	pNF := errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
	bNF := errcode.NewWithMessage(errcode.CodeParamError, "board not found in tenant")
	upd := model.UpdateBoardProjectReq{Name: "n"}

	get := func(c *utils.UserClaims, id string) error { _, err := svc.GetProject(id, c); return err }
	list := func(c *utils.UserClaims, board string) error {
		_, err := svc.ListProjects(model.BoardProjectListReq{BoardID: board}, c)
		return err
	}
	member := func(c *utils.UserClaims, board string) error { _, err := svc.MembershipOf(board, c); return err }
	update := func(c *utils.UserClaims, id string) error { _, err := svc.UpdateProject(id, upd, c); return err }
	del := func(c *utils.UserClaims, id string) error { return svc.DeleteProject(id, c) }
	add := func(c *utils.UserClaims, p, b string) error { return svc.AddBoard(p, b, c) }
	remove := func(c *utils.UserClaims, p, b string) error { return svc.RemoveBoard(p, b, c) }

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
		setupBoardProjectPinDB(t)
		check(t, []pin{
			{"get nil", get(nil, "p-a"), readGate},
			{"get empty tenant", get(empty, "p-a"), readGate},
			{"list nil", list(nil, ""), readGate},
			{"list empty tenant", list(empty, ""), readGate},
			{"member nil", member(nil, "b-a"), readGate},
			{"member empty tenant", member(empty, "b-a"), readGate},
			{"update nil", update(nil, "p-a"), authz.NoPermission("no permission to update board project")},
			{"delete blank", del(&utils.UserClaims{TenantID: " "}, "p-a"), authz.NoPermission("complete tenant initialization before delete board project")},
			{"get missing", get(ca, "nope"), pNF},
			{"get cross tenant", get(cb, "p-a"), pNF},
			{"update missing", update(ca, "nope"), pNF},
			{"update cross tenant", update(cb, "p-a"), pNF},
			{"delete missing", del(ca, "nope"), pNF},
			{"delete cross tenant", del(cb, "p-a"), pNF},
			{"add missing project", add(ca, "nope", "b-a"), pNF},
			{"add cross tenant project", add(cb, "p-a", "b-b"), pNF},
			{"add foreign board", add(ca, "p-a", "b-b"), bNF},
			{"remove missing project", remove(ca, "nope", "b-a"), pNF},
			{"remove not member ok", remove(ca, "p-a", "b-a"), nil},
			{"list foreign board", list(ca, "b-b"), bNF},
			{"member foreign board", member(ca, "b-b"), bNF},
			{"add ok", add(ca, "p-a", "b-a"), nil},
			{"delete ok", del(ca, "p-a"), nil},
			{"get after delete", get(ca, "p-a"), pNF},
		})
	})

	t.Run("shapes", func(t *testing.T) {
		setupBoardProjectPinDB(t)
		rows, err := svc.ListProjects(model.BoardProjectListReq{}, cb)
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := json.Marshal(rows); string(b) != "[]" {
			t.Fatalf("empty list json: %s", b)
		}
		if p, err := svc.MembershipOf("b-a", ca); err != nil || p != nil {
			t.Fatalf("no membership: %+v %v", p, err)
		}
		if err := svc.AddBoard("p-a", "b-a", ca); err != nil {
			t.Fatal(err)
		}
		if p, err := svc.MembershipOf("b-a", ca); err != nil || p == nil || p.ID != "p-a" {
			t.Fatalf("membership: %+v %v", p, err)
		}
		if rows, err := svc.ListProjects(model.BoardProjectListReq{BoardID: " b-a "}, ca); err != nil || len(rows) != 1 || rows[0].ID != "p-a" {
			t.Fatalf("reverse lookup: %+v %v", rows, err)
		}
		got, err := svc.UpdateProject("p-a", model.UpdateBoardProjectReq{Name: " renamed "}, ca)
		if err != nil || got.Name != "renamed" {
			t.Fatalf("update: %+v %v", got, err)
		}
	})

	t.Run("db faults", func(t *testing.T) {
		db := setupBoardProjectPinDB(t)
		sqlErr := func(err error) error {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if err := db.Migrator().DropTable(&model.Board{}); err != nil {
			t.Fatal(err)
		}
		boardErr := db.Where("id = ? AND tenant_id = ?", "b-a", "tenant-a").First(&model.Board{}).Error
		check(t, []pin{
			// 看板加载：List 只把 not-found 转参数错误，其余报 sql_error；Add/Membership 任何失败一律掩码。
			{"list board fault", list(ca, "b-a"), sqlErr(boardErr)},
			{"add board fault masked", add(ca, "p-a", "b-a"), bNF},
			{"member board fault masked", member(ca, "b-a"), bNF},
		})
		if err := db.Migrator().DropTable(&model.BoardProject{}); err != nil {
			t.Fatal(err)
		}
		projErr := db.Where("id = ? AND tenant_id = ?", "p-a", "tenant-a").First(&model.BoardProject{}).Error
		check(t, []pin{
			{"get", get(ca, "p-a"), sqlErr(projErr)},
			{"delete", del(ca, "p-a"), sqlErr(projErr)},
			{"add", add(ca, "p-a", "b-a"), sqlErr(projErr)},
			{"remove", remove(ca, "p-a", "b-a"), sqlErr(projErr)},
		})
	})
}
