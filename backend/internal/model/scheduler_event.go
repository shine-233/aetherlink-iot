// 文件用途：TB-48 统一调度器的数据模型——scheduler_events 注册表模型与聚合列表 DTO。
// 核心逻辑：SchedulerEvent 是统一注册面（scene/report/rpc 三类事件），SchedulerEventItem
//
//	是只读聚合列表的统一条目形状（三套存量调度 + 注册行归一为同一 JSON 契约，
//	含来源类型 source_type 与来源系统 origin）。
//
// 关键注意事项：
//  1. 注册面不接管执行：scene 事件落到既有 scene_automation_timers 机制执行
//     （定时行 id 与事件行 id 相同），report/rpc 注册行只做统一登记与展示，
//     执行仍归各存量系统——明确不做"三套存量调度迁移为统一执行器"。
//  2. 聚合条目按 next_run_at 排序、来源类型着色由前端依据 source_type 完成。
package model

import "time"

const (
	TableNameSchedulerEvent = "scheduler_events"

	// SchedulerEventType* 注册事件类型（与 137.sql CHECK 约束一致）。
	SchedulerEventTypeScene  = "scene"
	SchedulerEventTypeReport = "report"
	SchedulerEventTypeRPC    = "rpc"

	// SchedulerRefType* 目标类型口径（scene 事件固定 scene_automation，服务层校验同租户存在）。
	SchedulerRefTypeSceneAutomation = "scene_automation"
	SchedulerRefTypeReportSchedule  = "report_schedule"
	SchedulerRefTypeFleetCommandJob = "fleet_command_job"

	// SchedulerSource* 聚合条目的来源类型（前端按此着色）。
	SchedulerSourceScene  = "scene"
	SchedulerSourceReport = "report"
	SchedulerSourceRPC    = "rpc"

	// SchedulerOrigin* 聚合条目的来源系统（区别同一 source_type 下的注册行与存量行）。
	SchedulerOriginSceneTimer      = "scene_timer"       // scene_automation_timers 存量行
	SchedulerOriginReportSchedule  = "report_schedule"   // report_schedules 存量行
	SchedulerOriginFleetCommandJob = "fleet_command_job" // command_jobs 定时行（status='scheduled'）
	SchedulerOriginRegistry        = "scheduler_registry"
)

// SchedulerEvent 统一调度事件注册行（scheduler_events 表）。
type SchedulerEvent struct {
	ID        string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID  string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name      string     `gorm:"column:name;not null" json:"name"`
	EventType string     `gorm:"column:event_type;not null" json:"event_type"`
	RefType   string     `gorm:"column:ref_type" json:"ref_type"`
	RefID     string     `gorm:"column:ref_id" json:"ref_id"`
	CronExpr  string     `gorm:"column:cron" json:"cron"`
	NextRunAt *time.Time `gorm:"column:next_run_at" json:"next_run_at"`
	Enabled   bool       `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName SchedulerEvent's table name
func (*SchedulerEvent) TableName() string { return TableNameSchedulerEvent }

// SchedulerEventCreateReq 注册事件创建请求。
// scene：ref_id（场景自动化 ID）与 cron 必填，next_run_at 由服务层按 UTC 计算，不接受传入；
// report：cron 必填，ref_type/ref_id 可选登记；rpc：next_run_at 必填（一次性），cron 必须为空。
type SchedulerEventCreateReq struct {
	Name      string     `json:"name" validate:"required,max=128"`
	EventType string     `json:"event_type" validate:"required,oneof=scene report rpc"`
	RefType   string     `json:"ref_type" validate:"omitempty,max=50"`
	RefID     string     `json:"ref_id" validate:"omitempty,max=64"`
	CronExpr  string     `json:"cron" validate:"omitempty,max=64"`
	NextRunAt *time.Time `json:"next_run_at"`
	Enabled   *bool      `json:"enabled"`
}

// SchedulerEventUpdateReq 注册事件更新请求。event_type 不可变（不在请求里）；
// scene 事件的 ref_id/cron/enabled 变更会同步到同 id 的 scene_automation_timers 行。
type SchedulerEventUpdateReq struct {
	Name      *string    `json:"name" validate:"omitempty,max=128"`
	RefType   *string    `json:"ref_type" validate:"omitempty,max=50"`
	RefID     *string    `json:"ref_id" validate:"omitempty,max=64"`
	CronExpr  *string    `json:"cron" validate:"omitempty,max=64"`
	NextRunAt *time.Time `json:"next_run_at"`
	Enabled   *bool      `json:"enabled"`
}

// SchedulerEventListRequest 聚合列表请求（GET /api/v1/scheduler/events）。
// from_ms/to_ms 是毫秒时间戳窗口，过滤 next_run_at 落在窗口内的条目；
// 空窗口表示不限。月视图日历按窗口拉取当月事件。
type SchedulerEventListRequest struct {
	Page       int    `form:"page" json:"page" validate:"omitempty,min=1"`
	PageSize   int    `form:"page_size" json:"page_size" validate:"omitempty,min=1,max=500"`
	SourceType string `form:"source_type" json:"source_type" validate:"omitempty,oneof=scene report rpc"`
	Search     string `form:"search" json:"search" validate:"omitempty,max=128"`
	Enabled    *bool  `form:"enabled" json:"enabled"`
	FromMs     int64  `form:"from_ms" json:"from_ms"`
	ToMs       int64  `form:"to_ms" json:"to_ms"`
}

// SchedulerEventItem 聚合列表统一条目：三套存量调度 + 注册行归一后的形状。
// ID 是来源行主键（scene timer 行 id 与其注册行 id 相同，天然去重）。
type SchedulerEventItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	SourceType string     `json:"source_type"` // scene | report | rpc（前端按此着色）
	Origin     string     `json:"origin"`      // scene_timer | report_schedule | fleet_command_job | scheduler_registry
	RefType    string     `json:"ref_type"`
	RefID      string     `json:"ref_id"`
	CronExpr   string     `json:"cron"`
	Timezone   string     `json:"timezone"`
	Enabled    bool       `json:"enabled"`
	NextRunAt  *time.Time `json:"next_run_at"`
	LastRunAt  *time.Time `json:"last_run_at"`
}

// SchedulerEventListResponse 聚合列表响应（内存归并后分页，total 为过滤后总数）。
type SchedulerEventListResponse struct {
	List     []*SchedulerEventItem `json:"list"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}
