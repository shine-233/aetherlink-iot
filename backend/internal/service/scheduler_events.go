// 文件用途：TB-48 统一调度器服务——三源聚合列表与 scheduler_events 注册面 CRUD。
// 核心逻辑：
//   - ListSchedulerEvents 只读聚合既有三套调度（scene_automation_timers / report_schedules /
//     command_jobs 定时行）+ 注册行，归一为 SchedulerEventItem（含来源类型）后过滤、
//     排序、分页；scene 注册行与其同 id 的执行行天然去重；
//   - 注册面 CRUD：event_type=scene 的事件落到既有 scene automation timer 机制执行
//     （同一事务写 scheduler_events + scene_automation_timers，两行 id 相同，worker
//     scene_timer_worker 照常领取触发）；report/rpc 注册行仅统一登记展示，执行仍归
//     各存量系统——不迁移任何存量执行器（TB-48 明确不做）。
//
// 关键注意事项：
//  1. 租户边界 fail-closed：claims 缺失或 TenantID 为空一律拒绝（口径同 report_schedule.go），
//     本面不提供跨租户参数——SYS_ADMIN 也只操作自己租户上下文内的注册行；
//  2. 注册面 v1 统一按 UTC 评估 cron（NextSceneTimerRun 复用既有解析，5/6 段皆可，
//     时区非法/表达式非法 fail-closed），scene_automation_timers.timezone 固定 'UTC'；
//  3. 同一场景自动化仅允许一条启用定时触发（uq_scene_timers_automation 部分唯一索引）：
//     创建/改绑/重新启用前先 CountEnabledSceneTimersForAutomation 预检，冲突报业务错，
//     索引作并发兜底（错误文案含唯一索引名时同样映射为业务错）。
package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SchedulerService 统一调度器服务。
type SchedulerService struct{}

// schedulerEventPageSize 聚合列表未传分页时的默认页大小与上限（口径对齐 dal.defaultListLimit）。
const (
	schedulerEventDefaultPageSize = 200
	schedulerEventMaxPageSize     = 500
)

// schedulerClaimsTenant 解析请求租户边界：claims 缺失或无租户上下文一律拒绝（fail-closed）。
func schedulerClaimsTenant(claims *utils.UserClaims) (string, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "scheduler access denied: no tenant context")
	}
	return strings.TrimSpace(claims.TenantID), nil
}

// schedulerNotFound 统一的"注册事件不存在"业务错误（跨租户/不存在/并发已删一律同文案，
// 不向调用方泄露其他租户的行是否存在）。
func schedulerNotFound() error {
	return errcode.NewWithMessage(errcode.CodeNotFound, "scheduler event not found")
}

// schedulerTimerConflict 同场景仅允许一条启用定时触发的业务错误。
func schedulerTimerConflict(automationID string) error {
	return errcode.NewWithMessage(errcode.CodeOpDenied,
		"scene automation "+automationID+" already has an enabled timer; only one enabled timer per automation is allowed")
}

// isSceneTimerUniqueViolation 识别 uq_scene_timers_automation 唯一索引冲突
// （PG 报约束名、sqlite 报 UNIQUE constraint failed，双口径兜底）。
func isSceneTimerUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "uq_scene_timers_automation") ||
		strings.Contains(message, "unique constraint failed")
}

