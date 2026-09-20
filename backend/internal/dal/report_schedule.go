package dal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrReportScheduleRevisionConflict = errors.New("report schedule revision conflict")
	ErrReportScheduleHasActiveWork    = errors.New("report schedule has active work")
	ErrReportRunNotRetryable          = errors.New("report run is not retryable")
	ErrReportClaimLost                = errors.New("report claim lost")
	ErrReportIdempotencyConflict      = errors.New("report idempotency key conflicts with a different request")
)

const (
	defaultReportPageSize        = 20
	maxReportPageSize            = 100
	maxReportWorkerBatchSize     = 200
	defaultReportWorkerBatchSize = 50
)

type ReportScheduleChanges struct {
	Name          *string
	CronExpr      *string
	Timezone      *string
	Recipients    *string
	DeviceIDs     *[]string
	Keys          *[]string
	LookbackHours *int
	Format        *string
	Enabled       *bool
}

func normalizeReportPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultReportPageSize
	}
	if pageSize > maxReportPageSize {
		pageSize = maxReportPageSize
	}
	return page, pageSize
}

func clampReportWorkerLimit(limit int) int {
	if limit < 1 {
		return defaultReportWorkerBatchSize
	}
	if limit > maxReportWorkerBatchSize {
		return maxReportWorkerBatchSize
	}
	return limit
}

