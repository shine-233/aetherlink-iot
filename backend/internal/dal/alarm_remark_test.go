// 文件用途: 覆盖 DAL 层手写查询、缓存或聚合逻辑的回归测试，验证数据访问边界不会漂移。
// 核心逻辑: 构造最小依赖场景并断言查询条件、缓存键、事务副作用或租户过滤结果。
// 关键注意事项: 测试应显式覆盖租户隔离、权限前置假设和事务失败路径，避免只验证成功路径。
// 重构建议: 随 DAL 查询拆分同步拆小测试夹具，并优先补齐跨租户、空依赖和半提交风险用例。

package dal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMergeAlarmHistoryRemarkPreservesExistingJSON(t *testing.T) {
	raw := `{"sensor":"temperature","acknowledged":false,"nested":{"keep":true}}`

	got := mergeAlarmHistoryRemark(&raw, map[string]interface{}{
		"acknowledged":    true,
		"acknowledged_by": "user-1",
	})

	var remark map[string]interface{}
	if err := json.Unmarshal([]byte(got), &remark); err != nil {
		t.Fatalf("merged remark is not valid JSON: %v", err)
	}
	if remark["sensor"] != "temperature" {
		t.Fatalf("existing remark field was not preserved: %#v", remark)
	}
	if remark["acknowledged"] != true {
		t.Fatalf("acknowledged field was not updated: %#v", remark)
	}
	if remark["acknowledged_by"] != "user-1" {
		t.Fatalf("acknowledged_by field was not written: %#v", remark)
	}
	if _, ok := remark["nested"].(map[string]interface{}); !ok {
		t.Fatalf("nested JSON field was not preserved: %#v", remark)
	}
}

func TestMergeAlarmHistoryRemarkKeepsNonJSONRemark(t *testing.T) {
	raw := "manual note"

	got := mergeAlarmHistoryRemark(&raw, map[string]interface{}{
		"reset":    true,
		"reset_by": "user-2",
	})

	var remark map[string]interface{}
	if err := json.Unmarshal([]byte(got), &remark); err != nil {
		t.Fatalf("merged remark is not valid JSON: %v", err)
	}
	if remark["previous_remark"] != raw {
		t.Fatalf("non-JSON remark was not kept as previous_remark: %#v", remark)
	}
	if remark["reset"] != true || remark["reset_by"] != "user-2" {
		t.Fatalf("reset fields were not written: %#v", remark)
	}
}

func TestAlarmHistoryAcknowledgeRemarkWritesExactMetadata(t *testing.T) {
	raw := `{"sensor":"temperature","acknowledged":false,"nested":{"keep":true}}`
	got := alarmHistoryAcknowledgeRemark(&raw, "user-ack", "2026-06-29T10:30:00Z")

	var remark map[string]interface{}
	if err := json.Unmarshal([]byte(got), &remark); err != nil {
		t.Fatalf("acknowledge remark is not valid JSON: %v", err)
	}
	if remark["sensor"] != "temperature" {
		t.Fatalf("existing sensor field was not preserved: %#v", remark)
	}
	if _, ok := remark["nested"].(map[string]interface{}); !ok {
		t.Fatalf("nested JSON field was not preserved: %#v", remark)
	}
	if remark["acknowledged"] != true {
		t.Fatalf("acknowledged field = %#v, want true", remark["acknowledged"])
	}
	if remark["acknowledged_by"] != "user-ack" {
		t.Fatalf("acknowledged_by = %#v, want user-ack", remark["acknowledged_by"])
	}
	if remark["acknowledged_at"] != "2026-06-29T10:30:00Z" {
		t.Fatalf("acknowledged_at = %#v, want fixed timestamp", remark["acknowledged_at"])
	}
}

func TestAlarmHistoryResetUpdatesClearsAlarmStatusAndKeepsRemark(t *testing.T) {
	updates := alarmHistoryResetUpdates(`{"reset":true}`)

	if updates["alarm_status"] != "N" {
		t.Fatalf("reset update alarm_status = %#v, want N", updates["alarm_status"])
	}
	if updates["remark"] != `{"reset":true}` {
		t.Fatalf("reset update remark = %#v", updates["remark"])
	}
	if len(updates) != 2 {
		t.Fatalf("reset updates should only include alarm_status and remark, got %#v", updates)
	}
}

func TestAlarmHistoryDeviceIDs(t *testing.T) {
	got := alarmHistoryDeviceIDs(`["device-1","device-2"]`)
	if len(got) != 2 || got[0] != "device-1" || got[1] != "device-2" {
		t.Fatalf("unexpected device ids: %#v", got)
	}

	if got := alarmHistoryDeviceIDs(""); len(got) != 0 {
		t.Fatalf("blank alarm device list should return no ids: %#v", got)
	}

	if got := alarmHistoryDeviceIDs("not json"); len(got) != 0 {
		t.Fatalf("invalid alarm device list should return no ids: %#v", got)
	}
}

