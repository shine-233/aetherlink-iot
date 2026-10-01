// 文件用途：实体版本服务错误/响应契约钉桩（kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言 Create/List/Get/Restore/Diff 的 nil claims、空白租户、
//
//	版本不存在/跨租户、目标实体不存在、DB 故障（删表后原样透传非 errcode 错误）各分支，
//	以及列表响应 JSON 形状（total 在前、空列表的编码）。
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

func setupEntityVersionPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:evpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.EntityVersion{}, &model.Board{}); err != nil {
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
	must(&model.Board{ID: "b-a", Name: "board", TenantID: "tenant-a", HomeFlag: "N", CreatedAt: now, UpdatedAt: now})
	must(&model.EntityVersion{ID: "v-a", TenantID: "tenant-a", EntityType: "board", EntityID: "b-a", VersionNumber: 1, Snapshot: `{"id":"b-a","name":"old"}`, CreatedAt: now})
	must(&model.EntityVersion{ID: "v-orphan", TenantID: "tenant-a", EntityType: "board", EntityID: "b-gone", VersionNumber: 1, Snapshot: `{"name":"x"}`, CreatedAt: now})
	return db
}

func TestEntityVersionContractPins(t *testing.T) {
	svc := &EntityVersionService{}
	ca := &utils.UserClaims{ID: "u-a", TenantID: " tenant-a "}
	cb := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b"}
	blank := &utils.UserClaims{ID: "u-x", TenantID: " \t"}
	noPerm := errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage entity versions")
	noTenant := errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required to manage entity versions")
	vNF := errcode.NewWithMessage(errcode.CodeNotFound, "entity version not found")
	tNF := errcode.NewWithMessage(errcode.CodeNotFound, "entity version target not found")

	create := func(c *utils.UserClaims, id string) error {
		_, err := svc.CreateEntityVersion(&model.EntityVersionCreateReq{EntityType: "board", EntityID: id}, c)
		return err
	}
	list := func(c *utils.UserClaims, id string) error {
		_, err := svc.ListEntityVersions(&model.EntityVersionListReq{EntityType: "board", EntityID: id}, c)
		return err
	}
	get := func(c *utils.UserClaims, id string) error { _, err := svc.GetEntityVersion(id, c); return err }
	restore := func(c *utils.UserClaims, id string) error {
		dry := true
		_, _, err := svc.RestoreEntityVersion(id, &model.EntityVersionRestoreReq{DryRun: &dry}, c)
		return err
	}
	diffSrc := func(c *utils.UserClaims, id string) error { _, err := svc.DiffEntityVersion(id, "v-a", c); return err }
	diffDst := func(c *utils.UserClaims, id string) error { _, err := svc.DiffEntityVersion("v-a", id, c); return err }

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
		setupEntityVersionPinDB(t)
		check(t, []pin{
			{"create nil", create(nil, "b-a"), noPerm},
			{"create blank", create(blank, "b-a"), noTenant},
			{"list nil", list(nil, "b-a"), noPerm},
			{"list blank", list(blank, "b-a"), noTenant},
			{"get nil", get(nil, "v-a"), noPerm},
			{"get blank", get(blank, "v-a"), noTenant},
			{"restore nil", restore(nil, "v-a"), noPerm},
			{"restore blank", restore(blank, "v-a"), noTenant},
			{"diff nil", diffSrc(nil, "v-a"), noPerm},
			{"diff blank", diffSrc(blank, "v-a"), noTenant},
			{"create missing target", create(ca, "nope"), tNF},
			{"create cross tenant", create(cb, "b-a"), tNF},
			{"get missing", get(ca, "nope"), vNF},
			{"get cross tenant", get(cb, "v-a"), vNF},
			{"get trimmed id", get(ca, " v-a "), nil},
			{"restore missing", restore(ca, "nope"), vNF},
			{"restore cross tenant", restore(cb, "v-a"), vNF},
			{"restore orphan target", restore(ca, "v-orphan"), tNF},
			{"diff missing source", diffSrc(ca, "nope"), vNF},
			{"diff missing target", diffDst(ca, "nope"), vNF},
		})
	})

	t.Run("list shape", func(t *testing.T) {
		setupEntityVersionPinDB(t)
		rsp, err := svc.ListEntityVersions(&model.EntityVersionListReq{EntityType: "board", EntityID: "b-a"}, ca)
		if err != nil || rsp.Total != 1 || len(rsp.List) != 1 || rsp.List[0].ID != "v-a" {
			t.Fatalf("list: %+v %v", rsp, err)
		}
		empty, err := svc.ListEntityVersions(&model.EntityVersionListReq{EntityType: "board", EntityID: "none"}, ca)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(empty)
		if string(b) != `{"total":0,"list":[]}` {
			t.Fatalf("empty list json: %s", b)
		}
	})

	// DB 故障：非 not-found 错误原样透传（不包 errcode），调用方看到 raw。
	t.Run("db faults pass through", func(t *testing.T) {
		db := setupEntityVersionPinDB(t)
		if err := db.Migrator().DropTable(&model.Board{}); err != nil {
			t.Fatal(err)
		}
		for name, err := range map[string]error{"create": create(ca, "b-a"), "restore target": restore(ca, "v-a")} {
			if w := pinWire(err); !strings.HasPrefix(w, "raw:") || !strings.Contains(w, "no such table") {
				t.Errorf("%s: want raw passthrough, got %s", name, w)
			}
		}
		if err := db.Migrator().DropTable(&model.EntityVersion{}); err != nil {
			t.Fatal(err)
		}
		for name, err := range map[string]error{
			"list": list(ca, "b-a"), "get": get(ca, "v-a"), "restore": restore(ca, "v-a"),
			"diff source": diffSrc(ca, "v-a"),
		} {
			if w := pinWire(err); !strings.HasPrefix(w, "raw:") || !strings.Contains(w, "no such table") {
				t.Errorf("%s: want raw passthrough, got %s", name, w)
			}
		}
	})
}
