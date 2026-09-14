package dal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupDeviceGroupStatisticsTestDB 与 scope 测试的夹具同构，额外迁移 latest_device_alarms
// ——统计里的 alarm_total 依赖该表，不迁移会让整条 SQL 报错。
func setupDeviceGroupStatisticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Group{},
		&model.Device{},
		&model.RGroupDevice{},
		&model.LatestDeviceAlarm{},
	); err != nil {
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

type statisticsSeedDevice struct {
	id         string
	tenant     string
	owner      *string
	online     bool
	activate   string
	alarmState string
}

func seedStatisticsDevice(t *testing.T, db *gorm.DB, groupID string, d statisticsSeedDevice) {
	t.Helper()
	now := time.Now().UTC()
	online := int16(0)
	if d.online {
		online = 1
	}
	activate := d.activate
	if activate == "" {
		activate = "active"
	}
	if err := db.Create(&model.Device{
		ID:           d.id,
		TenantID:     d.tenant,
		OwnerUserID:  d.owner,
		IsEnabled:    "enabled",
		ActivateFlag: activate,
		IsOnline:     online,
		DeviceNumber: d.id,
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("seed device %s: %v", d.id, err)
	}
	if groupID != "" {
		if err := db.Create(&model.RGroupDevice{
			GroupID:  groupID,
			DeviceID: d.id,
			TenantID: d.tenant,
		}).Error; err != nil {
			t.Fatalf("seed relation %s/%s: %v", groupID, d.id, err)
		}
	}
	if d.alarmState != "" {
		alarmStatus := d.alarmState
		tenant := d.tenant
		deviceID := d.id
		if err := db.Create(&model.LatestDeviceAlarm{
			ID:          &deviceID,
			AlarmStatus: &alarmStatus,
			TenantID:    &tenant,
			DeviceID:    &deviceID,
		}).Error; err != nil {
			t.Fatalf("seed alarm %s: %v", d.id, err)
		}
	}
}

// TestGetDeviceGroupStatisticsBatchRollsUpDescendants 父分组统计必须含子孙分组的设备，
// 子分组只统计自己 —— 这正是"逐节点调用单分组版"与批量版必须语义一致的地方。
func TestGetDeviceGroupStatisticsBatchRollsUpDescendants(t *testing.T) {
	db := setupDeviceGroupStatisticsTestDB(t)
	seedDeviceGroup(t, db, "grp-root", "Root", "t1", "0", nil)
	seedDeviceGroup(t, db, "grp-child", "Child", "t1", "grp-root", nil)
	seedDeviceGroup(t, db, "grp-grand", "Grand", "t1", "grp-child", nil)

	seedStatisticsDevice(t, db, "grp-root", statisticsSeedDevice{id: "dev-root", tenant: "t1", online: true})
	seedStatisticsDevice(t, db, "grp-child", statisticsSeedDevice{id: "dev-child", tenant: "t1", online: false})
	seedStatisticsDevice(t, db, "grp-grand", statisticsSeedDevice{id: "dev-grand", tenant: "t1", online: true})

	stats, err := GetDeviceGroupStatisticsBatch([]string{"grp-root", "grp-child", "grp-grand"}, "t1", nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	root := stats["grp-root"]
	if root == nil || root.DeviceTotal != 3 || root.OnlineTotal != 2 || root.OfflineTotal != 1 {
		t.Fatalf("root stats = %#v, want total=3 online=2 offline=1", root)
	}

	child := stats["grp-child"]
	if child == nil || child.DeviceTotal != 2 || child.OnlineTotal != 1 {
		t.Fatalf("child stats = %#v, want total=2 online=1", child)
	}

	grand := stats["grp-grand"]
	if grand == nil || grand.DeviceTotal != 1 || grand.OnlineTotal != 1 {
		t.Fatalf("grand stats = %#v, want total=1 online=1", grand)
	}
}

// TestGetDeviceGroupStatisticsBatchZeroFillsEmptyGroups 无设备的分组必须出现零值，
// 否则调用方无法区分"没查到"与"零台设备"。
func TestGetDeviceGroupStatisticsBatchZeroFillsEmptyGroups(t *testing.T) {
	db := setupDeviceGroupStatisticsTestDB(t)
	seedDeviceGroup(t, db, "grp-empty", "Empty", "t1", "0", nil)
	seedDeviceGroup(t, db, "grp-has", "Has", "t1", "0", nil)
	seedStatisticsDevice(t, db, "grp-has", statisticsSeedDevice{id: "dev-1", tenant: "t1", online: true})

	stats, err := GetDeviceGroupStatisticsBatch([]string{"grp-empty", "grp-has"}, "t1", nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	empty, ok := stats["grp-empty"]
	if !ok || empty == nil {
		t.Fatalf("empty group must be present with zero value, got %#v", stats["grp-empty"])
	}
	if empty.DeviceTotal != 0 || empty.OnlineTotal != 0 || empty.OfflineTotal != 0 || empty.AlarmTotal != 0 {
		t.Fatalf("empty group stats = %#v, want all zero", empty)
	}
}

// TestGetDeviceGroupStatisticsBatchScopesTenantOwnerAndActivation 三条过滤必须与单分组版一致：
// 他租户设备、他人 owner 设备、未激活设备都不得计入。
func TestGetDeviceGroupStatisticsBatchScopesTenantOwnerAndActivation(t *testing.T) {
	db := setupDeviceGroupStatisticsTestDB(t)
	seedDeviceGroup(t, db, "grp-a", "A", "t1", "0", nil)

	ownerU1 := "u1"
	ownerU2 := "u2"
	seedStatisticsDevice(t, db, "grp-a", statisticsSeedDevice{id: "dev-u1", tenant: "t1", owner: &ownerU1, online: true})
	seedStatisticsDevice(t, db, "grp-a", statisticsSeedDevice{id: "dev-u2", tenant: "t1", owner: &ownerU2, online: true})
	seedStatisticsDevice(t, db, "grp-a", statisticsSeedDevice{id: "dev-other-tenant", tenant: "t2", online: true})
	seedStatisticsDevice(t, db, "grp-a", statisticsSeedDevice{id: "dev-inactive", tenant: "t1", online: true, activate: "inactive"})

	// 无 owner 过滤：t1 的 active 设备全部计入（dev-u1 + dev-u2），他租户与未激活排除。
	stats, err := GetDeviceGroupStatisticsBatch([]string{"grp-a"}, "t1", nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if got := stats["grp-a"].DeviceTotal; got != 2 {
		t.Fatalf("tenant scope total = %d, want 2 (other tenant + inactive must be excluded)", got)
	}

	// owner=u1：只剩 dev-u1。
	stats, err = GetDeviceGroupStatisticsBatch([]string{"grp-a"}, "t1", &ownerU1)
	if err != nil {
		t.Fatalf("batch owner: %v", err)
	}
	if got := stats["grp-a"].DeviceTotal; got != 1 {
		t.Fatalf("owner scope total = %d, want 1", got)
	}

	// 他租户视角看不到任何设备。
	stats, err = GetDeviceGroupStatisticsBatch([]string{"grp-a"}, "t2", nil)
	if err != nil {
		t.Fatalf("batch other tenant: %v", err)
	}
	if got := stats["grp-a"].DeviceTotal; got != 1 {
		t.Fatalf("other tenant total = %d, want 1 (its own device only)", got)
	}
}

// TestGetDeviceGroupStatisticsBatchCountsHighMediumLowAlarms 只有 H/M/L 计入 alarm_total，
// 其他状态（含空值）不得计数。
func TestGetDeviceGroupStatisticsBatchCountsHighMediumLowAlarms(t *testing.T) {
	db := setupDeviceGroupStatisticsTestDB(t)
	seedDeviceGroup(t, db, "grp-alarm", "Alarm", "t1", "0", nil)

	seedStatisticsDevice(t, db, "grp-alarm", statisticsSeedDevice{id: "dev-h", tenant: "t1", alarmState: "H"})
	seedStatisticsDevice(t, db, "grp-alarm", statisticsSeedDevice{id: "dev-m", tenant: "t1", alarmState: "M"})
	seedStatisticsDevice(t, db, "grp-alarm", statisticsSeedDevice{id: "dev-l", tenant: "t1", alarmState: "L"})
	seedStatisticsDevice(t, db, "grp-alarm", statisticsSeedDevice{id: "dev-none", tenant: "t1", alarmState: "N"})
	seedStatisticsDevice(t, db, "grp-alarm", statisticsSeedDevice{id: "dev-nil", tenant: "t1"})

	stats, err := GetDeviceGroupStatisticsBatch([]string{"grp-alarm"}, "t1", nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if got := stats["grp-alarm"].AlarmTotal; got != 3 {
		t.Fatalf("alarm_total = %d, want 3 (only H/M/L)", got)
	}
}

// TestGetDeviceGroupStatisticsBatchEmptyInput 空输入必须返回空 map 且不报错。
func TestGetDeviceGroupStatisticsBatchEmptyInput(t *testing.T) {
	setupDeviceGroupStatisticsTestDB(t)

	stats, err := GetDeviceGroupStatisticsBatch(nil, "t1", nil)
	if err != nil {
		t.Fatalf("nil input: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("nil input map len = %d, want 0", len(stats))
	}

	stats, err = GetDeviceGroupStatisticsBatch([]string{}, "t1", nil)
	if err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("empty input map len = %d, want 0", len(stats))
	}
}

// seedAlarmHistory 往基表 alarm_history 写一条告警，供视图链
// alarm_history → current_device_alarm_streams → latest_device_alarms 派生。
// 只能写基表：latest_device_alarms 是视图，不能 INSERT。
func seedAlarmHistory(t *testing.T, db *gorm.DB, id, tenant, status string, deviceIDs []string, createdAt time.Time) {
	t.Helper()
	deviceList, err := json.Marshal(deviceIDs)
	if err != nil {
		t.Fatalf("marshal alarm_device_list: %v", err)
	}
	row := model.AlarmHistory{
		ID:                id,
		AlarmConfigID:     "cfg-" + id,
		GroupID:           "grp-" + id,
		SceneAutomationID: "scene-" + id,
		Name:              "alarm-" + id,
		AlarmStatus:       status,
		TenantID:          tenant,
		CreateAt:          createdAt,
		AlarmDeviceList:   string(deviceList),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed alarm_history %s: %v", id, err)
	}
}

// TestGetDeviceGroupStatisticsBatchCountsAlarmsOnPostgres 在真实 PostgreSQL 上验证 alarm_total。
//
// 为什么必须单独有 PG 证据：alarm_total 读的是 latest_device_alarms，而它在 PostgreSQL 上
// 是**视图**（sql/44.sql 定义，链到基表 alarm_history）。SQLite 夹具里同名对象是
// AutoMigrate 出来的**表**，两者语义不同 —— SQLite 绿不能代表 PG 绿。
func TestGetDeviceGroupStatisticsBatchCountsAlarmsOnPostgres(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; alarm_total PostgreSQL verification skipped")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	const tenant = "pg-groupstats-alarm-verify"
	oldDB := global.DB
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		for _, stmt := range []string{
			"DELETE FROM " + model.TableNameAlarmHistory + " WHERE tenant_id = ?",
			"DELETE FROM " + model.TableNameRGroupDevice + " WHERE tenant_id = ?",
			"DELETE FROM " + model.TableNameDevice + " WHERE tenant_id = ?",
			"DELETE FROM " + model.TableNameGroup + " WHERE tenant_id = ?",
		} {
			_ = db.Exec(stmt, tenant).Error
		}
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	seedDeviceGroup(t, db, "pg-ga-grp", "GA Group", tenant, "0", nil)
	seedDeviceGroup(t, db, "pg-ga-child", "GA Child", tenant, "pg-ga-grp", nil)

	// 4 台设备，分别对应：H / M / N（不计）/ 老 H + 新 N
	for _, id := range []string{"pg-ga-h", "pg-ga-m", "pg-ga-n", "pg-ga-hn"} {
		groupID := "pg-ga-grp"
		if id == "pg-ga-hn" {
			groupID = "pg-ga-child" // 放到子分组，顺带验证告警数也按子孙汇总
		}
		seedStatisticsDevice(t, db, groupID, statisticsSeedDevice{id: id, tenant: tenant})
	}

	base := time.Now().UTC().Add(-time.Hour)
	seedAlarmHistory(t, db, "pg-ga-ah-h", tenant, "H", []string{"pg-ga-h"}, base)
	seedAlarmHistory(t, db, "pg-ga-ah-m", tenant, "M", []string{"pg-ga-m"}, base)
	seedAlarmHistory(t, db, "pg-ga-ah-n", tenant, "N", []string{"pg-ga-n"}, base)
	// 同一设备先 H 后 N：视图的排序是 "H/M/L 优先于 N"，再比 create_at，
	// 因此该设备的当前状态仍是 H —— 必须计入。
	seedAlarmHistory(t, db, "pg-ga-ah-hn-old", tenant, "H", []string{"pg-ga-hn"}, base)
	seedAlarmHistory(t, db, "pg-ga-ah-hn-new", tenant, "N", []string{"pg-ga-hn"}, base.Add(30*time.Minute))

	stats, err := GetDeviceGroupStatisticsBatch([]string{"pg-ga-grp", "pg-ga-child"}, tenant, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	root := stats["pg-ga-grp"]
	if root == nil {
		t.Fatal("root stats missing")
	}
	if root.DeviceTotal != 4 {
		t.Fatalf("root device_total = %d, want 4", root.DeviceTotal)
	}
	if root.AlarmTotal != 3 {
		t.Fatalf("root alarm_total = %d, want 3 (H + M + 老H压过新N; N 不计)", root.AlarmTotal)
	}

	child := stats["pg-ga-child"]
	if child == nil || child.AlarmTotal != 1 {
		t.Fatalf("child stats = %#v, want alarm_total=1", child)
	}

	// 与单分组版对齐：告警计数在两条路径上必须一致。
	single, err := GetDeviceGroupStatistics("pg-ga-grp", tenant, nil)
	if err != nil {
		t.Fatalf("single: %v", err)
	}
	if *single != *root {
		t.Fatalf("alarm rollup mismatch: single=%#v batch=%#v", *single, *root)
	}
}

// TestGetDeviceGroupStatisticsBatchMatchesSingleGroupVersion 批量结果必须与逐分组调用
// 单分组版逐个对齐 —— 这是"把 N 次往返压成一条 SQL"不改变语义的回归护栏。
//
// 只在真实 PostgreSQL 上执行：单分组版 GetDeviceGroupStatistics 在 SQLite 夹具下会报
// "bad parameter or other API misuse"（驱动层限制），SQLite 绿不能代表 PG 绿。
// 与其造一个恒真或恒假的对照，不如在没有 DSN 时明确 skip。
//
// 需要 AETHERLINK_TEST_PSQL_DSN（与 scada_postgres_test.go 同一约定）。
func TestGetDeviceGroupStatisticsBatchMatchesSingleGroupVersion(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; batch-vs-single equivalence check skipped")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	const tenant = "pg-groupstats-verify"
	oldDB := global.DB
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		// 注意：latest_device_alarms 在 PostgreSQL 上是**视图**（见 sql/35.sql、
		// sql/43.sql），不能 INSERT/DELETE；这里也不造告警数据，故无需清理它。
		for _, stmt := range []string{
			"DELETE FROM " + model.TableNameRGroupDevice + " WHERE tenant_id = ?",
			"DELETE FROM " + model.TableNameDevice + " WHERE tenant_id = ?",
			"DELETE FROM " + model.TableNameGroup + " WHERE tenant_id = ?",
		} {
			_ = db.Exec(stmt, tenant).Error
		}
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	seedDeviceGroup(t, db, "pg-grp-root", "PG Root", tenant, "0", nil)
	seedDeviceGroup(t, db, "pg-grp-child", "PG Child", tenant, "pg-grp-root", nil)
	seedStatisticsDevice(t, db, "pg-grp-root", statisticsSeedDevice{id: "pg-dev-a", tenant: tenant, online: true})
	seedStatisticsDevice(t, db, "pg-grp-child", statisticsSeedDevice{id: "pg-dev-b", tenant: tenant, online: false})
	seedStatisticsDevice(t, db, "pg-grp-child", statisticsSeedDevice{id: "pg-dev-c", tenant: tenant, online: true})

	ids := []string{"pg-grp-root", "pg-grp-child"}
	batch, err := GetDeviceGroupStatisticsBatch(ids, tenant, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	for _, id := range ids {
		single, err := GetDeviceGroupStatistics(id, tenant, nil)
		if err != nil {
			t.Fatalf("single %s: %v", id, err)
		}
		got := batch[id]
		if got == nil {
			t.Fatalf("batch missing %s", id)
		}
		if *got != *single {
			t.Fatalf("group %s batch=%#v single=%#v (must match)", id, *got, *single)
		}
	}
}
