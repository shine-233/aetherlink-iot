// 文件用途：批次命令作业明细（command_job_details）的读路径：分页、筛选、计数与支持诊断查询。
// 关键注意事项：所有查询均带 command_job_id + tenant_id 双锚点；关键词检索走 whereKeywordContains（已转义）。
package dal

import (
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	return whereKeywordContains(query, opLike, strings.ToLower(search), commandJobDetailSearchColumns...)
}

// commandJobDetailSearchColumns 命令任务明细关键词检索覆盖的投影列（大小写不敏感）。
var commandJobDetailSearchColumns = []string{
	"LOWER(device_id)", "LOWER(device_number)", "LOWER(name)", "LOWER(message_id)",
	"LOWER(response_error)", "LOWER(response_payload)", "LOWER(reason)", "LOWER(advice)",
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