// mapSchedulerDBError 数据库错误统一包装（约束冲突中的 scene timer 唯一索引映射为业务错）。
func mapSchedulerDBError(err error) error {
	if err == nil {
		return nil
	}
	if isSceneTimerUniqueViolation(err) {
		return errcode.NewWithMessage(errcode.CodeOpDenied,
			"scene automation already has an enabled timer; only one enabled timer per automation is allowed")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schedulerNotFound()
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
}

// schedulerEventSources 聚合的四个来源（DAL 各自带租户过滤）。
type schedulerEventSources struct {
	timers   []*model.SchedulerEventItem
	reports  []*model.SchedulerEventItem
	jobs     []*model.SchedulerEventItem
	registry []*model.SchedulerEventItem
}

// assembleSchedulerEventItems 归并四源并按 id 去重 scene 注册行：
// scene 注册行的执行行（scene_automation_timers）与注册行同 id，执行行已在 timers 列表里
// 呈现，注册行不再重复输出；执行行缺失的孤儿注册行保留（可见、可删，不静默吞掉）。
func assembleSchedulerEventItems(sources schedulerEventSources) []*model.SchedulerEventItem {
	timerIDs := make(map[string]bool, len(sources.timers))
	for _, item := range sources.timers {
		timerIDs[item.ID] = true
	}
	items := make([]*model.SchedulerEventItem, 0,
		len(sources.timers)+len(sources.reports)+len(sources.jobs)+len(sources.registry))
	items = append(items, sources.timers...)
	items = append(items, sources.reports...)
	items = append(items, sources.jobs...)
	for _, item := range sources.registry {
		if item.SourceType == model.SchedulerEventTypeScene && timerIDs[item.ID] {
			continue
		}
		items = append(items, item)
	}
	return items
}

// schedulerEventFilter 聚合列表的内存过滤条件（DAL 已按租户收敛，此处做业务过滤）。
type schedulerEventFilter struct {
	SourceType string
	Search     string
	Enabled    *bool
	From       time.Time // 零值 = 不限
	To         time.Time // 零值 = 不限
}

// filterSchedulerEventItems 过滤聚合条目：来源类型精确匹配、名称不区分大小写包含、
// 启用态精确匹配、next_run_at 落在 [From, To] 窗口内（设置了窗口而条目无 next_run_at 的
// 不可调度条目排除——日历按时间轴聚合，无时刻条目无处可放）。
func filterSchedulerEventItems(items []*model.SchedulerEventItem, f schedulerEventFilter) []*model.SchedulerEventItem {
	search := strings.ToLower(strings.TrimSpace(f.Search))
	out := make([]*model.SchedulerEventItem, 0, len(items))
	for _, item := range items {
		if f.SourceType != "" && item.SourceType != f.SourceType {
			continue
		}
		if f.Enabled != nil && item.Enabled != *f.Enabled {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(item.Name), search) {
			continue
		}
		if !f.From.IsZero() || !f.To.IsZero() {
			if item.NextRunAt == nil {
				continue
			}
			if !f.From.IsZero() && item.NextRunAt.Before(f.From) {
				continue
			}
			if !f.To.IsZero() && item.NextRunAt.After(f.To) {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

// sortSchedulerEventItems 按 next_run_at 升序（空值排最后），同刻按 id 稳定排序。
func sortSchedulerEventItems(items []*model.SchedulerEventItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].NextRunAt, items[j].NextRunAt
		switch {
		case a == nil && b == nil:
			return items[i].ID < items[j].ID
		case a == nil:
			return false
		case b == nil:
			return true
		case !a.Equal(*b):
			return a.Before(*b)
		default:
			return items[i].ID < items[j].ID
		}
	})
}

// ListSchedulerEvents 只读聚合既有三套调度 + 注册行为统一事件列表。
func (SchedulerService) ListSchedulerEvents(ctx context.Context, req *model.SchedulerEventListRequest, claims *utils.UserClaims) (*model.SchedulerEventListResponse, error) {
	tenantID, err := schedulerClaimsTenant(claims)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &model.SchedulerEventListRequest{}
	}
	filter := schedulerEventFilter{SourceType: strings.TrimSpace(req.SourceType), Search: req.Search, Enabled: req.Enabled}
	if req.FromMs > 0 {
		filter.From = time.UnixMilli(req.FromMs).UTC()
	}
	if req.ToMs > 0 {
		filter.To = time.UnixMilli(req.ToMs).UTC()
	}
	if !filter.From.IsZero() && !filter.To.IsZero() && filter.From.After(filter.To) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "from_ms must not be after to_ms")
	}

	timers, err := dal.ListSceneTimerScheduleItems(ctx, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	reports, err := dal.ListReportScheduleItems(ctx, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	jobs, err := dal.ListFleetScheduledJobItems(ctx, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	registry, err := dal.ListSchedulerRegistryItems(ctx, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}

	items := filterSchedulerEventItems(
		assembleSchedulerEventItems(schedulerEventSources{timers: timers, reports: reports, jobs: jobs, registry: registry}),
		filter)
	sortSchedulerEventItems(items)

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = schedulerEventDefaultPageSize
	}
	if pageSize > schedulerEventMaxPageSize {
		pageSize = schedulerEventMaxPageSize
	}
	offset := (page - 1) * pageSize
	if offset >= len(items) {
		return &model.SchedulerEventListResponse{List: []*model.SchedulerEventItem{}, Total: int64(len(items)), Page: page, PageSize: pageSize}, nil
	}
	end := offset + pageSize
	if end > len(items) {
		end = len(items)
	}
	return &model.SchedulerEventListResponse{
		List: items[offset:end], Total: int64(len(items)), Page: page, PageSize: pageSize,
	}, nil
}

// computeSchedulerNextRun 按 UTC 计算 cron 的下一次触发时刻（scene/report 注册事件共用）。
func computeSchedulerNextRun(cronExpr string, from time.Time) (time.Time, error) {
	return NextSceneTimerRun(cronExpr, "UTC", from)
}

// CreateSchedulerEvent 注册调度事件。scene 事件在同一事务里落 scheduler_events 与
// scene_automation_timers（执行走既有 scene timer worker 机制）。
func (SchedulerService) CreateSchedulerEvent(ctx context.Context, req *model.SchedulerEventCreateReq, claims *utils.UserClaims) (*model.SchedulerEvent, error) {
	tenantID, err := schedulerClaimsTenant(claims)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request body is required")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	event := &model.SchedulerEvent{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(req.Name),
		EventType: strings.TrimSpace(req.EventType),
		RefType:   strings.TrimSpace(req.RefType),
		RefID:     strings.TrimSpace(req.RefID),
		Enabled:   enabled,
	}
	now := time.Now().UTC()

	switch event.EventType {
	case model.SchedulerEventTypeScene:
		if event.RefID == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "scene event requires ref_id (scene automation id)")
		}
		cronExpr := strings.TrimSpace(req.CronExpr)
		if cronExpr == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "scene event requires cron expression")
		}
		if req.NextRunAt != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "next_run_at is computed from cron for scene events; drop the field")
		}
		next, err := computeSchedulerNextRun(cronExpr, now)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
		}
		exists, err := dal.SceneAutomationExistsInTenant(ctx, tenantID, event.RefID)
		if err != nil {
			return nil, mapSchedulerDBError(err)
		}
		if !exists {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "scene automation does not exist in this tenant")
		}
		conflicts, err := dal.CountEnabledSceneTimersForAutomation(ctx, tenantID, event.RefID, "")
		if err != nil {
			return nil, mapSchedulerDBError(err)
		}
		if conflicts > 0 {
			return nil, schedulerTimerConflict(event.RefID)
		}
		event.RefType = model.SchedulerRefTypeSceneAutomation
		event.CronExpr = cronExpr
		event.NextRunAt = &next
		timer := &model.SceneAutomationTimer{
			ID: event.ID, TenantID: tenantID, SceneAutomationID: event.RefID,
			CronExpr: cronExpr, Timezone: "UTC", Enabled: enabled, NextRunAt: next,
		}
		if err := dal.CreateSchedulerSceneEventPair(ctx, event, timer); err != nil {
			return nil, mapSchedulerDBError(err)
		}
	case model.SchedulerEventTypeReport:
		cronExpr := strings.TrimSpace(req.CronExpr)
		if cronExpr == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "report event requires cron expression")
		}
		next, err := computeSchedulerNextRun(cronExpr, now)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
		}
		if event.RefType == "" && event.RefID != "" {
			event.RefType = model.SchedulerRefTypeReportSchedule
		}
		event.CronExpr = cronExpr
		event.NextRunAt = &next
		if err := dal.CreateSchedulerEvent(ctx, event); err != nil {
			return nil, mapSchedulerDBError(err)
		}
	case model.SchedulerEventTypeRPC:
		if strings.TrimSpace(req.CronExpr) != "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "rpc event is one-shot; cron must be empty, provide next_run_at instead")
		}
		if req.NextRunAt == nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "rpc event requires next_run_at (one-shot fire time)")
		}
		runAt := req.NextRunAt.UTC()
		if !runAt.After(now) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "next_run_at must be in the future")
		}
		if event.RefType == "" && event.RefID != "" {
			event.RefType = model.SchedulerRefTypeFleetCommandJob
		}
		event.NextRunAt = &runAt
		if err := dal.CreateSchedulerEvent(ctx, event); err != nil {
			return nil, mapSchedulerDBError(err)
		}
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "event_type must be one of scene, report, rpc")
	}
	return event, nil
}

