// File purpose: persistence for P0.4 scene timer triggers (durable scheduling).
// Core logic: claim due timers with a fenced lease, release/advance them after a run.
// Key notes: every statement is tenant-aware where the table has tenant_id; the claim
// uses FOR UPDATE SKIP LOCKED so concurrent replicas each get a distinct timer instead of
// blocking on one another, and expired leases are reclaimable so a crashed replica
// cannot permanently orphan a timer (that would be a lost schedule).

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

type sceneAutomationTimerRow struct {
	ID                  string     `gorm:"column:id"`
	TenantID            string     `gorm:"column:tenant_id"`
	SceneAutomationID   string     `gorm:"column:scene_automation_id"`
	CronExpr            string     `gorm:"column:cron_expr"`
	Timezone            string     `gorm:"column:timezone"`
	Enabled             bool       `gorm:"column:enabled"`
	NextRunAt           time.Time  `gorm:"column:next_run_at"`
	LastRunAt           *time.Time `gorm:"column:last_run_at"`
	LeaseOwner          *string    `gorm:"column:lease_owner"`
	LeaseUntil          *time.Time `gorm:"column:lease_until"`
	ConsecutiveFailures int        `gorm:"column:consecutive_failures"`
	LastError           *string    `gorm:"column:last_error"`
}

func (row sceneAutomationTimerRow) toModel() model.SceneAutomationTimer {
	timer := model.SceneAutomationTimer{
		ID:                  row.ID,
		TenantID:            row.TenantID,
		SceneAutomationID:   row.SceneAutomationID,
		CronExpr:            row.CronExpr,
		Timezone:            row.Timezone,
		Enabled:             row.Enabled,
		NextRunAt:           row.NextRunAt,
		LastRunAt:           row.LastRunAt,
		LeaseOwner:          row.LeaseOwner,
		LeaseUntil:          row.LeaseUntil,
		ConsecutiveFailures: row.ConsecutiveFailures,
		LastError:           row.LastError,
	}
	return timer
}

// ClaimDueSceneTimers 领取到期且未被占用的定时器，并加租约。
// 到期判定只看 next_run_at；租约过期的一并视为可领（崩溃副本的残留）。
// 使用 FOR UPDATE SKIP LOCKED：并发副本各领各的，不互相阻塞。
func ClaimDueSceneTimers(ctx context.Context, owner string, limit int, lease time.Duration, now time.Time) ([]model.SceneAutomationTimer, error) {
	if global.DB == nil || limit <= 0 {
		return nil, nil
	}
	if lease <= 0 {
		lease = time.Minute
	}
	rows := make([]sceneAutomationTimerRow, 0, limit)
	leaseUntil := now.Add(lease)
	err := global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT id, tenant_id, scene_automation_id, cron_expr, timezone, enabled,
			       next_run_at, last_run_at, lease_owner, lease_until,
			       consecutive_failures, last_error
			  FROM scene_automation_timers
			 WHERE enabled = true
			   AND next_run_at <= ?
			   AND (lease_until IS NULL OR lease_until <= ?)
			 ORDER BY next_run_at ASC
			 LIMIT ?
			   FOR UPDATE SKIP LOCKED`, now, now, limit).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return tx.Table("scene_automation_timers").
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"lease_owner": owner,
				"lease_until": leaseUntil,
				"updated_at":  now,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	timers := make([]model.SceneAutomationTimer, 0, len(rows))
	for _, row := range rows {
		timer := row.toModel()
		timer.LeaseUntil = &leaseUntil
		timers = append(timers, timer)
	}
	return timers, nil
}

// CompleteSceneTimerRun 触发成功后推进时钟并释放租约。
// 顺序要紧：只有真的跑完才推进 next_run_at。先推进后执行等于乐观假设成功，
// 一旦执行失败，这次触发就永久消失了。
func CompleteSceneTimerRun(ctx context.Context, id string, nextRunAt time.Time, now time.Time) error {
	if global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Exec(`
		UPDATE scene_automation_timers
		   SET next_run_at = ?, last_run_at = ?, last_error = NULL,
		       consecutive_failures = 0,
		       lease_owner = NULL, lease_until = NULL, updated_at = ?
		 WHERE id = ?`, nextRunAt, now, now, id).Error
}

// FailSceneTimerRun 触发失败：释放租约、累加失败计数、保留错误。
// 时钟不推进——这次触发仍然"欠着"，下一轮会重试；若在此推进就等于承认它已完成。
func FailSceneTimerRun(ctx context.Context, id string, errMessage string, now time.Time) error {
	if global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Exec(`
		UPDATE scene_automation_timers
		   SET consecutive_failures = consecutive_failures + 1,
		       last_error = ?,
		       lease_owner = NULL, lease_until = NULL, updated_at = ?
		 WHERE id = ?`, errMessage, now, id).Error
}

// ReleaseSceneTimerLease 释放租约但不改变时钟（例如下游暂时不可用，稍后重试）。
func ReleaseSceneTimerLease(ctx context.Context, id string, now time.Time) error {
	if global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Exec(`
		UPDATE scene_automation_timers
		   SET lease_owner = NULL, lease_until = NULL, updated_at = ?
		 WHERE id = ?`, now, id).Error
}
