// 文件用途：覆盖 Command Job 公开列表分页查询的 Go 测试。
// 核心逻辑：基于内存 sqlite 种子数据，验证翻页正确性、total 统计、无参默认与越界收敛。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的种子构造器。
package dal

import (
	"fmt"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
)

func seedCommandJobListPagination(t *testing.T, db *gorm.DB, count int) time.Time {
	t.Helper()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	rows := make([]*model.CommandJob, 0, count)
	for i := 0; i < count; i++ {
		createdAt := base.Add(time.Duration(count-i) * time.Minute)
		rows = append(rows, &model.CommandJob{
			ID:             fmt.Sprintf("job-%03d", i),
			TenantID:       "tenant-a",
			OperatorID:     "operator",
			JobType:        "command",
			ScopeType:      "all",
			Identify:       fmt.Sprintf("identify-%03d", i),
			Status:         "completed",
			TimeoutSeconds: 60,
			RequestedCount: 1,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}
	return base
}

func TestListCommandJobsPaginatesWithTotal(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.CommandJob{}); err != nil {
		t.Fatalf("migrate command jobs: %v", err)
	}
	const seededJobs = 25
	base := seedCommandJobListPagination(t, db, seededJobs)

	// 翻页正确性：第 1 页与第 2 页不得重叠，且按 created_at DESC 排序。
	total, firstPage, err := ListCommandJobs("tenant-a", "", "", "", 1, 10, 3, base)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if total != seededJobs {
		t.Fatalf("first page total = %d, want %d", total, seededJobs)
	}
	if len(firstPage) != 10 {
		t.Fatalf("first page returned %d rows, want 10", len(firstPage))
	}

	total, secondPage, err := ListCommandJobs("tenant-a", "", "", "", 2, 10, 3, base)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if total != seededJobs {
		t.Fatalf("second page total = %d, want %d", total, seededJobs)
	}
	if len(secondPage) != 10 {
		t.Fatalf("second page returned %d rows, want 10", len(secondPage))
	}
	seen := map[string]bool{}
	for _, job := range append(append([]*model.CommandJob{}, firstPage...), secondPage...) {
		if seen[job.ID] {
			t.Fatalf("job %s appeared on both pages", job.ID)
		}
		seen[job.ID] = true
	}
	if firstPage[0].CreatedAt.Before(firstPage[9].CreatedAt) {
		t.Fatalf("first page is not ordered by created_at DESC")
	}

	// 尾页只返回剩余行。
	total, lastPage, err := ListCommandJobs("tenant-a", "", "", "", 3, 10, 3, base)
	if err != nil {
		t.Fatalf("list last page: %v", err)
	}
	if total != seededJobs || len(lastPage) != seededJobs-20 {
		t.Fatalf("last page total = %d len = %d, want total %d len %d", total, len(lastPage), seededJobs, seededJobs-20)
	}
}

func TestListCommandJobsAppliesDefaultAndClampsOversize(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.CommandJob{}); err != nil {
		t.Fatalf("migrate command jobs: %v", err)
	}
	// 种子行数必须大于单页上限，否则无法区分“截断”与“数据不足”。
	const seededJobs = 55
	base := seedCommandJobListPagination(t, db, seededJobs)

	// 无参默认：page=0/pageSize=0 应回退第 1 页固定页大小。
	total, jobs, err := ListCommandJobs("tenant-a", "", "", "", 0, 0, 3, base)
	if err != nil {
		t.Fatalf("list with zero paging params: %v", err)
	}
	if len(jobs) != defaultCommandJobListPageSize {
		t.Fatalf("zero paging params returned %d rows, want default page size %d", len(jobs), defaultCommandJobListPageSize)
	}
	if total != seededJobs {
		t.Fatalf("zero paging params total = %d, want %d", total, seededJobs)
	}

	// 越界收敛：超大 page_size 必须截断到上限；负数页码回退第 1 页。
	total, jobs, err = ListCommandJobs("tenant-a", "", "", "", -4, 10000, 3, base)
	if err != nil {
		t.Fatalf("list with negative page and oversize page_size: %v", err)
	}
	if total != seededJobs || len(jobs) != maxCommandJobListPageSize {
		t.Fatalf("oversized page size total=%d returned %d rows, want total %d capped at %d", total, len(jobs), seededJobs, maxCommandJobListPageSize)
	}

	// 租户隔离不受分页参数影响。
	total, _, err = ListCommandJobs("tenant-b", "", "", "", 1, 10, 3, base)
	if err != nil {
		t.Fatalf("list other tenant: %v", err)
	}
	if total != 0 {
		t.Fatalf("other tenant total = %d, want 0", total)
	}
}
