// 文件用途：迁移 87（明细行进度回写）在真实 PostgreSQL 上的证据。
// sqlite 测不出 CHECK 约束与 timestamptz 比较语义，因此这几件事必须落到真库上：
//   1. 四列真的建出来了，且类型对（progress_at 必须是 timestamptz，不是裸 timestamp）；
//   2. 0..100 的 CHECK 真的会拒绝越界值——服务层校验不是唯一一道闸；
//   3. "只接受不早于当前 progress_at 的上报"这条守卫在真库的时区/比较语义下成立；
//   4. 租户隔离在真库上成立。
// 不设置 AETHERLINK_TEST_PSQL_DSN 时本测试如实地 Skip，不伪造通过。
package dal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func commandJobProgress87MigrationPath(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration-87 test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "sql", "87.sql"))
}

func TestCommandJobDetailProgressMigration87Postgres(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; migration-87 progress write-back requires PostgreSQL")
	}
	pg, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	migration, err := os.ReadFile(commandJobProgress87MigrationPath(t))
	if err != nil {
		t.Fatalf("read migration 87: %v", err)
	}
	if err := pg.Exec(string(migration)).Error; err != nil {
		t.Fatalf("apply migration 87: %v", err)
	}

	// 1. 列存在且类型正确。
	for column, wantType := range map[string]string{
		"progress_percent": "integer",
		"progress_status":  "character varying",
		"progress_error":   "text",
		"progress_at":      "timestamp with time zone",
	} {
		var gotType string
		err := pg.Raw(
			`SELECT data_type FROM information_schema.columns
			  WHERE table_schema = current_schema() AND table_name = 'command_job_details' AND column_name = ?`,
			column,
		).Scan(&gotType).Error
		if err != nil {
			t.Fatalf("look up column %s: %v", column, err)
		}
		if gotType != wantType {
			t.Fatalf("column %s type = %q, want %q", column, gotType, wantType)
		}
	}

	// 2. 准备批次与明细行（command_job_details 有到 command_jobs 的外键，先建父行）。
	const (
		jobID    = "11111111-2222-3333-4444-00000000jb87"
		detailID = "11111111-2222-3333-4444-00000000dt87"
		tenantID = "tenant-progress-87"
		deviceID = "device-progress-87"
	)
	if err := pg.Exec(
		`INSERT INTO command_jobs (id, tenant_id, operator_id, job_type, scope_type, identify, status)
		 VALUES (?, ?, ?, 'ota', 'device', 'progress-87', 'running')
		 ON CONFLICT (id) DO NOTHING`,
		jobID, tenantID, "operator-87",
	).Error; err != nil {
		t.Fatalf("seed command job: %v", err)
	}
	defer pg.Exec(`DELETE FROM command_jobs WHERE id = ?`, jobID)

	now := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	if err := pg.Exec(
		`INSERT INTO command_job_details
		   (id, command_job_id, tenant_id, device_id, status, online, eligible, log_recorded, can_retry,
		    telemetry_current_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'submitted', false, true, false, false, 0, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, progress_percent = NULL,
		     progress_status = NULL, progress_error = NULL, progress_at = NULL`,
		detailID, jobID, tenantID, deviceID, now, now,
	).Error; err != nil {
		t.Fatalf("seed command job detail: %v", err)
	}
	defer pg.Exec(`DELETE FROM command_job_details WHERE id = ?`, detailID)

	// 3. CHECK 必须拒绝越界百分比：数据库是最后一道闸，不能只靠服务层自觉。
	for _, illegal := range []int{101, -1, 1000} {
		err := pg.Exec(`UPDATE command_job_details SET progress_percent = ? WHERE id = ?`, illegal, detailID).Error
		if err == nil {
			t.Fatalf("progress_percent = %d must be rejected by the CHECK constraint", illegal)
		}
	}
	for _, legal := range []int{0, 100} {
		if err := pg.Exec(`UPDATE command_job_details SET progress_percent = ? WHERE id = ?`, legal, detailID).Error; err != nil {
			t.Fatalf("progress_percent = %d must be accepted, got %v", legal, err)
		}
	}
	// NULL 表示"从未上报"，必须仍然可写（运维要能把行的进度清回未知态）。
	if err := pg.Exec(`UPDATE command_job_details SET progress_percent = NULL WHERE id = ?`, detailID).Error; err != nil {
		t.Fatalf("NULL progress_percent must be accepted, got %v", err)
	}

	// 4. 回写守卫在真库上成立。
	oldDB := global.DB
	global.DB = pg
	t.Cleanup(func() { global.DB = oldDB })

	reportAt := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	got, err := UpdateFleetCommandJobDetailProgress(FleetCommandJobProgressWriteback{
		JobID: jobID, TenantID: tenantID, DeviceID: deviceID,
		Percent: 40, Status: "running", At: reportAt,
	})
	if err != nil {
		t.Fatalf("writeback on postgres: %v", err)
	}
	if got != CommandJobProgressApplied {
		t.Fatalf("writeback result = %q, want %q", got, CommandJobProgressApplied)
	}

	var stored model.CommandJobDetail
	if err := pg.Where("id = ?", detailID).First(&stored).Error; err != nil {
		t.Fatalf("reload detail: %v", err)
	}
	if stored.ProgressPercent == nil || *stored.ProgressPercent != 40 {
		t.Fatalf("progress_percent = %v, want 40", stored.ProgressPercent)
	}
	if stored.ProgressAt == nil || !stored.ProgressAt.UTC().Equal(reportAt) {
		t.Fatalf("progress_at = %v, want %v (UTC)", stored.ProgressAt, reportAt)
	}

	// 乱序旧进度：发生时间早于行内已存进度，必须被拒且不覆盖。
	stale, err := UpdateFleetCommandJobDetailProgress(FleetCommandJobProgressWriteback{
		JobID: jobID, TenantID: tenantID, DeviceID: deviceID,
		Percent: 10, At: reportAt.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("stale writeback on postgres: %v", err)
	}
	if stale != CommandJobProgressStale {
		t.Fatalf("stale writeback result = %q, want %q", stale, CommandJobProgressStale)
	}
	if err := pg.Where("id = ?", detailID).First(&stored).Error; err != nil {
		t.Fatalf("reload detail after stale: %v", err)
	}
	if stored.ProgressPercent == nil || *stored.ProgressPercent != 40 {
		t.Fatalf("stale report overwrote newer progress: %v", stored.ProgressPercent)
	}

	// 5. 租户隔离：别的上报不得改写本租户的行。
	if _, err := UpdateFleetCommandJobDetailProgress(FleetCommandJobProgressWriteback{
		JobID: jobID, TenantID: "tenant-progress-87-intruder", DeviceID: deviceID,
		Percent: 99, At: reportAt.Add(time.Hour),
	}); err != nil {
		t.Fatalf("cross-tenant writeback: %v", err)
	}
	if err := pg.Where("id = ?", detailID).First(&stored).Error; err != nil {
		t.Fatalf("reload detail after cross-tenant attempt: %v", err)
	}
	if stored.ProgressPercent == nil || *stored.ProgressPercent != 40 {
		t.Fatalf("cross-tenant writeback mutated this row: %v", stored.ProgressPercent)
	}
}
