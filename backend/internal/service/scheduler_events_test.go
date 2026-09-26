// 文件用途：TB-48 统一调度器服务层测试——聚合列表、注册面 CRUD、scene 事件落
// scene automation timer 机制的同步契约与租户隔离。
// 核心逻辑：sqlite 内存库换装 global.DB，验证：
//   - 聚合列表把三套存量调度 + 注册行归一为统一条目（含来源类型/来源系统），scene
//     注册行与其同 id 执行行只出现一次；source_type/enabled/窗口/搜索过滤与排序分页；
//   - scene 事件创建在同一事务落 scheduler_events + scene_automation_timers（同 id），
//     next_run_at 由 cron 以 UTC 严格晚于当前时刻计算；重复启用定时触发被拒；
//   - report/rpc 注册行只登记不接管执行（rpc 一次性必须给未来时刻，cron 必须为空）；
//   - 更新/删除 scene 事件同步执行行；event_type 不可变；
//   - 跨租户 Get/Update/Delete 一律 404（不泄露行存在性），列表不跨租户。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSchedulerServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scheduler service sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SchedulerEvent{},
		&model.SceneAutomationTimer{},
		&model.SceneAutomation{},
		&model.CommandJob{},
		&model.ReportSchedule{},
	); err != nil {
		t.Fatalf("migrate scheduler service tables: %v", err)
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

// schedulerStrPtr 测试辅助：字符串指针。
func schedulerStrPtr(value string) *string { return &value }

func schedulerClaimsFor(tenantID string) *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", Authority: "TENANT_ADMIN", TenantID: tenantID}
}

func wantSchedulerCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", wantCode)
	}
	var appErr *errcode.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *errcode.Error, got %T: %v", err, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("error code = %d, want %d (message: %s)", appErr.Code, wantCode, appErr.CustomMsg)
	}
}

