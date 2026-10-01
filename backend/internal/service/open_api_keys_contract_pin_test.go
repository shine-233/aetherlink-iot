// 文件用途：开放 API key 列表/更新/删除的错误码与响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言不存在（{"error","id"} DBError）、跨租户/角色不符（vars 载荷）、
//
//	列表 api_key 脱敏与值类型（int64 / []model.OpenAPIKeyListRsp）、列表 DB 故障（{"error"}）。
//
// 关键注意事项：期望值用 errcode 原始构造器手写，不经 kit。
package service

import (
	"fmt"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOpenAPIKeyContractPins(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:oakpin_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT, email TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OpenAPIKey{}); err != nil {
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
	if err := db.Create(&model.OpenAPIKey{ID: "k-a", TenantID: "tenant-a", APIKey: "digest", Name: "k", KeyPrefix: "ak_"}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &OpenAPIKey{}
	admin := &utils.UserClaims{ID: "ta", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
	other := &utils.UserClaims{ID: "tb", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN}
	user := &utils.UserClaims{ID: "tu", TenantID: "tenant-a", Authority: constant.TENANT_USER}
	roleDeny := func(role string) string {
		return pinWire(errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"required_role": "SYS_ADMIN or TENANT_ADMIN", "current_role": role,
		}))
	}
	missing := func(id string) string {
		return pinWire(errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "record not found", "id": id,
		}))
	}
	name := "renamed"
	upd := func(id string, c *utils.UserClaims) error {
		return svc.UpdateOpenAPIKey(&model.UpdateOpenAPIKeyReq{ID: id, Name: &name}, c)
	}
	del := func(id string, c *utils.UserClaims) error { return svc.DeleteOpenAPIKey(id, c) }

	for _, tc := range []struct {
		name string
		fn   func(string, *utils.UserClaims) error
		id   string
		c    *utils.UserClaims
		want string
	}{
		{"update missing", upd, "nope", admin, missing("nope")},
		{"update nil claims", upd, "k-a", nil, roleDeny("")},
		{"update tenant user", upd, "k-a", user, roleDeny(constant.TENANT_USER)},
		{"update cross tenant", upd, "k-a", other, roleDeny(constant.TENANT_ADMIN)},
		{"update own", upd, "k-a", admin, "<nil>"},
		{"delete missing", del, "nope", admin, missing("nope")},
		{"delete cross tenant", del, "k-a", other, roleDeny(constant.TENANT_ADMIN)},
	} {
		if got := pinWire(tc.fn(tc.id, tc.c)); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}

	res, err := svc.GetOpenAPIKeyList(&model.OpenAPIKeyListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 1 || len(res) != 2 {
		t.Errorf("list shape: %#v", res)
	}
	rows, ok := res["list"].([]model.OpenAPIKeyListRsp)
	if !ok || len(rows) != 1 || rows[0].APIKey != "" || rows[0].Name != "renamed" || rows[0].KeyPrefix != "ak_" {
		t.Errorf("list rows: %T %#v", res["list"], res["list"])
	}
	if res, _ := svc.GetOpenAPIKeyList(&model.OpenAPIKeyListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}, other); res["total"] != int64(0) {
		t.Errorf("list tenant clamp: %#v", res)
	}

	if err := del("k-a", admin); err != nil {
		t.Fatalf("delete own: %v", err)
	}
	if got := pinWire(del("k-a", admin)); got != missing("k-a") {
		t.Errorf("delete again: %s", got)
	}

	if err := db.Migrator().DropTable(&model.OpenAPIKey{}); err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.GetOpenAPIKeyList(&model.OpenAPIKeyListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}, admin)
	if lw := pinWire(lerr); !strings.HasPrefix(lw, fmt.Sprintf(`{"code":%d,"data":{"error":`, errcode.CodeDBError)) {
		t.Errorf("list db fault: %s", lw)
	}
}
