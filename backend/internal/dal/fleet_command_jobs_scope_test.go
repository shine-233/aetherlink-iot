// 文件用途：验证命令任务列表与 attention 指标读路径的 tenant scopes 三态契约
// （ROADMAP C2 自上而下）：0→fail-closed 空结果、1→tenant_id =（旧单租户等价）、
// >1→tenant_id IN（self∪子孙）；列表、逐任务指标与整体摘要随同一作用域展开。
package dal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestListCommandJobsScopes(t *testing.T) {
	db := setupFleetCommandJobScopeTestDB(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	jobs := []*model.CommandJob{
		newFleetCommandJobScopeSeed("job-hq-1", "tenant-hq", "completed", base.Add(-3*time.Minute)),
		newFleetCommandJobScopeSeed("job-hq-2", "tenant-hq", "running", base.Add(-2*time.Minute)),
		newFleetCommandJobScopeSeed("job-child-1", "tenant-child", "completed", base.Add(-time.Minute)),
		newFleetCommandJobScopeSeed("job-foreign", "tenant-x", "completed", base),
	}
	if err := db.CreateInBatches(jobs, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}

	t.Run("parent scope returns self and descendants only", func(t *testing.T) {
		total, list, err := ListCommandJobs([]string{"tenant-hq", "tenant-child"}, "", "", "", 1, 10, 3, base)
		if err != nil {
			t.Fatalf("ListCommandJobs(): %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total = %d, list = %#v, want 3 in-scope jobs", total, list)
		}
		seen := map[string]bool{}
		for _, job := range list {
			seen[job.ID] = true
			if job.TenantID != "tenant-hq" && job.TenantID != "tenant-child" {
				t.Fatalf("job %q escaped scope with tenant %q", job.ID, job.TenantID)
			}
		}
		if !seen["job-hq-1"] || !seen["job-hq-2"] || !seen["job-child-1"] {
			t.Fatalf("jobs = %v, want hq and child rows", seen)
		}
	})

	t.Run("single scope keeps legacy tenant filter", func(t *testing.T) {
		total, list, err := ListCommandJobs([]string{"tenant-hq"}, "", "", "", 1, 10, 3, base)
		if err != nil {
			t.Fatalf("ListCommandJobs(): %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total = %d, list = %#v, want 2 hq jobs", total, list)
		}
	})

	t.Run("nil and empty scopes fail closed", func(t *testing.T) {
		for _, scopes := range [][]string{nil, {}} {
			total, list, err := ListCommandJobs(scopes, "", "", "", 1, 10, 3, base)
			if err != nil {
				t.Fatalf("ListCommandJobs(scopes=%v): %v", scopes, err)
			}
			if total != 0 || list != nil {
				t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, nil, nil)", total, list, err)
			}
		}
	})
}

