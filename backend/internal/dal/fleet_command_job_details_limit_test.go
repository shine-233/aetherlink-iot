// 文件用途：覆盖 Command Job 详情/支持包内联读取的 limit 收敛行为。
// 核心逻辑：基于内存 sqlite 种子数据，验证无参默认、越界截断、排序与租户隔离。
// 关键注意事项：内联上限与 maxInternalCommandJobScanLimit 对齐，防止超大任务无界查询。
package dal

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
)

func seedCommandJobDetails(t *testing.T, jobID, tenantID string, count int, base time.Time) {
	t.Helper()
	rows := make([]*model.CommandJobDetail, 0, count)
	for i := 0; i < count; i++ {
		createdAt := base.Add(time.Duration(i) * time.Minute)
		rows = append(rows, &model.CommandJobDetail{
			ID:           fmt.Sprintf("%s-detail-%04d", jobID, i),
			CommandJobID: jobID,
			TenantID:     tenantID,
			DeviceID:     fmt.Sprintf("device-%04d", i),
			Status:       "completed",
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		})
	}
	if err := global.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed command job details: %v", err)
	}
}

func TestGetCommandJobDetailsAppliesInlineLimit(t *testing.T) {
	newDalListLimitTestDB(t)
	if err := global.DB.AutoMigrate(&model.CommandJobDetail{}); err != nil {
		t.Fatalf("migrate command job details: %v", err)
	}
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	const seededRows = commandJobDetailInlineLimit + 20
	seedCommandJobDetails(t, "job-a", "tenant-a", seededRows, base)
	seedCommandJobDetails(t, "job-b", "tenant-b", 5, base)

	// 无参默认：limit=0 应回退到内联上限，而不是取消 LIMIT 造成无界查询。
	details, err := GetCommandJobDetails("job-a", "tenant-a", 0)
	if err != nil {
		t.Fatalf("get details with zero limit: %v", err)
	}
	if len(details) != commandJobDetailInlineLimit {
		t.Fatalf("zero limit returned %d rows, want capped at %d", len(details), commandJobDetailInlineLimit)
	}
	if details[0].ID != "job-a-detail-0000" || details[len(details)-1].ID != fmt.Sprintf("job-a-detail-%04d", commandJobDetailInlineLimit-1) {
		t.Fatalf("rows are not ordered by created_at ASC from the oldest row")
	}

	// 越界收敛：超大 limit 必须截断到内联上限。
	details, err = GetCommandJobDetails("job-a", "tenant-a", 100000)
	if err != nil {
		t.Fatalf("get details with oversize limit: %v", err)
	}
	if len(details) != commandJobDetailInlineLimit {
		t.Fatalf("oversize limit returned %d rows, want capped at %d", len(details), commandJobDetailInlineLimit)
	}

	// 显式小 limit 生效。
	details, err = GetCommandJobDetails("job-a", "tenant-a", 10)
	if err != nil {
		t.Fatalf("get details with explicit limit: %v", err)
	}
	if len(details) != 10 {
		t.Fatalf("explicit limit returned %d rows, want 10", len(details))
	}

	// 租户隔离不受 limit 影响。
	details, err = GetCommandJobDetails("job-b", "tenant-b", 0)
	if err != nil {
		t.Fatalf("get other tenant details: %v", err)
	}
	if len(details) != 5 {
		t.Fatalf("other tenant returned %d rows, want 5", len(details))
	}
	details, err = GetCommandJobDetails("job-b", "tenant-a", 0)
	if err != nil {
		t.Fatalf("get cross-tenant details: %v", err)
	}
	if len(details) != 0 {
		t.Fatalf("cross-tenant read returned %d rows, want 0", len(details))
	}
}

