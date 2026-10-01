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

// setupDeviceHealthAlarmTestDB 建立 alarm_history（模型迁移）与 alarm_history_devices
// （按 142.sql 列名手工建表）。SQLite 没有 142.sql 的同步触发器，关联行由用例显式写入。
func setupDeviceHealthAlarmTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open device health sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AlarmHistory{}); err != nil {
		t.Fatalf("migrate alarm history: %v", err)
	}
	if err := db.Exec(`CREATE TABLE alarm_history_devices (
		alarm_history_id text NOT NULL,
		device_id text NOT NULL,
		tenant_id text NOT NULL,
		PRIMARY KEY (alarm_history_id, device_id)
	)`).Error; err != nil {
		t.Fatalf("create alarm_history_devices fixture: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// seedDeviceHealthAlarm 写入一条告警历史及其关联行（模拟 142.sql 触发器的投影）。
func seedDeviceHealthAlarm(t *testing.T, db *gorm.DB, row model.AlarmHistory, deviceIDs ...string) {
	t.Helper()
	row.AlarmDeviceList = `["` + strings.Join(deviceIDs, `","`) + `"]`
	if len(deviceIDs) == 0 {
		row.AlarmDeviceList = `[]`
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed alarm history %s: %v", row.ID, err)
	}
	for _, deviceID := range deviceIDs {
		if err := db.Exec(
			`INSERT INTO alarm_history_devices (alarm_history_id, device_id, tenant_id) VALUES (?, ?, ?)`,
			row.ID, deviceID, row.TenantID,
		).Error; err != nil {
			t.Fatalf("seed alarm_history_devices %s/%s: %v", row.ID, deviceID, err)
		}
	}
}

func alarmHistoryIDs(list []*model.AlarmHistory) []string {
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestDeviceHealthGetDeviceActiveAlarmsUsesDeviceLinkTable(t *testing.T) {
	db := setupDeviceHealthAlarmTestDB(t)
	now := time.Now().UTC()

	// 命中：dev-1 的活跃告警（H/M/L 三态），含与其他设备共享的多设备告警。
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "h-old", AlarmConfigID: "c", Name: "high", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now.Add(-3 * time.Minute)}, "dev-1")
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "m-mid", AlarmConfigID: "c", Name: "mid", AlarmStatus: "M", TenantID: "tenant-a", CreateAt: now.Add(-2 * time.Minute)}, "dev-2", "dev-1")
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "l-new", AlarmConfigID: "c", Name: "low", AlarmStatus: "L", TenantID: "tenant-a", CreateAt: now.Add(-1 * time.Minute)}, "dev-1")
	// 不命中：已恢复。
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "n-resolved", AlarmConfigID: "c", Name: "resolved", AlarmStatus: "N", TenantID: "tenant-a", CreateAt: now}, "dev-1")
	// 不命中：前缀相同的另一设备（旧 LIKE '%dev-1%' 会误命中 dev-10）。
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "h-prefix", AlarmConfigID: "c", Name: "prefix", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now}, "dev-10")
	// 不命中：其他租户下同 id 设备。
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "h-other-tenant", AlarmConfigID: "c", Name: "other", AlarmStatus: "H", TenantID: "tenant-b", CreateAt: now}, "dev-1")
	// 不命中：JSON 列有、关联表没有的行，证明读路径已不再扫描 alarm_device_list。
	if err := db.Create(&model.AlarmHistory{ID: "json-only", AlarmConfigID: "c", Name: "json", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now, AlarmDeviceList: `["dev-1"]`}).Error; err != nil {
		t.Fatalf("seed json-only alarm: %v", err)
	}

	list, err := GetDeviceActiveAlarms("tenant-a", "dev-1")
	if err != nil {
		t.Fatalf("GetDeviceActiveAlarms: %v", err)
	}
	got := strings.Join(alarmHistoryIDs(list), ",")
	if want := "l-new,m-mid,h-old"; got != want {
		t.Fatalf("active alarms = %s, want %s (create_at DESC, link-table match only)", got, want)
	}
}

func TestDeviceHealthGetDeviceActiveAlarmsEmptyResults(t *testing.T) {
	db := setupDeviceHealthAlarmTestDB(t)
	seedDeviceHealthAlarm(t, db, model.AlarmHistory{ID: "h-1", AlarmConfigID: "c", Name: "high", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: time.Now().UTC()}, "dev-1")

	cases := []struct {
		name     string
		tenantID string
		deviceID string
	}{
		// 旧 LIKE 写法下空 deviceID 退化为 '%%'，会把租户全部活跃告警算到该设备头上。
		{name: "empty device id", tenantID: "tenant-a", deviceID: ""},
		{name: "unknown device", tenantID: "tenant-a", deviceID: "dev-404"},
		{name: "wrong tenant", tenantID: "tenant-b", deviceID: "dev-1"},
		{name: "wildcard device id", tenantID: "tenant-a", deviceID: "%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list, err := GetDeviceActiveAlarms(tc.tenantID, tc.deviceID)
			if err != nil {
				t.Fatalf("GetDeviceActiveAlarms: %v", err)
			}
			if list == nil {
				t.Fatalf("list is nil, want empty non-nil slice")
			}
			if len(list) != 0 {
				t.Fatalf("active alarms = %v, want none", alarmHistoryIDs(list))
			}
		})
	}
}
