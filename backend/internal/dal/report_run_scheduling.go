package dal

// 文件用途：报表计划 → 运行实例的调度侧写路径（初始化 next_run_at、物化到期槽位）。
// 核心逻辑：两处都先在事务内用 FOR UPDATE SKIP LOCKED 批量取候选计划（多 worker 并行互不阻塞），
//   再由数据库时钟（clock_timestamp()）统一裁决窗口与 misfire 合并，避免调用方时钟漂移。
// 关键注意事项：
//   - 每次 UPDATE 都必须带 tenant_id 谓词，防止跨租户写。
//   - next 回调由调用方注入；返回不推进的时间戳即视为非法计划，直接隔离（enabled=false）。

import (
	"context"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func InitializeReportScheduleNextRuns(ctx context.Context, limit int, next ReportNextOccurrence) (ReportScheduleInitializationResult, error) {
	if next == nil {
		return ReportScheduleInitializationResult{}, fmt.Errorf("report next-occurrence callback is required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return ReportScheduleInitializationResult{}, err
	}
	limit = clampReportWorkerLimit(limit)
	result := ReportScheduleInitializationResult{}
	err = db.Transaction(func(tx *gorm.DB) error {
		var schedules []*model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", true).
			Order("created_at ASC, id ASC").Limit(limit).Find(&schedules).Error; err != nil {
			return err
		}
		if len(schedules) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, schedule := range schedules {
			occurrence, nextErr := next(schedule, now)
			if nextErr != nil || occurrence.IsZero() || !occurrence.After(now) {
				if updateErr := tx.Model(&model.ReportSchedule{}).
					Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", schedule.ID, schedule.TenantID, true).
					Updates(map[string]interface{}{"enabled": false, "schedule_error_code": "invalid_schedule", "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error; updateErr != nil {
					return updateErr
				}
				result.Invalid++
				continue
			}
			if updateErr := tx.Model(&model.ReportSchedule{}).
				Where("id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", schedule.ID, true).
				Updates(map[string]interface{}{"next_run_at": occurrence.UTC(), "updated_at": now}).Error; updateErr != nil {
				return updateErr
			}
			result.Initialized++
		}
		return nil
	})
	return result, err
}

func MaterializeDueReportScheduleRuns(ctx context.Context, limit, maxAttempts int, next ReportNextOccurrence) ([]ReportMaterializationResult, error) {
	if next == nil {
		return nil, fmt.Errorf("report next-occurrence callback is required")
	}
	if maxAttempts < 1 {
		return nil, fmt.Errorf("report generation max attempts must be positive")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	limit = clampReportWorkerLimit(limit)
	materialized := make([]ReportMaterializationResult, 0, limit)
	err = db.Transaction(func(tx *gorm.DB) error {
		var schedules []*model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND deleted_at IS NULL AND next_run_at IS NOT NULL", true).
			Order("next_run_at ASC, id ASC").Limit(limit).Find(&schedules).Error; err != nil {
			return err
		}
		if len(schedules) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, schedule := range schedules {
			if schedule.NextRunAt == nil || schedule.NextRunAt.After(now) {
				continue
			}
			cursor := schedule.NextRunAt.UTC()
			firstFuture, missed, lastMissed, nextErr := firstFutureReportOccurrenceBounded(schedule, cursor, now, next)
			if nextErr != nil {
				if quarantineErr := quarantineInvalidReportScheduleTx(tx, schedule, cursor, now); quarantineErr != nil {
					return quarantineErr
				}
				continue
			}
			slot := lastMissed.UTC()
			windowStart := slot.Add(-time.Duration(schedule.LookbackHours) * time.Hour)
			run := snapshotReportRun(schedule, uuid.NewString(), model.ReportRunTriggerScheduled, nil, &slot, windowStart, slot, missed-1, maxAttempts, now)
			if missed > 1 {
				first, last := cursor, slot
				run.MisfireFirstSlot, run.MisfireLastSlot = &first, &last
			}
			create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(run)
			if create.Error != nil {
				return create.Error
			}
			// Keep the tenant predicate: the sibling statements in this function all
			// scope by tenant_id, and dropping it here is how a constraint drift
			// turns into a cross-tenant write if IDs ever stop being globally unique.
			if updateErr := tx.Model(&model.ReportSchedule{}).
				Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", schedule.ID, schedule.TenantID, true, cursor).
				Updates(map[string]interface{}{"next_run_at": firstFuture, "updated_at": now}).Error; updateErr != nil {
				return updateErr
			}
			if create.RowsAffected == 1 {
				materialized = append(materialized, ReportMaterializationResult{Run: run, CoalescedMisfires: missed - 1})
			}
		}
		return nil
	})
	return materialized, err
}

func quarantineInvalidReportScheduleTx(tx *gorm.DB, schedule *model.ReportSchedule, cursor, now time.Time) error {
	result := tx.Model(&model.ReportSchedule{}).
		Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", schedule.ID, schedule.TenantID, true, cursor).
		Updates(map[string]interface{}{
			"enabled": false, "next_run_at": nil, "schedule_error_code": "invalid_schedule",
			"revision": gorm.Expr("revision + 1"), "updated_at": now,
		})
	return result.Error
}

func firstFutureReportOccurrenceBounded(schedule *model.ReportSchedule, cursor, now time.Time, next ReportNextOccurrence) (time.Time, int, time.Time, error) {
	first := cursor.UTC()
	if now.Sub(first) > reportOccurrenceScanHorizon {
		return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s stale occurrence exceeds scan horizon", schedule.ID)
	}
	missed, lastMissed := 1, first
	for !cursor.After(now) {
		if missed > reportOccurrenceScanCap {
			return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s stale occurrence scan limit exceeded", schedule.ID)
		}
		following, err := next(schedule, cursor)
		if err != nil {
			return time.Time{}, 0, time.Time{}, err
		}
		if following.IsZero() || !following.After(cursor) {
			return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s next occurrence did not advance", schedule.ID)
		}
		cursor = following.UTC()
		if !cursor.After(now) {
			missed++
			lastMissed = cursor
		}
	}
	return cursor, missed, lastMissed, nil
}

func snapshotReportRun(schedule *model.ReportSchedule, id, trigger string, parentID *string, scheduledSlot *time.Time, windowStart, windowEnd time.Time, misfires, maxAttempts int, now time.Time) *model.ReportScheduleRun {
	if maxAttempts < 1 {
		maxAttempts = defaultReportMaxAttempts
	}
	snapshot := model.ReportRunConfigSnapshot{
		ScheduleName: schedule.Name, Recipients: schedule.Recipients,
		DeviceIDs: append([]string(nil), schedule.DeviceIDs...), Keys: append([]string(nil), schedule.Keys...),
		Format: schedule.Format,
	}
	return &model.ReportScheduleRun{
		ID: id, TenantID: schedule.TenantID, ScheduleID: schedule.ID, RetryParentRunID: parentID,
		Trigger: trigger, ScheduledSlot: scheduledSlot, MisfireCount: misfires,
		WindowStartAt: windowStart.UTC(), WindowEndAt: windowEnd.UTC(), ConfigSnapshot: snapshot,
		GenerationStatus: model.ReportGenerationStatusPending, MaxAttempts: maxAttempts,
		NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
}
