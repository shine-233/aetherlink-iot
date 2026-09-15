// 文件用途: 告警评论 DAL 的回归测试（ROADMAP TB-1 第一片 / 2026-09-14 台账）。
// 核心逻辑: 用 sqlite 内存库验证评论 CRUD 的租户隔离与告警归属过滤。
// 关键注意事项: 与 alarm_history_actions_test.go 同模式（直接换 global.DB）；
// 租户条件在 SQL 层收口是本片的安全边界，测试锁死这一行为。
// 重构建议: 若评论补分页，把分页边界用例加进本文件。
package dal

import (
	"fmt"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAlarmCommentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AlarmComment{}))
	return db
}

func withAlarmCommentDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB := global.DB
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

func TestAlarmCommentCRUDTenantIsolation(t *testing.T) {
	db := newAlarmCommentTestDB(t)
	withAlarmCommentDB(t, db)

	rowA := &model.AlarmComment{ID: "c-a1", TenantID: "t-1", AlarmHistoryID: "h-1", Content: "first", AuthorUserID: "u-1"}
	rowB := &model.AlarmComment{ID: "c-b1", TenantID: "t-2", AlarmHistoryID: "h-1", Content: "other tenant same alarm id", AuthorUserID: "u-2"}
	require.NoError(t, CreateAlarmComment(rowA))
	require.NoError(t, CreateAlarmComment(rowB))

	// 租户隔离：t-1 只能看到自己的评论，即使同一 alarm_history_id。
	rows, err := ListAlarmComments("t-1", "h-1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "c-a1", rows[0].ID)

	// 时间正序（对话顺序）。
	require.NoError(t, CreateAlarmComment(&model.AlarmComment{ID: "c-a2", TenantID: "t-1", AlarmHistoryID: "h-1", Content: "second", AuthorUserID: "u-1"}))
	rows, err = ListAlarmComments("t-1", "h-1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "c-a1", rows[0].ID)
	require.Equal(t, "c-a2", rows[1].ID)

	// 按 ID + 租户取。
	got, err := GetAlarmCommentByID("t-1", "c-a1")
	require.NoError(t, err)
	require.Equal(t, "first", got.Content)

	// 跨租户按 ID 取必须落空（表现为未命中而非存在但无权）。
	_, err = GetAlarmCommentByID("t-2", "c-a1")
	require.Error(t, err)

	// 删除带租户条件：t-2 删不掉 t-1 的行。
	require.NoError(t, DeleteAlarmComment("t-2", "c-a1"))
	still, err := GetAlarmCommentByID("t-1", "c-a1")
	require.NoError(t, err)
	require.Equal(t, "first", still.Content)

	// 本人租户可删。
	require.NoError(t, DeleteAlarmComment("t-1", "c-a1"))
	_, err = GetAlarmCommentByID("t-1", "c-a1")
	require.Error(t, err)
}

func TestAlarmCommentUnscopedLookupOnlyForAdminPath(t *testing.T) {
	db := newAlarmCommentTestDB(t)
	withAlarmCommentDB(t, db)

	require.NoError(t, CreateAlarmComment(&model.AlarmComment{ID: "c-x1", TenantID: "t-9", AlarmHistoryID: "h-9", Content: "x", AuthorUserID: "u-9"}))

	got, err := GetAlarmCommentByIDUnscoped("c-x1")
	require.NoError(t, err)
	require.Equal(t, "t-9", got.TenantID)

	_, err = GetAlarmCommentByIDUnscoped("c-missing")
	require.Error(t, err)
}