func seedCommandJobSupportDetails(t *testing.T) {
	t.Helper()
	if err := global.DB.AutoMigrate(&model.CommandJobDetail{}); err != nil {
		t.Fatalf("migrate command job details: %v", err)
	}
	base := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	responseFailed := strconv.Itoa(constant.ResponseSStatusFailed)
	rows := []*model.CommandJobDetail{
		{ID: "d-failed-retry", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-1", Status: "failed", CanRetry: true, CreatedAt: base, UpdatedAt: base},
		{ID: "d-blocked", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-2", Status: "blocked", CreatedAt: base.Add(time.Minute), UpdatedAt: base.Add(time.Minute)},
		{ID: "d-canceled", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-3", Status: "canceled", CreatedAt: base.Add(2 * time.Minute), UpdatedAt: base.Add(2 * time.Minute)},
		{ID: "d-missing-log", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-4", Status: "submitted", LogRecorded: false, CreatedAt: base.Add(3 * time.Minute), UpdatedAt: base.Add(3 * time.Minute)},
		{ID: "d-resp-failed", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-5", Status: "submitted", LogRecorded: true, ResponseStatus: &responseFailed, CreatedAt: base.Add(4 * time.Minute), UpdatedAt: base.Add(4 * time.Minute)},
		{ID: "d-submitted-log", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-6", Status: "submitted", LogRecorded: true, CreatedAt: base.Add(5 * time.Minute), UpdatedAt: base.Add(5 * time.Minute)},
		{ID: "d-ready", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-7", Status: "ready", CreatedAt: base.Add(6 * time.Minute), UpdatedAt: base.Add(6 * time.Minute)},
		{ID: "d-dispatching", CommandJobID: "job-s", TenantID: "tenant-a", DeviceID: "dev-8", Status: "dispatching", CreatedAt: base.Add(7 * time.Minute), UpdatedAt: base.Add(7 * time.Minute)},
	}
	if err := global.DB.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed support details: %v", err)
	}
}

func TestFindCommandJobSupportDetailsFiltersAndAppliesLimit(t *testing.T) {
	newDalListLimitTestDB(t)
	seedCommandJobSupportDetails(t)

	// 不含下发中：支持集 = failed+retry / blocked / canceled / missing-log / resp-failed，共 5 行。
	details, err := FindCommandJobSupportDetails("job-s", "tenant-a", false, 0)
	if err != nil {
		t.Fatalf("find support details without in-flight: %v", err)
	}
	if len(details) != 5 {
		t.Fatalf("support details (no in-flight) = %d rows, want 5", len(details))
	}
	if details[0].ID != "d-failed-retry" || details[len(details)-1].ID != "d-resp-failed" {
		t.Fatalf("support details are not ordered by updated_at ASC")
	}

	// 含下发中：追加 dispatching 行，共 6 行。
	details, err = FindCommandJobSupportDetails("job-s", "tenant-a", true, 0)
	if err != nil {
		t.Fatalf("find support details with in-flight: %v", err)
	}
	if len(details) != 6 {
		t.Fatalf("support details (in-flight) = %d rows, want 6", len(details))
	}

	// 显式 limit 截断并保持排序。
	details, err = FindCommandJobSupportDetails("job-s", "tenant-a", false, 2)
	if err != nil {
		t.Fatalf("find support details with limit: %v", err)
	}
	if len(details) != 2 || details[0].ID != "d-failed-retry" || details[1].ID != "d-blocked" {
		t.Fatalf("limited support details = %#v, want first two rows by updated_at ASC", detailIDs(details))
	}

	// 越界收敛：超大 limit 不改变结果集大小。
	details, err = FindCommandJobSupportDetails("job-s", "tenant-a", false, 100000)
	if err != nil {
		t.Fatalf("find support details with oversize limit: %v", err)
	}
	if len(details) != 5 {
		t.Fatalf("oversize limit returned %d rows, want 5", len(details))
	}
}

func detailIDs(details []*model.CommandJobDetail) []string {
	ids := make([]string, 0, len(details))
	for _, detail := range details {
		ids = append(ids, detail.ID)
	}
	return ids
}