func TestGetCommandJobListAttentionMetricsScopes(t *testing.T) {
	db := setupFleetCommandJobScopeTestDB(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	jobs := []*model.CommandJob{
		newFleetCommandJobScopeSeed("job-hq-1", "tenant-hq", "completed", base.Add(-2*time.Minute)),
		newFleetCommandJobScopeSeed("job-child-1", "tenant-child", "completed", base.Add(-time.Minute)),
	}
	if err := db.CreateInBatches(jobs, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}
	details := []*model.CommandJobDetail{
		newFleetCommandJobDetailScopeSeed("det-hq-1", "job-hq-1", "tenant-hq"),
		newFleetCommandJobDetailScopeSeed("det-child-1", "job-child-1", "tenant-child"),
	}
	if err := db.CreateInBatches(details, 100).Error; err != nil {
		t.Fatalf("seed command job details: %v", err)
	}

	t.Run("metrics follow expanded scope", func(t *testing.T) {
		metrics, err := GetCommandJobListAttentionMetrics([]string{"job-hq-1", "job-child-1"}, []string{"tenant-hq", "tenant-child"}, 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionMetrics(): %v", err)
		}
		if len(metrics) != 2 {
			t.Fatalf("metrics jobs = %v, want job-hq-1 and job-child-1", metrics)
		}
		if metrics["job-hq-1"].BlockedCount != 1 || metrics["job-child-1"].BlockedCount != 1 {
			t.Fatalf("blocked counts = %d/%d, want 1/1", metrics["job-hq-1"].BlockedCount, metrics["job-child-1"].BlockedCount)
		}
	})

	t.Run("single scope excludes descendant job metrics", func(t *testing.T) {
		metrics, err := GetCommandJobListAttentionMetrics([]string{"job-hq-1", "job-child-1"}, []string{"tenant-hq"}, 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionMetrics(): %v", err)
		}
		if _, ok := metrics["job-child-1"]; ok {
			t.Fatalf("child job metric escaped single scope: %#v", metrics)
		}
		if metrics["job-hq-1"].BlockedCount != 1 {
			t.Fatalf("hq blocked count = %d, want 1", metrics["job-hq-1"].BlockedCount)
		}
	})

	t.Run("empty scopes fail closed", func(t *testing.T) {
		metrics, err := GetCommandJobListAttentionMetrics([]string{"job-hq-1"}, nil, 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionMetrics(nil scopes): %v", err)
		}
		if len(metrics) != 0 {
			t.Fatalf("fail-closed metrics = %#v, want empty", metrics)
		}
	})
}

func TestGetCommandJobListAttentionSummaryScopes(t *testing.T) {
	db := setupFleetCommandJobScopeTestDB(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	jobs := []*model.CommandJob{
		newFleetCommandJobScopeSeed("job-hq-1", "tenant-hq", "completed", base.Add(-2*time.Minute)),
		newFleetCommandJobScopeSeed("job-child-1", "tenant-child", "completed", base.Add(-time.Minute)),
		newFleetCommandJobScopeSeed("job-foreign", "tenant-x", "completed", base),
	}
	if err := db.CreateInBatches(jobs, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}
	details := []*model.CommandJobDetail{
		newFleetCommandJobDetailScopeSeed("det-hq-1", "job-hq-1", "tenant-hq"),
		newFleetCommandJobDetailScopeSeed("det-child-1", "job-child-1", "tenant-child"),
		newFleetCommandJobDetailScopeSeed("det-foreign-1", "job-foreign", "tenant-x"),
	}
	if err := db.CreateInBatches(details, 100).Error; err != nil {
		t.Fatalf("seed command job details: %v", err)
	}

	t.Run("summary follows expanded scope", func(t *testing.T) {
		summary, err := GetCommandJobListAttentionSummary([]string{"tenant-hq", "tenant-child"}, "", "", "", 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionSummary(): %v", err)
		}
		if summary.BlockedCount != 2 {
			t.Fatalf("summary blocked count = %d, want 2", summary.BlockedCount)
		}
	})

	t.Run("single scope keeps legacy summary", func(t *testing.T) {
		summary, err := GetCommandJobListAttentionSummary([]string{"tenant-hq"}, "", "", "", 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionSummary(): %v", err)
		}
		if summary.BlockedCount != 1 {
			t.Fatalf("summary blocked count = %d, want 1", summary.BlockedCount)
		}
	})

	t.Run("nil scopes fail closed", func(t *testing.T) {
		summary, err := GetCommandJobListAttentionSummary(nil, "", "", "", 3, base)
		if err != nil {
			t.Fatalf("GetCommandJobListAttentionSummary(nil scopes): %v", err)
		}
		if !reflect.DeepEqual(summary, CommandJobListAttentionMetrics{}) {
			t.Fatalf("fail-closed summary = %#v, want zero metrics", summary)
		}
	})
}

func newFleetCommandJobScopeSeed(id, tenantID, status string, createdAt time.Time) *model.CommandJob {
	return &model.CommandJob{
		ID:             id,
		TenantID:       tenantID,
		OperatorID:     "operator",
		JobType:        "command",
		ScopeType:      "all",
		Identify:       "identify-" + id,
		Status:         status,
		TimeoutSeconds: 60,
		RequestedCount: 1,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
}

func newFleetCommandJobDetailScopeSeed(id, jobID, tenantID string) *model.CommandJobDetail {
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	return &model.CommandJobDetail{
		ID:                    id,
		CommandJobID:          jobID,
		TenantID:              tenantID,
		DeviceID:              "device-" + id,
		Online:                true,
		Eligible:              true,
		Status:                "blocked",
		DispatchAttempts:      0,
		LogRecorded:           false,
		CanRetry:              false,
		TelemetryCurrentCount: 0,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func setupFleetCommandJobScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open fleet command job scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.CommandJob{}, &model.CommandJobDetail{}); err != nil {
		t.Fatalf("migrate fleet command job scope tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}
