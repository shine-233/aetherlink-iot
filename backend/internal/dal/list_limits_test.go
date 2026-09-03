package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDalListLimitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func TestListPayloadSchemasCapsListLimit(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.PayloadSchemaRecord{}); err != nil {
		t.Fatalf("migrate payload schemas: %v", err)
	}

	now := time.Now().UTC()
	rows := make([]*model.PayloadSchemaRecord, 0, 505)
	for i := 0; i < 505; i++ {
		rows = append(rows, &model.PayloadSchemaRecord{
			ID:        fmt.Sprintf("schema-%03d", i),
			TenantID:  "tenant-a",
			Name:      fmt.Sprintf("schema-%03d", i),
			Fields:    "[]",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed payload schemas: %v", err)
	}

	got, err := ListPayloadSchemas("tenant-a", 5000)
	if err != nil {
		t.Fatalf("list payload schemas: %v", err)
	}
	if len(got) != maxPayloadSchemaListLimit {
		t.Fatalf("oversized limit returned %d rows, want capped at %d", len(got), maxPayloadSchemaListLimit)
	}

	// 负数 limit 曾会取消 LIMIT 造成无界查询，现在必须收敛到具名上限。
	got, err = ListPayloadSchemas("tenant-a", -1)
	if err != nil {
		t.Fatalf("list payload schemas with negative limit: %v", err)
	}
	if len(got) != maxPayloadSchemaListLimit {
		t.Fatalf("negative limit returned %d rows, want capped at %d", len(got), maxPayloadSchemaListLimit)
	}

	got, err = ListPayloadSchemas("tenant-a", 3)
	if err != nil {
		t.Fatalf("list payload schemas with small limit: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("small limit returned %d rows, want 3", len(got))
	}
}

func TestListEmailTemplatesCapsUnpagedAndOversizedQueries(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.EmailTemplate{}); err != nil {
		t.Fatalf("migrate email templates: %v", err)
	}

	now := time.Now().UTC()
	rows := make([]*model.EmailTemplate, 0, 505)
	for i := 0; i < 505; i++ {
		rows = append(rows, &model.EmailTemplate{
			ID:              fmt.Sprintf("template-%03d", i),
			TenantID:        "tenant-a",
			Name:            fmt.Sprintf("template-%03d", i),
			Purpose:         model.EmailTemplatePurposeAlarm,
			SubjectTemplate: "subject",
			BodyTemplate:    "body",
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed email templates: %v", err)
	}

	// 未分页调用同样必须封顶，避免一次性拉取全表。
	total, list, err := ListEmailTemplates([]string{"tenant-a"}, 0, 0)
	if err != nil {
		t.Fatalf("unpaged list email templates: %v", err)
	}
	if total != 505 {
		t.Fatalf("unpaged total = %d, want full count 505", total)
	}
	if len(list) != maxEmailTemplateListLimit {
		t.Fatalf("unpaged list returned %d rows, want capped at %d", len(list), maxEmailTemplateListLimit)
	}

	_, list, err = ListEmailTemplates([]string{"tenant-a"}, 1, 5000)
	if err != nil {
		t.Fatalf("oversized page size list email templates: %v", err)
	}
	if len(list) != maxEmailTemplateListLimit {
		t.Fatalf("oversized page size returned %d rows, want capped at %d", len(list), maxEmailTemplateListLimit)
	}

	total, list, err = ListEmailTemplates([]string{"tenant-a"}, 2, 300)
	if err != nil {
		t.Fatalf("second page list email templates: %v", err)
	}
	if total != 505 {
		t.Fatalf("second page total = %d, want 505", total)
	}
	if len(list) != 205 {
		t.Fatalf("second page returned %d rows, want remaining 205", len(list))
	}
}

func TestClampInternalCommandJobScanLimit(t *testing.T) {
	cases := []struct {
		limit int
		want  int
	}{
		{limit: 0, want: 100},
		{limit: -5, want: 100},
		{limit: 100, want: 100},
		{limit: maxInternalCommandJobScanLimit, want: maxInternalCommandJobScanLimit},
		{limit: 10000, want: maxInternalCommandJobScanLimit},
	}
	for _, tc := range cases {
		if got := clampInternalCommandJobScanLimit(tc.limit); got != tc.want {
			t.Fatalf("clampInternalCommandJobScanLimit(%d) = %d, want %d", tc.limit, got, tc.want)
		}
	}
}

func TestListTimedOutRunningCommandJobsCapsInternalScan(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.CommandJob{}); err != nil {
		t.Fatalf("migrate command jobs: %v", err)
	}

	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	rows := make([]*model.CommandJob, 0, 505)
	for i := 0; i < 505; i++ {
		timeoutAt := past
		rows = append(rows, &model.CommandJob{
			ID:             fmt.Sprintf("job-%03d", i),
			TenantID:       "tenant-a",
			OperatorID:     "operator",
			JobType:        "command",
			ScopeType:      "all",
			Identify:       "reboot",
			Status:         "running",
			TimeoutSeconds: 60,
			RequestedCount: 1,
			CreatedAt:      now,
			UpdatedAt:      now,
			TimeoutAt:      &timeoutAt,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}

	jobs, err := ListTimedOutRunningCommandJobs(now, 10000)
	if err != nil {
		t.Fatalf("list timed out command jobs: %v", err)
	}
	if len(jobs) != maxInternalCommandJobScanLimit {
		t.Fatalf("timed-out scan returned %d jobs, want capped at %d", len(jobs), maxInternalCommandJobScanLimit)
	}
}