func TestAlarmHistoryStatusFilterValuesExpandsActiveQueryAliasOnly(t *testing.T) {
	active := model.AlarmHistoryQueryStatusActive
	if got := alarmHistoryStatusFilterValues(&active); !reflect.DeepEqual(got, []string{"H", "M", "L"}) {
		t.Fatalf("ACTIVE query values = %#v, want H/M/L", got)
	}

	for _, persistedStatus := range []string{"H", "M", "L", "N"} {
		status := persistedStatus
		if got := alarmHistoryStatusFilterValues(&status); !reflect.DeepEqual(got, []string{persistedStatus}) {
			t.Fatalf("stored status %q query values = %#v, want exact value", persistedStatus, got)
		}
	}

	blank := "   "
	if got := alarmHistoryStatusFilterValues(&blank); len(got) != 0 {
		t.Fatalf("blank status should leave the list unfiltered, got %#v", got)
	}
	if got := alarmHistoryStatusFilterValues(nil); len(got) != 0 {
		t.Fatalf("nil status should leave the list unfiltered, got %#v", got)
	}
}

func TestApplyAlarmHistoryScopedFiltersBuildsCurrentStreamPredicate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	active := model.AlarmHistoryQueryStatusActive
	result := applyAlarmHistoryScopedFilters(
		db.Table("alarm_history AS ah"),
		&model.GetAlarmHisttoryListByPage{AlarmStatus: &active},
		nil,
	).Find(&[]model.AlarmHistory{})
	if result.Error != nil {
		t.Fatalf("build ACTIVE alarm-history query: %v", result.Error)
	}

	sql := result.Statement.SQL.String()
	if !strings.Contains(sql, "current_device_alarm_streams current_alarm") ||
		!strings.Contains(sql, "current_alarm.id = ah.id") ||
		!strings.Contains(sql, "current_alarm.alarm_status IN ('H', 'M', 'L')") {
		t.Fatalf("ACTIVE query SQL = %q, want current alarm-stream predicate", sql)
	}
	if !reflect.DeepEqual(result.Statement.Vars, []interface{}{"", "", "", ""}) {
		t.Fatalf("ACTIVE query vars = %#v, want blank device/owner correlation values", result.Statement.Vars)
	}
}

func TestApplyAlarmHistoryScopedFiltersCorrelatesActiveStreamToOwner(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	active := model.AlarmHistoryQueryStatusActive
	ownerUserID := "owner-1"
	result := applyAlarmHistoryScopedFilters(
		db.Table("alarm_history AS ah"),
		&model.GetAlarmHisttoryListByPage{AlarmStatus: &active},
		&ownerUserID,
	).Find(&[]model.AlarmHistory{})
	if result.Error != nil {
		t.Fatalf("build owner-scoped ACTIVE alarm-history query: %v", result.Error)
	}

	sql := result.Statement.SQL.String()
	if !strings.Contains(sql, "INNER JOIN devices current_device") ||
		!strings.Contains(sql, "current_device.id = current_alarm.device_id") ||
		!strings.Contains(sql, "current_device.owner_user_id = ?") {
		t.Fatalf("owner ACTIVE query SQL = %q, want active-device ownership correlation", sql)
	}
	if !reflect.DeepEqual(result.Statement.Vars, []interface{}{"", "", ownerUserID, ownerUserID}) {
		t.Fatalf("owner ACTIVE query vars = %#v, want owner id", result.Statement.Vars)
	}
}

func TestApplyAlarmHistoryScopedFiltersCorrelatesActiveStreamToRequestedDevice(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	active := model.AlarmHistoryQueryStatusActive
	deviceID := "device-b"
	result := applyAlarmHistoryScopedFilters(
		db.Table("alarm_history AS ah"),
		&model.GetAlarmHisttoryListByPage{AlarmStatus: &active, DeviceId: &deviceID},
		nil,
	).Find(&[]model.AlarmHistory{})
	if result.Error != nil {
		t.Fatalf("build device-scoped ACTIVE alarm-history query: %v", result.Error)
	}

	sql := result.Statement.SQL.String()
	if !strings.Contains(sql, "current_alarm.device_id = ?") {
		t.Fatalf("device ACTIVE query SQL = %q, want current-device correlation", sql)
	}
	if !reflect.DeepEqual(result.Statement.Vars, []interface{}{deviceID, deviceID, "", ""}) {
		t.Fatalf("device ACTIVE query vars = %#v, want requested device id", result.Statement.Vars)
	}
}
