// 文件用途：P0.3 报告导出的运行期证据（真实 PostgreSQL）。
// 覆盖：进度 NULL 与 0 是两种事实（NULL 不导出成 0）、CSV 渲染区分二者、
// 租户隔离、行序固定可比对、limit 收敛。
// 说明：缺 DSN 或缺表一律 Skip，不得把 Skip 当作通过。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
	"github.com/go-basic/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openJobReportPostgres 打开数据库并校验明细表存在。
func openJobReportPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; fleet job report tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='command_job_details')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe command_job_details: %v", err)
	}
	if !exists {
		t.Skip("command_job_details missing; apply migrations first")
	}
	return db
}

// seedCommandJob 写入父作业行。command_job_details 有外键 command_job_details_job_fkey，
// 直接插明细行会因 23503 失败——必须先有父作业。
func seedCommandJob(t *testing.T, db *gorm.DB, jobID, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	err := db.Exec(`INSERT INTO command_jobs
		(id, tenant_id, operator_id, job_type, scope_type, identify, timeout_seconds,
		 status, requested_count, eligible_count, blocked_count, submitted_count, failed_count,
		 can_cancel, created_at, updated_at)
		VALUES (?,?,?,1,1,?,60,'pending',0,0,0,0,0,false,?,?) ON CONFLICT (id) DO NOTHING`,
		jobID, tenantID, uuid.New(), uuid.New(), now, now).Error
	if err != nil {
		t.Fatalf("insert command_jobs: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM command_jobs WHERE id = ?", jobID).Error
	})
}

// seedJobDetail 写入一条明细行。progressPercent 为 nil 表示"从未上报进度"，
// 与传 0（"上报了 0%"）是两种事实——本测试正是要钉住这个区别。
func seedJobDetail(t *testing.T, db *gorm.DB, jobID, tenantID, deviceID string, progressPercent *int) {
	t.Helper()
	seedCommandJob(t, db, jobID, tenantID)
	now := time.Now().UTC()
	err := db.Exec(`INSERT INTO command_job_details
		(id, command_job_id, tenant_id, device_id, online, eligible, status,
		 progress_percent, log_recorded, can_retry, telemetry_current_count, created_at, updated_at)
		VALUES (?,?,?,?,false,true,'pending',?,false,false,0,?,?)`,
		uuid.New(), jobID, tenantID, deviceID, progressPercent, now, now).Error
	if err != nil {
		t.Fatalf("insert command_job_details: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM command_job_details WHERE command_job_id = ?", jobID).Error
	})
}

// TestJobReportKeepsNullProgressDistinctFromZero 核心不变式：
// NULL 进度导出为空串，不得写成 0。写成 0 等于给"从未上报"的设备
// 凭空记一笔"已开始 0%"，让运维以为整批都在推进。
func TestJobReportKeepsNullProgressDistinctFromZero(t *testing.T) {
	db := openJobReportPostgres(t)
	jobID := uuid.New()
	tenant := "tenant-report-evidence"

	zero := 0
	seedJobDetail(t, db, jobID, tenant, "device-null-progress", nil)   // 从未上报
	seedJobDetail(t, db, jobID, tenant, "device-zero-progress", &zero) // 上报了 0%

	rows, err := dal.ListFleetCommandJobReportRows(nil, jobID, tenant, 0)
	if err != nil {
		t.Fatalf("list report rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	var nullRow, zeroRow *dal.CommandJobReportRow
	for i := range rows {
		switch rows[i].DeviceID {
		case "device-null-progress":
			nullRow = &rows[i]
		case "device-zero-progress":
			zeroRow = &rows[i]
		}
	}
	if nullRow == nil || zeroRow == nil {
		t.Fatalf("expected both seeded rows, got %+v", rows)
	}
	// 读出来就必须是可区分的：一个是 nil，一个指向 0。
	if nullRow.ProgressPercent != nil {
		t.Fatalf("never-reported progress must read back as nil, got %d", *nullRow.ProgressPercent)
	}
	if zeroRow.ProgressPercent == nil || *zeroRow.ProgressPercent != 0 {
		t.Fatalf("reported 0%% must read back as pointer to 0, got %v", zeroRow.ProgressPercent)
	}

	// CSV 渲染同样要区分：NULL -> 空串，0 -> "0"。
	csvText := FormatFleetCommandJobReportCSV(rows)
	lines := strings.Split(strings.TrimRight(csvText, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("csv lines = %d, want 3 (header + 2 rows)", len(lines))
	}
	nullLine, zeroLine := "", ""
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "device-null-progress") {
			nullLine = line
		}
		if strings.HasPrefix(line, "device-zero-progress") {
			zeroLine = line
		}
	}
	if nullLine == "" || zeroLine == "" {
		t.Fatalf("csv missing seeded rows:\n%s", csvText)
	}
	if !strings.Contains(nullLine, ",,") {
		t.Fatalf("NULL progress must render as empty field, got: %s", nullLine)
	}
	if !strings.Contains(zeroLine, ",0,") {
		t.Fatalf("zero progress must render as 0, got: %s", zeroLine)
	}
}

// TestJobReportScopedToTenant 别的租户读不到同一作业的明细行。
func TestJobReportScopedToTenant(t *testing.T) {
	db := openJobReportPostgres(t)
	jobID := uuid.New()
	seedJobDetail(t, db, jobID, "tenant-report-owner", "device-owned", nil)

	rows, err := dal.ListFleetCommandJobReportRows(nil, jobID, "tenant-report-other", 0)
	if err != nil {
		t.Fatalf("list as other tenant: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("other tenant must see 0 rows, got %d", len(rows))
	}
}

// TestJobReportRejectsMissingTenant 服务层：缺租户或非法格式必须报错。
func TestJobReportRejectsMissingTenant(t *testing.T) {
	openJobReportPostgres(t)
	if _, err := (&CommandData{}).GetFleetCommandJobReport("job-1", "csv", 0, nil); err == nil {
		t.Fatalf("missing claims must be rejected")
	}
	if _, err := (&CommandData{}).GetFleetCommandJobReport("job-1", "csv", 0, &utils.UserClaims{}); err == nil {
		t.Fatalf("empty tenant must be rejected")
	}
	if _, err := (&CommandData{}).GetFleetCommandJobReport("job-1", "pdf", 0, &utils.UserClaims{TenantID: "t"}); err == nil {
		t.Fatalf("unsupported format must be rejected")
	}
}
