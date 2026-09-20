// 文件用途：明细行进度回写的定向证据（ROADMAP P0.3，迁移 87）。
// 核心逻辑：锁定"能写 / 不能写"的边界——租户隔离、终态拒绝、乱序旧进度拒绝、
// 以及 NULL 与 0 的语义区分。
// 关键注意事项：这里用 sqlite 覆盖判定逻辑本身；0..100 的 CHECK 约束只有
// PostgreSQL 才有，单独由 TestCommandJobDetailProgressMigration87Postgres 验证。
package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
)

func setupCommandJobProgressTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.CommandJobDetail{}); err != nil {
		t.Fatalf("migrate command job details: %v", err)
	}
	return db
}

func seedProgressDetail(t *testing.T, db *gorm.DB, detail model.CommandJobDetail) {
	t.Helper()
	if err := db.Create(&detail).Error; err != nil {
		t.Fatalf("seed detail %s: %v", detail.ID, err)
	}
}

func loadProgressDetail(t *testing.T, db *gorm.DB, id string) model.CommandJobDetail {
	t.Helper()
	var detail model.CommandJobDetail
	if err := db.Where("id = ?", id).First(&detail).Error; err != nil {
		t.Fatalf("load detail %s: %v", id, err)
	}
	return detail
}

func baseProgressWriteback() FleetCommandJobProgressWriteback {
	return FleetCommandJobProgressWriteback{
		JobID:    "job-1",
		TenantID: "tenant-1",
		DeviceID: "dev-1",
		Percent:  40,
		Status:   "running",
		At:       time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
	}
}

// 明细行状态在此处刻意写成字面量而非引用服务层常量：测试要用独立预言值，
// 引用生产常量等于让被测代码自己给自己判卷（常量拼错时测试会一起错且全绿）。
const (
	progressTestStatusSubmitted = "submitted"
	progressTestStatusFailed    = "failed"
	progressTestStatusCanceled  = "canceled"
)

