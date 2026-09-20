package dal

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const report83PostgresDSNEnv = "AETHERLINK_TEST_PSQL_DSN"

func TestReportMigration83Postgres(t *testing.T) {
	db := openReport83Postgres(t)

	t.Run("fresh schema constraints and vocabulary", func(t *testing.T) {
		assertReport83Schema(t, db)
		resetReport83Tables(t, db)
	})
	t.Run("explicit disabled schedule stays disabled", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83DisabledSchedule(t, db)
	})
	t.Run("materialization quarantines poison and uses latest missed slot", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83PoisonAndLatestSlot(t, db)
	})
	t.Run("competing materializers keep one scheduled slot", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83CompetingMaterializers(t, db)
	})
	t.Run("competing claimers have one owner", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83CompetingClaimers(t, db)
	})
	t.Run("stale generation fence cannot renew or settle", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83StaleGenerationFence(t, db)
	})
	t.Run("expired generation owner loses every settlement before reap", func(t *testing.T) {
		for _, settle := range []struct {
			name      string
			operation func(string, string) error
		}{
			{"fail", func(id, token string) error {
				return FailClaimedReportGeneration(context.Background(), id, token, "expired")
			}},
			{"retry", func(id, token string) error {
				return RetryClaimedReportGeneration(context.Background(), id, token, "expired", "expired")
			}},
			{"complete", func(id, token string) error {
				return CompleteReportGeneration(context.Background(), id, token, "sender@example.test", []string{"ops@example.test"}, "<"+id+"@example.test>", "report", []byte("payload"), 1)
			}},
		} {
			t.Run(settle.name, func(t *testing.T) {
				resetReport83Tables(t, db)
				testReport83ExpiredGenerationOwner(t, db, settle.operation)
			})
		}
	})
	t.Run("expired generation leases recover or reap", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83ExpiredGenerationLeases(t, db)
	})
	t.Run("persisted max attempts controls generation retry", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83PersistedAttemptPolicy(t, db)
	})
	t.Run("generation success and delivery insertion are atomic", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83GenerationOutboxAtomicity(t, db)
	})
	t.Run("composite tenant relations reject mismatches", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83CompositeTenantIntegrity(t, db)
	})
	t.Run("soft delete rejects active work and retains terminal history", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83SoftDeleteLifecycle(t, db)
	})
	t.Run("soft delete rejects active delivery", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83SoftDeleteActiveDelivery(t, db)
	})
	t.Run("soft delete retains terminal delivery histories", func(t *testing.T) {
		for _, status := range []string{
			model.ReportDeliveryStatusAccepted,
			model.ReportDeliveryStatusFailed,
			model.ReportDeliveryStatusAmbiguous,
		} {
			t.Run(status, func(t *testing.T) {
				resetReport83Tables(t, db)
				testReport83SoftDeleteTerminalDelivery(t, db, status)
			})
		}
	})
	t.Run("expired processing delivery becomes ambiguous", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83ExpiredDeliveryAmbiguous(t, db)
	})
	// P1：生成失败必须推进计划汇总，否则计划会一直对外宣称上一次运行的结果。
	t.Run("generation failure advances schedule summary", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83FailureAdvancesScheduleSummary(t, db)
	})
	t.Run("expired delivery owner loses every settlement before reap", func(t *testing.T) {
		for _, settle := range []struct {
			name      string
			operation func(string, string) error
		}{
			{"renew", func(id, token string) error {
				_, err := RenewReportDeliveryLease(context.Background(), id, token, time.Minute)
				return err
			}},
			{"accepted", func(id, token string) error { return SettleReportDeliveryAccepted(context.Background(), id, token) }},
			{"retry", func(id, token string) error {
				return RetryClaimedReportDelivery(context.Background(), id, token, "expired")
			}},
			{"failed", func(id, token string) error {
				return SettleReportDeliveryFailed(context.Background(), id, token, "expired")
			}},
		} {
			t.Run(settle.name, func(t *testing.T) {
				resetReport83Tables(t, db)
				testReport83ExpiredDeliveryOwner(t, db, settle.operation)
			})
		}
	})
	t.Run("idempotency scope and fingerprint conflict", func(t *testing.T) {
		resetReport83Tables(t, db)
		testReport83Idempotency(t, db)
	})
}