// GetSchedulerEvent 注册事件详情。
func (SchedulerService) GetSchedulerEvent(ctx context.Context, id string, claims *utils.UserClaims) (*model.SchedulerEvent, error) {
	tenantID, err := schedulerClaimsTenant(claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "event id is required")
	}
	event, err := dal.GetSchedulerEventInTenant(ctx, id, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	return event, nil
}

// UpdateSchedulerEvent 更新注册事件。event_type 不可变；scene 事件的 ref_id/cron/enabled
// 变更同步到同 id 的 scene_automation_timers 行；scene/report 的 next_run_at 始终由 cron
// 重算，rpc 的 next_run_at 直接给定。
func (SchedulerService) UpdateSchedulerEvent(ctx context.Context, id string, req *model.SchedulerEventUpdateReq, claims *utils.UserClaims) (*model.SchedulerEvent, error) {
	tenantID, err := schedulerClaimsTenant(claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "event id is required")
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request body is required")
	}
	existing, err := dal.GetSchedulerEventInTenant(ctx, id, tenantID)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	now := time.Now().UTC()
	isScene := existing.EventType == model.SchedulerEventTypeScene

	changes := map[string]interface{}{"updated_at": now}
	var timerChanges dal.SceneTimerChanges
	// scene 事件本次请求可能改绑的目标自动化（先按旧值，改绑时刷新）。
	targetAutomation := existing.RefID

	if req.Name != nil {
		changes["name"] = strings.TrimSpace(*req.Name)
	}
	if req.RefType != nil {
		changes["ref_type"] = strings.TrimSpace(*req.RefType)
	}
	if req.RefID != nil {
		newRef := strings.TrimSpace(*req.RefID)
		if isScene {
			if newRef == "" {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "scene event ref_id cannot be empty")
			}
			if newRef != existing.RefID {
				exists, err := dal.SceneAutomationExistsInTenant(ctx, tenantID, newRef)
				if err != nil {
					return nil, mapSchedulerDBError(err)
				}
				if !exists {
					return nil, errcode.NewWithMessage(errcode.CodeParamError, "scene automation does not exist in this tenant")
				}
				conflicts, err := dal.CountEnabledSceneTimersForAutomation(ctx, tenantID, newRef, existing.ID)
				if err != nil {
					return nil, mapSchedulerDBError(err)
				}
				// 仅当本次请求没有同时停用事件时，改绑才需要冲突预检。
				if conflicts > 0 && (req.Enabled == nil || *req.Enabled) {
					return nil, schedulerTimerConflict(newRef)
				}
				timerChanges.SceneAutomationID = &newRef
			}
			targetAutomation = newRef
		}
		changes["ref_id"] = newRef
	}
	if req.CronExpr != nil {
		newCron := strings.TrimSpace(*req.CronExpr)
		switch existing.EventType {
		case model.SchedulerEventTypeScene, model.SchedulerEventTypeReport:
			if newCron == "" {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, existing.EventType+" event requires cron expression")
			}
			next, err := computeSchedulerNextRun(newCron, now)
			if err != nil {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
			}
			changes["cron"] = newCron
			changes["next_run_at"] = next
			if isScene {
				timerChanges.CronExpr = &newCron
				timerChanges.NextRunAt = &next
			}
		case model.SchedulerEventTypeRPC:
			if newCron != "" {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "rpc event is one-shot; cron must stay empty")
			}
		}
	}
	if req.NextRunAt != nil {
		if existing.EventType == model.SchedulerEventTypeRPC {
			runAt := req.NextRunAt.UTC()
			if !runAt.After(now) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "next_run_at must be in the future")
			}
			changes["next_run_at"] = runAt
		} else {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "next_run_at is computed from cron for scene/report events; drop the field")
		}
	}
	if req.Enabled != nil {
		changes["enabled"] = *req.Enabled
		if isScene {
			if *req.Enabled {
				conflicts, err := dal.CountEnabledSceneTimersForAutomation(ctx, tenantID, targetAutomation, existing.ID)
				if err != nil {
					return nil, mapSchedulerDBError(err)
				}
				if conflicts > 0 {
					return nil, schedulerTimerConflict(targetAutomation)
				}
			}
			timerChanges.Enabled = req.Enabled
		}
	}

	rows, err := dal.UpdateSchedulerEventInTenant(ctx, id, tenantID, changes)
	if err != nil {
		return nil, mapSchedulerDBError(err)
	}
	if rows == 0 {
		return nil, schedulerNotFound()
	}
	if isScene && hasSceneTimerChanges(timerChanges) {
		timerRows, err := dal.UpdateSchedulerSceneTimer(ctx, tenantID, existing.ID, timerChanges, now)
		if err != nil {
			return nil, mapSchedulerDBError(err)
		}
		if timerRows == 0 {
			// 注册行在但执行行不在：半状态，如实报错而不是静默吞掉（创建走同事务，正常不会发生）。
			return nil, errcode.NewWithMessage(errcode.CodeDBError, "scene timer row is missing for a scene event; data consistency broken")
		}
	}
	return dal.GetSchedulerEventInTenant(ctx, id, tenantID)
}

