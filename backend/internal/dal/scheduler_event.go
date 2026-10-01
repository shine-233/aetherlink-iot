// 文件用途：TB-48 统一调度器持久化——scheduler_events 注册表 CRUD 与四源聚合读路径。
// 核心逻辑：
//   - 注册面 CRUD：全部显式携带 tenant_id 条件（tenant-scope: caller-enforced，由 service
//     层注入 claims 推导出的租户），id 唯一定位行；
//   - scene 事件同步：CreateSchedulerSceneEventPair 在同一事务里写 scheduler_events 与
//     scene_automation_timers（两行 id 相同），保证"注册行存在 ⇒ 执行行存在"；
//   - 聚合读路径：四个 List*ScheduleItems 分别把 scene_automation_timers（联
//     scene_automations 取名）、report_schedules、command_jobs 定时行（status='scheduled'
//     且 scheduled_at 非空）与 scheduler_events 注册行归一为 model.SchedulerEventItem，
//     归并/过滤/排序/分页在 service 层完成（租户内行数量级小，单源 2000 行封顶）。
//
// 关键注意事项：
//  1. 本文件不迁移任何存量执行器：对 scene_automation_timers 只做注册面的 insert/update/
//     delete 同步，领取/推进/失败计数仍走 scene_automation_timer.go 的既有租约机制；
//  2. scene_automation_timers 存在部分唯一索引 uq_scene_timers_automation
//     (scene_automation_id) WHERE enabled=true——同一场景只允许一条启用定时器；
//     服务层创建/改绑/启用前用 CountEnabledSceneTimersForAutomation 预检，冲突报业务错，
//     约束本身作为并发兜底；
//  3. 聚合查询兼容 PG 与 sqlite 测试库：只用标准 SQL（COALESCE/NULLIF/TRIM），不用
//     ILIKE / ::cast / 部分索引语法。
package dal

import (
	"context"
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// errSchedulerDBNotReady 全局 DB 未初始化时的统一哨兵错误（fail-closed，不猜空结果）。
var errSchedulerDBNotReady = errors.New("database is not initialized")

// schedulerEventSourceLimit 单源聚合读取上限：租户内各源行数量级小，封顶防放大。
const schedulerEventSourceLimit = 2000

// SceneTimerChanges scene 事件对 scene_automation_timers 同步行的结构化变更集。
// 仅非 nil 字段会被更新；updated_at 总是刷新。
type SceneTimerChanges struct {
	SceneAutomationID *string
	CronExpr          *string
	NextRunAt         *time.Time
	Enabled           *bool
}

// schedulerEventItemScan 聚合条目扫描目标（列序对应各聚合查询的统一投影）。
type schedulerEventItemScan struct {
	ID         string     `gorm:"column:id"`
	Name       string     `gorm:"column:name"`
	SourceType string     `gorm:"column:source_type"`
	Origin     string     `gorm:"column:origin"`
	RefType    string     `gorm:"column:ref_type"`
	RefID      string     `gorm:"column:ref_id"`
	CronExpr   string     `gorm:"column:cron_expr"`
	Timezone   string     `gorm:"column:timezone"`
	Enabled    bool       `gorm:"column:enabled"`
	NextRunAt  *time.Time `gorm:"column:next_run_at"`
	LastRunAt  *time.Time `gorm:"column:last_run_at"`
}

func scanSchedulerEventItems(rows []schedulerEventItemScan) []*model.SchedulerEventItem {
	items := make([]*model.SchedulerEventItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &model.SchedulerEventItem{
			ID: row.ID, Name: row.Name, SourceType: row.SourceType, Origin: row.Origin,
			RefType: row.RefType, RefID: row.RefID, CronExpr: row.CronExpr, Timezone: row.Timezone,
			Enabled: row.Enabled, NextRunAt: row.NextRunAt, LastRunAt: row.LastRunAt,
		})
	}
	return items
}

// CreateSchedulerEvent 插入一条注册事件。
func CreateSchedulerEvent(ctx context.Context, event *model.SchedulerEvent) error {
	if global.DB == nil {
		return errSchedulerDBNotReady
	}
	return global.DB.WithContext(ctx).Create(event).Error
}

// CreateSchedulerSceneEventPair 同一事务写入 scene 注册事件与其 scene_automation_timers
// 执行行（两行 id 相同）：任一失败整体回滚，避免出现"有注册无执行"或反之的半状态。
// timer 必须已带 ID/TenantID/SceneAutomationID/CronExpr/Timezone/Enabled/NextRunAt。
func CreateSchedulerSceneEventPair(ctx context.Context, event *model.SchedulerEvent, timer *model.SceneAutomationTimer) error {
	if global.DB == nil {
		return errSchedulerDBNotReady
	}
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		return tx.Table("scene_automation_timers").Create(timer).Error
	})
}

