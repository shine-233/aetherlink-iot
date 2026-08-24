// 文件用途: 覆盖 alarm.go 租户 scope SQL 与告警配置/信息列表 raw 链的回归测试。
// 核心逻辑: 构造多租户 sqlite 行，断言列表查询的租户过滤、JOIN 投影与显式列更新语义。
// 关键注意事项: 列表查询必须先限定租户；trigger_duration 的零值回写只能走显式列更新。
// 重构建议: jsonb 相关 PostgreSQL 专用路径需在 PG 集成层覆盖，sqlite 用例聚焦可移植语义。

package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAlarmDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "alarm_dal_" + t.Name()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(
		&model.AlarmConfig{},
		&model.AlarmInfo{},
		&model.NotificationGroup{},
		&model.User{},
	); err != nil {
		t.Fatalf("migrate test tables: %v", err)
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

func TestGetAlarmConfigListByPageTenantScope(t *testing.T) {
	db := setupAlarmDALTestDB(t)
	now := time.Now().UTC()
	configs := []model.AlarmConfig{
		{ID: "ac-t1-a", Name: "tenant1-high", AlarmLevel: "H", NotificationGroupID: "ng-1", TenantID: "tenant-1", Enabled: "Y", CreatedAt: now, UpdatedAt: now},
		{ID: "ac-t1-b", Name: "tenant1-low", AlarmLevel: "L", NotificationGroupID: "", TenantID: "tenant-1", Enabled: "Y", CreatedAt: now.Add(time.Second), UpdatedAt: now},
		{ID: "ac-t2-a", Name: "tenant2-high", AlarmLevel: "H", NotificationGroupID: "", TenantID: "tenant-2", Enabled: "Y", CreatedAt: now, UpdatedAt: now},
	}
	for _, cfg := range configs {
		cfg := cfg
		if err := db.Create(&cfg).Error; err != nil {
			t.Fatalf("create alarm config %s: %v", cfg.ID, err)
		}
	}

	count, list, err := GetAlarmConfigListByPage(&model.GetAlarmConfigListByPageReq{
		TenantID: "tenant-1",
		PageReq:  model.PageReq{Page: 1, PageSize: 20},
	})
	if err != nil {
		t.Fatalf("GetAlarmConfigListByPage returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (tenant-1 rows only)", count)
	}
	ids := map[string]bool{}
	for _, row := range list.([]map[string]interface{}) {
		ids[row["id"].(string)] = true
	}
	if !ids["ac-t1-a"] || !ids["ac-t1-b"] || ids["ac-t2-a"] {
		t.Fatalf("ids = %#v, want tenant-1 rows without cross-tenant leak", ids)
	}
}

func TestGetAlarmConfigListByPageJoinsNotificationGroupNameAndFilters(t *testing.T) {
	db := setupAlarmDALTestDB(t)
	now := time.Now().UTC()
	group := model.NotificationGroup{ID: "ng-1", Name: "oncall"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create notification group: %v", err)
	}
	level := "M"
	if err := db.Create(&model.AlarmConfig{
		ID: "ac-filtered", Name: "pressure-watch", AlarmLevel: level,
		NotificationGroupID: "ng-1", TenantID: "tenant-1", Enabled: "Y",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create alarm config: %v", err)
	}
	if err := db.Create(&model.AlarmConfig{
		ID: "ac-other", Name: "other", AlarmLevel: level,
		NotificationGroupID: "", TenantID: "tenant-1", Enabled: "N",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create second alarm config: %v", err)
	}

	name := "pressure"
	count, list, err := GetAlarmConfigListByPage(&model.GetAlarmConfigListByPageReq{
		TenantID:   "tenant-1",
		Name:       &name,
		Enabled:    "Y",
		AlarmLevel: &level,
	})
	if err != nil {
		t.Fatalf("filtered GetAlarmConfigListByPage returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	rows := list.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["id"].(string) != "ac-filtered" {
		t.Fatalf("rows = %#v, want only ac-filtered", rows)
	}
	if rows[0]["notification_group_name"] != "oncall" {
		t.Fatalf("notification_group_name = %#v, want oncall join projection", rows[0]["notification_group_name"])
	}
}

func TestUpdateAlarmConfigTriggerDurationWritesZeroValue(t *testing.T) {
	db := setupAlarmDALTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&model.AlarmConfig{
		ID: "ac-duration", Name: "config", AlarmLevel: "H",
		TriggerDuration: 30, TenantID: "tenant-1", Enabled: "Y",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create alarm config: %v", err)
	}

	if err := UpdateAlarmConfigTriggerDuration("ac-duration", 0); err != nil {
		t.Fatalf("UpdateAlarmConfigTriggerDuration returned error: %v", err)
	}
	var stored model.AlarmConfig
	if err := db.Where("id = ?", "ac-duration").First(&stored).Error; err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if stored.TriggerDuration != 0 {
		t.Fatalf("trigger_duration = %d, want 0 (zero value must be writable)", stored.TriggerDuration)
	}

	// 不存在的 id 必须报错而不是静默成功。
	if err := UpdateAlarmConfigTriggerDuration("missing-id", 5); err == nil {
		t.Fatal("expected error for missing config id")
	}
}

func TestGetAlarmInfoListByPageTenantScopeAndJoinProjection(t *testing.T) {
	db := setupAlarmDALTestDB(t)
	now := time.Now().UTC()
	statusN := "N"
	processor := "user-processor"
	processorName := "Processor"
	if err := db.Create(&model.User{
		ID: processor, Name: &processorName, Email: "proc@example.com", Password: "x", Status: &statusN,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&model.AlarmConfig{
		ID: "ac-info-src", Name: "temp-alarm", AlarmLevel: "H",
		TenantID: "tenant-1", Enabled: "Y", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create alarm config: %v", err)
	}
	infos := []model.AlarmInfo{
		{ID: "ai-t1", AlarmConfigID: "ac-info-src", Name: "temp", AlarmTime: now, ProcessingResult: "UND", TenantID: "tenant-1", Processor: &processor},
		{ID: "ai-t2", AlarmConfigID: "ac-info-src", Name: "temp", AlarmTime: now, ProcessingResult: "UND", TenantID: "tenant-2"},
	}
	for _, info := range infos {
		info := info
		if err := db.Create(&info).Error; err != nil {
			t.Fatalf("create alarm info %s: %v", info.ID, err)
		}
	}

	count, list, err := GetAlarmInfoListByPage(&model.GetAlarmInfoListByPageReq{
		TenantID: "tenant-1",
		PageReq:  model.PageReq{Page: 1, PageSize: 20},
	})
	if err != nil {
		t.Fatalf("GetAlarmInfoListByPage returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (tenant-1 rows only)", count)
	}
	rows := list.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["id"].(string) != "ai-t1" {
		t.Fatalf("rows = %#v, want only ai-t1", rows)
	}
	if rows[0]["alarm_config_name"] != "temp-alarm" {
		t.Fatalf("alarm_config_name = %#v, want temp-alarm", rows[0]["alarm_config_name"])
	}
	if rows[0]["processor_name"] == nil || rows[0]["processor_name"] == "" {
		t.Fatalf("processor_name = %#v, want joined user name", rows[0]["processor_name"])
	}
}
