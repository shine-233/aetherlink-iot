package dal

// 文件用途：报表运行（ReportScheduleRun）的共享契约与读路径。
// 写路径已按生命周期拆出：
//   - report_run_scheduling.go —— 计划初始化与到期槽位物化；
//   - report_run_submit.go     —— 手动触发 / 重试的幂等提交；
//   - report_run_claim.go      —— 生成租约的 claim / renew / settle / reap。
// 核心逻辑（本文件）：定义幂等哈希与提交结果契约；列表与详情两个读接口。
// 关键注意事项：
//   - 列表接口用一条 IN 查询批量取投递状态（见 reportRunListDeliveries），不得退回逐 run 查询。
//   - 投递投影刻意不含 payload 列，报表正文只在单条详情 / 出队投递时读取。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
)

const (
	reportOccurrenceScanCap     = 10000
	reportOccurrenceScanHorizon = 10 * 365 * 24 * time.Hour
	reportRetryBaseDelay        = time.Minute
	reportRetryMaxDelay         = time.Hour
	defaultReportMaxAttempts    = 3
)

type ReportNextOccurrence func(schedule *model.ReportSchedule, after time.Time) (time.Time, error)

type ReportScheduleInitializationResult struct {
	Initialized int
	Invalid     int
}

type ReportMaterializationResult struct {
	Run               *model.ReportScheduleRun
	CoalescedMisfires int
}

type ReportGenerationClaim struct {
	Run        *model.ReportScheduleRun
	Token      string
	LeaseUntil time.Time
}

type ReportRunSubmission struct {
	Run                   *model.ReportScheduleRun
	IdempotentReplay      bool
	DuplicateDeliveryRisk bool
	// OverallStatus is the real projection of Run at submission time. A replay can
	// return a run that already reached a terminal state, so callers must not
	// assume a fresh submission.
	OverallStatus string
}

type ManualReportRunInput struct {
	ID                 string
	TenantID           string
	ScheduleID         string
	IdempotencyKey     string
	RequestFingerprint []byte
	MaxAttempts        int
	// Deprecated caller-clock fields are intentionally ignored; DB time owns the window.
	WindowStartAt time.Time
	WindowEndAt   time.Time
}

type RetryReportRunInput struct {
	ID                 string
	TenantID           string
	ScheduleID         string
	ParentRunID        string
	IdempotencyKey     string
	RequestFingerprint []byte
	MaxAttempts        int
}

func HashReportIdempotencyKey(value string) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.TrimSpace(value)))
}

func HashReportRequestFingerprint(value []byte) [sha256.Size]byte {
	return sha256.Sum256(value)
}

func reportHexHash(value [sha256.Size]byte) string { return hex.EncodeToString(value[:]) }

// reportDatabaseNow 取数据库时钟。所有报表状态迁移都必须以它为基准，
// 否则多 worker 各自的系统时钟漂移会让租约与窗口计算失去可比性。
func reportDatabaseNow(tx *gorm.DB) (time.Time, error) {
	var now time.Time
	err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error
	return now.UTC(), err
}

func ListReportRunsContext(ctx context.Context, tenantID, scheduleID string, request model.ReportRunListRequest) (*model.ReportRunListResponse, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureReportScheduleTenantScope(db, tenantID, scheduleID); err != nil {
		return nil, err
	}
	page, pageSize := normalizeReportPage(request.Page, request.PageSize)
	query := db.Model(&model.ReportScheduleRun{}).Where("tenant_id = ? AND schedule_id = ?", tenantID, scheduleID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var runs []*model.ReportScheduleRun
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&runs).Error; err != nil {
		return nil, err
	}
	deliveries, err := reportRunListDeliveries(db, tenantID, runs)
	if err != nil {
		return nil, err
	}
	responses := make([]*model.ReportRunResponse, 0, len(runs))
	for _, run := range runs {
		// 无投递行时传 nil，ToResponse 投影为 pending（与逐行 Take 的 NotFound 分支一致）。
		responses = append(responses, run.ToResponse(deliveries[run.ID]))
	}
	return &model.ReportRunListResponse{List: responses, Total: total, Page: page, PageSize: pageSize}, nil
}

// reportRunListDeliveryColumns 是 ReportScheduleRun.ToResponse 实际读取的投递列。
// 列表页刻意不取 payload（bytea 报表正文）与信封字段，避免每页把整份报表读进内存。
var reportRunListDeliveryColumns = []string{"run_id", "status", "attempt_count", "last_error", "completed_at"}

// reportRunListDeliveries 以一条 IN 查询批量取当前页全部 run 的投递状态（替代逐 run Take 的 N+1）。
// run_id 是 report_schedule_deliveries 主键，故每个 run 至多一行。
func reportRunListDeliveries(db *gorm.DB, tenantID string, runs []*model.ReportScheduleRun) (map[string]*model.ReportScheduleDelivery, error) {
	result := make(map[string]*model.ReportScheduleDelivery, len(runs))
	if len(runs) == 0 {
		return result, nil
	}
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		runIDs = append(runIDs, run.ID)
	}
	var rows []*model.ReportScheduleDelivery
	if err := db.Select(reportRunListDeliveryColumns).
		Where("tenant_id = ? AND run_id IN ?", tenantID, runIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.RunID] = row
	}
	return result, nil
}

func GetReportRunContext(ctx context.Context, tenantID, scheduleID, runID string) (*model.ReportRunResponse, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	var run model.ReportScheduleRun
	if err := db.Where("id = ? AND schedule_id = ? AND tenant_id = ?", runID, scheduleID, tenantID).Take(&run).Error; err != nil {
		return nil, err
	}
	var delivery model.ReportScheduleDelivery
	err = db.Where("run_id = ? AND tenant_id = ?", runID, tenantID).Take(&delivery).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return run.ToResponse(nil), nil
	}
	if err != nil {
		return nil, err
	}
	return run.ToResponse(&delivery), nil
}

func ensureReportScheduleTenantScope(db *gorm.DB, tenantID, scheduleID string) error {
	var count int64
	if err := db.Model(&model.ReportSchedule{}).Unscoped().Where("id = ? AND tenant_id = ?", scheduleID, tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
