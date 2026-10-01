package dal

import (
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

var ErrAmbiguousCommandJobDetailResponse = errors.New("ambiguous command job detail response match")

// maxInternalCommandJobScanLimit 内部调度/恢复辅助查询的单次扫描上限。
// 仅约束后台 worker 的批量扫描，防止调用方误传超大 limit 造成无界查询；
// 公开分页列表与详情接口不使用该上限（它们应做真分页）。
const maxInternalCommandJobScanLimit = 500

// clampInternalCommandJobScanLimit 收敛内部扫描的 limit：非正数回退默认 100，超过上限截断。
func clampInternalCommandJobScanLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > maxInternalCommandJobScanLimit {
		return maxInternalCommandJobScanLimit
	}
	return limit
}

// commandJobDetailInlineLimit 任务详情/支持包内联读取 detail 行的单次上限，
// 与 maxInternalCommandJobScanLimit 保持一致；超大任务的完整行集应走分页 rows 接口。
const commandJobDetailInlineLimit = 500

// clampCommandJobDetailInlineLimit 收敛详情/支持包内联读取的 limit：
// 非正数回退到内联上限本身（调用方未显式给量时按完整上限读取），超过上限截断。
func clampCommandJobDetailInlineLimit(limit int) int {
	if limit <= 0 {
		return commandJobDetailInlineLimit
	}
	if limit > commandJobDetailInlineLimit {
		return commandJobDetailInlineLimit
	}
	return limit
}

func CreateCommandJobWithDetails(job *model.CommandJob, details []*model.CommandJobDetail) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		if len(details) == 0 {
			return nil
		}
		return tx.CreateInBatches(details, 500).Error
	})
}

func PreviewCommandJobDeviceFilter(req *model.GetDeviceListByPageReq, tenantID string) (int64, []model.GetDeviceListByPageRsp, error) {
	return GetDeviceListByPage(req, tenantID)
}

func GetCommandJobByID(jobID, tenantID string) (*model.CommandJob, error) {
	var job model.CommandJob
	err := global.DB.
		Where("id = ? AND tenant_id = ?", jobID, tenantID).
		First(&job).Error
	return &job, err
}

const (
	// defaultCommandJobListPageSize 公开列表缺省页大小，与 service 层分页契约保持一致。
	defaultCommandJobListPageSize = 10
	// maxCommandJobListPageSize 公开列表单页上限，防止超大 page_size 造成无界查询。
	maxCommandJobListPageSize = 50
)

// clampCommandJobListPage 收敛公开列表分页参数：非正数回退默认值，超过上限截断。
// DAL 边界兜底，保证即使调用方漏做归一化也不会产生无界 Offset/Limit。
func clampCommandJobListPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultCommandJobListPageSize
	}
	if pageSize > maxCommandJobListPageSize {
		pageSize = maxCommandJobListPageSize
	}
	return page, pageSize
}

// ListCommandJobs 分页返回作用域内的命令任务（ROADMAP C2 自上而下读）。
// scopes 语义：0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN。
// 设备筛选预览/提交路径仍锚定操作员本租户（C2 仅放列表读，不扩大向子树设备下发命令的写路径）。
// tenant-scope: scopes 由 service 层展开并校验（TENANT_ADMIN/SYS_ADMIN self∪子孙；
// TENANT_USER 保持 self-only；空租户由 service 映射为 [""]，提交路径本就拒绝空租户）。
func ListCommandJobs(scopes []string, status, search, attentionFilter string, page, pageSize int, maxAttempts int, now time.Time) (int64, []*model.CommandJob, error) {
	if len(scopes) == 0 {
		return 0, nil, nil
	}
	page, pageSize = clampCommandJobListPage(page, pageSize)
	var total int64
	var jobs []*model.CommandJob
	query := commandJobListBaseQuery(scopes, status, search, attentionFilter, maxAttempts, now)
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	err := query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&jobs).Error
	return total, jobs, err
}

func commandJobListBaseQuery(scopes []string, status, search, attentionFilter string, maxAttempts int, now time.Time) *gorm.DB {
	query := global.DB.Model(&model.CommandJob{})
	switch len(scopes) {
	case 1:
		query = query.Where("tenant_id = ?", scopes[0])
	default:
		query = query.Where("tenant_id IN ?", scopes)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query = applyCommandJobListAttentionFilter(query, attentionFilter, maxAttempts, now)
	return whereKeywordContains(query, opLike, strings.ToLower(search), "LOWER(id)", "LOWER(identify)")
}

// tenant-scope: system-internal?2026-08-26 ?????
func ListTimedOutRunningCommandJobs(now time.Time, limit int) ([]*model.CommandJob, error) {
	return listTimedOutRunningCommandJobs("", now, limit)
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func ListTimedOutRunningCommandJobsForTenant(tenantID string, now time.Time, limit int) ([]*model.CommandJob, error) {
	return listTimedOutRunningCommandJobs(tenantID, now, limit)
}

func ListRunnableCommandJobs(now time.Time, detailStatuses []string, limit int) ([]*model.CommandJob, error) {
	limit = clampInternalCommandJobScanLimit(limit)
	if len(detailStatuses) == 0 {
		return []*model.CommandJob{}, nil
	}
	var jobs []*model.CommandJob
	err := global.DB.Model(&model.CommandJob{}).
		Where(
			"((status = ?) OR (status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?)) AND (timeout_at IS NULL OR timeout_at > ?)",
			"running",
			"scheduled",
			now,
			now,
		).
		Where("next_dispatch_at IS NULL OR next_dispatch_at <= ?", now).
		Where(
			"EXISTS (SELECT 1 FROM "+model.TableNameCommandJobDetail+" d WHERE d.command_job_id = "+model.TableNameCommandJob+".id AND d.tenant_id = "+model.TableNameCommandJob+".tenant_id AND d.status IN ? AND d.eligible = ?)",
			detailStatuses,
			true,
		).
		Order("COALESCE(next_dispatch_at, updated_at) ASC, created_at ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func listTimedOutRunningCommandJobs(tenantID string, now time.Time, limit int) ([]*model.CommandJob, error) {
	limit = clampInternalCommandJobScanLimit(limit)
	var jobs []*model.CommandJob
	query := global.DB.
		Where("status IN ? AND timeout_at IS NOT NULL AND timeout_at <= ?", []string{"running", "scheduled"}, now).
		Order("timeout_at ASC")
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func ActivateScheduledCommandJob(jobID, tenantID string, now time.Time) (bool, error) {
	result := global.DB.Model(&model.CommandJob{}).
		Where(
			"id = ? AND tenant_id = ? AND status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ? AND (timeout_at IS NULL OR timeout_at > ?)",
			jobID,
			tenantID,
			"scheduled",
			now,
			now,
		).
		Updates(map[string]interface{}{
			"status":           "running",
			"next_dispatch_at": nil,
			"updated_at":       now,
		})
	return result.RowsAffected == 1, result.Error
}
