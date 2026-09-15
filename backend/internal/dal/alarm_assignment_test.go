// 文件用途: 告警指派 DAL 的回归测试（ROADMAP TB-1 第二片）。
// 核心逻辑: 用 sqlite 内存库验证指派流水的租户隔离、按告警过滤、时间倒序与
// "空 assignee 必须被拒"这条边界。
// 关键注意事项:
//  1. 建表用**手写 DDL**而不是 AutoMigrate：本片的安全边界是 103.sql 里的两个 CHECK
//     （禁止空 tenant / 禁止空串冒充"取消指派"），而 GORM 的 AutoMigrate 不产生 CHECK，
//     用它建表等于把最关键的一条约束从测试里抹掉。DDL 内容镜像 103.sql，
//     只把 timestamptz/now() 换成 sqlite 认识的 datetime/CURRENT_TIMESTAMP。
//  2. 与 alarm_comment_test.go 同模式：newAlarmAssignmentTestDB 只负责返回句柄，
//     换 global.DB 的动作显式放在 withAlarmAssignmentDB 里并在 t.Cleanup 恢复。
//  3. 租户条件在 SQL 层收口是本片的安全边界，测试锁死这一行为：跨租户读必须是
//     "未命中"（空列表）而不是"命中但无权"。
// 重构建议: 若流水补分页，把分页边界用例加进本文件。
package dal

import (
	"fmt"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// alarmAssignmentDDL 镜像 backend/sql/103.sql 的表定义（含 CHECK），适配 sqlite 方言。
const alarmAssignmentDDL = `
CREATE TABLE IF NOT EXISTS alarm_assignment (
    id               varchar(36) PRIMARY KEY,
    tenant_id        varchar(36) NOT NULL,
    alarm_history_id varchar(36) NOT NULL,
    assignee_user_id varchar(36),
    operator_user_id varchar(36) NOT NULL,
    remark           text,
    created_at       datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT alarm_assignment_tenant_check   CHECK (tenant_id <> ''),
    CONSTRAINT alarm_assignment_alarm_check    CHECK (alarm_history_id <> ''),
    CONSTRAINT alarm_assignment_assignee_check CHECK (assignee_user_id IS NULL OR assignee_user_id <> ''),
    CONSTRAINT alarm_assignment_operator_check CHECK (operator_user_id <> '')
)`

func newAlarmAssignmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(alarmAssignmentDDL).Error)
	return db
}

func withAlarmAssignmentDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB := global.DB
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

func ptr(s string) *string { return &s }

func TestAlarmAssignmentTenantIsolationAndOrdering(t *testing.T) {
	db := newAlarmAssignmentTestDB(t)
	withAlarmAssignmentDB(t, db)

	base := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)

	// 同一 alarm_history_id 在 t-1 / t-2 两个租户下各有一条，验证租户收口。
	require.NoError(t, CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-1", TenantID: "t-1", AlarmHistoryID: "h-1", AssigneeUserID: ptr("u-1"),
		OperatorUserID: "op-1", CreatedAt: base,
	}))
	require.NoError(t, CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-2", TenantID: "t-2", AlarmHistoryID: "h-1", AssigneeUserID: ptr("u-2"),
		OperatorUserID: "op-2", CreatedAt: base.Add(time.Second),
	}))
	// t-1 名下的第二条、第三条，用于验证倒序与按告警过滤。
	require.NoError(t, CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-3", TenantID: "t-1", AlarmHistoryID: "h-1", AssigneeUserID: ptr("u-3"),
		OperatorUserID: "op-1", CreatedAt: base.Add(2 * time.Second),
	}))
	require.NoError(t, CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-4", TenantID: "t-1", AlarmHistoryID: "h-2", AssigneeUserID: ptr("u-4"),
		OperatorUserID: "op-1", CreatedAt: base.Add(3 * time.Second),
	}))

	rows, err := ListAlarmAssignments("t-1", "h-1")
	require.NoError(t, err)
	require.Len(t, rows, 2, "t-1 只能看到本租户 h-1 的流水，t-2 的同 ID 流水必须不可见")
	// 倒序：最新一条（a-3）落在首行，"当前处理人 = 首行"这个约定才有意义。
	require.Equal(t, "a-3", rows[0].ID)
	require.Equal(t, "a-1", rows[1].ID)
	require.Equal(t, "u-3", *rows[0].AssigneeUserID)

	// 按告警过滤：h-2 的流水不出现在 h-1 的列表里。
	rowsH2, err := ListAlarmAssignments("t-1", "h-2")
	require.NoError(t, err)
	require.Len(t, rowsH2, 1)
	require.Equal(t, "a-4", rowsH2[0].ID)

	// 跨租户读必须是"未命中"（空列表），不是"命中但无权"。
	rowsOther, err := ListAlarmAssignments("t-3", "h-1")
	require.NoError(t, err)
	require.Empty(t, rowsOther, "不存在的租户读别人的告警必须落空")

	// 取消指派 = 新写一行 NULL，历史行不动。
	require.NoError(t, CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-5", TenantID: "t-1", AlarmHistoryID: "h-1", AssigneeUserID: nil,
		OperatorUserID: "op-1", CreatedAt: base.Add(4 * time.Second),
	}))
	rows, err = ListAlarmAssignments("t-1", "h-1")
	require.NoError(t, err)
	require.Len(t, rows, 3, "取消指派是追加流水，不是删除上一行")
	require.Nil(t, rows[0].AssigneeUserID, "最新一行必须是 NULL（当前无人指派）")
	require.Equal(t, "a-3", rows[1].ID, "历史流水不得被改写")
}

func TestAlarmAssignmentRejectsEmptyAssigneeAndTenant(t *testing.T) {
	db := newAlarmAssignmentTestDB(t)
	withAlarmAssignmentDB(t, db)

	now := time.Now().UTC()

	// 空串 assignee 必须被 CHECK 拒绝：否则 NULL 与 "" 无法区分，
	// "当前处理人 = 最新一行"的判定会失效。
	err := CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-empty", TenantID: "t-1", AlarmHistoryID: "h-1", AssigneeUserID: ptr(""),
		OperatorUserID: "op-1", CreatedAt: now,
	})
	require.Error(t, err, "空串 assignee 必须被 CHECK 拒绝")

	// 空 tenant 同样必须被拒（SYS_ADMIN 走 claims.TenantID 就会写出这种行）。
	err = CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-empty-tenant", TenantID: "", AlarmHistoryID: "h-1", AssigneeUserID: ptr("u-1"),
		OperatorUserID: "op-1", CreatedAt: now,
	})
	require.Error(t, err, "空 tenant_id 必须被 CHECK 拒绝")

	// 空 operator 也必须被拒：每次指派都要能追责。
	err = CreateAlarmAssignment(&model.AlarmAssignment{
		ID: "a-empty-operator", TenantID: "t-1", AlarmHistoryID: "h-1", AssigneeUserID: ptr("u-1"),
		OperatorUserID: "", CreatedAt: now,
	})
	require.Error(t, err, "空 operator_user_id 必须被 CHECK 拒绝")

	// 上述非法行一条都没落库。
	rows, err := ListAlarmAssignments("t-1", "h-1")
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestAlarmAssignmentDBNotReadyIsReported(t *testing.T) {
	oldDB := global.DB
	global.DB = nil
	t.Cleanup(func() { global.DB = oldDB })

	require.Error(t, CreateAlarmAssignment(&model.AlarmAssignment{ID: "a-x"}))
	_, err := ListAlarmAssignments("t-1", "h-1")
	require.Error(t, err)
}
