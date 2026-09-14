// 文件用途: 告警评论 DAL 的回归测试（ROADMAP TB-1 第一片）。
// 核心逻辑: 用 sqlite 内存库验证评论 CRUD 的租户隔离、告警归属过滤与时间序。
// 关键注意事项: 租户条件在 SQL 层收口是本片的安全边界 —— 跨租户的读/删必须
// "未命中"而不是"命中但无权"，否则 ID 可枚举时就成了越权读取。
// 重构建议: 若评论补分页，把分页边界用例加进本文件。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAlarmCommentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AlarmComment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func newAlarmComment(id, tenant, alarmID, author, content string, at time.Time) *model.AlarmComment {
	return &model.AlarmComment{
		ID:             id,
		TenantID:       tenant,
		AlarmHistoryID: alarmID,
		Content:        content,
		AuthorUserID:   author,
		CreatedAt:      at,
	}
}

// TestAlarmCommentCRUDTenantIsolation 租户隔离 + 告警归属过滤 + 时间序。
func TestAlarmCommentCRUDTenantIsolation(t *testing.T) {
	db := newAlarmCommentTestDB(t)
	base := time.Now().UTC().Add(-time.Hour)

	rows := []*model.AlarmComment{
		newAlarmComment("c-a1", "t-1", "h-1", "u-1", "first", base),
		newAlarmComment("c-b1", "t-2", "h-1", "u-2", "other tenant same alarm id", base),
		newAlarmComment("c-a9", "t-1", "h-2", "u-1", "same tenant other alarm", base),
	}
	for _, row := range rows {
		if err := CreateAlarmComment(row); err != nil {
			t.Fatalf("create %s: %v", row.ID, err)
		}
	}

	// t-1 在 h-1 上只能看到自己的那条：既排除他租户，也排除同租户的别的告警。
	got, err := ListAlarmComments("t-1", "h-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c-a1" {
		t.Fatalf("t-1/h-1 = %#v, want only c-a1", got)
	}

	// 时间正序（对话顺序）。
	if err := CreateAlarmComment(newAlarmComment("c-a2", "t-1", "h-1", "u-1", "second", base.Add(time.Minute))); err != nil {
		t.Fatalf("create c-a2: %v", err)
	}
	got, err = ListAlarmComments("t-1", "h-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != "c-a1" || got[1].ID != "c-a2" {
		t.Fatalf("order must be ascending, got %#v", got)
	}

	// 按 ID + 租户取。
	row, err := GetAlarmCommentByID("t-1", "c-a1")
	if err != nil || row.Content != "first" {
		t.Fatalf("same-tenant get: row=%#v err=%v", row, err)
	}

	// 跨租户按 ID 取必须落空（未命中，而不是"存在但无权"）。
	if _, err := GetAlarmCommentByID("t-2", "c-a1"); err == nil {
		t.Fatal("cross-tenant get must not return the row")
	}

	// 跨租户删除必须是 no-op，不能删掉别人的行。
	if err := DeleteAlarmComment("t-2", "c-a1"); err != nil {
		t.Fatalf("cross-tenant delete should be a no-op, got error: %v", err)
	}
	if _, err := GetAlarmCommentByID("t-1", "c-a1"); err != nil {
		t.Fatalf("row must survive a cross-tenant delete attempt: %v", err)
	}

	// 本人租户可删。
	if err := DeleteAlarmComment("t-1", "c-a1"); err != nil {
		t.Fatalf("same-tenant delete: %v", err)
	}
	if _, err := GetAlarmCommentByID("t-1", "c-a1"); err == nil {
		t.Fatal("row must be gone after same-tenant delete")
	}

	_ = db
}

// TestAlarmCommentUnscopedLookupOnlyForAdminPath unscoped 查询本身不做租户判断，
// 只供 SYS_ADMIN 路径使用（调用方必须自己补校验）。
func TestAlarmCommentUnscopedLookupOnlyForAdminPath(t *testing.T) {
	newAlarmCommentTestDB(t)

	if err := CreateAlarmComment(newAlarmComment("c-x1", "t-9", "h-9", "u-9", "x", time.Now().UTC())); err != nil {
		t.Fatalf("create: %v", err)
	}

	row, err := GetAlarmCommentByIDUnscoped("c-x1")
	if err != nil || row == nil || row.TenantID != "t-9" {
		t.Fatalf("unscoped get: row=%#v err=%v", row, err)
	}
	if _, err := GetAlarmCommentByIDUnscoped("c-missing"); err == nil {
		t.Fatal("missing id must error")
	}
}

// TestAlarmCommentListReturnsEmptySliceNotNil 无评论时返回空切片，
// 让上层能直接序列化成 [] 而不是 null。
func TestAlarmCommentListReturnsEmptySliceNotNil(t *testing.T) {
	newAlarmCommentTestDB(t)

	got, err := ListAlarmComments("t-1", "no-such-alarm")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got == nil {
		t.Fatal("list must return an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}
