// 文件用途：TB-48 统一调度器 DAL 层测试——聚合读路径与注册面 CRUD 的租户隔离契约。
// 核心逻辑：sqlite 内存库播种多租户行，验证四个聚合源（scene_automation_timers 联名、
//
//	report_schedules 未软删、command_jobs 定时行、scheduler_events 注册行）只吐本租户
//	行，以及注册面 Get/Update/Delete 按 (id, tenant_id) 双条件收敛、scene 事件配对写入
//	同事务落两行、执行行同步/删除同样带租户条件。
//
// 关键注意事项：隔离口径是 fail-closed——错误租户一律查无/0 行/404，绝不静默跨租户。
package dal

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupSchedulerEventTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scheduler event sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SchedulerEvent{},
		&model.SceneAutomationTimer{},
		&model.SceneAutomation{},
		&model.CommandJob{},
		&model.ReportSchedule{},
	); err != nil {
		t.Fatalf("migrate scheduler event tables: %v", err)
	}
	// model.SceneAutomationTimer 未映射 updated_at（89.sql 真实表带该列），
	// 测试库按真实 schema 补列，保证同步路径的 updated_at 刷新可执行。
	if !db.Migrator().HasColumn(&model.SceneAutomationTimer{}, "updated_at") {
		if err := db.Exec("ALTER TABLE scene_automation_timers ADD COLUMN updated_at timestamp").Error; err != nil {
			t.Fatalf("align scene timer test schema with 89.sql: %v", err)
		}
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}

func schedulerTestTimer(id, tenantID, automationID string, enabled bool, nextRunAt time.Time) *model.SceneAutomationTimer {
	return &model.SceneAutomationTimer{
		ID: id, TenantID: tenantID, SceneAutomationID: automationID,
		CronExpr: "*/5 * * * *", Timezone: "UTC", Enabled: enabled, NextRunAt: nextRunAt,
	}
}

func schedulerTestAutomation(id, tenantID, name string, now time.Time) *model.SceneAutomation {
	return &model.SceneAutomation{
		ID: id, Name: name, Enabled: "Y", TenantID: tenantID,
		Creator: "user", Updator: "user", CreatedAt: now,
	}
}

func TestListSceneTimerScheduleItemsTenantScope(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	rows := []*model.SceneAutomation{
		schedulerTestAutomation("auto-hq", "tenant-hq", "hq automation", base),
		schedulerTestAutomation("auto-x", "tenant-x", "foreign automation", base),
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed automations: %v", err)
	}
	timers := []*model.SceneAutomationTimer{
		schedulerTestTimer("timer-hq", "tenant-hq", "auto-hq", true, base.Add(5*time.Minute)),
		schedulerTestTimer("timer-x", "tenant-x", "auto-x", true, base.Add(6*time.Minute)),
	}
	for _, timer := range timers {
		if err := db.Create(timer).Error; err != nil {
			t.Fatalf("seed timers: %v", err)
		}
	}

	items, err := ListSceneTimerScheduleItems(ctx, "tenant-hq")
	if err != nil {
		t.Fatalf("ListSceneTimerScheduleItems(): %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want only the tenant-hq timer", len(items))
	}
	item := items[0]
	if item.ID != "timer-hq" || item.SourceType != model.SchedulerEventTypeScene ||
		item.Origin != model.SchedulerOriginSceneTimer {
		t.Fatalf("item = %#v, want tenant-hq scene_timer row", item)
	}
	if item.Name != "hq automation" {
		t.Fatalf("joined name = %q, want the automation name", item.Name)
	}
	if item.RefType != model.SchedulerRefTypeSceneAutomation || item.RefID != "auto-hq" {
		t.Fatalf("ref = %q/%q, want scene_automation/auto-hq", item.RefType, item.RefID)
	}
	if item.NextRunAt == nil || !item.NextRunAt.Equal(base.Add(5*time.Minute)) {
		t.Fatalf("next_run_at = %v, want %v", item.NextRunAt, base.Add(5*time.Minute))
	}
}

