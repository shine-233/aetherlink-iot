package dal

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	search = strings.TrimSpace(search)
	if search != "" {
		keyword := "%" + strings.ToLower(search) + "%"
		query = query.Where("(LOWER(id) LIKE ? OR LOWER(identify) LIKE ?)", keyword, keyword)
	}
	return query
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

func GetCommandJobDetails(jobID, tenantID string, limit int) ([]*model.CommandJobDetail, error) {
	var details []*model.CommandJobDetail
	err := global.DB.
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Order("created_at ASC, id ASC").
		Limit(clampCommandJobDetailInlineLimit(limit)).
		Find(&details).Error
	return details, err
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func CountCommandJobDetails(jobID, tenantID string) (int64, error) {
	return CountCommandJobDetailsByFilter(jobID, tenantID, "", "", 0, time.Now().UTC())
}

func CountCommandJobDetailsByFilter(jobID, tenantID, statusFilter, search string, maxAttempts int, now time.Time) (int64, error) {
	var total int64
	err := applyCommandJobRowsFilter(
		global.DB.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID),
		statusFilter,
		search,
		maxAttempts,
		now,
	).
		Count(&total).Error
	return total, err
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandJobDetailsByPage(jobID, tenantID string, page, pageSize int) ([]*model.CommandJobDetail, error) {
	return GetCommandJobDetailsByPageAndFilter(jobID, tenantID, page, pageSize, "", "", 0, time.Now().UTC())
}

func GetCommandJobDetailsByPageAndFilter(jobID, tenantID string, page, pageSize int, statusFilter, search string, maxAttempts int, now time.Time) ([]*model.CommandJobDetail, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	var details []*model.CommandJobDetail
	err := applyCommandJobRowsFilter(
		global.DB.Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID),
		statusFilter,
		search,
		maxAttempts,
		now,
	).
		Order(clause.Expr{
			SQL: `CASE
				WHEN can_retry = ? THEN 0
				WHEN status = ? THEN 1
				WHEN status = ? AND log_recorded = ? THEN 2
				WHEN status = ? THEN 3
				WHEN status IN ? THEN 4
				WHEN status = ? THEN 5
				ELSE 6
			END ASC`,
			Vars: []interface{}{
				true,
				"failed",
				"submitted",
				false,
				"blocked",
				[]string{"ready", "dispatching"},
				"canceled",
			},
		}).
		Order("updated_at DESC, created_at ASC, id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&details).Error
	return details, err
}

func applyCommandJobRowsFilter(query *gorm.DB, statusFilter, search string, maxAttempts int, now time.Time) *gorm.DB {
	if predicate, vars := commandJobListAttentionPredicate("", statusFilter, maxAttempts, now); predicate != "" {
		query = query.Where(predicate, vars...)
	}

	search = strings.TrimSpace(search)
	if search == "" {
		return query
	}
	keyword := "%" + strings.ToLower(search) + "%"
	return query.Where(
		`(LOWER(device_id) LIKE ?
			OR LOWER(device_number) LIKE ?
			OR LOWER(name) LIKE ?
			OR LOWER(message_id) LIKE ?
			OR LOWER(response_error) LIKE ?
			OR LOWER(response_payload) LIKE ?
			OR LOWER(reason) LIKE ?
			OR LOWER(advice) LIKE ?)`,
		keyword, keyword, keyword, keyword,
		keyword, keyword, keyword, keyword,
	)
}

func applyCommandJobListAttentionFilter(query *gorm.DB, attentionFilter string, maxAttempts int, now time.Time) *gorm.DB {
	predicate, vars := commandJobListAttentionPredicate("d", attentionFilter, maxAttempts, now)
	if predicate == "" {
		return query
	}
	return query.Where(
		"EXISTS (SELECT 1 FROM "+model.TableNameCommandJobDetail+" d WHERE d.command_job_id = "+model.TableNameCommandJob+".id AND d.tenant_id = "+model.TableNameCommandJob+".tenant_id AND "+predicate+")",
		vars...,
	)
}

func commandJobListAttentionPredicate(alias, attentionFilter string, maxAttempts int, now time.Time) (string, []interface{}) {
	column := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	switch strings.TrimSpace(attentionFilter) {
	case "retry_ready":
		return column("status") + " = ? AND " + column("can_retry") + " = ? AND " + column("dispatch_attempts") + " < ? AND (" + column("next_retry_after") + " IS NULL OR " + column("next_retry_after") + " <= ?)",
			[]interface{}{"failed", true, maxAttempts, now}
	case "retry_waiting":
		return column("status") + " = ? AND " + column("can_retry") + " = ? AND " + column("dispatch_attempts") + " < ? AND " + column("next_retry_after") + " IS NOT NULL AND " + column("next_retry_after") + " > ?",
			[]interface{}{"failed", true, maxAttempts, now}
	case "retry_exhausted":
		return column("status") + " = ? AND " + column("can_retry") + " = ? AND " + column("dispatch_attempts") + " >= ?",
			[]interface{}{"failed", true, maxAttempts}
	default:
		return commandJobDetailAttentionPredicate(alias, attentionFilter)
	}
}

func commandJobDetailAttentionPredicate(alias, attentionFilter string) (string, []interface{}) {
	column := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	switch strings.TrimSpace(attentionFilter) {
	case "needs_operator_action", "needs_attention":
		return "(" + column("can_retry") + " = ? OR " + column("status") + " = ? OR (" + column("status") + " = ? AND " + column("log_recorded") + " = ?) OR " + column("status") + " IN ? OR " + column("response_status") + " = ?)",
			[]interface{}{true, "failed", "submitted", false, []string{"blocked", "canceled"}, strconv.Itoa(constant.ResponseSStatusFailed)}
	case "retryable":
		return column("status") + " = ? AND " + column("can_retry") + " = ?", []interface{}{"failed", true}
	case "failed":
		return "(" + column("status") + " IN ? OR " + column("response_status") + " = ?)", []interface{}{[]string{"failed", "blocked"}, strconv.Itoa(constant.ResponseSStatusFailed)}
	case "missing_log":
		return column("status") + " = ? AND " + column("log_recorded") + " = ?", []interface{}{"submitted", false}
	case "device_failed":
		return column("response_status") + " = ?", []interface{}{strconv.Itoa(constant.ResponseSStatusFailed)}
	case "blocked":
		return column("status") + " = ?", []interface{}{"blocked"}
	case "in_progress":
		return column("status") + " IN ?", []interface{}{[]string{"ready", "dispatching", "submitted"}}
	case "canceled":
		return column("status") + " = ?", []interface{}{"canceled"}
	default:
		return "", nil
	}
}

func FindCommandJobSupportDetails(jobID, tenantID string, includeInFlight bool, limit int) ([]*model.CommandJobDetail, error) {
	var details []*model.CommandJobDetail
	supportStatuses := []string{"failed", "blocked", "canceled"}
	if includeInFlight {
		supportStatuses = append(supportStatuses, "dispatching")
	}
	err := global.DB.
		Where(
			"command_job_id = ? AND tenant_id = ? AND (can_retry = ? OR (status = ? AND log_recorded = ?) OR status IN ? OR response_status = ?)",
			jobID,
			tenantID,
			true,
			"submitted",
			false,
			supportStatuses,
			strconv.Itoa(constant.ResponseSStatusFailed),
		).
		Order("updated_at ASC, created_at ASC").
		Limit(clampCommandJobDetailInlineLimit(limit)).
		Find(&details).Error
	return details, err
}

func UpdateCommandJob(job *model.CommandJob) error {
	return global.DB.Save(job).Error
}

func UpdateCommandJobDetail(detail *model.CommandJobDetail) error {
	return global.DB.Save(detail).Error
}

func UpdateClaimedCommandJobDetailAfterDispatch(detail *model.CommandJobDetail, leaseToken string) (int64, error) {
	if detail == nil || detail.ID == "" || detail.TenantID == "" || leaseToken == "" {
		return 0, nil
	}
	updates := map[string]interface{}{
		"status":               detail.Status,
		"message_id":           detail.MessageID,
		"log_recorded":         detail.LogRecorded,
		"reason":               detail.Reason,
		"can_retry":            detail.CanRetry,
		"dispatch_lease_token": nil,
		"dispatch_lease_until": nil,
		"next_retry_after":     detail.NextRetryAfter,
		"updated_at":           detail.UpdatedAt,
		"submitted_at":         detail.SubmittedAt,
		"completed_at":         detail.CompletedAt,
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("id = ? AND tenant_id = ? AND status = ? AND dispatch_lease_token = ?", detail.ID, detail.TenantID, "dispatching", leaseToken).
		Updates(updates)
	return result.RowsAffected, result.Error
}

func UpdateCommandJobDetailResponseByMessageID(deviceID, messageID, status, payload, responseError string, responseAt time.Time) (*model.CommandJobDetail, int64, error) {
	if deviceID == "" || messageID == "" {
		return nil, 0, nil
	}
	matches, err := FindCommandJobDetailResponseCandidates(deviceID, messageID, 2)
	if err != nil {
		return nil, 0, err
	}
	if len(matches) == 0 {
		return nil, 0, nil
	}
	if commandJobDetailResponseMatchIsAmbiguous(len(matches)) {
		return nil, 0, ErrAmbiguousCommandJobDetailResponse
	}
	detail := *matches[0]
	if commandJobDetailResponseIsStale(detail.ResponseAt, responseAt) {
		return nil, 0, nil
	}
	updates := map[string]interface{}{
		"response_status":  status,
		"response_payload": payload,
		"response_at":      responseAt,
		"updated_at":       responseAt,
	}
	if responseError != "" {
		updates["response_error"] = responseError
	} else {
		updates["response_error"] = nil
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("id = ? AND tenant_id = ? AND (response_at IS NULL OR response_at <= ?)", detail.ID, detail.TenantID, responseAt).
		Updates(updates)
	if result.Error != nil {
		return nil, result.RowsAffected, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, 0, nil
	}
	detail.ResponseStatus = &status
	detail.ResponsePayload = &payload
	detail.ResponseAt = &responseAt
	if responseError != "" {
		detail.ResponseError = &responseError
	} else {
		detail.ResponseError = nil
	}
	return &detail, result.RowsAffected, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func FindCommandJobDetailResponseCandidates(deviceID, messageID string, limit int) ([]*model.CommandJobDetail, error) {
	if deviceID == "" || messageID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 2
	}
	var matches []*model.CommandJobDetail
	err := global.DB.
		Where("device_id = ? AND message_id = ?", deviceID, messageID).
		Order("created_at DESC").
		Limit(limit).
		Find(&matches).Error
	return matches, err
}

func commandJobDetailResponseMatchIsAmbiguous(matchCount int) bool {
	return matchCount > 1
}

func commandJobDetailResponseIsStale(current *time.Time, incoming time.Time) bool {
	return current != nil && current.After(incoming)
}

func UpdateCommandJobDetailsStatus(jobID, tenantID string, fromStatuses []string, toStatus, reason string, canRetry bool) (int64, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":               toStatus,
		"reason":               reason,
		"can_retry":            canRetry,
		"dispatch_lease_token": nil,
		"dispatch_lease_until": nil,
		"next_retry_after":     nil,
		"updated_at":           now,
		"completed_at":         now,
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status IN ?", jobID, tenantID, fromStatuses).
		Updates(updates)
	return result.RowsAffected, result.Error
}

func commandJobNextRetryAfterExpression(maxAttempts int, nextRetryAfter time.Time) clause.Expr {
	return gorm.Expr(
		"CASE WHEN dispatch_attempts < ? THEN CAST(? AS timestamptz) ELSE NULL END",
		maxAttempts,
		nextRetryAfter,
	)
}

func FailRecoverableCommandJobDetailsWithRetryPolicy(jobID, tenantID string, fromStatuses []string, reason string, maxAttempts int, nextRetryAfter time.Time, now time.Time) (int64, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status IN ? AND eligible = ?", jobID, tenantID, fromStatuses, true).
		Updates(map[string]interface{}{
			"status":               "failed",
			"reason":               reason,
			"can_retry":            gorm.Expr("dispatch_attempts < ?", maxAttempts),
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     commandJobNextRetryAfterExpression(maxAttempts, nextRetryAfter),
			"advice": gorm.Expr(
				"CASE WHEN dispatch_attempts < ? THEN ? ELSE ? END",
				maxAttempts,
				"Retry becomes available after a short backoff; review the failure reason before retrying.",
				"Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
			),
			"updated_at":   now,
			"completed_at": now,
		})
	return result.RowsAffected, result.Error
}

func FailTimedOutCommandJobDetailsWithRetryPolicy(jobID, tenantID string, maxAttempts int, nextRetryAfter time.Time, now time.Time) (int64, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var affected int64
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		preserved := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND message_id IS NOT NULL AND message_id <> ''", jobID, tenantID, "dispatching", true).
			Updates(map[string]interface{}{
				"status":               "failed",
				"reason":               "job timed out after this command row received a message id; confirm command logs and device state before creating a new attempt",
				"can_retry":            false,
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     nil,
				"advice":               "A message id was already allocated before timeout; confirm command logs and device state before creating a fresh attempt.",
				"updated_at":           now,
				"completed_at":         now,
			})
		if preserved.Error != nil {
			return preserved.Error
		}
		affected += preserved.RowsAffected

		recoverable := tx.Model(&model.CommandJobDetail{}).
			Where(
				"command_job_id = ? AND tenant_id = ? AND eligible = ? AND (status = ? OR (status = ? AND (message_id IS NULL OR message_id = '')))",
				jobID,
				tenantID,
				true,
				"ready",
				"dispatching",
			).
			Updates(map[string]interface{}{
				"status":               "failed",
				"message_id":           nil,
				"log_recorded":         false,
				"reason":               "job timed out before command delivery completed",
				"can_retry":            gorm.Expr("dispatch_attempts < ?", maxAttempts),
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     commandJobNextRetryAfterExpression(maxAttempts, nextRetryAfter),
				"advice": gorm.Expr(
					"CASE WHEN dispatch_attempts < ? THEN ? ELSE ? END",
					maxAttempts,
					"Retry becomes available after a short backoff; review the timeout evidence before retrying.",
					"Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
				),
				"updated_at":   now,
				"submitted_at": nil,
				"completed_at": now,
			})
		if recoverable.Error != nil {
			return recoverable.Error
		}
		affected += recoverable.RowsAffected
		return nil
	})
	return affected, err
}

func FailInterruptedCommandJobDetails(jobID, tenantID string, maxAttempts int, nextRetryAfter time.Time, now time.Time) (int64, error) {
	preserved := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND message_id IS NOT NULL AND message_id <> '' AND (dispatch_lease_until IS NULL OR dispatch_lease_until <= ?)", jobID, tenantID, "dispatching", true, now).
		Updates(map[string]interface{}{
			"status":               "failed",
			"reason":               "backend restarted after this command row received a message id; confirm device state and command logs before creating a new attempt",
			"can_retry":            false,
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     nil,
			"advice":               "A message id was already allocated before restart; confirm command logs and device state before creating a fresh attempt.",
			"updated_at":           now,
			"completed_at":         now,
		})
	if preserved.Error != nil {
		return 0, preserved.Error
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND (message_id IS NULL OR message_id = '') AND (dispatch_lease_until IS NULL OR dispatch_lease_until <= ?)", jobID, tenantID, "dispatching", true, now).
		Updates(map[string]interface{}{
			"status":               "failed",
			"message_id":           nil,
			"log_recorded":         false,
			"reason":               "backend restarted while this command row was dispatching; confirm device state before retry",
			"can_retry":            gorm.Expr("dispatch_attempts < ?", maxAttempts),
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     commandJobNextRetryAfterExpression(maxAttempts, nextRetryAfter),
			"advice": gorm.Expr(
				"CASE WHEN dispatch_attempts < ? THEN ? ELSE ? END",
				maxAttempts,
				"Retry becomes available after a short backoff; review the restart evidence before retrying.",
				"Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
			),
			"updated_at":   now,
			"submitted_at": nil,
			"completed_at": now,
		})
	return preserved.RowsAffected + result.RowsAffected, result.Error
}

func CountCommandJobDetailsByStatus(jobID, tenantID string) (map[string]int, error) {
	type row struct {
		Status string
		Count  int
	}
	var rows []row
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select("status, count(*) as count").
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := newCommandJobDetailStatusCounts()
	for _, item := range rows {
		counts[item.Status] = item.Count
	}
	return counts, nil
}

// newCommandJobDetailStatusCounts keeps the status-count response shape
// stable when a job has no rows in one or more lifecycle states. Consumers use
// these keys to render the same summary fields for every command job.
func newCommandJobDetailStatusCounts() map[string]int {
	return map[string]int{
		"ready":       0,
		"dispatching": 0,
		"submitted":   0,
		"failed":      0,
		"blocked":     0,
		"canceled":    0,
	}
}

type CommandJobSummaryMetrics struct {
	StatusCounts        map[string]int
	RetryableCount      int
	RetryReadyCount     int
	RetryWaitingCount   int
	RetryExhaustedCount int
	LogMissingCount     int
}

func normalizeCommandJobMaxAttempts(maxAttempts int) int {
	if maxAttempts <= 0 {
		return 1
	}
	return maxAttempts
}

func commandJobRetryMetricsSelect() string {
	return strings.Join([]string{
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? THEN 1 ELSE 0 END), 0) AS retryable_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts < ? AND (next_retry_after IS NULL OR next_retry_after <= ?) THEN 1 ELSE 0 END), 0) AS retry_ready_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts < ? AND next_retry_after IS NOT NULL AND next_retry_after > ? THEN 1 ELSE 0 END), 0) AS retry_waiting_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts >= ? THEN 1 ELSE 0 END), 0) AS retry_exhausted_count",
		"COALESCE(SUM(CASE WHEN status = ? AND log_recorded = ? THEN 1 ELSE 0 END), 0) AS log_missing_count",
	}, ",\n")
}