func openReport83Postgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(report83PostgresDSNEnv))
	if dsn == "" {
		t.Skip(report83PostgresDSNEnv + " not set; migration-83 report tests require PostgreSQL")
	}
	configuration, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	schema := "report83_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	adminSQL, err := adminDB.DB()
	if err != nil {
		t.Fatalf("access PostgreSQL admin pool: %v", err)
	}
	if _, err := adminSQL.ExecContext(context.Background(), "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminSQL.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_ = adminSQL.Close()
	})
	if err := createReport83Prerequisites(adminSQL, quotedSchema); err != nil {
		t.Fatalf("create migration-83 prerequisites: %v", err)
	}
	if err := seedReport83LegacySchedule(adminSQL, quotedSchema); err != nil {
		t.Fatalf("seed migration-78 report schedule: %v", err)
	}
	migration, err := os.ReadFile(report83MigrationPath(t))
	if err != nil {
		t.Fatalf("read migration 83: %v", err)
	}
	rewritten := strings.ReplaceAll(string(migration), "public.", quotedSchema+".")
	// 迁移里的 DO 块用字符串字面量 'public' 查询 information_schema 判定列类型，
	// 只替换 "public." 前缀不会命中这些字面量，导致转换分支在隔离 schema 下永不执行，
	// 前置表停留在 naive TIMESTAMP，进而让模式断言失败——这是测试隔离缺陷，不是产品缺陷。
	rewritten = strings.ReplaceAll(rewritten, "'public'", "'"+schema+"'")
	if _, err := adminSQL.ExecContext(context.Background(), rewritten); err != nil {
		t.Fatalf("apply migration 83 in isolated schema: %v", err)
	}
	configuration.RuntimeParams["search_path"] = schema
	// stdlib.RegisterConnConfig 返回的是 database/sql 的注册名（如 registeredConnConfig0），
	// gorm 的 postgres.Open 不认这个名字，会把它当普通 DSN 解析并失败——
	// 这导致本测试在 DSN 未配置时只是 Skip，一旦真正配置就必然在连接阶段报错，
	// 等于从未产生过任何真实 PostgreSQL 证据。
	// 正确做法：用 stdlib.OpenDB 拿到 *sql.DB 再交给 postgres.New，search_path 才会生效。
	schemaDB := stdlib.OpenDB(*configuration)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: schemaDB}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open schema-scoped PostgreSQL connection: %v", err)
	}
	previous := global.DB
	global.DB = db
	t.Cleanup(func() {
		global.DB = previous
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createReport83Prerequisites(db *sql.DB, schema string) error {
	statements := []string{
		"CREATE TABLE " + schema + ".report_schedules (id varchar(36) PRIMARY KEY, tenant_id varchar(36) NOT NULL, name varchar(128) NOT NULL, cron_expr varchar(128) NOT NULL, recipients text NOT NULL, device_ids jsonb, keys jsonb, lookback_hours integer NOT NULL DEFAULT 24, format varchar(16) NOT NULL DEFAULT 'csv', enabled boolean NOT NULL DEFAULT true, last_run_at timestamp, last_status varchar(64), created_at timestamp NOT NULL DEFAULT now(), updated_at timestamp NOT NULL DEFAULT now())",
		"CREATE INDEX idx_report_schedules_tenant ON " + schema + ".report_schedules (tenant_id)",
		"CREATE INDEX idx_report_schedules_enabled ON " + schema + ".report_schedules (enabled)",
		"CREATE TABLE " + schema + ".casbin_rule (id bigserial PRIMARY KEY, ptype varchar(100), v0 varchar(100), v1 varchar(100), v2 varchar(100))",
		"CREATE TABLE " + schema + ".sys_ui_elements (id varchar(36) PRIMARY KEY, parent_id varchar(36), element_code varchar(255), element_type integer, orders integer, param1 text, param2 text, param3 text, authority jsonb, description text, created_at timestamptz, remark text, multilingual text, route_path text)",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(context.Background(), statement); err != nil {
			return err
		}
	}
	return nil
}

func seedReport83LegacySchedule(db *sql.DB, schema string) error {
	_, err := db.ExecContext(context.Background(), "INSERT INTO "+schema+".report_schedules (id, tenant_id, name, cron_expr, recipients, device_ids, keys, enabled, last_run_at, last_status, created_at, updated_at) VALUES ('legacy-schedule', 'legacy-tenant', 'legacy', '0 * * * *', 'ops@example.test', '[\"device-1\"]', '[\"temperature\"]', false, TIMESTAMP '2026-01-02 03:04:05', NULL, TIMESTAMP '2026-01-01 01:02:03', TIMESTAMP '2026-01-01 02:03:04')")
	return err
}

func report83MigrationPath(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration-83 test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "sql", "83.sql"))
}

func resetReport83Tables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("TRUNCATE report_schedule_deliveries, report_schedule_runs, report_schedules RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("reset migration-83 report tables: %v", err)
	}
}

func assertReport83Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	var legacy struct {
		Enabled    bool
		Timezone   string
		NextRunAt  *time.Time
		LastRunAt  *time.Time
		CreatedAt  time.Time
		UpdatedAt  time.Time
		LastStatus *string
	}
	if err := db.Table(model.TableNameReportSchedule).Where("id = ?", "legacy-schedule").Take(&legacy).Error; err != nil {
		t.Fatalf("load migration-78 fixture after migration 83: %v", err)
	}
	if legacy.Enabled || legacy.Timezone != "UTC" || legacy.NextRunAt != nil || legacy.LastRunAt == nil || !legacy.LastRunAt.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) || legacy.LastStatus != nil {
		t.Fatalf("legacy schedule conversion = enabled %v timezone %q next %v last %v status %v", legacy.Enabled, legacy.Timezone, legacy.NextRunAt, legacy.LastRunAt, legacy.LastStatus)
	}
	for column, wantType := range map[string]string{"last_run_at": "timestamp with time zone", "created_at": "timestamp with time zone", "updated_at": "timestamp with time zone"} {
		var dataType string
		if err := db.Raw("SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'report_schedules' AND column_name = ?", column).Scan(&dataType).Error; err != nil {
			t.Fatalf("read report_schedules.%s type: %v", column, err)
		}
		if dataType != wantType {
			t.Errorf("report_schedules.%s type = %q, want %q", column, dataType, wantType)
		}
	}
	requiredColumns := map[string][]string{
		"report_schedules":           {"timezone", "next_run_at", "revision", "last_run_id", "schedule_error_code", "deleted_at"},
		"report_schedule_runs":       {"scheduled_slot", "idempotency_key_hash", "request_fingerprint", "generation_status", "attempt_count", "max_attempts", "next_attempt_at", "claim_token", "lease_until", "result"},
		"report_schedule_deliveries": {"envelope_from", "envelope_recipients", "message_id", "payload", "payload_digest", "payload_size", "row_count", "status", "attempt_count", "max_attempts", "next_attempt_at", "claim_token", "lease_until"},
	}
	for table, columns := range requiredColumns {
		var found []string
		if err := db.Raw("SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ?", table).Scan(&found).Error; err != nil {
			t.Fatalf("read %s columns: %v", table, err)
		}
		set := make(map[string]bool, len(found))
		for _, column := range found {
			set[column] = true
		}
		for _, column := range columns {
			if !set[column] {
				t.Errorf("migration 83 missing %s.%s", table, column)
			}
		}
	}
	assertReport83ConstraintAccepts(t, db, "report_schedule_runs", "generation_status", []string{
		model.ReportGenerationStatusPending, model.ReportGenerationStatusProcessing, model.ReportGenerationStatusRetrying,
		model.ReportGenerationStatusSucceeded, model.ReportGenerationStatusFailed,
	})
	assertReport83ConstraintAccepts(t, db, "report_schedule_deliveries", "status", []string{
		model.ReportDeliveryStatusPending, model.ReportDeliveryStatusProcessing, model.ReportDeliveryStatusRetrying,
		model.ReportDeliveryStatusAccepted, model.ReportDeliveryStatusFailed, model.ReportDeliveryStatusAmbiguous,
	})
	assertReport83ConstraintAccepts(t, db, "report_schedules", "schedule_error_code", []string{"invalid_schedule"})
	var scheduleIDNullable string
	if err := db.Raw("SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'report_schedule_runs' AND column_name = 'schedule_id'").Scan(&scheduleIDNullable).Error; err != nil {
		t.Fatalf("read report_schedule_runs.schedule_id nullability: %v", err)
	}
	if scheduleIDNullable != "NO" {
		t.Errorf("report_schedule_runs.schedule_id is_nullable = %q, want NO", scheduleIDNullable)
	}
	for _, constraint := range []string{
		"report_schedule_runs_schedule_fk",
		"report_schedule_runs_retry_parent_fk",
		"report_schedule_deliveries_run_fk",
		"report_schedules_last_run_fk",
	} {
		var count int64
		if err := db.Raw(`SELECT count(*) FROM pg_constraint c JOIN pg_class r ON r.oid = c.conrelid JOIN pg_namespace n ON n.oid = r.relnamespace WHERE n.nspname = current_schema() AND c.conname = ? AND c.contype = 'f'`, constraint).Scan(&count).Error; err != nil {
			t.Fatalf("read foreign key %s: %v", constraint, err)
		}
		if count != 1 {
			t.Errorf("migration 83 foreign key %s count = %d, want 1", constraint, count)
		}
	}
	for _, index := range []string{"uq_report_schedule_runs_scheduled_slot", "uq_report_schedule_runs_idempotency", "uq_report_schedule_deliveries_message_id"} {
		var count int64
		if err := db.Raw("SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?", index).Scan(&count).Error; err != nil {
			t.Fatalf("read index %s: %v", index, err)
		}
		if count != 1 {
			t.Errorf("migration 83 index %s count = %d, want 1", index, count)
		}
	}
	for _, route := range []string{
		"api/v1/report/schedules/:id/runs",
		"api/v1/report/schedules/:id/runs/:run_id",
		"api/v1/report/schedules/:id/runs/:run_id/retry",
	} {
		var resourceCount, grantCount int64
		if err := db.Table("casbin_rule").Where("ptype = ? AND v0 = ? AND v1 = ?", "g2", route, route).Count(&resourceCount).Error; err != nil {
			t.Fatalf("count Casbin resource %s: %v", route, err)
		}
		if err := db.Table("casbin_rule").Where("ptype = ? AND v0 IN ? AND v1 = ? AND v2 = ?", "p", []string{"SYS_ADMIN", "TENANT_ADMIN"}, route, "allow").Count(&grantCount).Error; err != nil {
			t.Fatalf("count Casbin grants %s: %v", route, err)
		}
		if resourceCount != 1 || grantCount != 2 {
			t.Errorf("Casbin registration %s = resources %d grants %d, want 1/2", route, resourceCount, grantCount)
		}
	}
	var menuCount int64
	if err := db.Table("sys_ui_elements").Where("element_code = ? AND param1 = ? AND route_path = ? AND authority @> ?::jsonb", "visualization_report", "/visualization/report", "view.visualization_report", `["SYS_ADMIN","TENANT_ADMIN"]`).Count(&menuCount).Error; err != nil {
		t.Fatalf("count report workspace menu: %v", err)
	}
	if menuCount != 1 {
		t.Errorf("report workspace menu count = %d, want 1", menuCount)
	}
}

