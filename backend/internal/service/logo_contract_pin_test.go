// 文件用途：Logo 品牌配置 列表/更新 的错误码与响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言角色门禁消息、列表 map 值类型（int64 / []*model.Logo）、
//
//	更新失败（含非本租户行）与列表 DB 故障统一包成 {"err": msg} 的 DBError。
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

func TestLogoContractPins(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:logopin_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Logo{}); err != nil {
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
	if err := db.Create(&model.Logo{ID: "logo-a", TenantID: "tenant-a", SystemName: "A"}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Logo{}
	admin := &utils.UserClaims{ID: "ta", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
	other := &utils.UserClaims{ID: "tb", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN}
	name := "renamed"

	deny := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update logo settings"))
	for _, tc := range []struct {
		name string
		c    *utils.UserClaims
		want string
	}{
		{"nil", nil, deny},
		{"tenant user", &utils.UserClaims{ID: "u", TenantID: "tenant-a", Authority: constant.TENANT_USER}, deny},
		{"admin no tenant", &utils.UserClaims{ID: "x", Authority: constant.TENANT_ADMIN},
			pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "complete tenant initialization before update logo"))},
		{"own", admin, "<nil>"},
		// 非本租户行：DAL 的 NoPermission 被原样包进 {"err": ...}。
		{"cross tenant", other, pinWire(errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"err": errcode.NewWithMessage(errcode.CodeNoPermission, "logo not found or not owned by tenant").Error(),
		}))},
	} {
		if got := pinWire(svc.UpdateLogo(&model.UpdateLogoReq{Id: "logo-a", SystemName: &name}, tc.c)); got != tc.want {
			t.Errorf("update %s: got %s want %s", tc.name, got, tc.want)
		}
	}

	res, err := svc.GetLogoList("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 1 || len(res) != 2 {
		t.Errorf("list shape: %#v", res)
	}
	if rows, ok := res["list"].([]*model.Logo); !ok || len(rows) != 1 || rows[0].SystemName != "renamed" {
		t.Errorf("list rows: %T %#v", res["list"], res["list"])
	}

	if err := db.Migrator().DropTable(&model.Logo{}); err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.GetLogoList("tenant-a")
	if lw := pinWire(lerr); !strings.HasPrefix(lw, fmt.Sprintf(`{"code":%d,"data":{"err":`, errcode.CodeDBError)) {
		t.Errorf("list db fault: %s", lw)
	}
	if uw := pinWire(svc.UpdateLogo(&model.UpdateLogoReq{Id: "logo-a", SystemName: &name}, admin)); !strings.HasPrefix(uw, fmt.Sprintf(`{"code":%d,"data":{"err":`, errcode.CodeDBError)) {
		t.Errorf("update db fault: %s", uw)
	}
}