func newProgressDetail(id, tenantID, deviceID, status string) model.CommandJobDetail {
	now := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	return model.CommandJobDetail{
		ID:           id,
		CommandJobID: "job-1",
		TenantID:     tenantID,
		DeviceID:     deviceID,
		Status:       status,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestCommandJobDetailProgressWritebackApplied(t *testing.T) {
	db := setupCommandJobProgressTestDB(t)
	seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", progressTestStatusSubmitted))

	got, err := UpdateFleetCommandJobDetailProgress(baseProgressWriteback())
	if err != nil {
		t.Fatalf("writeback: %v", err)
	}
	if got != CommandJobProgressApplied {
		t.Fatalf("writeback result = %q, want %q", got, CommandJobProgressApplied)
	}

	detail := loadProgressDetail(t, db, "d-1")
	if detail.ProgressPercent == nil || *detail.ProgressPercent != 40 {
		t.Fatalf("progress_percent = %v, want 40", detail.ProgressPercent)
	}
	if detail.ProgressStatus == nil || *detail.ProgressStatus != "running" {
		t.Fatalf("progress_status = %v, want running", detail.ProgressStatus)
	}
	if detail.ProgressError != nil {
		t.Fatalf("progress_error = %v, want NULL when the report carries no error", *detail.ProgressError)
	}
	if detail.ProgressAt == nil || !detail.ProgressAt.Equal(baseProgressWriteback().At) {
		t.Fatalf("progress_at = %v, want the report occurrence time", detail.ProgressAt)
	}
}

// TestCommandJobDetailProgressWritebackMirrorsLatestReport 锁定"明细行是最近一次上报的镜像"：
// 一份不带状态/错误的新进度必须清掉上一次的状态与错误，否则上一次的失败会残留
// 在一份全新的成功进度上，运维看到的是拼接出来的假状态。
func TestCommandJobDetailProgressWritebackMirrorsLatestReport(t *testing.T) {
	db := setupCommandJobProgressTestDB(t)
	seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", progressTestStatusSubmitted))

	failing := baseProgressWriteback()
	failing.Status = "failed"
	failing.Error = "boom"
	if got, err := UpdateFleetCommandJobDetailProgress(failing); err != nil || got != CommandJobProgressApplied {
		t.Fatalf("first writeback = (%q, %v), want applied", got, err)
	}

	recovered := baseProgressWriteback()
	recovered.Percent = 100
	recovered.Status = ""
	recovered.Error = ""
	recovered.At = failing.At.Add(time.Minute)
	if got, err := UpdateFleetCommandJobDetailProgress(recovered); err != nil || got != CommandJobProgressApplied {
		t.Fatalf("second writeback = (%q, %v), want applied", got, err)
	}

	detail := loadProgressDetail(t, db, "d-1")
	if detail.ProgressPercent == nil || *detail.ProgressPercent != 100 {
		t.Fatalf("progress_percent = %v, want 100", detail.ProgressPercent)
	}
	if detail.ProgressStatus != nil {
		t.Fatalf("progress_status must be NULL when the latest report carries none, got %q", *detail.ProgressStatus)
	}
	if detail.ProgressError != nil {
		t.Fatalf("progress_error must be cleared when the latest report carries none, got %q", *detail.ProgressError)
	}
}

// TestCommandJobDetailProgressWritebackRespectsTenantScope 锁定租户隔离：
// 同一批次、同一设备编号在不同租户下是两行，别的上报只能改到自己那一行。
func TestCommandJobDetailProgressWritebackRespectsTenantScope(t *testing.T) {
	db := setupCommandJobProgressTestDB(t)
	seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", progressTestStatusSubmitted))
	seedProgressDetail(t, db, newProgressDetail("d-2", "tenant-2", "dev-1", progressTestStatusSubmitted))

	got, err := UpdateFleetCommandJobDetailProgress(baseProgressWriteback())
	if err != nil || got != CommandJobProgressApplied {
		t.Fatalf("writeback = (%q, %v), want applied", got, err)
	}

	if detail := loadProgressDetail(t, db, "d-2"); detail.ProgressPercent != nil {
		t.Fatalf("cross-tenant row was written: progress_percent = %d", *detail.ProgressPercent)
	}
}

func TestCommandJobDetailProgressWritebackRejectsTerminalRows(t *testing.T) {
	for _, status := range []string{progressTestStatusFailed, progressTestStatusCanceled} {
		t.Run(status, func(t *testing.T) {
			db := setupCommandJobProgressTestDB(t)
			seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", status))

			got, err := UpdateFleetCommandJobDetailProgress(baseProgressWriteback())
			if err != nil {
				t.Fatalf("writeback: %v", err)
			}
			if got != CommandJobProgressTerminal {
				t.Fatalf("writeback result = %q, want %q", got, CommandJobProgressTerminal)
			}
			if detail := loadProgressDetail(t, db, "d-1"); detail.ProgressPercent != nil {
				t.Fatalf("terminal row must stay untouched, got progress_percent = %d", *detail.ProgressPercent)
			}
		})
	}
}

// TestCommandJobDetailProgressWritebackRejectsStaleReports 锁定"乱序到达的旧进度不得覆盖新进度"，
// 同时确认真实回退（设备确实从 80 退回 30）会被如实写入——被挡的只是到达顺序造成的假回退。
func TestCommandJobDetailProgressWritebackRejectsStaleReports(t *testing.T) {
	db := setupCommandJobProgressTestDB(t)
	seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", progressTestStatusSubmitted))

	base := baseProgressWriteback()
	newer := base
	newer.Percent = 80
	newer.At = base.At.Add(time.Hour)
	if got, err := UpdateFleetCommandJobDetailProgress(newer); err != nil || got != CommandJobProgressApplied {
		t.Fatalf("seed 80%% writeback = (%q, %v), want applied", got, err)
	}

	stale := base
	stale.Percent = 10
	if got, err := UpdateFleetCommandJobDetailProgress(stale); err != nil {
		t.Fatalf("stale writeback: %v", err)
	} else if got != CommandJobProgressStale {
		t.Fatalf("stale writeback result = %q, want %q", got, CommandJobProgressStale)
	}
	if detail := loadProgressDetail(t, db, "d-1"); detail.ProgressPercent == nil || *detail.ProgressPercent != 80 {
		t.Fatalf("stale report must not overwrite newer progress, got %v", detail.ProgressPercent)
	}

	// 发生时间更新的真实回退必须被接受：设备说退回 30 就是 30，不能为了好看卡在 80。
	regressed := newer
	regressed.Percent = 30
	regressed.At = newer.At.Add(time.Hour)
	if got, err := UpdateFleetCommandJobDetailProgress(regressed); err != nil || got != CommandJobProgressApplied {
		t.Fatalf("regression writeback = (%q, %v), want applied", got, err)
	}
	if detail := loadProgressDetail(t, db, "d-1"); detail.ProgressPercent == nil || *detail.ProgressPercent != 30 {
		t.Fatalf("genuine regression must be recorded, got %v", detail.ProgressPercent)
	}
}

// TestCommandJobDetailProgressWritebackZeroIsNotMissingProgress 锁定 NULL 与 0 的区分：
// 0% 是"已开始但未推进"，NULL 是"从未上报"。把 NULL 当成 0（或反之）都会让
// 进度看板凭空多出一批"已开始"的设备。
func TestCommandJobDetailProgressWritebackZeroIsNotMissingProgress(t *testing.T) {
	db := setupCommandJobProgressTestDB(t)
	seedProgressDetail(t, db, newProgressDetail("d-1", "tenant-1", "dev-1", progressTestStatusSubmitted))

	if detail := loadProgressDetail(t, db, "d-1"); detail.ProgressPercent != nil {
		t.Fatalf("unreported progress must stay NULL, got %d", *detail.ProgressPercent)
	}

	zero := baseProgressWriteback()
	zero.Percent = 0
	if got, err := UpdateFleetCommandJobDetailProgress(zero); err != nil || got != CommandJobProgressApplied {
		t.Fatalf("0%% writeback = (%q, %v), want applied", got, err)
	}
	detail := loadProgressDetail(t, db, "d-1")
	if detail.ProgressPercent == nil {
		t.Fatalf("0%% must be stored as a real value, not left NULL")
	}
	if *detail.ProgressPercent != 0 {
		t.Fatalf("progress_percent = %d, want 0", *detail.ProgressPercent)
	}
}

func TestCommandJobDetailProgressWritebackNoRow(t *testing.T) {
	setupCommandJobProgressTestDB(t)

	got, err := UpdateFleetCommandJobDetailProgress(baseProgressWriteback())
	if err != nil {
		t.Fatalf("writeback: %v", err)
	}
	if got != CommandJobProgressNoRow {
		t.Fatalf("writeback result = %q, want %q", got, CommandJobProgressNoRow)
	}
}
