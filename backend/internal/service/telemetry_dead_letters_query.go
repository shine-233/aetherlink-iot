package service

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/authz"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// Read side of telemetry dead letters: scoping, predicates, pagination and
// response mapping. Nothing in this file mutates rows.

func (*TelemetryData) GetTelemetryDeadLetterList(req *model.GetTelemetryDeadLetterListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireTelemetryClaims(claims, telemetryReadPermissionMessage); err != nil {
		return nil, err
	}

	query := telemetryDeadLetterScopedQuery(req, claims)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, telemetryDeadLetterDBError(err, nil)
	}

	page, pageSize := normalizeTelemetryDeadLetterPage(req.Page, req.PageSize)
	var rows []storage.TelemetryDeadLetter
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, telemetryDeadLetterDBError(err, nil)
	}

	return map[string]interface{}{
		"total": total,
		"list":  buildTelemetryDeadLetterList(rows),
	}, nil
}

// telemetryDeadLetterScopedQuery restricts dead letters to what the caller may
// see: own tenant for non-sysadmins (optional tenant filter for sysadmins) and
// only owned devices for tenant users, then applies the optional filters.
func telemetryDeadLetterScopedQuery(req *model.GetTelemetryDeadLetterListReq, claims *utils.UserClaims) *gorm.DB {
	query := global.DB.Model(&storage.TelemetryDeadLetter{})
	if !authz.IsSysAdmin(claims) {
		query = query.Where("tenant_id = ?", claims.TenantID)
	} else if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if authz.HasRole(claims, authz.TenantUser) {
		query = query.Where(
			"EXISTS (SELECT 1 FROM devices d WHERE d.id = telemetry_dead_letters.device_id AND d.tenant_id = telemetry_dead_letters.tenant_id AND d.owner_user_id = ?)",
			strings.TrimSpace(claims.ID),
		)
	}
	if deviceID := strings.TrimSpace(req.DeviceID); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if key := strings.TrimSpace(req.Key); key != "" {
		query = query.Where(`"key" = ?`, key)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	return query
}

// telemetryDeadLetterReadyDrainQuery is the scoped set of rows an operator
// drain request may claim right now.
func telemetryDeadLetterReadyDrainQuery(req *model.DrainTelemetryDeadLetterReq, claims *utils.UserClaims, now time.Time) *gorm.DB {
	listReq := &model.GetTelemetryDeadLetterListReq{
		TenantID: req.TenantID,
		DeviceID: req.DeviceID,
		Key:      req.Key,
	}
	return telemetryDeadLetterReadyQuery(telemetryDeadLetterScopedQuery(listReq, claims), now)
}

// telemetryDeadLetterReadyQuery selects rows a drain should pick up. It is
// intentionally identical to the claim predicate so that a row counted as
// "ready" is always one the claim CAS can win.
func telemetryDeadLetterReadyQuery(query *gorm.DB, now time.Time) *gorm.DB {
	return telemetryDeadLetterClaimableQuery(query, now)
}

// telemetryDeadLetterClaimableQuery is the claim predicate: pending/retrying
// rows, or processing rows whose lease expired, that still have attempts left
// and whose backoff has elapsed.
func telemetryDeadLetterClaimableQuery(query *gorm.DB, now time.Time) *gorm.DB {
	staleProcessingBefore := now.Add(-telemetryDeadLetterProcessingTimeout)
	return query.
		Where(
			"(status IN ? OR (status = ? AND updated_at <= ?))",
			[]string{storage.TelemetryDeadLetterStatusPending, storage.TelemetryDeadLetterStatusRetrying},
			storage.TelemetryDeadLetterStatusProcessing,
			staleProcessingBefore,
		).
		Where("attempts < ?", storage.TelemetryDeadLetterMaxAttempts()).
		Where("(next_retry_at IS NULL OR next_retry_at <= ?)", now)
}

func normalizeTelemetryDeadLetterDrainLimit(limit int) int {
	if limit < 1 {
		return telemetryDeadLetterDefaultDrainLimit
	}
	if limit > telemetryDeadLetterMaxDrainLimit {
		return telemetryDeadLetterMaxDrainLimit
	}
	return limit
}

func normalizeTelemetryDeadLetterPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = telemetryDeadLetterDefaultPageSize
	}
	if pageSize > telemetryDeadLetterMaxPageSize {
		pageSize = telemetryDeadLetterMaxPageSize
	}
	return page, pageSize
}

func buildTelemetryDeadLetterList(rows []storage.TelemetryDeadLetter) []model.TelemetryDeadLetterRsp {
	list := make([]model.TelemetryDeadLetterRsp, 0, len(rows))
	for _, row := range rows {
		list = append(list, buildTelemetryDeadLetterRsp(row))
	}
	return list
}

func buildTelemetryDeadLetterRsp(row storage.TelemetryDeadLetter) model.TelemetryDeadLetterRsp {
	return model.TelemetryDeadLetterRsp{
		ID:          row.ID,
		DeviceID:    row.DeviceID,
		TenantID:    row.TenantID,
		Key:         row.Key,
		TS:          row.TS,
		BoolV:       row.BoolV,
		NumberV:     row.NumberV,
		StringV:     row.StringV,
		Status:      row.Status,
		Attempts:    row.Attempts,
		LastError:   row.LastError,
		NextRetryAt: formatOptionalTelemetryDeadLetterTime(row.NextRetryAt),
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   row.UpdatedAt.Format(time.RFC3339),
	}
}

func formatOptionalTelemetryDeadLetterTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func getTelemetryDeadLetterForAccess(id string) (storage.TelemetryDeadLetter, error) {
	var row storage.TelemetryDeadLetter
	if err := global.DB.Where("id = ?", id).First(&row).Error; err != nil {
		return row, telemetryDeadLetterDBError(err, map[string]interface{}{"id": id})
	}
	return row, nil
}

// ensureTelemetryDeadLetterAccess mirrors telemetryDeadLetterScopedQuery for a
// single already-loaded row.
func ensureTelemetryDeadLetterAccess(row storage.TelemetryDeadLetter, claims *utils.UserClaims) error {
	if authz.IsSysAdmin(claims) {
		return nil
	}
	if row.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, telemetryReadPermissionMessage)
	}
	if authz.HasRole(claims, authz.TenantUser) {
		if _, err := ensureTelemetryDeviceWriteAccess(row.DeviceID, claims); err != nil {
			return err
		}
	}
	return nil
}

// telemetryDeadLetterDBError wraps a DB failure in the CodeDBError envelope
// used across the dead-letter API; extra carries identifying fields (id/ids).
func telemetryDeadLetterDBError(err error, extra map[string]interface{}) error {
	data := make(map[string]interface{}, len(extra)+1)
	for k, v := range extra {
		data[k] = v
	}
	data["error"] = err.Error()
	return errcode.WithData(errcode.CodeDBError, data)
}