func reportDB(ctx context.Context) (*gorm.DB, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("report database is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return global.DB.WithContext(ctx), nil
}

func CreateReportScheduleContext(ctx context.Context, schedule *model.ReportSchedule, next ReportNextOccurrence) error {
	db, err := reportDB(ctx)
	if err != nil {
		return err
	}
	if schedule == nil {
		return fmt.Errorf("report schedule is nil")
	}
	if schedule.Enabled && next == nil {
		return fmt.Errorf("report next-occurrence callback is required")
	}
	if schedule.Revision < 1 {
		schedule.Revision = 1
	}
	return db.Transaction(func(tx *gorm.DB) error {
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		if schedule.Enabled {
			occurrence, err := next(schedule, now)
			if err != nil {
				return err
			}
			if occurrence.IsZero() || !occurrence.After(now) {
				return fmt.Errorf("report next occurrence must follow database time")
			}
			occurrence = occurrence.UTC()
			schedule.NextRunAt = &occurrence
		} else {
			schedule.NextRunAt = nil
		}
		schedule.CreatedAt, schedule.UpdatedAt = now, now
		return tx.Create(schedule).Error
	})
}

func UpdateReportScheduleContext(ctx context.Context, id, tenantID string, revision int64, changes ReportScheduleChanges, next ReportNextOccurrence) (*model.ReportSchedule, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(tenantID) == "" || revision < 1 {
		return nil, fmt.Errorf("report schedule update identity and revision are required")
	}
	var updated model.ReportSchedule
	err = db.Transaction(func(tx *gorm.DB) error {
		var schedule model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID).
			Take(&schedule).Error; err != nil {
			return err
		}
		if schedule.Revision != revision {
			return ErrReportScheduleRevisionConflict
		}
		cronExpr, timezone := schedule.CronExpr, schedule.Timezone
		if changes.CronExpr != nil {
			cronExpr = strings.TrimSpace(*changes.CronExpr)
		}
		if changes.Timezone != nil {
			timezone = strings.TrimSpace(*changes.Timezone)
		}
		cadenceChange := changes.CronExpr != nil || changes.Timezone != nil
		enabled := schedule.Enabled
		if changes.Enabled != nil {
			enabled = *changes.Enabled
		}
		explicitReenable := changes.Enabled != nil && *changes.Enabled && !schedule.Enabled
		needsCadence := enabled && schedule.NextRunAt == nil
		if next == nil && (cadenceChange || explicitReenable || needsCadence) {
			return fmt.Errorf("report next-occurrence callback is required for cadence recomputation")
		}
		candidate := schedule
		candidate.CronExpr, candidate.Timezone = cronExpr, timezone
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		updates := reportScheduleUpdates(changes)
		switch {
		case !enabled:
			updates["next_run_at"] = nil
		case cadenceChange || explicitReenable || needsCadence:
			occurrence, nextErr := next(&candidate, now)
			if nextErr != nil {
				return nextErr
			}
			if occurrence.IsZero() || !occurrence.After(now) {
				return fmt.Errorf("report next occurrence must follow database time")
			}
			updates["next_run_at"] = occurrence.UTC()
			updates["schedule_error_code"] = nil
		}
		updates["revision"] = gorm.Expr("revision + 1")
		updates["updated_at"] = now
		result := tx.Model(&model.ReportSchedule{}).
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL AND revision = ?", id, tenantID, revision).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReportScheduleRevisionConflict
		}
		return tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID).Take(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func reportScheduleUpdates(changes ReportScheduleChanges) map[string]interface{} {
	updates := make(map[string]interface{})
	if changes.Name != nil {
		updates["name"] = *changes.Name
	}
	if changes.CronExpr != nil {
		updates["cron_expr"] = *changes.CronExpr
	}
	if changes.Timezone != nil {
		updates["timezone"] = *changes.Timezone
	}
	if changes.Recipients != nil {
		updates["recipients"] = *changes.Recipients
	}
	if changes.DeviceIDs != nil {
		updates["device_ids"] = *changes.DeviceIDs
	}
	if changes.Keys != nil {
		updates["keys"] = *changes.Keys
	}
	if changes.LookbackHours != nil {
		updates["lookback_hours"] = *changes.LookbackHours
	}
	if changes.Format != nil {
		updates["format"] = *changes.Format
	}
	if changes.Enabled != nil {
		updates["enabled"] = *changes.Enabled
	}
	return updates
}

func reportScheduleHasActiveWork(tx *gorm.DB, id, tenantID string) (bool, error) {
	var active int64
	if err := tx.Model(&model.ReportScheduleRun{}).
		Where("schedule_id = ? AND tenant_id = ? AND generation_status IN ?", id, tenantID, reportActiveGenerationStatuses()).
		Count(&active).Error; err != nil {
		return false, err
	}
	if active > 0 {
		return true, nil
	}
	if err := tx.Table(model.TableNameReportScheduleDelivery+" AS delivery").
		Joins("JOIN "+model.TableNameReportScheduleRun+" AS run ON run.id = delivery.run_id").
		Where("run.schedule_id = ? AND delivery.tenant_id = ? AND delivery.status IN ?", id, tenantID, reportActiveDeliveryStatuses()).
		Count(&active).Error; err != nil {
		return false, err
	}
	return active > 0, nil
}

func reportActiveGenerationStatuses() []string {
	return []string{model.ReportGenerationStatusPending, model.ReportGenerationStatusRetrying, model.ReportGenerationStatusProcessing}
}

func reportActiveDeliveryStatuses() []string {
	return []string{model.ReportDeliveryStatusPending, model.ReportDeliveryStatusRetrying, model.ReportDeliveryStatusProcessing}
}

func SoftDeleteReportScheduleContext(ctx context.Context, id, tenantID string, revision int64) error {
	db, err := reportDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var schedule model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID).
			Take(&schedule).Error; err != nil {
			return err
		}
		if revision > 0 && schedule.Revision != revision {
			return ErrReportScheduleRevisionConflict
		}
		active, err := reportScheduleHasActiveWork(tx, id, tenantID)
		if err != nil {
			return err
		}
		if active {
			return ErrReportScheduleHasActiveWork
		}
		result := tx.Model(&model.ReportSchedule{}).
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL AND revision = ?", id, tenantID, schedule.Revision).
			Updates(map[string]interface{}{
				"enabled": false, "next_run_at": nil, "deleted_at": gorm.Expr("clock_timestamp()"),
				"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("clock_timestamp()"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReportScheduleRevisionConflict
		}
		return nil
	})
}

func GetReportScheduleInTenantContext(ctx context.Context, id, tenantID string) (*model.ReportSchedule, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	var schedule model.ReportSchedule
	err = db.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID).Take(&schedule).Error
	return &schedule, err
}

func ListReportSchedulesContext(ctx context.Context, tenantID string, request model.ReportScheduleListRequest) (*model.ReportScheduleListResponse, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizeReportPage(request.Page, request.PageSize)
	query := db.Model(&model.ReportSchedule{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)
	if request.Enabled != nil {
		query = query.Where("enabled = ?", *request.Enabled)
	}
	if search := strings.TrimSpace(request.Search); search != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(search)+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []*model.ReportSchedule
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &model.ReportScheduleListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

func GetTelemetryDataForReportContext(ctx context.Context, tenantID, deviceID, key string, startMs, endMs int64, limit int) ([]*model.TelemetryData, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*model.TelemetryData
	err = db.Where("tenant_id = ? AND device_id = ? AND key = ? AND ts >= ? AND ts <= ?", tenantID, deviceID, key, startMs, endMs).
		Order("ts DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