func commandJobRetryMetricsArgs(maxAttempts int, now time.Time) []interface{} {
	maxAttempts = normalizeCommandJobMaxAttempts(maxAttempts)
	return []interface{}{
		"failed",
		true,
		"failed",
		true,
		maxAttempts,
		now,
		"failed",
		true,
		maxAttempts,
		now,
		"failed",
		false,
		maxAttempts,
		"submitted",
		false,
	}
}

func GetCommandJobSummaryMetrics(jobID, tenantID string, maxAttempts int, now time.Time) (CommandJobSummaryMetrics, error) {
	type row struct {
		ReadyCount          int
		DispatchingCount    int
		SubmittedCount      int
		FailedCount         int
		BlockedCount        int
		CanceledCount       int
		RetryableCount      int
		RetryReadyCount     int
		RetryWaitingCount   int
		RetryExhaustedCount int
		LogMissingCount     int
	}
	var item row
	selectSQL := `COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS ready_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS dispatching_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS submitted_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS blocked_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS canceled_count,
			` + commandJobRetryMetricsSelect()
	selectArgs := append(
		[]interface{}{"ready", "dispatching", "submitted", "failed", "blocked", "canceled"},
		commandJobRetryMetricsArgs(maxAttempts, now)...,
	)
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select(selectSQL, selectArgs...).
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Scan(&item).Error
	if err != nil {
		return CommandJobSummaryMetrics{}, err
	}
	return CommandJobSummaryMetrics{
		StatusCounts: map[string]int{
			"ready":       item.ReadyCount,
			"dispatching": item.DispatchingCount,
			"submitted":   item.SubmittedCount,
			"failed":      item.FailedCount,
			"blocked":     item.BlockedCount,
			"canceled":    item.CanceledCount,
		},
		RetryableCount:      item.RetryableCount,
		RetryReadyCount:     item.RetryReadyCount,
		RetryWaitingCount:   item.RetryWaitingCount,
		RetryExhaustedCount: item.RetryExhaustedCount,
		LogMissingCount:     item.LogMissingCount,
	}, nil
}