func TestListReportScheduleItemsTenantScope(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	next := base.Add(time.Hour)
	schedules := []*model.ReportSchedule{
		{ID: "rep-hq", TenantID: "tenant-hq", Name: "daily report", CronExpr: "0 9 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, NextRunAt: &next, CreatedAt: base, UpdatedAt: base},
		{ID: "rep-deleted", TenantID: "tenant-hq", Name: "deleted report", CronExpr: "0 10 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, CreatedAt: base, UpdatedAt: base,
			DeletedAt: &base},
		{ID: "rep-x", TenantID: "tenant-x", Name: "foreign report", CronExpr: "0 11 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, CreatedAt: base, UpdatedAt: base},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("seed report schedules: %v", err)
	}

	items, err := ListReportScheduleItems(ctx, "tenant-hq")
	if err != nil {
		t.Fatalf("ListReportScheduleItems(): %v", err)
	}
	if len(items) != 1 || items[0].ID != "rep-hq" {
		t.Fatalf("items = %#v, want only the live tenant-hq schedule (soft-deleted excluded)", items)
	}
	if items[0].SourceType != model.SchedulerEventTypeReport || items[0].Origin != model.SchedulerOriginReportSchedule {
		t.Fatalf("item taxonomy = %s/%s, want report/report_schedule", items[0].SourceType, items[0].Origin)
	}
	if items[0].CronExpr != "0 9 * * *" {
		t.Fatalf("cron = %q, want the schedule cron", items[0].CronExpr)
	}
}

func TestListFleetScheduledJobItemsTenantScope(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	scheduledAt := base.Add(30 * time.Minute)
	newJob := func(id, tenantID, status, remark string) *model.CommandJob {
		job := &model.CommandJob{
			ID: id, TenantID: tenantID, OperatorID: "operator", JobType: "command",
			ScopeType: "all", Identify: "identify-" + id, Status: status,
			TimeoutSeconds: 60, RequestedCount: 1, CreatedAt: base, UpdatedAt: base,
		}
		if status == "scheduled" {
			job.ScheduledAt = &scheduledAt
		}
		if remark != "" {
			job.Remark = &remark
		}
		return job
	}
	jobs := []*model.CommandJob{
		newJob("job-hq", "tenant-hq", "scheduled", "reboot fleet"),
		newJob("job-hq-no-remark", "tenant-hq", "scheduled", ""),
		newJob("job-hq-running", "tenant-hq", "running", "not a scheduled row"),
		newJob("job-x", "tenant-x", "scheduled", "foreign job"),
	}
	if err := db.CreateInBatches(jobs, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}

	items, err := ListFleetScheduledJobItems(ctx, "tenant-hq")
	if err != nil {
		t.Fatalf("ListFleetScheduledJobItems(): %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want only scheduled tenant-hq rows: %#v", len(items), items)
	}
	byID := make(map[string]*model.SchedulerEventItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	if byID["job-hq"] == nil || byID["job-hq-no-remark"] == nil {
		t.Fatalf("missing scheduled rows in %v", byID)
	}
	if byID["job-hq"].Name != "reboot fleet" {
		t.Fatalf("name = %q, want the remark", byID["job-hq"].Name)
	}
	if byID["job-hq-no-remark"].Name != "identify-job-hq-no-remark" {
		t.Fatalf("name = %q, want identify fallback", byID["job-hq-no-remark"].Name)
	}
	if byID["job-hq"].SourceType != model.SchedulerEventTypeRPC ||
		byID["job-hq"].Origin != model.SchedulerOriginFleetCommandJob {
		t.Fatalf("item taxonomy = %s/%s, want rpc/fleet_command_job", byID["job-hq"].SourceType, byID["job-hq"].Origin)
	}
	if byID["job-hq"].NextRunAt == nil || !byID["job-hq"].NextRunAt.Equal(scheduledAt) {
		t.Fatalf("next_run_at = %v, want scheduled_at %v", byID["job-hq"].NextRunAt, scheduledAt)
	}
	if byID["job-hq"].CronExpr != "" {
		t.Fatalf("cron = %q, want empty for one-shot jobs", byID["job-hq"].CronExpr)
	}
}

func TestListSchedulerRegistryItemsTenantScope(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	runAt := base.Add(2 * time.Hour)
	events := []*model.SchedulerEvent{
		{ID: "evt-hq", TenantID: "tenant-hq", Name: "plan rpc", EventType: model.SchedulerEventTypeRPC,
			RefType: model.SchedulerRefTypeFleetCommandJob, RefID: "device-1", NextRunAt: &runAt,
			Enabled: true, CreatedAt: base, UpdatedAt: base},
		{ID: "evt-x", TenantID: "tenant-x", Name: "foreign event", EventType: model.SchedulerEventTypeReport,
			Enabled: true, CreatedAt: base, UpdatedAt: base},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed scheduler events: %v", err)
	}

	items, err := ListSchedulerRegistryItems(ctx, "tenant-hq")
	if err != nil {
		t.Fatalf("ListSchedulerRegistryItems(): %v", err)
	}
	if len(items) != 1 || items[0].ID != "evt-hq" {
		t.Fatalf("items = %#v, want only evt-hq", items)
	}
	if items[0].Origin != model.SchedulerOriginRegistry || items[0].SourceType != model.SchedulerEventTypeRPC {
		t.Fatalf("item taxonomy = %s/%s, want rpc/scheduler_registry", items[0].SourceType, items[0].Origin)
	}
}

func TestSchedulerEventCRUDTenantIsolation(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	event := &model.SchedulerEvent{
		ID: "evt-iso", TenantID: "tenant-hq", Name: "iso event", EventType: model.SchedulerEventTypeReport,
		CronExpr: "0 9 * * *", Enabled: true, CreatedAt: base, UpdatedAt: base,
	}
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed event: %v", err)
	}

	t.Run("get with wrong tenant fails closed", func(t *testing.T) {
		if _, err := GetSchedulerEventInTenant(ctx, "evt-iso", "tenant-x"); err == nil {
			t.Fatal("cross-tenant get must fail")
		}
	})

	t.Run("update with wrong tenant touches zero rows", func(t *testing.T) {
		rows, err := UpdateSchedulerEventInTenant(ctx, "evt-iso", "tenant-x", map[string]interface{}{"name": "hijacked"})
		if err != nil {
			t.Fatalf("UpdateSchedulerEventInTenant(): %v", err)
		}
		if rows != 0 {
			t.Fatalf("rows = %d, want 0 for cross-tenant update", rows)
		}
	})

	t.Run("delete with wrong tenant touches zero rows", func(t *testing.T) {
		rows, err := DeleteSchedulerEventInTenant(ctx, "evt-iso", "tenant-x")
		if err != nil {
			t.Fatalf("DeleteSchedulerEventInTenant(): %v", err)
		}
		if rows != 0 {
			t.Fatalf("rows = %d, want 0 for cross-tenant delete", rows)
		}
	})

	t.Run("update and delete in tenant succeed", func(t *testing.T) {
		rows, err := UpdateSchedulerEventInTenant(ctx, "evt-iso", "tenant-hq", map[string]interface{}{"name": "renamed"})
		if err != nil || rows != 1 {
			t.Fatalf("rows = %d, err = %v, want 1 updated row", rows, err)
		}
		got, err := GetSchedulerEventInTenant(ctx, "evt-iso", "tenant-hq")
		if err != nil || got.Name != "renamed" {
			t.Fatalf("got = %#v, err = %v, want renamed event", got, err)
		}
		rows, err = DeleteSchedulerEventInTenant(ctx, "evt-iso", "tenant-hq")
		if err != nil || rows != 1 {
			t.Fatalf("delete rows = %d, err = %v, want 1", rows, err)
		}
	})
}

func TestCreateSchedulerSceneEventPairAndSync(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	next := now.Add(10 * time.Minute)

	t.Run("pair insert lands both rows with the same id", func(t *testing.T) {
		eventID := uuid.NewString()
		event := &model.SchedulerEvent{ID: eventID, TenantID: "tenant-hq", Name: "pair event",
			EventType: model.SchedulerEventTypeScene, RefType: model.SchedulerRefTypeSceneAutomation,
			RefID: "auto-pair", CronExpr: "*/5 * * * *", NextRunAt: &next, Enabled: true,
			CreatedAt: now, UpdatedAt: now}
		timer := schedulerTestTimer(eventID, "tenant-hq", "auto-pair", true, next)
		if err := CreateSchedulerSceneEventPair(ctx, event, timer); err != nil {
			t.Fatalf("CreateSchedulerSceneEventPair(): %v", err)
		}
		var gotTimer model.SceneAutomationTimer
		if err := db.Where("id = ?", eventID).First(&gotTimer).Error; err != nil {
			t.Fatalf("timer row missing: %v", err)
		}
		if gotTimer.SceneAutomationID != "auto-pair" || gotTimer.CronExpr != "*/5 * * * *" {
			t.Fatalf("timer row = %#v, want auto-pair execution row", gotTimer)
		}
		var gotEvent model.SchedulerEvent
		if err := db.Where("id = ?", eventID).First(&gotEvent).Error; err != nil {
			t.Fatalf("event row missing: %v", err)
		}
	})

	t.Run("timer sync update and delete are tenant-scoped", func(t *testing.T) {
		eventID := "pair-timer"
		event := &model.SchedulerEvent{ID: eventID, TenantID: "tenant-hq", Name: "sync event",
			EventType: model.SchedulerEventTypeScene, RefType: model.SchedulerRefTypeSceneAutomation,
			RefID: "auto-sync", CronExpr: "*/5 * * * *", NextRunAt: &next, Enabled: true,
			CreatedAt: now, UpdatedAt: now}
		if err := CreateSchedulerSceneEventPair(ctx, event, schedulerTestTimer(eventID, "tenant-hq", "auto-sync", true, next)); err != nil {
			t.Fatalf("CreateSchedulerSceneEventPair(): %v", err)
		}

		newCron := "0 12 * * *"
		newNext := now.Add(4 * time.Hour)
		rows, err := UpdateSchedulerSceneTimer(ctx, "tenant-hq", "pair-timer", SceneTimerChanges{CronExpr: &newCron, NextRunAt: &newNext}, now)
		if err != nil || rows != 1 {
			t.Fatalf("rows = %d, err = %v, want 1", rows, err)
		}
		var got model.SceneAutomationTimer
		if err := db.Where("id = ?", "pair-timer").First(&got).Error; err != nil {
			t.Fatalf("timer missing: %v", err)
		}
		if got.CronExpr != newCron || !got.NextRunAt.Equal(newNext) {
			t.Fatalf("timer = %#v, want synced cron/next_run_at", got)
		}

		// 错误租户删除必须 0 行，执行行仍在。
		rows, err = DeleteSchedulerSceneTimer(ctx, "tenant-x", "pair-timer")
		if err != nil || rows != 0 {
			t.Fatalf("cross-tenant delete rows = %d, err = %v, want 0", rows, err)
		}
		rows, err = DeleteSchedulerSceneTimer(ctx, "tenant-hq", "pair-timer")
		if err != nil || rows != 1 {
			t.Fatalf("delete rows = %d, err = %v, want 1", rows, err)
		}
	})
}

func TestCountEnabledSceneTimersForAutomation(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	timers := []*model.SceneAutomationTimer{
		schedulerTestTimer("timer-1", "tenant-hq", "auto-1", true, base),
		schedulerTestTimer("timer-2", "tenant-hq", "auto-1", false, base), // 停用不计数
		schedulerTestTimer("timer-3", "tenant-x", "auto-1", true, base),   // 跨租户不计数
	}
	for _, timer := range timers {
		if err := db.Create(timer).Error; err != nil {
			t.Fatalf("seed timers: %v", err)
		}
	}

	count, err := CountEnabledSceneTimersForAutomation(ctx, "tenant-hq", "auto-1", "")
	if err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v, want 1 enabled timer", count, err)
	}
	count, err = CountEnabledSceneTimersForAutomation(ctx, "tenant-hq", "auto-1", "timer-1")
	if err != nil || count != 0 {
		t.Fatalf("count with exclusion = %d, err = %v, want 0", count, err)
	}
}

func TestSceneAutomationExistsInTenant(t *testing.T) {
	db := setupSchedulerEventTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	if err := db.Create(schedulerTestAutomation("auto-hq", "tenant-hq", "hq", base)).Error; err != nil {
		t.Fatalf("seed automation: %v", err)
	}
	exists, err := SceneAutomationExistsInTenant(ctx, "tenant-hq", "auto-hq")
	if err != nil || !exists {
		t.Fatalf("exists = %v, err = %v, want true", exists, err)
	}
	exists, err = SceneAutomationExistsInTenant(ctx, "tenant-x", "auto-hq")
	if err != nil || exists {
		t.Fatalf("cross-tenant exists = %v, err = %v, want false", exists, err)
	}
}