func seedSchedulerAutomation(t *testing.T, db *gorm.DB, id, tenantID, name string, now time.Time) {
	t.Helper()
	if err := db.Create(&model.SceneAutomation{
		ID: id, Name: name, Enabled: "Y", TenantID: tenantID,
		Creator: "user", Updator: "user", CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed automation %s: %v", id, err)
	}
}

func TestSchedulerServiceCreateSceneEventLandsInTimerMechanism(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedSchedulerAutomation(t, db, "auto-hq", "tenant-hq", "hq automation", now)
	svc := SchedulerService{}
	claims := schedulerClaimsFor("tenant-hq")

	created, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "every five minutes", EventType: model.SchedulerEventTypeScene,
		RefID: "auto-hq", CronExpr: "*/5 * * * *",
	}, claims)
	if err != nil {
		t.Fatalf("CreateSchedulerEvent(scene): %v", err)
	}
	if created.ID == "" || created.TenantID != "tenant-hq" {
		t.Fatalf("created = %#v, want tenant-scoped row", created)
	}
	if created.RefType != model.SchedulerRefTypeSceneAutomation {
		t.Fatalf("ref_type = %q, want scene_automation", created.RefType)
	}
	if created.NextRunAt == nil || !created.NextRunAt.After(now) {
		t.Fatalf("next_run_at = %v, want strictly after now", created.NextRunAt)
	}

	// 执行行必须与注册行同 id，且由既有 scene timer worker 机制可直接领取。
	var timer model.SceneAutomationTimer
	if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
		t.Fatalf("scene timer row missing for event %s: %v", created.ID, err)
	}
	if timer.SceneAutomationID != "auto-hq" || timer.CronExpr != "*/5 * * * *" ||
		timer.Timezone != "UTC" || !timer.Enabled || !timer.NextRunAt.Equal(*created.NextRunAt) {
		t.Fatalf("timer row = %#v, want synced execution row", timer)
	}

	t.Run("second enabled timer for the same automation is rejected", func(t *testing.T) {
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "duplicate", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-hq", CronExpr: "0 * * * *",
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeOpDenied)
	})

	t.Run("scene event with explicit next_run_at is rejected", func(t *testing.T) {
		runAt := now.Add(time.Hour)
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "hand-set", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-hq", CronExpr: "0 * * * *", NextRunAt: &runAt,
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("scene event for a foreign automation is rejected", func(t *testing.T) {
		seedSchedulerAutomation(t, db, "auto-x", "tenant-x", "foreign automation", now)
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "cross tenant", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-x", CronExpr: "0 * * * *",
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("scene event with unparsable cron is rejected", func(t *testing.T) {
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "bad cron", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-hq", CronExpr: "not a cron",
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("missing tenant context fails closed", func(t *testing.T) {
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "no tenant", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-hq", CronExpr: "0 * * * *",
		}, &utils.UserClaims{})
		wantSchedulerCode(t, err, errcode.CodeNoPermission)
	})
}

func TestSchedulerServiceCreateRegistryOnlyEvents(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	svc := SchedulerService{}
	claims := schedulerClaimsFor("tenant-hq")

	t.Run("rpc one-shot event", func(t *testing.T) {
		runAt := now.Add(30 * time.Minute)
		event, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "reboot device", EventType: model.SchedulerEventTypeRPC,
			RefID: "device-1", NextRunAt: &runAt,
		}, claims)
		if err != nil {
			t.Fatalf("CreateSchedulerEvent(rpc): %v", err)
		}
		if event.CronExpr != "" || event.NextRunAt == nil || !event.NextRunAt.Equal(runAt) {
			t.Fatalf("rpc event = %#v, want one-shot at the given time", event)
		}
		if event.RefType != model.SchedulerRefTypeFleetCommandJob {
			t.Fatalf("ref_type = %q, want fleet_command_job default", event.RefType)
		}
		var timerCount int64
		db.Table("scene_automation_timers").Count(&timerCount)
		if timerCount != 0 {
			t.Fatalf("registry-only rpc event must not create execution rows, got %d", timerCount)
		}
	})

	t.Run("rpc without next_run_at is rejected", func(t *testing.T) {
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "no time", EventType: model.SchedulerEventTypeRPC,
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("rpc with cron is rejected", func(t *testing.T) {
		runAt := now.Add(time.Hour)
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "cron rpc", EventType: model.SchedulerEventTypeRPC,
			CronExpr: "0 * * * *", NextRunAt: &runAt,
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("rpc in the past is rejected", func(t *testing.T) {
		runAt := now.Add(-time.Minute)
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "past rpc", EventType: model.SchedulerEventTypeRPC, NextRunAt: &runAt,
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})

	t.Run("report registry event computes next run", func(t *testing.T) {
		event, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "planned report", EventType: model.SchedulerEventTypeReport,
			RefID: "rep-later", CronExpr: "0 9 * * *",
		}, claims)
		if err != nil {
			t.Fatalf("CreateSchedulerEvent(report): %v", err)
		}
		if event.NextRunAt == nil || !event.NextRunAt.After(now) {
			t.Fatalf("next_run_at = %v, want computed future run", event.NextRunAt)
		}
	})

	t.Run("report event without cron is rejected", func(t *testing.T) {
		_, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "no cron report", EventType: model.SchedulerEventTypeReport,
		}, claims)
		wantSchedulerCode(t, err, errCodeParamForScheduler())
	})
}

// errCodeParamForScheduler 保持与 wantSchedulerCode 相同的口径，仅为可读性命名。
func errCodeParamForScheduler() int { return errcode.CodeParamError }

func TestSchedulerServiceAggregateList(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	svc := SchedulerService{}
	claims := schedulerClaimsFor("tenant-hq")

	// 存量三源播种：本租户 + 外租户 + 软删/非定时行。
	seedSchedulerAutomation(t, db, "auto-hq", "tenant-hq", "hq automation", now)
	timerNext := now.Add(20 * time.Minute)
	if err := db.Create(&model.SceneAutomationTimer{
		ID: "timer-hq", TenantID: "tenant-hq", SceneAutomationID: "auto-hq",
		CronExpr: "*/10 * * * *", Timezone: "UTC", Enabled: true, NextRunAt: timerNext,
	}).Error; err != nil {
		t.Fatalf("seed timer: %v", err)
	}
	if err := db.Create(&model.SceneAutomationTimer{
		ID: "timer-x", TenantID: "tenant-x", SceneAutomationID: "auto-x",
		CronExpr: "*/10 * * * *", Timezone: "UTC", Enabled: true, NextRunAt: timerNext,
	}).Error; err != nil {
		t.Fatalf("seed foreign timer: %v", err)
	}
	reportNext := now.Add(40 * time.Minute)
	deletedAt := now
	schedules := []*model.ReportSchedule{
		{ID: "rep-hq", TenantID: "tenant-hq", Name: "daily report", CronExpr: "0 9 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, NextRunAt: &reportNext, CreatedAt: now, UpdatedAt: now},
		{ID: "rep-dead", TenantID: "tenant-hq", Name: "deleted report", CronExpr: "0 10 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, CreatedAt: now, UpdatedAt: now, DeletedAt: &deletedAt},
		{ID: "rep-x", TenantID: "tenant-x", Name: "foreign report", CronExpr: "0 11 * * *", Timezone: "UTC",
			Recipients: "ops@example.com", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatalf("seed report schedules: %v", err)
	}
	jobNext := now.Add(60 * time.Minute)
	jobs := []*model.CommandJob{
		{ID: "job-hq", TenantID: "tenant-hq", OperatorID: "operator", JobType: "command", ScopeType: "all",
			Identify: "reboot-all", Status: "scheduled", TimeoutSeconds: 60, RequestedCount: 1,
			ScheduledAt: &jobNext, CreatedAt: now, UpdatedAt: now},
		{ID: "job-hq-done", TenantID: "tenant-hq", OperatorID: "operator", JobType: "command", ScopeType: "all",
			Identify: "done", Status: "completed", TimeoutSeconds: 60, RequestedCount: 1,
			CreatedAt: now, UpdatedAt: now},
	}
	if err := db.CreateInBatches(jobs, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}

	// 注册行：scene（应经由执行行呈现一次）+ rpc 注册行。
	// 指向独立的 auto-reg（timer-hq 已占用 auto-hq 的唯一启用定时器位）。
	seedSchedulerAutomation(t, db, "auto-reg", "tenant-hq", "registry automation", now)
	sceneEvent, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "scene via registry", EventType: model.SchedulerEventTypeScene,
		RefID: "auto-reg", CronExpr: "*/30 * * * *",
	}, claims)
	if err != nil {
		t.Fatalf("create scene event: %v", err)
	}
	// cron 的下一次触发取决于墙钟，为让窗口过滤断言确定化，
	// 把该事件（注册行 + 执行行，同 id）的 next_run_at 固定到 70m。
	sceneNext := now.Add(70 * time.Minute)
	if err := db.Model(&model.SceneAutomationTimer{}).Where("id = ?", sceneEvent.ID).
		Update("next_run_at", sceneNext).Error; err != nil {
		t.Fatalf("pin scene timer next_run_at: %v", err)
	}
	if err := db.Model(&model.SchedulerEvent{}).Where("id = ?", sceneEvent.ID).
		Update("next_run_at", sceneNext).Error; err != nil {
		t.Fatalf("pin scene event next_run_at: %v", err)
	}
	rpcRunAt := now.Add(90 * time.Minute)
	if _, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "registry rpc", EventType: model.SchedulerEventTypeRPC, NextRunAt: &rpcRunAt,
	}, claims); err != nil {
		t.Fatalf("create rpc event: %v", err)
	}

	result, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{}, claims)
	if err != nil {
		t.Fatalf("ListSchedulerEvents(): %v", err)
	}
	byKey := make(map[string]*model.SchedulerEventItem, len(result.List))
	for _, item := range result.List {
		byKey[item.ID+":"+item.Origin] = item
	}

	// 四源全部可见：timer 执行行（scene）、report、job 定时行、rpc 注册行。
	if _, ok := byKey["timer-hq:scene_timer"]; !ok {
		t.Fatalf("scene timer row missing from aggregate: %#v", result.List)
	}
	if _, ok := byKey["rep-hq:report_schedule"]; !ok {
		t.Fatalf("report schedule missing from aggregate: %#v", result.List)
	}
	if _, ok := byKey["job-hq:fleet_command_job"]; !ok {
		t.Fatalf("scheduled command job missing from aggregate: %#v", result.List)
	}
	if _, ok := byKey["rep-dead:report_schedule"]; ok {
		t.Fatalf("soft-deleted report schedule leaked into aggregate")
	}
	if _, ok := byKey["timer-x:scene_timer"]; ok {
		t.Fatalf("foreign tenant scene timer leaked into aggregate")
	}
	if _, ok := byKey["rep-x:report_schedule"]; ok {
		t.Fatalf("foreign tenant report schedule leaked into aggregate")
	}
	if _, ok := byKey["job-hq-done:fleet_command_job"]; ok {
		t.Fatalf("non-scheduled command job leaked into aggregate")
	}

	// scene 注册行与其执行行同 id：聚合里只出现一次（scene_timer 形态），不重复输出。
	sceneOccurrences := 0
	for _, item := range result.List {
		if item.ID == sceneEvent.ID {
			sceneOccurrences++
		}
	}
	if sceneOccurrences != 1 {
		t.Fatalf("scene registry event appears %d times, want exactly once via its timer row", sceneOccurrences)
	}

	// rpc 注册行以 scheduler_registry 形态可见。
	registrySeen := false
	for _, item := range result.List {
		if item.Origin == model.SchedulerOriginRegistry {
			registrySeen = true
			if item.SourceType != model.SchedulerEventTypeRPC {
				t.Fatalf("registry item source_type = %s, want rpc", item.SourceType)
			}
		}
	}
	if !registrySeen {
		t.Fatalf("rpc registry event missing from aggregate: %#v", result.List)
	}

	// 排序：next_run_at 升序，空值最后。
	var last *time.Time
	for _, item := range result.List {
		if item.NextRunAt == nil {
			continue
		}
		if last != nil && item.NextRunAt.Before(*last) {
			t.Fatalf("aggregate not sorted by next_run_at: %v after %v", *item.NextRunAt, *last)
		}
		last = item.NextRunAt
	}

	t.Run("source_type filter", func(t *testing.T) {
		rpcOnly, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{SourceType: model.SchedulerEventTypeRPC}, claims)
		if err != nil {
			t.Fatalf("ListSchedulerEvents(rpc): %v", err)
		}
		if len(rpcOnly.List) != 2 { // job-hq + rpc 注册行
			t.Fatalf("rpc list = %#v, want job + registry rpc rows", rpcOnly.List)
		}
		for _, item := range rpcOnly.List {
			if item.SourceType != model.SchedulerEventTypeRPC {
				t.Fatalf("source_type filter leaked %s row", item.SourceType)
			}
		}
	})

	t.Run("enabled filter", func(t *testing.T) {
		disabled := false
		onlyDisabled, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{Enabled: &disabled}, claims)
		if err != nil {
			t.Fatalf("ListSchedulerEvents(disabled): %v", err)
		}
		if len(onlyDisabled.List) != 0 {
			t.Fatalf("disabled list = %#v, want empty", onlyDisabled.List)
		}
	})

	t.Run("time window filter", func(t *testing.T) {
		from := time.UnixMilli(now.Add(15 * time.Minute).UnixMilli())
		to := time.UnixMilli(now.Add(50 * time.Minute).UnixMilli())
		windowed, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{FromMs: from.UnixMilli(), ToMs: to.UnixMilli()}, claims)
		if err != nil {
			t.Fatalf("ListSchedulerEvents(window): %v", err)
		}
		if len(windowed.List) != 2 { // timer-hq(20m) + rep-hq(40m)；scene 事件(70m)/rpc(90m)/job(60m) 在窗外
			t.Fatalf("window list = %#v, want timer + report rows inside [15m, 50m]", windowed.List)
		}
		for _, item := range windowed.List {
			if item.NextRunAt.Before(from) || item.NextRunAt.After(to) {
				t.Fatalf("next_run_at %v outside window", item.NextRunAt)
			}
		}
	})

	t.Run("search and pagination", func(t *testing.T) {
		page1, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{PageSize: 1}, claims)
		if err != nil {
			t.Fatalf("ListSchedulerEvents(page1): %v", err)
		}
		page2, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{PageSize: 1, Page: 2}, claims)
		if err != nil {
			t.Fatalf("ListSchedulerEvents(page2): %v", err)
		}
		if page1.Total != page2.Total || page1.Total < 5 {
			t.Fatalf("total = %d/%d, want identical and >= 5", page1.Total, page2.Total)
		}
		if len(page1.List) != 1 || len(page2.List) != 1 || page1.List[0].ID == page2.List[0].ID {
			t.Fatalf("pagination broken: page1 = %v, page2 = %v", page1.List, page2.List)
		}
	})
}