type CommandJobRequeueResult struct {
	Requeued    int64
	CoolingDown int64
	Exhausted   int64
}

func RequeueRetryableCommandJobDetails(jobID, tenantID string, maxAttempts int, now time.Time) (CommandJobRequeueResult, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	result := CommandJobRequeueResult{}
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		exhausted := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts >= ?", jobID, tenantID, "failed", true, maxAttempts).
			Updates(map[string]interface{}{
				"can_retry":        false,
				"next_retry_after": nil,
				"advice":           "Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
				"updated_at":       now,
			})
		if exhausted.Error != nil {
			return exhausted.Error
		}
		result.Exhausted = exhausted.RowsAffected

		if err := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts < ? AND next_retry_after IS NOT NULL AND next_retry_after > ?", jobID, tenantID, "failed", true, maxAttempts, now).
			Count(&result.CoolingDown).Error; err != nil {
			return err
		}

		requeued := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts < ? AND (next_retry_after IS NULL OR next_retry_after <= ?)", jobID, tenantID, "failed", true, maxAttempts, now).
			Updates(map[string]interface{}{
				"status":               "ready",
				"message_id":           nil,
				"log_recorded":         false,
				"reason":               "queued for retry",
				"can_retry":            false,
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     nil,
				"updated_at":           now,
				"submitted_at":         nil,
				"completed_at":         nil,
			})
		if requeued.Error != nil {
			return requeued.Error
		}
		result.Requeued = requeued.RowsAffected
		return nil
	})
	return result, err
}

func RequeueAllRetryableCommandJobDetails(jobID, tenantID string) (int64, error) {
	now := time.Now().UTC()
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ?", jobID, tenantID, "failed", true).
		Updates(map[string]interface{}{
			"status":               "ready",
			"message_id":           nil,
			"log_recorded":         false,
			"reason":               "queued for retry",
			"can_retry":            false,
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     nil,
			"updated_at":           now,
			"submitted_at":         nil,
			"completed_at":         nil,
		})
	return result.RowsAffected, result.Error
}

func CreateCommandJobEvent(event *model.CommandJobEvent) error {
	return global.DB.Create(event).Error
}

func GetCommandJobEvents(jobID, tenantID string) ([]*model.CommandJobEvent, error) {
	var events []*model.CommandJobEvent
	err := global.DB.
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}

func GetRecentCommandJobEvents(jobID, tenantID string, limit int) ([]*model.CommandJobEvent, error) {
	if limit <= 0 {
		return GetCommandJobEvents(jobID, tenantID)
	}
	var events []*model.CommandJobEvent
	err := global.DB.
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	return events, nil
}