// GetSchedulerEventInTenant 按 id + 租户查询注册事件（租户隔离 fail-closed）。
func GetSchedulerEventInTenant(ctx context.Context, id, tenantID string) (*model.SchedulerEvent, error) {
	if global.DB == nil {
		return nil, errSchedulerDBNotReady
	}
	var event model.SchedulerEvent
	err := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// UpdateSchedulerEventInTenant 按 id + 租户更新注册事件字段，返回受影响行数
// （0 = 行不存在或已不属于该租户，服务层按 404 处理，绝不跨租户改写）。
func UpdateSchedulerEventInTenant(ctx context.Context, id, tenantID string, changes map[string]interface{}) (int64, error) {
	if global.DB == nil {
		return 0, errSchedulerDBNotReady
	}
	result := global.DB.WithContext(ctx).Model(&model.SchedulerEvent{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(changes)
	return result.RowsAffected, result.Error
}

// DeleteSchedulerEventInTenant 按 id + 租户删除注册事件，返回受影响行数。
func DeleteSchedulerEventInTenant(ctx context.Context, id, tenantID string) (int64, error) {
	if global.DB == nil {
		return 0, errSchedulerDBNotReady
	}
	result := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.SchedulerEvent{})
	return result.RowsAffected, result.Error
}

// CountEnabledSceneTimersForAutomation 统计某场景自动化当前已启用的定时器数量
// （excludeTimerID 用于改绑/启用时排除自身）。tenant-scope: caller-enforced——
// 租户条件由调用方传入 tenantID 落地为 WHERE。
func CountEnabledSceneTimersForAutomation(ctx context.Context, tenantID, automationID, excludeTimerID string) (int64, error) {
	if global.DB == nil {
		return 0, errSchedulerDBNotReady
	}
	query := global.DB.WithContext(ctx).Table("scene_automation_timers").
		Where("tenant_id = ? AND scene_automation_id = ? AND enabled = ?", tenantID, automationID, true)
	if excludeTimerID != "" {
		query = query.Where("id <> ?", excludeTimerID)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

// SceneAutomationExistsInTenant 校验场景自动化属于该租户（scene 事件注册前置检查）。
func SceneAutomationExistsInTenant(ctx context.Context, tenantID, automationID string) (bool, error) {
	if global.DB == nil {
		return false, errSchedulerDBNotReady
	}
	var count int64
	err := global.DB.WithContext(ctx).Model(&model.SceneAutomation{}).
		Where("id = ? AND tenant_id = ?", automationID, tenantID).
		Count(&count).Error
	return count > 0, err
}

// UpdateSchedulerSceneTimer 按 id + 租户对 scene 执行行施加结构化变更（仅非 nil 字段）。
func UpdateSchedulerSceneTimer(ctx context.Context, tenantID, timerID string, changes SceneTimerChanges, now time.Time) (int64, error) {
	if global.DB == nil {
		return 0, errSchedulerDBNotReady
	}
	updates := map[string]interface{}{"updated_at": now}
	if changes.SceneAutomationID != nil {
		updates["scene_automation_id"] = *changes.SceneAutomationID
	}
	if changes.CronExpr != nil {
		updates["cron_expr"] = *changes.CronExpr
	}
	if changes.NextRunAt != nil {
		updates["next_run_at"] = *changes.NextRunAt
	}
	if changes.Enabled != nil {
		updates["enabled"] = *changes.Enabled
	}
	result := global.DB.WithContext(ctx).Table("scene_automation_timers").
		Where("id = ? AND tenant_id = ?", timerID, tenantID).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// DeleteSchedulerSceneTimer 按 id + 租户删除 scene 执行行（注册行删除时同步）。
// model.SceneAutomationTimer 的默认表名即 scene_automation_timers，无需 Table 覆盖。
func DeleteSchedulerSceneTimer(ctx context.Context, tenantID, timerID string) (int64, error) {
	if global.DB == nil {
		return 0, errSchedulerDBNotReady
	}
	result := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", timerID, tenantID).
		Delete(&model.SceneAutomationTimer{})
	return result.RowsAffected, result.Error
}

// ListSceneTimerScheduleItems 聚合 scene_automation_timers 存量行为统一事件条目
// （LEFT JOIN scene_automations 取联动名称，联动已删时名称回退为空串）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListSceneTimerScheduleItems(ctx context.Context, tenantID string) ([]*model.SchedulerEventItem, error) {
	if global.DB == nil {
		return nil, errSchedulerDBNotReady
	}
	rows := make([]schedulerEventItemScan, 0, 16)
	err := global.DB.WithContext(ctx).Raw(`
		SELECT t.id                        AS id,
		       COALESCE(a.name, '')        AS name,
		       'scene'                     AS source_type,
		       'scene_timer'               AS origin,
		       'scene_automation'          AS ref_type,
		       t.scene_automation_id       AS ref_id,
		       t.cron_expr                 AS cron_expr,
		       t.timezone                  AS timezone,
		       t.enabled                   AS enabled,
		       t.next_run_at               AS next_run_at,
		       t.last_run_at               AS last_run_at
		  FROM scene_automation_timers t
		  LEFT JOIN scene_automations a ON a.id = t.scene_automation_id
		 WHERE t.tenant_id = ?
		 ORDER BY t.next_run_at ASC
		 LIMIT ?`, tenantID, schedulerEventSourceLimit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return scanSchedulerEventItems(rows), nil
}

// ListReportScheduleItems 聚合 report_schedules 存量行（未软删）为统一事件条目。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListReportScheduleItems(ctx context.Context, tenantID string) ([]*model.SchedulerEventItem, error) {
	if global.DB == nil {
		return nil, errSchedulerDBNotReady
	}
	rows := make([]schedulerEventItemScan, 0, 16)
	err := global.DB.WithContext(ctx).Raw(`
		SELECT id          AS id,
		       name        AS name,
		       'report'    AS source_type,
		       'report_schedule' AS origin,
		       'report_schedule' AS ref_type,
		       id          AS ref_id,
		       cron_expr   AS cron_expr,
		       timezone    AS timezone,
		       enabled     AS enabled,
		       next_run_at AS next_run_at,
		       last_run_at AS last_run_at
		  FROM report_schedules
		 WHERE tenant_id = ? AND deleted_at IS NULL
		 ORDER BY next_run_at ASC
		 LIMIT ?`, tenantID, schedulerEventSourceLimit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return scanSchedulerEventItems(rows), nil
}

// ListFleetScheduledJobItems 聚合舰队命令定时行（command_jobs 中 status='scheduled'
// 且 scheduled_at 非空的行）为统一事件条目：一次性下发，cron 为空、next_run_at=scheduled_at。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListFleetScheduledJobItems(ctx context.Context, tenantID string) ([]*model.SchedulerEventItem, error) {
	if global.DB == nil {
		return nil, errSchedulerDBNotReady
	}
	rows := make([]schedulerEventItemScan, 0, 16)
	err := global.DB.WithContext(ctx).Raw(`
		SELECT id                                          AS id,
		       COALESCE(NULLIF(TRIM(COALESCE(remark, '')), ''), identify) AS name,
		       'rpc'                                       AS source_type,
		       'fleet_command_job'                         AS origin,
		       'fleet_command_job'                         AS ref_type,
		       id                                          AS ref_id,
		       ''                                          AS cron_expr,
		       'UTC'                                       AS timezone,
		       1                                           AS enabled,
		       scheduled_at                                AS next_run_at,
		       NULL                                        AS last_run_at
		  FROM command_jobs
		 WHERE tenant_id = ? AND status = 'scheduled' AND scheduled_at IS NOT NULL
		 ORDER BY scheduled_at ASC
		 LIMIT ?`, tenantID, schedulerEventSourceLimit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return scanSchedulerEventItems(rows), nil
}

// ListSchedulerRegistryItems 聚合 scheduler_events 注册行为统一事件条目
// （origin=scheduler_registry；scene 注册行与同 id 的 scene_timer 存量行由 service 层去重）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListSchedulerRegistryItems(ctx context.Context, tenantID string) ([]*model.SchedulerEventItem, error) {
	if global.DB == nil {
		return nil, errSchedulerDBNotReady
	}
	rows := make([]schedulerEventItemScan, 0, 16)
	err := global.DB.WithContext(ctx).Raw(`
		SELECT id          AS id,
		       name        AS name,
		       event_type  AS source_type,
		       'scheduler_registry' AS origin,
		       COALESCE(ref_type, '') AS ref_type,
		       COALESCE(ref_id, '')   AS ref_id,
		       COALESCE(cron, '')     AS cron_expr,
		       'UTC'       AS timezone,
		       enabled     AS enabled,
		       next_run_at AS next_run_at,
		       NULL        AS last_run_at
		  FROM scheduler_events
		 WHERE tenant_id = ?
		 ORDER BY next_run_at ASC
		 LIMIT ?`, tenantID, schedulerEventSourceLimit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return scanSchedulerEventItems(rows), nil
}