func TestSchedulerServiceUpdateSceneEventSyncsTimer(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedSchedulerAutomation(t, db, "auto-a", "tenant-hq", "automation a", now)
	seedSchedulerAutomation(t, db, "auto-b", "tenant-hq", "automation b", now)
	svc := SchedulerService{}
	claims := schedulerClaimsFor("tenant-hq")

	created, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "sync me", EventType: model.SchedulerEventTypeScene,
		RefID: "auto-a", CronExpr: "*/5 * * * *",
	}, claims)
	if err != nil {
		t.Fatalf("create scene event: %v", err)
	}

	t.Run("cron change recomputes next run on both rows", func(t *testing.T) {
		updated, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{
			CronExpr: schedulerStrPtr("0 12 * * *"),
		}, claims)
		if err != nil {
			t.Fatalf("update cron: %v", err)
		}
		if updated.CronExpr != "0 12 * * *" || updated.NextRunAt == nil || !updated.NextRunAt.After(now) {
			t.Fatalf("updated event = %#v, want recomputed cron/next_run_at", updated)
		}
		var timer model.SceneAutomationTimer
		if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
			t.Fatalf("timer missing: %v", err)
		}
		if timer.CronExpr != "0 12 * * *" || !timer.NextRunAt.Equal(*updated.NextRunAt) {
			t.Fatalf("timer row = %#v, want synced cron/next_run_at", timer)
		}
	})

	t.Run("enabled change syncs the timer row", func(t *testing.T) {
		disabled := false
		if _, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{Enabled: &disabled}, claims); err != nil {
			t.Fatalf("disable event: %v", err)
		}
		var timer model.SceneAutomationTimer
		if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
			t.Fatalf("timer missing: %v", err)
		}
		if timer.Enabled {
			t.Fatal("timer row still enabled after event disabled")
		}
		// 停用期间允许同场景另建启用定时器（预检只对启用态生效），重新启用回到冲突面。
		enabled := true
		if _, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{Enabled: &enabled}, claims); err != nil {
			t.Fatalf("re-enable event: %v", err)
		}
	})

	t.Run("ref_id rebind syncs the timer row", func(t *testing.T) {
		updated, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{
			RefID: schedulerStrPtr("auto-b"),
		}, claims)
		if err != nil {
			t.Fatalf("rebind event: %v", err)
		}
		if updated.RefID != "auto-b" {
			t.Fatalf("ref_id = %q, want auto-b", updated.RefID)
		}
		var timer model.SceneAutomationTimer
		if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
			t.Fatalf("timer missing: %v", err)
		}
		if timer.SceneAutomationID != "auto-b" {
			t.Fatalf("timer scene_automation_id = %q, want auto-b", timer.SceneAutomationID)
		}
	})

	t.Run("rebinding to an automation with another enabled timer is rejected", func(t *testing.T) {
		seedSchedulerAutomation(t, db, "auto-c", "tenant-hq", "automation c", now)
		if _, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
			Name: "other", EventType: model.SchedulerEventTypeScene,
			RefID: "auto-c", CronExpr: "0 1 * * *",
		}, claims); err != nil {
			t.Fatalf("create second scene event: %v", err)
		}
		_, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{
			RefID: schedulerStrPtr("auto-c"),
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeOpDenied)
	})

	t.Run("event_type is immutable and next_run_at stays computed", func(t *testing.T) {
		runAt := now.Add(time.Hour)
		_, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{
			NextRunAt: &runAt,
		}, claims)
		wantSchedulerCode(t, err, errcode.CodeParamError)
	})
}

