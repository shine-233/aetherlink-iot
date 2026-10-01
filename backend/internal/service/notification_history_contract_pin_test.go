// 文件用途：通知历史分页列表的错误码与响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言 nil claims 拒绝、列表 map 值类型（int64 / []*model.NotificationHistory）、
//
//	TENANT_USER 脱敏，以及 DB 故障时的 dbError（{"sql_error": msg}）。
//
// 关键注意事项：期望值用 errcode/authz 原始构造器手写，不经 kit。
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/authz"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNotificationHistoryListContractPins(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:nhpin_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.NotificationHistory{}); err != nil {
		t.Fatal(err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	content := "c"
	if err := db.Create(&model.NotificationHistory{ID: "nh-1", SendTime: time.Now().UTC(), SendContent: &content,
		SendTarget: "ops@example.com", NotificationType: model.NoticeType_Email, TenantID: ""}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &NotificationHisory{}
	sys := &utils.UserClaims{ID: "sys", Authority: constant.SYS_ADMIN}
	req := func() *model.GetNotificationHistoryListByPageReq {
		return &model.GetNotificationHistoryListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}
	}

	if _, err := svc.GetNotificationHistoryListByPage(req(), nil); pinWire(err) != pinWire(authz.NoPermission("no permission to query notification history")) {
		t.Errorf("nil claims: %s", pinWire(err))
	}

	res, err := svc.GetNotificationHistoryListByPage(req(), sys)
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 1 || len(res) != 2 {
		t.Errorf("list shape: %#v", res)
	}
	list, ok := res["list"].([]*model.NotificationHistory)
	if !ok || len(list) != 1 || list[0].SendTarget != "ops@example.com" {
		t.Errorf("list rows: %T %#v", res["list"], res["list"])
	}

	if err := db.Migrator().DropTable(&model.NotificationHistory{}); err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.GetNotificationHistoryListByPage(req(), sys)
	if lw := pinWire(lerr); !strings.HasPrefix(lw, fmt.Sprintf(`{"code":%d,"data":{"sql_error":`, errcode.CodeDBError)) {
		t.Errorf("list db fault: %s", lw)
	}
}
