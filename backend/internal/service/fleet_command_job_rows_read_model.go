package service

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
)

func normalizeFleetCommandJobRowsReq(req *model.FleetCommandJobRowsReq) *model.FleetCommandJobRowsReq {
	if req == nil {
		req = &model.FleetCommandJobRowsReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultFleetCommandJobRowsPageSize
	}
	if req.PageSize > maxFleetCommandJobRowsPageSize {
		req.PageSize = maxFleetCommandJobRowsPageSize
	}
	req.StatusFilter = normalizeFleetCommandJobRowsStatusFilter(req.StatusFilter)
	req.Search = normalizeFleetCommandJobRowsSearch(req.Search)
	return req
}

func commandJobRowsResultFromPersistence(
	total int64,
	req *model.FleetCommandJobRowsReq,
	details []*model.CommandJobDetail,
) *model.FleetCommandJobRowsResult {
	return &model.FleetCommandJobRowsResult{
		Total:         total,
		Page:          req.Page,
		PageSize:      req.PageSize,
		StatusFilter:  req.StatusFilter,
		Search:        req.Search,
		Rows:          commandJobRowsFromDetails(details),
		RowsTruncated: int64(req.Page*req.PageSize) < total,
	}
}

func normalizeFleetCommandJobRowsSearch(search string) string {
	search = strings.TrimSpace(search)
	// Limit by Unicode code points rather than bytes. Slicing a UTF-8 string at
	// an arbitrary byte boundary produces invalid text and makes the echoed
	// search contract differ from the user's query.
	runes := []rune(search)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return search
}

func normalizeFleetCommandJobRowsStatusFilter(statusFilter string) string {
	switch strings.TrimSpace(statusFilter) {
	case "needs_attention", "retryable", "retry_ready", "retry_waiting", "retry_exhausted", "device_failed", "failed", "missing_log", "in_progress", "canceled":
		return strings.TrimSpace(statusFilter)
	default:
		return "all"
	}
}