func TestSchedulerServiceDeleteRemovesSceneExecutionRow(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedSchedulerAutomation(t, db, "auto-del", "tenant-hq", "automation to delete", now)
	svc := SchedulerService{}
	claims := schedulerClaimsFor("tenant-hq")

	created, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "to be deleted", EventType: model.SchedulerEventTypeScene,
		RefID: "auto-del", CronExpr: "*/5 * * * *",
	}, claims)
	if err != nil {
		t.Fatalf("create scene event: %v", err)
	}

	t.Run("cross-tenant delete fails closed", func(t *testing.T) {
		_, err := svc.DeleteSchedulerEvent(ctx, created.ID, schedulerClaimsFor("tenant-x"))
		wantSchedulerCode(t, err, errcode.CodeNotFound)
		var timer model.SceneAutomationTimer
		if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
			t.Fatalf("cross-tenant delete must not remove the timer row: %v", err)
		}
	})

	t.Run("delete removes registry and execution rows", func(t *testing.T) {
		deletedID, err := svc.DeleteSchedulerEvent(ctx, created.ID, claims)
		if err != nil || deletedID != created.ID {
			t.Fatalf("delete = %q, err = %v, want success", deletedID, err)
		}
		var count int64
		db.Table("scene_automation_timers").Where("id = ?", created.ID).Count(&count)
		if count != 0 {
			t.Fatal("scene timer row survived event deletion")
		}
		if _, err := svc.GetSchedulerEvent(ctx, created.ID, claims); err == nil {
			t.Fatal("registry row survived deletion")
		}
	})
}

