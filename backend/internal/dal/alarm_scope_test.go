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

func TestAlarmHistoryCountsRespectDefaultAndAllTenantScope(t *testing.T) {
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open alarm scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AlarmHistory{}); err != nil {
		t.Fatalf("migrate alarm history: %v", err)
	}
	if err := db.Exec(`CREATE TABLE devices (
		id text NOT NULL,
		name text,
		tenant_id text NOT NULL,
		owner_user_id text,
		activate_flag text NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create device scope fixture: %v", err)
	}
	if err := db.Exec(`CREATE TABLE current_device_alarm_streams (
		id text NOT NULL,
		tenant_id text NOT NULL,
		device_id text NOT NULL,
		alarm_status text NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create current alarm stream fixture: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	now := time.Now().UTC()
	rows := []model.AlarmHistory{
		{ID: "tenant-a-active", AlarmConfigID: "config-a", GroupID: "group-a", SceneAutomationID: "scene-a", Name: "active", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now, AlarmDeviceList: `[]`},
		{ID: "tenant-a-recovered-old", AlarmConfigID: "config-b", GroupID: "group-b", SceneAutomationID: "scene-b", Name: "old active", AlarmStatus: "M", TenantID: "tenant-a", CreateAt: now.Add(-time.Minute), AlarmDeviceList: `[]`},
		{ID: "tenant-a-recovery", AlarmConfigID: "config-b", GroupID: "group-b", SceneAutomationID: "scene-b", Name: "recovery", AlarmStatus: "N", TenantID: "tenant-a", CreateAt: now, AlarmDeviceList: `[]`},
		{ID: "tenant-b-active", AlarmConfigID: "config-c", GroupID: "group-c", SceneAutomationID: "scene-c", Name: "active", AlarmStatus: "L", TenantID: "tenant-b", CreateAt: now, AlarmDeviceList: `[]`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed alarm history: %v", err)
	}
	if err := db.Exec(`INSERT INTO current_device_alarm_streams (id, tenant_id, device_id, alarm_status) VALUES
		('tenant-a-active', 'tenant-a', 'device-a', 'H'),
		('tenant-a-recovery', 'tenant-a', 'device-a', 'N'),
		('tenant-b-active', 'tenant-b', 'device-b', 'L')`).Error; err != nil {
		t.Fatalf("seed current alarm streams: %v", err)
	}
	if err := db.Exec(`INSERT INTO devices (id, name, tenant_id, owner_user_id, activate_flag) VALUES
		('device-a', 'Device A', 'tenant-a', 'owner-a', 'active'),
		('device-recovered', 'Recovered Device', 'tenant-a', 'owner-a', 'active'),
		('device-b', 'Device B', 'tenant-b', 'owner-b', 'active')`).Error; err != nil {
		t.Fatalf("seed device scope fixture: %v", err)
	}

	assertAlarmScopeCount(t, "tenant history", 3, func() (int64, error) {
		return CountAlarmHistoryByScope("tenant-a", nil, false)
	})
	assertAlarmScopeCount(t, "tenant active history", 1, func() (int64, error) {
		return CountActiveAlarmHistoryByScope("tenant-a", nil, false)
	})
	assertAlarmScopeCount(t, "all-tenant history", 4, func() (int64, error) {
		return CountAlarmHistoryByScope("tenant-a", nil, true)
	})
	assertAlarmScopeCount(t, "all-tenant active history", 2, func() (int64, error) {
		return CountActiveAlarmHistoryByScope("tenant-a", nil, true)
	})

	active := model.AlarmHistoryQueryStatusActive
	assertAlarmScopeCount(t, "tenant ACTIVE list filter", 1, func() (int64, error) {
		var count int64
		err := applyAlarmHistoryScopedFilters(
			newAlarmHistoryScopedDB("tenant-a", nil, false),
			&model.GetAlarmHisttoryListByPage{AlarmStatus: &active},
			nil,
		).Count(&count).Error
		return count, err
	})

	activeList := []map[string]interface{}{
		{
			"id":                "tenant-a-active",
			"alarm_device_list": `["device-recovered","device-a"]`,
		},
	}
	if err := expandCurrentActiveAlarmHistoryDeviceFields(activeList, nil); err != nil {
		t.Fatalf("expand current active devices: %v", err)
	}
	deviceRows, ok := activeList[0]["alarm_device_list"].([]map[string]interface{})
	if !ok || len(deviceRows) != 1 || alarmHistoryDeviceRowID(deviceRows[0]) != "device-a" {
		t.Fatalf("ACTIVE device rows = %#v, want only current active device-a", activeList[0]["alarm_device_list"])
	}
}

func assertAlarmScopeCount(t *testing.T, label string, want int64, count func() (int64, error)) {
	t.Helper()
	got, err := count()
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", label, got, want)
	}
}

// setupAlarmHistoryListTestDB 为 GetAlarmHistoryListByPage 的 raw 链提供最小依赖：
// alarm_history 由模型迁移，alarm_config/devices 以生产列名手工建表，
// 供 LEFT JOIN 与设备摘要展开使用。
func setupAlarmHistoryListTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open alarm history sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AlarmHistory{}); err != nil {
		t.Fatalf("migrate alarm history: %v", err)
	}
	if err := db.Exec(`CREATE TABLE alarm_config (
		id text NOT NULL,
		name text,
		alarm_level text,
		tenant_id text
	)`).Error; err != nil {
		t.Fatalf("create alarm_config fixture: %v", err)
	}
	if err := db.Exec(`CREATE TABLE devices (
		id text NOT NULL,
		name text,
		tenant_id text NOT NULL,
		owner_user_id text,
		activate_flag text NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create device fixture: %v", err)
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

func TestGetAlarmHistoryListByPageIsolatesTenants(t *testing.T) {
	db := setupAlarmHistoryListTestDB(t)

	now := time.Now().UTC()
	rows := []model.AlarmHistory{
		{ID: "a-h-1", AlarmConfigID: "config-a", Name: "a high", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now.Add(-time.Minute), AlarmDeviceList: `[]`},
		{ID: "a-l-1", AlarmConfigID: "config-b", Name: "a low", AlarmStatus: "L", TenantID: "tenant-a", CreateAt: now, AlarmDeviceList: `[]`},
		{ID: "b-m-1", AlarmConfigID: "config-c", Name: "b mid", AlarmStatus: "M", TenantID: "tenant-b", CreateAt: now, AlarmDeviceList: `[]`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed alarm history: %v", err)
	}
	if err := db.Exec(`INSERT INTO alarm_config (id, name, alarm_level, tenant_id) VALUES
		('config-a', 'Config A', 'H', 'tenant-a'),
		('config-b', 'Config B', 'L', 'tenant-a'),
		('config-c', 'Config C', 'M', 'tenant-b')`).Error; err != nil {
		t.Fatalf("seed alarm configs: %v", err)
	}

	countA, listA, err := GetAlarmHistoryListByPage(&model.GetAlarmHisttoryListByPage{}, "tenant-a", nil)
	if err != nil {
		t.Fatalf("tenant-a list: %v", err)
	}
	if countA != 2 {
		t.Fatalf("tenant-a count = %d, want 2", countA)
	}
	rowsA, ok := listA.([]map[string]interface{})
	if !ok || len(rowsA) != 2 {
		t.Fatalf("tenant-a list type/len = %T/%d, want 2 joined rows", listA, len(rowsA))
	}
	for _, row := range rowsA {
		if row["tenant_id"] != "tenant-a" {
			t.Fatalf("tenant-a list leaked row of tenant %v", row["tenant_id"])
		}
	}
	// 最新在前（create_at DESC），且证明 raw 链上的 LEFT JOIN 命中 alarm_config。
	if rowsA[0]["id"] != "a-l-1" || rowsA[1]["id"] != "a-h-1" {
		t.Fatalf("tenant-a order = %v,%v, want a-l-1,a-h-1", rowsA[0]["id"], rowsA[1]["id"])
	}
	if rowsA[0]["alarm_config_name"] != "Config B" || rowsA[1]["alarm_config_name"] != "Config A" {
		t.Fatalf("joined config names = %v,%v, want Config B,Config A", rowsA[0]["alarm_config_name"], rowsA[1]["alarm_config_name"])
	}

	countB, listB, err := GetAlarmHistoryListByPage(&model.GetAlarmHisttoryListByPage{}, "tenant-b", nil)
	if err != nil {
		t.Fatalf("tenant-b list: %v", err)
	}
	if countB != 1 {
		t.Fatalf("tenant-b count = %d, want 1", countB)
	}
	rowsB, ok := listB.([]map[string]interface{})
	if !ok || len(rowsB) != 1 || rowsB[0]["id"] != "b-m-1" {
		t.Fatalf("tenant-b rows = %#v, want only b-m-1", rowsB)
	}
	if rowsB[0]["alarm_config_name"] != "Config C" {
		t.Fatalf("tenant-b joined config name = %v, want Config C", rowsB[0]["alarm_config_name"])
	}
}

func TestGetAlarmHistoryListByPageBlankTenantGuardBehavior(t *testing.T) {
	setupAlarmHistoryListTestDB(t)

	count, list, err := GetAlarmHistoryListByPage(&model.GetAlarmHisttoryListByPage{}, "   ", nil)
	if err == nil {
		t.Fatal("empty tenant without all-tenants scope must be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "tenant id is required") {
		t.Fatalf("error = %v, want explicit tenant-required message", err)
	}
	if count != 0 || list != nil {
		t.Fatalf("rejected query returned count/list = %d/%#v, want zero values", count, list)
	}

	allTenants := true
	countAll, _, errAll := GetAlarmHistoryListByPage(&model.GetAlarmHisttoryListByPage{AllTenants: allTenants}, "", nil)
	if errAll != nil {
		t.Fatalf("all-tenants scope with blank tenant should pass guard: %v", errAll)
	}
	if countAll != 0 {
		t.Fatalf("empty table under all-tenants scope count = %d, want 0", countAll)
	}
}

func TestGetAlarmHistoryListByPageAppliesStatusFilterAndPagination(t *testing.T) {
	db := setupAlarmHistoryListTestDB(t)

	now := time.Now().UTC()
	rows := []model.AlarmHistory{
		{ID: "row-high", AlarmConfigID: "config-a", Name: "high", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now.Add(-2 * time.Minute), AlarmDeviceList: `[]`},
		{ID: "row-mid", AlarmConfigID: "config-a", Name: "mid", AlarmStatus: "M", TenantID: "tenant-a", CreateAt: now.Add(-time.Minute), AlarmDeviceList: `[]`},
		{ID: "row-recovered", AlarmConfigID: "config-a", Name: "recovered", AlarmStatus: "N", TenantID: "tenant-a", CreateAt: now, AlarmDeviceList: `[]`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed alarm history: %v", err)
	}
	if err := db.Exec(`INSERT INTO alarm_config (id, name, alarm_level, tenant_id) VALUES
		('config-a', 'Config A', 'H', 'tenant-a')`).Error; err != nil {
		t.Fatalf("seed alarm config: %v", err)
	}

	mid := "M"
	count, list, err := GetAlarmHistoryListByPage(
		&model.GetAlarmHisttoryListByPage{AlarmStatus: &mid},
		"tenant-a",
		nil,
	)
	if err != nil {
		t.Fatalf("status-filtered list: %v", err)
	}
	if count != 1 {
		t.Fatalf("status M count = %d, want 1", count)
	}
	filteredRows, ok := list.([]map[string]interface{})
	if !ok || len(filteredRows) != 1 || filteredRows[0]["id"] != "row-mid" {
		t.Fatalf("status M rows = %#v, want only row-mid", filteredRows)
	}

	countPaged, pagedList, err := GetAlarmHistoryListByPage(
		&model.GetAlarmHisttoryListByPage{PageReq: model.PageReq{Page: 1, PageSize: 2}},
		"tenant-a",
		nil,
	)
	if err != nil {
		t.Fatalf("paged list: %v", err)
	}
	if countPaged != 3 {
		t.Fatalf("paged total count = %d, want 3 (count ignores pagination)", countPaged)
	}
	pagedRows, ok := pagedList.([]map[string]interface{})
	if !ok || len(pagedRows) != 2 {
		t.Fatalf("paged rows len = %d, want 2", len(pagedRows))
	}
	if pagedRows[0]["id"] != "row-recovered" {
		t.Fatalf("first page row = %v, want newest row-recovered", pagedRows[0]["id"])
	}
}