// hasSceneTimerChanges 判断本次请求是否携带执行行变更。
func hasSceneTimerChanges(changes dal.SceneTimerChanges) bool {
	return changes.SceneAutomationID != nil || changes.CronExpr != nil ||
		changes.NextRunAt != nil || changes.Enabled != nil
}

// DeleteSchedulerEvent 删除注册事件；scene 事件先删执行行（timer 先于注册行：
// 半状态宁可留下可见的孤儿注册行，也不留一个失去登记的隐形执行器）。
func (SchedulerService) DeleteSchedulerEvent(ctx context.Context, id string, claims *utils.UserClaims) (string, error) {
	tenantID, err := schedulerClaimsTenant(claims)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(id) == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "event id is required")
	}
	existing, err := dal.GetSchedulerEventInTenant(ctx, id, tenantID)
	if err != nil {
		return "", mapSchedulerDBError(err)
	}
	if existing.EventType == model.SchedulerEventTypeScene {
		if _, err := dal.DeleteSchedulerSceneTimer(ctx, tenantID, existing.ID); err != nil {
			return "", mapSchedulerDBError(err)
		}
	}
	rows, err := dal.DeleteSchedulerEventInTenant(ctx, id, tenantID)
	if err != nil {
		return "", mapSchedulerDBError(err)
	}
	if rows == 0 {
		return "", schedulerNotFound()
	}
	return existing.ID, nil
}