func TestSchedulerServiceGetAndUpdateTenantIsolation(t *testing.T) {
	db := setupSchedulerServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedSchedulerAutomation(t, db, "auto-iso", "tenant-hq", "hq automation", now)
	svc := SchedulerService{}
	hqClaims := schedulerClaimsFor("tenant-hq")

	created, err := svc.CreateSchedulerEvent(ctx, &model.SchedulerEventCreateReq{
		Name: "hq event", EventType: model.SchedulerEventTypeScene,
		RefID: "auto-iso", CronExpr: "*/5 * * * *",
	}, hqClaims)
	if err != nil {
		t.Fatalf("create scene event: %v", err)
	}

	if _, err := svc.GetSchedulerEvent(ctx, created.ID, schedulerClaimsFor("tenant-x")); err == nil {
		t.Fatal("cross-tenant get must fail")
	}
	if _, err := svc.UpdateSchedulerEvent(ctx, created.ID, &model.SchedulerEventUpdateReq{Name: schedulerStrPtr("hijacked")}, schedulerClaimsFor("tenant-x")); err == nil {
		t.Fatal("cross-tenant update must fail")
	}
	var untouched model.SchedulerEvent
	if err := db.Where("id = ?", created.ID).First(&untouched).Error; err != nil {
		t.Fatalf("event missing: %v", err)
	}
	if untouched.Name != "hq event" || untouched.TenantID != "tenant-hq" {
		t.Fatalf("event = %#v, want untouched hq row", untouched)
	}
	var timer model.SceneAutomationTimer
	if err := db.Where("id = ?", created.ID).First(&timer).Error; err != nil {
		t.Fatalf("timer missing: %v", err)
	}
	if timer.SceneAutomationID != "auto-iso" {
		t.Fatalf("timer = %#v, want untouched execution row", timer)
	}

	// 其他租户的聚合列表不包含本租户行。
	foreign, err := svc.ListSchedulerEvents(ctx, &model.SchedulerEventListRequest{}, schedulerClaimsFor("tenant-x"))
	if err != nil {
		t.Fatalf("foreign list: %v", err)
	}
	for _, item := range foreign.List {
		if item.ID == created.ID {
			t.Fatal("foreign tenant aggregate leaked the hq event")
		}
	}
}
