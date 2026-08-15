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
