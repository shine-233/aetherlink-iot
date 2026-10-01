// 文件用途：通知组 CRUD 的错误码与列表响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言跨租户、不存在、DB 故障分支的 errcode JSON，
//
//	创建行的 ID/时间戳/租户回填，以及列表 map 的值类型（int64 / []*model.NotificationGroup）。
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
)

func TestNotificationGroupContractPins(t *testing.T) {
	setupNotificationGroupServiceTestDB(t)
	svc := &NotificationGroup{}
	ownA := notificationGroupTestClaims()
	ownB := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN}

	created, err := svc.CreateNotificationGroup(&model.CreateNotificationGroupReq{
		Name: "g", NotificationType: model.NoticeType_Email, Status: "OPEN",
	}, ownA)
	if err != nil {
		t.Fatal(err)
	}
	skew := created.CreatedAt.Sub(created.UpdatedAt)
	if len(created.ID) != 36 || created.TenantID != "tenant-a" || created.CreatedAt.IsZero() ||
		skew < -time.Second || skew > time.Second ||
		created.CreatedAt.Location() != time.UTC || created.UpdatedAt.Location() != time.UTC {
		t.Errorf("create stamp: %#v", created)
	}

	readDeny := pinWire(authz.NoPermission("no permission to query notification group"))
	// 跨租户写先命中读规则（Guard.RequireWrite 先 RequireRead），消息为 query 而非 modify。
	writeDeny := readDeny
	get := func(id string, c *utils.UserClaims) error { _, err := svc.GetNotificationGroupById(id, c); return err }
	upd := func(id string, c *utils.UserClaims) error {
		_, err := svc.UpdateNotificationGroup(id, &model.UpdateNotificationGroupReq{}, c)
		return err
	}
	del := func(id string, c *utils.UserClaims) error { return svc.DeleteNotificationGroup(id, c) }

	missingGet := pinWire(get("nope", ownA))
	if !strings.Contains(missingGet, fmt.Sprint(errcode.CodeNotFound)) && !strings.Contains(missingGet, fmt.Sprint(errcode.CodeDBError)) {
		t.Errorf("get missing: %s", missingGet)
	}
	for _, tc := range []struct {
		name string
		fn   func(string, *utils.UserClaims) error
		id   string
		c    *utils.UserClaims
		want string
	}{
		{"get cross tenant", get, created.ID, ownB, readDeny},
		{"get own", get, created.ID, ownA, "<nil>"},
		{"update cross tenant", upd, created.ID, ownB, writeDeny},
		{"update missing", upd, "nope", ownA, missingGet},
		{"update own", upd, created.ID, ownA, "<nil>"},
		{"delete cross tenant", del, created.ID, ownB, writeDeny},
		{"delete missing", del, "nope", ownA, missingGet},
	} {
		if got := pinWire(tc.fn(tc.id, tc.c)); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}

	res, err := svc.GetNotificationGroupListByPage(&model.GetNotificationGroupListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, ownA)
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 1 || len(res) != 2 {
		t.Errorf("list shape: %#v", res)
	}
	if got := fmt.Sprintf("%T", res["list"]); got != "[]*model.NotificationGroup" {
		t.Errorf("list type: %s", got)
	}

	if err := del(created.ID, ownA); err != nil {
		t.Fatalf("delete own: %v", err)
	}
	if got := pinWire(del(created.ID, ownA)); got != missingGet {
		t.Errorf("delete again: %s", got)
	}

	// DB 故障：列表走 dbError（{"sql_error": msg}）。
	if err := global.DB.Migrator().DropTable(&model.NotificationGroup{}); err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.GetNotificationGroupListByPage(&model.GetNotificationGroupListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, ownA)
	if lw := pinWire(lerr); !strings.HasPrefix(lw, fmt.Sprintf(`{"code":%d,"data":{"sql_error":`, errcode.CodeDBError)) {
		t.Errorf("list db fault: %s", lw)
	}
	if got := pinWire(get(created.ID, ownA)); got == "<nil>" {
		t.Errorf("get db fault: %s", got)
	}
}