func assertReport83ConstraintAccepts(t *testing.T, db *gorm.DB, table, column string, values []string) {
	t.Helper()
	var definitions []string
	if err := db.Raw(`SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c JOIN pg_class r ON r.oid = c.conrelid JOIN pg_namespace n ON n.oid = r.relnamespace WHERE n.nspname = current_schema() AND r.relname = ? AND c.contype = 'c' AND pg_get_constraintdef(c.oid) LIKE ?`, table, "%"+column+"%").Scan(&definitions).Error; err != nil {
		t.Fatalf("read %s.%s constraint: %v", table, column, err)
	}
	definition := strings.Join(definitions, " ")
	for _, value := range values {
		if !strings.Contains(definition, "'"+value+"'") {
			t.Errorf("%s.%s constraint %q does not accept %q", table, column, definition, value)
		}
	}
}

func insertReport83Schedule(t *testing.T, db *gorm.DB, tenantID, scheduleID string, dueAt time.Time) *model.ReportSchedule {
	t.Helper()
	now := time.Now().UTC()
	schedule := &model.ReportSchedule{
		ID: scheduleID, TenantID: tenantID, Name: "migration-83 report", CronExpr: "* * * * *", Timezone: "UTC",
		Recipients: "ops@example.test", DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"},
		LookbackHours: 1, Format: "csv", Enabled: true, NextRunAt: &dueAt, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(schedule).Error; err != nil {
		t.Fatalf("insert report schedule: %v", err)
	}
	return schedule
}

func insertReport83Run(t *testing.T, db *gorm.DB, tenantID, scheduleID, runID, status string, attempts, maxAttempts int) *model.ReportScheduleRun {
	t.Helper()
	now := time.Now().UTC()
	nextAttempt := now.Add(-time.Minute)
	run := &model.ReportScheduleRun{
		ID: runID, TenantID: tenantID, ScheduleID: scheduleID, Trigger: model.ReportRunTriggerManual,
		WindowStartAt: now.Add(-time.Hour), WindowEndAt: now, ConfigSnapshot: model.ReportRunConfigSnapshot{
			ScheduleName: "migration-83 report", Recipients: "ops@example.test", DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"}, Format: "csv",
		}, GenerationStatus: status, AttemptCount: attempts, MaxAttempts: maxAttempts, NextAttemptAt: &nextAttempt, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("insert report run: %v", err)
	}
	return run
}

func insertTerminalReport83Run(t *testing.T, db *gorm.DB, tenantID, scheduleID, runID, status string, attempts, maxAttempts int) *model.ReportScheduleRun {
	t.Helper()
	now := time.Now().UTC()
	errorCode := "generation_failed"
	run := &model.ReportScheduleRun{
		ID: runID, TenantID: tenantID, ScheduleID: scheduleID, Trigger: model.ReportRunTriggerManual,
		WindowStartAt: now.Add(-time.Hour), WindowEndAt: now, ConfigSnapshot: model.ReportRunConfigSnapshot{
			ScheduleName: "migration-83 report", Recipients: "ops@example.test", DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"}, Format: "csv",
		}, GenerationStatus: status, AttemptCount: attempts, MaxAttempts: maxAttempts,
		ErrorCode: &errorCode, CreatedAt: now, CompletedAt: &now, UpdatedAt: now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("insert terminal report run: %v", err)
	}
	return run
}

func runConcurrently(count int, operation func() error) []error {
	start := make(chan struct{})
	errorsByWorker := make([]error, count)
	var wait sync.WaitGroup
	wait.Add(count)
	for index := 0; index < count; index++ {
		go func(worker int) {
			defer wait.Done()
			<-start
			errorsByWorker[worker] = operation()
		}(index)
	}
	close(start)
	wait.Wait()
	return errorsByWorker
}

func testReport83DisabledSchedule(t *testing.T, db *gorm.DB) {
	now := time.Now().UTC()
	schedule := &model.ReportSchedule{
		ID: uuid.NewString(), TenantID: "tenant-disabled", Name: "disabled", CronExpr: "* * * * *", Timezone: "UTC",
		Recipients: "ops@example.test", DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"},
		LookbackHours: 1, Format: "csv", Enabled: false, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(schedule).Error; err != nil {
		t.Fatalf("insert disabled schedule: %v", err)
	}
	var stored model.ReportSchedule
	if err := db.Where("id = ?", schedule.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load disabled schedule: %v", err)
	}
	if stored.Enabled || stored.NextRunAt != nil {
		t.Fatalf("disabled schedule persisted enabled=%v next=%v", stored.Enabled, stored.NextRunAt)
	}
	result, err := InitializeReportScheduleNextRuns(context.Background(), 10, func(_ *model.ReportSchedule, after time.Time) (time.Time, error) { return after.Add(time.Hour), nil })
	if err != nil || result.Initialized != 0 || result.Invalid != 0 {
		t.Fatalf("initialize disabled schedule = %#v, %v", result, err)
	}
}

func testReport83PoisonAndLatestSlot(t *testing.T, db *gorm.DB) {
	now := time.Now().UTC().Truncate(time.Second)
	poison := insertReport83Schedule(t, db, "tenant-materialize", uuid.NewString(), now.Add(-4*time.Hour))
	healthy := insertReport83Schedule(t, db, "tenant-materialize", uuid.NewString(), now.Add(-3*time.Hour))
	results, err := MaterializeDueReportScheduleRuns(context.Background(), 10, 3, func(schedule *model.ReportSchedule, after time.Time) (time.Time, error) {
		if schedule.ID == poison.ID {
			return time.Time{}, errors.New("invalid cadence")
		}
		return after.Add(time.Hour), nil
	})
	if err != nil || len(results) != 1 || results[0].Run.ScheduleID != healthy.ID {
		t.Fatalf("materialize poison and healthy = %#v, %v", results, err)
	}
	run := results[0].Run
	wantSlot := now
	if run.ScheduledSlot == nil || !run.ScheduledSlot.Equal(wantSlot) || !run.WindowEndAt.Equal(wantSlot) || run.MisfireCount != 3 || run.MisfireFirstSlot == nil || !run.MisfireFirstSlot.Equal(now.Add(-3*time.Hour)) || run.MisfireLastSlot == nil || !run.MisfireLastSlot.Equal(wantSlot) {
		t.Fatalf("coalesced run slot=%v window=%v misfires=%d first=%v last=%v", run.ScheduledSlot, run.WindowEndAt, run.MisfireCount, run.MisfireFirstSlot, run.MisfireLastSlot)
	}
	var quarantined model.ReportSchedule
	if err := db.Where("id = ?", poison.ID).Take(&quarantined).Error; err != nil {
		t.Fatalf("load poison schedule: %v", err)
	}
	if quarantined.Enabled || quarantined.NextRunAt != nil || quarantined.ScheduleErrorCode == nil || *quarantined.ScheduleErrorCode != "invalid_schedule" || quarantined.LastStatus != "" || quarantined.Revision != 2 {
		t.Fatalf("poison quarantine = enabled %v next %v schedule error %v last status %q revision %d", quarantined.Enabled, quarantined.NextRunAt, quarantined.ScheduleErrorCode, quarantined.LastStatus, quarantined.Revision)
	}
}

func testReport83CompetingMaterializers(t *testing.T, db *gorm.DB) {
	now := time.Now().UTC().Truncate(time.Second)
	schedule := insertReport83Schedule(t, db, "tenant-materialize", uuid.NewString(), now.Add(-time.Minute))
	next := func(_ *model.ReportSchedule, after time.Time) (time.Time, error) { return after.Add(time.Minute), nil }
	var resultCount int
	var lock sync.Mutex
	errs := runConcurrently(8, func() error {
		results, err := MaterializeDueReportScheduleRuns(context.Background(), 1, 3, next)
		lock.Lock()
		resultCount += len(results)
		lock.Unlock()
		return err
	})
	for _, err := range errs {
		if err != nil {
			t.Fatalf("competing materializer: %v", err)
		}
	}
	var count int64
	if err := db.Model(&model.ReportScheduleRun{}).Where("schedule_id = ? AND trigger = ?", schedule.ID, model.ReportRunTriggerScheduled).Count(&count).Error; err != nil {
		t.Fatalf("count materialized runs: %v", err)
	}
	if count != 1 || resultCount != 1 {
		t.Fatalf("materialized database/results count = %d/%d, want 1/1", count, resultCount)
	}
}

func testReport83CompetingClaimers(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-claim", scheduleID, time.Now().Add(time.Hour))
	run := insertReport83Run(t, db, "tenant-claim", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	claims := make([]ReportGenerationClaim, 0, 1)
	var lock sync.Mutex
	errs := runConcurrently(8, func() error {
		claimed, err := ClaimReportGenerations(context.Background(), 1, time.Minute)
		lock.Lock()
		claims = append(claims, claimed...)
		lock.Unlock()
		return err
	})
	for _, err := range errs {
		if err != nil {
			t.Fatalf("competing generation claimer: %v", err)
		}
	}
	if len(claims) != 1 || claims[0].Run.ID != run.ID || strings.TrimSpace(claims[0].Token) == "" {
		t.Fatalf("claims = %#v, want one owner for %s", claims, run.ID)
	}
	var stored model.ReportScheduleRun
	if err := db.Where("id = ?", run.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load claimed run: %v", err)
	}
	if stored.GenerationStatus != model.ReportGenerationStatusProcessing || stored.AttemptCount != 1 || stored.ClaimToken == nil || *stored.ClaimToken != claims[0].Token {
		t.Fatalf("stored claim = status %q attempts %d token %v", stored.GenerationStatus, stored.AttemptCount, stored.ClaimToken)
	}
}

func testReport83StaleGenerationFence(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-fence", scheduleID, time.Now().Add(time.Hour))
	run := insertReport83Run(t, db, "tenant-fence", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	first := requireOneReport83GenerationClaim(t, run.ID)
	if err := db.Model(&model.ReportScheduleRun{}).Where("id = ?", run.ID).Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire first generation lease: %v", err)
	}
	if reaped, err := ReapExpiredReportGenerations(context.Background(), "lease_expired"); err != nil || reaped != 1 {
		t.Fatalf("reap first generation claim = %d, %v", reaped, err)
	}
	if err := db.Model(&model.ReportScheduleRun{}).Where("id = ?", run.ID).Update("next_attempt_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("make retry due: %v", err)
	}
	second := requireOneReport83GenerationClaim(t, run.ID)
	if second.Token == first.Token {
		t.Fatal("reclaimed generation reused fencing token")
	}
	if _, err := RenewReportGenerationLease(context.Background(), run.ID, first.Token, time.Minute); !errors.Is(err, ErrReportClaimLost) {
		t.Fatalf("renew stale token error = %v, want ErrReportClaimLost", err)
	}
	if err := FailClaimedReportGeneration(context.Background(), run.ID, first.Token, "stale"); !errors.Is(err, ErrReportClaimLost) {
		t.Fatalf("settle stale token error = %v, want ErrReportClaimLost", err)
	}
	if err := FailClaimedReportGeneration(context.Background(), run.ID, second.Token, "current"); err != nil {
		t.Fatalf("settle current token: %v", err)
	}
}

func requireOneReport83GenerationClaim(t *testing.T, runID string) ReportGenerationClaim {
	t.Helper()
	claims, err := ClaimReportGenerations(context.Background(), 1, time.Minute)
	if err != nil {
		t.Fatalf("claim generation: %v", err)
	}
	if len(claims) != 1 || claims[0].Run.ID != runID {
		t.Fatalf("generation claims = %#v, want run %s", claims, runID)
	}
	return claims[0]
}

func testReport83ExpiredGenerationOwner(t *testing.T, db *gorm.DB, settle func(string, string) error) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-expired-owner", scheduleID, time.Now().Add(time.Hour))
	run := insertReport83Run(t, db, "tenant-expired-owner", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	claim := requireOneReport83GenerationClaim(t, run.ID)
	if err := db.Model(&model.ReportScheduleRun{}).Where("id = ?", run.ID).Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire generation owner: %v", err)
	}
	if err := settle(run.ID, claim.Token); !errors.Is(err, ErrReportClaimLost) {
		t.Fatalf("expired generation settlement error = %v, want ErrReportClaimLost", err)
	}
	var stored model.ReportScheduleRun
	if err := db.Where("id = ?", run.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load expired generation owner: %v", err)
	}
	if stored.GenerationStatus != model.ReportGenerationStatusProcessing || stored.ClaimToken == nil || *stored.ClaimToken != claim.Token {
		t.Fatalf("expired generation mutated = status %q token %v", stored.GenerationStatus, stored.ClaimToken)
	}
}

func testReport83ExpiredGenerationLeases(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-reap", scheduleID, time.Now().Add(time.Hour))
	nonFinal := insertReport83Run(t, db, "tenant-reap", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	final := insertReport83Run(t, db, "tenant-reap", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 2, 3)
	claims, err := ClaimReportGenerations(context.Background(), 2, time.Minute)
	if err != nil || len(claims) != 2 {
		t.Fatalf("claim expiring generations = %d, %v", len(claims), err)
	}
	if err := db.Model(&model.ReportScheduleRun{}).Where("id IN ?", []string{nonFinal.ID, final.ID}).Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire generation leases: %v", err)
	}
	reaped, err := ReapExpiredReportGenerations(context.Background(), "lease_expired")
	if err != nil || reaped != 2 {
		t.Fatalf("reap expired generations = %d, %v", reaped, err)
	}
	var recovered, failed model.ReportScheduleRun
	if err := db.Where("id = ?", nonFinal.ID).Take(&recovered).Error; err != nil {
		t.Fatalf("load recovered generation: %v", err)
	}
	if err := db.Where("id = ?", final.ID).Take(&failed).Error; err != nil {
		t.Fatalf("load failed generation: %v", err)
	}
	if recovered.GenerationStatus != model.ReportGenerationStatusRetrying || recovered.NextAttemptAt == nil || recovered.CompletedAt != nil || recovered.ClaimToken != nil {
		t.Fatalf("non-final expired generation = status %q next %v completed %v token %v", recovered.GenerationStatus, recovered.NextAttemptAt, recovered.CompletedAt, recovered.ClaimToken)
	}
	if failed.GenerationStatus != model.ReportGenerationStatusFailed || failed.CompletedAt == nil || failed.NextAttemptAt != nil || failed.ClaimToken != nil {
		t.Fatalf("final expired generation = status %q next %v completed %v token %v", failed.GenerationStatus, failed.NextAttemptAt, failed.CompletedAt, failed.ClaimToken)
	}
}

func testReport83PersistedAttemptPolicy(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-attempt-policy", scheduleID, time.Now().Add(time.Hour))
	terminal := insertReport83Run(t, db, "tenant-attempt-policy", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 1)
	retryable := insertReport83Run(t, db, "tenant-attempt-policy", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 4)
	claims, err := ClaimReportGenerations(context.Background(), 2, time.Minute)
	if err != nil || len(claims) != 2 {
		t.Fatalf("claim persisted-attempt runs = %d, %v", len(claims), err)
	}
	byID := make(map[string]ReportGenerationClaim, len(claims))
	for _, claim := range claims {
		byID[claim.Run.ID] = claim
	}
	for _, runID := range []string{terminal.ID, retryable.ID} {
		claim, ok := byID[runID]
		if !ok {
			t.Fatalf("run %s was not claimed: %#v", runID, claims)
		}
		if err := RetryClaimedReportGeneration(context.Background(), runID, claim.Token, "transient", "temporary failure"); err != nil {
			t.Fatalf("settle transient failure for %s: %v", runID, err)
		}
	}
	var terminalStored, retryableStored model.ReportScheduleRun
	if err := db.Where("id = ?", terminal.ID).Take(&terminalStored).Error; err != nil {
		t.Fatalf("load max-attempts=1 run: %v", err)
	}
	if err := db.Where("id = ?", retryable.ID).Take(&retryableStored).Error; err != nil {
		t.Fatalf("load max-attempts=4 run: %v", err)
	}
	if terminalStored.MaxAttempts != 1 || terminalStored.AttemptCount != 1 || terminalStored.GenerationStatus != model.ReportGenerationStatusFailed || terminalStored.CompletedAt == nil || terminalStored.NextAttemptAt != nil {
		t.Fatalf("max-attempts=1 settlement = max %d attempts %d status %q completed %v next %v", terminalStored.MaxAttempts, terminalStored.AttemptCount, terminalStored.GenerationStatus, terminalStored.CompletedAt, terminalStored.NextAttemptAt)
	}
	if retryableStored.MaxAttempts != 4 || retryableStored.AttemptCount != 1 || retryableStored.GenerationStatus != model.ReportGenerationStatusRetrying || retryableStored.CompletedAt != nil || retryableStored.NextAttemptAt == nil {
		t.Fatalf("max-attempts=4 settlement = max %d attempts %d status %q completed %v next %v", retryableStored.MaxAttempts, retryableStored.AttemptCount, retryableStored.GenerationStatus, retryableStored.CompletedAt, retryableStored.NextAttemptAt)
	}
	if err := db.Model(&model.ReportScheduleRun{}).Where("id = ?", retryable.ID).Update("next_attempt_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("make persisted-policy retry due: %v", err)
	}
	reclaimed := requireOneReport83GenerationClaim(t, retryable.ID)
	if reclaimed.Run.MaxAttempts != 4 || reclaimed.Run.AttemptCount != 2 {
		t.Fatalf("reclaimed persisted policy = max %d attempts %d", reclaimed.Run.MaxAttempts, reclaimed.Run.AttemptCount)
	}
}

// testReport83FailureAdvancesScheduleSummary 覆盖 P1：
// 生成失败必须把计划汇总推进到 failed。修复前 last_run_id 只由生成成功写入，
// 于是 IfCurrent 守卫会静默 no-op，计划会一直对外宣称上一次运行的 succeeded。
func testReport83FailureAdvancesScheduleSummary(t *testing.T, db *gorm.DB) {
	tenantID := "tenant-fail-summary"
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, tenantID, scheduleID, time.Now().Add(time.Hour))

	// 先制造"上一次运行成功"的历史，让计划汇总停在 succeeded。
	if err := db.Model(&model.ReportSchedule{}).Where("id = ?", scheduleID).
		Updates(map[string]interface{}{"last_status": model.ReportProjectedStatusSucceeded}).Error; err != nil {
		t.Fatalf("seed schedule summary: %v", err)
	}

	run := insertReport83Run(t, db, tenantID, scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 1)
	claimed, err := ClaimReportGenerations(context.Background(), 1, time.Minute)
	if err != nil {
		t.Fatalf("claim generation: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Run.ID != run.ID {
		t.Fatalf("claimed = %#v, want one owner for %s", claimed, run.ID)
	}
	if err := FailClaimedReportGeneration(context.Background(), claimed[0].Run.ID, claimed[0].Token, "boom"); err != nil {
		t.Fatalf("fail generation: %v", err)
	}

	var schedule model.ReportSchedule
	if err := db.Where("id = ?", scheduleID).Take(&schedule).Error; err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if schedule.LastStatus != model.ReportProjectedStatusFailed {
		t.Errorf("last_status = %q, want %q：生成失败后计划不得继续宣称上一次运行的结果",
			schedule.LastStatus, model.ReportProjectedStatusFailed)
	}
	if schedule.LastRunID == nil || *schedule.LastRunID != run.ID {
		t.Errorf("last_run_id = %v, want %s", schedule.LastRunID, run.ID)
	}
}

func testReport83GenerationOutboxAtomicity(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-atomic", scheduleID, time.Now().Add(time.Hour))
	firstRun := insertReport83Run(t, db, "tenant-atomic", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	firstClaim := requireOneReport83GenerationClaim(t, firstRun.ID)
	messageID := "<atomic@example.test>"
	if err := CompleteReportGeneration(context.Background(), firstRun.ID, firstClaim.Token, "sender@example.test", []string{"ops@example.test"}, messageID, "report", []byte("first"), 1); err != nil {
		t.Fatalf("complete first generation: %v", err)
	}
	secondRun := insertReport83Run(t, db, "tenant-atomic", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	secondClaim := requireOneReport83GenerationClaim(t, secondRun.ID)
	if err := CompleteReportGeneration(context.Background(), secondRun.ID, secondClaim.Token, "sender@example.test", []string{"ops@example.test"}, messageID, "report", []byte("second"), 1); err == nil {
		t.Fatal("duplicate message ID unexpectedly completed generation")
	}
	var stored model.ReportScheduleRun
	if err := db.Where("id = ?", secondRun.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load rolled-back generation: %v", err)
	}
	if stored.GenerationStatus != model.ReportGenerationStatusProcessing || stored.ClaimToken == nil || *stored.ClaimToken != secondClaim.Token || stored.CompletedAt != nil {
		t.Fatalf("generation update did not roll back: status %q token %v completed %v", stored.GenerationStatus, stored.ClaimToken, stored.CompletedAt)
	}
	var deliveries int64
	if err := db.Model(&model.ReportScheduleDelivery{}).Count(&deliveries).Error; err != nil {
		t.Fatalf("count atomic outbox rows: %v", err)
	}
	if deliveries != 1 {
		t.Fatalf("delivery count = %d, want 1", deliveries)
	}
}

func testReport83CompositeTenantIntegrity(t *testing.T, db *gorm.DB) {
	schedule := insertReport83Schedule(t, db, "tenant-a", uuid.NewString(), time.Now().Add(time.Hour))
	now := time.Now().UTC()
	invalid := &model.ReportScheduleRun{
		ID: uuid.NewString(), TenantID: "tenant-b", ScheduleID: schedule.ID, Trigger: model.ReportRunTriggerManual,
		WindowStartAt: now.Add(-time.Hour), WindowEndAt: now,
		ConfigSnapshot:   model.ReportRunConfigSnapshot{ScheduleName: "invalid", Recipients: "ops@example.test", Format: "csv"},
		GenerationStatus: model.ReportGenerationStatusPending, MaxAttempts: 1, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(invalid).Error; err == nil {
		t.Fatal("cross-tenant schedule run unexpectedly satisfied composite foreign key")
	}
	run := insertReport83Run(t, db, schedule.TenantID, schedule.ID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 1)
	claim := requireOneReport83GenerationClaim(t, run.ID)
	if err := CompleteReportGeneration(context.Background(), run.ID, claim.Token, "sender@example.test", []string{"ops@example.test"}, "<integrity@example.test>", "report", []byte("payload"), 1); err != nil {
		t.Fatalf("create valid delivery before tenant mismatch: %v", err)
	}
	if err := db.Model(&model.ReportScheduleDelivery{}).Where("run_id = ?", run.ID).Update("tenant_id", "tenant-b").Error; err == nil {
		t.Fatal("cross-tenant delivery update unexpectedly satisfied composite foreign key")
	}
}

func testReport83SoftDeleteLifecycle(t *testing.T, db *gorm.DB) {
	schedule := insertReport83Schedule(t, db, "tenant-delete", uuid.NewString(), time.Now().Add(time.Hour))
	active := insertReport83Run(t, db, schedule.TenantID, schedule.ID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 1)
	if err := SoftDeleteReportScheduleContext(context.Background(), schedule.ID, schedule.TenantID, schedule.Revision); !errors.Is(err, ErrReportScheduleHasActiveWork) {
		t.Fatalf("delete active generation error = %v, want ErrReportScheduleHasActiveWork", err)
	}
	claim := requireOneReport83GenerationClaim(t, active.ID)
	if err := db.Model(&model.ReportSchedule{}).Where("id = ?", schedule.ID).Updates(map[string]interface{}{"last_run_id": active.ID, "last_run_at": active.WindowEndAt}).Error; err != nil {
		t.Fatalf("set active schedule summary: %v", err)
	}
	if err := FailClaimedReportGeneration(context.Background(), active.ID, claim.Token, "terminal"); err != nil {
		t.Fatalf("terminalize active generation: %v", err)
	}
	if err := SoftDeleteReportScheduleContext(context.Background(), schedule.ID, schedule.TenantID, schedule.Revision); err != nil {
		t.Fatalf("soft delete terminal schedule: %v", err)
	}
	var stored model.ReportSchedule
	if err := db.Unscoped().Where("id = ?", schedule.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load soft-deleted schedule: %v", err)
	}
	if stored.DeletedAt == nil || stored.Enabled || stored.NextRunAt != nil {
		t.Fatalf("soft-deleted schedule = deleted %v enabled %v next %v", stored.DeletedAt, stored.Enabled, stored.NextRunAt)
	}
	var history int64
	if err := db.Model(&model.ReportScheduleRun{}).Where("schedule_id = ?", schedule.ID).Count(&history).Error; err != nil || history != 1 {
		t.Fatalf("terminal history count = %d, %v", history, err)
	}
}

func createReport83Delivery(t *testing.T, db *gorm.DB, schedule *model.ReportSchedule, maxAttempts int) (*model.ReportScheduleRun, ReportDeliveryClaim) {
	t.Helper()
	run := insertReport83Run(t, db, schedule.TenantID, schedule.ID, uuid.NewString(), model.ReportGenerationStatusPending, 0, maxAttempts)
	claim := requireOneReport83GenerationClaim(t, run.ID)
	if err := CompleteReportGeneration(context.Background(), run.ID, claim.Token, "sender@example.test", []string{"ops@example.test"}, "<"+run.ID+"@example.test>", "report", []byte("payload"), 1); err != nil {
		t.Fatalf("complete generation before delivery lifecycle: %v", err)
	}
	claims, err := ClaimReportDeliveries(context.Background(), 1, time.Minute)
	if err != nil || len(claims) != 1 || claims[0].Run.ID != run.ID {
		t.Fatalf("claim delivery lifecycle = %#v, %v", claims, err)
	}
	return run, claims[0]
}

func testReport83SoftDeleteActiveDelivery(t *testing.T, db *gorm.DB) {
	schedule := insertReport83Schedule(t, db, "tenant-delete-delivery", uuid.NewString(), time.Now().Add(time.Hour))
	_, _ = createReport83Delivery(t, db, schedule, 3)
	if err := SoftDeleteReportScheduleContext(context.Background(), schedule.ID, schedule.TenantID, schedule.Revision); !errors.Is(err, ErrReportScheduleHasActiveWork) {
		t.Fatalf("delete active delivery error = %v, want ErrReportScheduleHasActiveWork", err)
	}
}

func testReport83SoftDeleteTerminalDelivery(t *testing.T, db *gorm.DB, status string) {
	schedule := insertReport83Schedule(t, db, "tenant-delete-"+status, uuid.NewString(), time.Now().Add(time.Hour))
	run, claim := createReport83Delivery(t, db, schedule, 3)
	var err error
	switch status {
	case model.ReportDeliveryStatusAccepted:
		err = SettleReportDeliveryAccepted(context.Background(), run.ID, claim.Token)
	case model.ReportDeliveryStatusFailed:
		err = SettleReportDeliveryFailed(context.Background(), run.ID, claim.Token, "smtp_pre_accept_failure")
	case model.ReportDeliveryStatusAmbiguous:
		err = SettleReportDeliveryAmbiguous(context.Background(), run.ID, claim.Token, "smtp_acceptance_unknown")
	default:
		t.Fatalf("unsupported terminal delivery status %q", status)
	}
	if err != nil {
		t.Fatalf("settle %s delivery: %v", status, err)
	}
	if err := SoftDeleteReportScheduleContext(context.Background(), schedule.ID, schedule.TenantID, schedule.Revision); err != nil {
		t.Fatalf("soft delete schedule with %s delivery: %v", status, err)
	}
	var storedRun model.ReportScheduleRun
	if err := db.Where("id = ?", run.ID).Take(&storedRun).Error; err != nil {
		t.Fatalf("load retained run for %s delivery: %v", status, err)
	}
	var storedDelivery model.ReportScheduleDelivery
	if err := db.Where("run_id = ?", run.ID).Take(&storedDelivery).Error; err != nil {
		t.Fatalf("load retained %s delivery: %v", status, err)
	}
	if storedRun.GenerationStatus != model.ReportGenerationStatusSucceeded || storedDelivery.Status != status || storedDelivery.CompletedAt == nil || storedDelivery.Payload != nil {
		t.Fatalf("retained %s history = generation %q delivery %q completed %v payload %v", status, storedRun.GenerationStatus, storedDelivery.Status, storedDelivery.CompletedAt, storedDelivery.Payload)
	}
}

func testReport83ExpiredDeliveryAmbiguous(t *testing.T, db *gorm.DB) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-delivery", scheduleID, time.Now().Add(time.Hour))
	run := insertReport83Run(t, db, "tenant-delivery", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	claim := requireOneReport83GenerationClaim(t, run.ID)
	if err := CompleteReportGeneration(context.Background(), run.ID, claim.Token, "sender@example.test", []string{"ops@example.test"}, "<ambiguous@example.test>", "report", []byte("payload"), 1); err != nil {
		t.Fatalf("complete generation before delivery claim: %v", err)
	}
	claims, err := ClaimReportDeliveries(context.Background(), 1, time.Minute)
	if err != nil || len(claims) != 1 || claims[0].Delivery.RunID != run.ID {
		t.Fatalf("claim delivery = %#v, %v", claims, err)
	}
	if err := db.Model(&model.ReportScheduleDelivery{}).Where("run_id = ?", run.ID).Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire delivery lease: %v", err)
	}
	marked, err := MarkExpiredReportDeliveriesAmbiguous(context.Background(), "smtp_acceptance_unknown")
	if err != nil || marked != 1 {
		t.Fatalf("mark expired delivery ambiguous = %d, %v", marked, err)
	}
	var delivery model.ReportScheduleDelivery
	if err := db.Where("run_id = ?", run.ID).Take(&delivery).Error; err != nil {
		t.Fatalf("load ambiguous delivery: %v", err)
	}
	if delivery.Status != model.ReportDeliveryStatusAmbiguous || delivery.AmbiguousAt == nil || delivery.CompletedAt == nil || delivery.ClaimToken != nil || delivery.LeaseUntil != nil || delivery.Payload != nil {
		t.Fatalf("expired delivery = status %q ambiguous %v completed %v token %v lease %v payload %v", delivery.Status, delivery.AmbiguousAt, delivery.CompletedAt, delivery.ClaimToken, delivery.LeaseUntil, delivery.Payload)
	}
}

func testReport83ExpiredDeliveryOwner(t *testing.T, db *gorm.DB, settle func(string, string) error) {
	scheduleID := uuid.NewString()
	insertReport83Schedule(t, db, "tenant-delivery-owner", scheduleID, time.Now().Add(time.Hour))
	run := insertReport83Run(t, db, "tenant-delivery-owner", scheduleID, uuid.NewString(), model.ReportGenerationStatusPending, 0, 3)
	claim := requireOneReport83GenerationClaim(t, run.ID)
	if err := CompleteReportGeneration(context.Background(), run.ID, claim.Token, "sender@example.test", []string{"ops@example.test"}, "<"+run.ID+"@example.test>", "report", []byte("payload"), 1); err != nil {
		t.Fatalf("complete generation before expired delivery owner: %v", err)
	}
	claims, err := ClaimReportDeliveries(context.Background(), 1, time.Minute)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim delivery owner = %#v, %v", claims, err)
	}
	if err := db.Model(&model.ReportScheduleDelivery{}).Where("run_id = ?", run.ID).Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire delivery owner: %v", err)
	}
	if err := settle(run.ID, claims[0].Token); !errors.Is(err, ErrReportClaimLost) {
		t.Fatalf("expired delivery settlement error = %v, want ErrReportClaimLost", err)
	}
	var stored model.ReportScheduleDelivery
	if err := db.Where("run_id = ?", run.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load expired delivery owner: %v", err)
	}
	if stored.Status != model.ReportDeliveryStatusProcessing || stored.ClaimToken == nil || *stored.ClaimToken != claims[0].Token || stored.Payload == nil {
		t.Fatalf("expired delivery mutated = status %q token %v payload %v", stored.Status, stored.ClaimToken, stored.Payload)
	}
}

func testReport83Idempotency(t *testing.T, db *gorm.DB) {
	scheduleOne := insertReport83Schedule(t, db, "tenant-one", uuid.NewString(), time.Now().Add(time.Hour))
	scheduleTwo := insertReport83Schedule(t, db, "tenant-one", uuid.NewString(), time.Now().Add(time.Hour))
	scheduleOtherTenant := insertReport83Schedule(t, db, "tenant-two", uuid.NewString(), time.Now().Add(time.Hour))
	fingerprint := []byte("same request")
	first, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, IdempotencyKey: "scope-key", RequestFingerprint: fingerprint,
	})
	if err != nil || first.IdempotentReplay {
		t.Fatalf("first idempotent submission = %#v, %v", first, err)
	}
	replay, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, IdempotencyKey: "scope-key", RequestFingerprint: fingerprint,
	})
	if err != nil || !replay.IdempotentReplay || replay.Run.ID != first.Run.ID {
		t.Fatalf("idempotent replay = %#v, %v; first run %s", replay, err, first.Run.ID)
	}
	if err := db.Model(&model.ReportSchedule{}).Where("id = ?", scheduleOne.ID).Updates(map[string]interface{}{"enabled": false, "deleted_at": time.Now(), "next_run_at": nil}).Error; err != nil {
		t.Fatalf("disable schedule before idempotent replay: %v", err)
	}
	replayAfterDelete, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, IdempotencyKey: "scope-key", RequestFingerprint: fingerprint,
	})
	if err != nil || !replayAfterDelete.IdempotentReplay || replayAfterDelete.Run.ID != first.Run.ID {
		t.Fatalf("idempotent replay after schedule deletion = %#v, %v", replayAfterDelete, err)
	}
	if _, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, IdempotencyKey: "new-key", RequestFingerprint: fingerprint,
	}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("new submission after schedule deletion error = %v, want not found", err)
	}
	if err := db.Model(&model.ReportSchedule{}).Where("id = ?", scheduleOne.ID).Updates(map[string]interface{}{"enabled": true, "deleted_at": nil}).Error; err != nil {
		t.Fatalf("restore schedule for retry scope test: %v", err)
	}
	if _, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, IdempotencyKey: "scope-key", RequestFingerprint: []byte("different request"),
	}); !errors.Is(err, ErrReportIdempotencyConflict) {
		t.Fatalf("incompatible fingerprint error = %v, want ErrReportIdempotencyConflict", err)
	}
	for _, schedule := range []*model.ReportSchedule{scheduleTwo, scheduleOtherTenant} {
		submission, err := SubmitManualReportRun(context.Background(), ManualReportRunInput{
			ID: uuid.NewString(), TenantID: schedule.TenantID, ScheduleID: schedule.ID, IdempotencyKey: "scope-key", RequestFingerprint: fingerprint,
		})
		if err != nil || submission.IdempotentReplay || submission.Run.ID == first.Run.ID {
			t.Fatalf("tenant/schedule-scoped submission for %s/%s = %#v, %v", schedule.TenantID, schedule.ID, submission, err)
		}
	}
	parentID := uuid.NewString()
	parent := insertTerminalReport83Run(t, db, scheduleOne.TenantID, scheduleOne.ID, parentID, model.ReportGenerationStatusFailed, 1, 3)
	retry, err := SubmitRetryReportRun(context.Background(), RetryReportRunInput{
		ID: uuid.NewString(), TenantID: scheduleOne.TenantID, ScheduleID: scheduleOne.ID, ParentRunID: parent.ID, IdempotencyKey: "scope-key", RequestFingerprint: fingerprint,
	})
	if err != nil || retry.IdempotentReplay || retry.Run.ID == first.Run.ID || retry.Run.Trigger != model.ReportRunTriggerRetry {
		t.Fatalf("trigger-scoped retry submission = %#v, %v", retry, err)
	}
	keyHash := reportHexHash(HashReportIdempotencyKey("scope-key"))
	var count int64
	if err := db.Model(&model.ReportScheduleRun{}).Where("idempotency_key_hash = ?", keyHash).Count(&count).Error; err != nil {
		t.Fatalf("count scoped idempotency rows: %v", err)
	}
	if count != 4 {
		t.Fatalf("idempotency rows = %d, want 4 tenant+schedule+trigger scopes", count)
	}
}
