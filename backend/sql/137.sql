-- 137.sql — TB-48 统一调度器（聚合 + 日历）第一批：scheduler_events 注册面 + 三源聚合后端
--
-- 背景：
--   1. 平台已有三套互不相通的调度：场景定时触发（89.sql scene_automation_timers，由
--      app 层 worker 领取租约后执行场景联动）、定时报表（report_schedules，PHASE-D-D3）、
--      舰队命令定时下发（command_jobs 中 status='scheduled' 且 scheduled_at 非空的定时行）。
--      三处各管各的，没有一处能回答"这个租户接下来有什么调度"。TB-48 建统一注册面
--      scheduler_events 与只读聚合 API GET /api/v1/scheduler/events，先让调度"可见"；
--   2. 本迁移只建注册表与授权面，不迁移任何既有执行器：三套存量的执行语义保持现网
--      行为（明确不做统一执行器迁移）。新建 scene 事件落到既有 scene automation timer
--      机制执行（服务层在同一事务里写 scheduler_events + scene_automation_timers，
--      定时行 id 与事件行 id 相同，供后续更新/删除同步与聚合去重）；
--   3. 表结构（按 TB-48 缺口单给定的列集）：
--        id / tenant_id / name / event_type(scene|report|rpc) / ref_type / ref_id /
--        cron / next_run_at / enabled。
--      ref_type 口径：scene→scene_automation、report→report_schedule、rpc→fleet_command_job
--      （rpc 注册行也可指向设备等自由目标，服务层不强校验）；cron 列存 5/6 段 cron
--      表达式，rpc 一次性事件留空、直接给 next_run_at。注册面 v1 统一按 UTC 评估
--      cron（scene_automation_timers.timezone 固定 'UTC'），按事件自定义时区留后续版本；
--   4. API 面（Casbin g2/p 登记，仅 SYS_ADMIN / TENANT_ADMIN；TENANT_USER fail-closed，
--      口径同 131/136.sql）：
--      - GET    /api/v1/scheduler/events        只读聚合三套存量调度 + 注册行（含来源类型）
--      - POST   /api/v1/scheduler/events        注册事件（scene 事件同步建 scene timer 行）
--      - GET    /api/v1/scheduler/events/:id    注册事件详情
--      - PUT    /api/v1/scheduler/events/:id    更新（event_type 不可变；scene 事件同步 timer 行）
--      - DELETE /api/v1/scheduler/events/:id    删除（scene 事件同步删 timer 行）
--   5. 管理菜单（sys_ui_elements）：/scheduler/calendar 页面行挂新建 scheduler 顶级目录。
--      前端日历页四件套（下一阶段交付）必须与本种子对齐：路由名 scheduler、子路由
--      scheduler_calendar、组件 view.scheduler_calendar、i18n 键 route.scheduler /
--      route.scheduler_calendar、页面文案键 page.scheduler_calendar.*。

-- ---- 1. 统一调度事件注册表 (scheduler_events) ----
CREATE TABLE IF NOT EXISTS public.scheduler_events (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    event_type VARCHAR(20) NOT NULL,
    ref_type VARCHAR(50),
    ref_id VARCHAR(64),
    cron VARCHAR(64),
    next_run_at TIMESTAMP WITH TIME ZONE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_scheduler_events_event_type CHECK (event_type IN ('scene', 'report', 'rpc'))
);

CREATE INDEX IF NOT EXISTS idx_scheduler_events_tenant_id ON public.scheduler_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_scheduler_events_tenant_type ON public.scheduler_events(tenant_id, event_type);
CREATE INDEX IF NOT EXISTS idx_scheduler_events_tenant_next_run ON public.scheduler_events(tenant_id, next_run_at);

COMMENT ON TABLE public.scheduler_events IS
    'TB-48 统一调度事件注册面；scene 事件同步 scene_automation_timers 执行，report/rpc 注册行仅做统一登记展示';
COMMENT ON COLUMN public.scheduler_events.event_type IS
    '事件类型：scene（场景联动，落到 scene_automation_timers 执行）/ report（定时报表登记）/ rpc（一次性命令登记）';
COMMENT ON COLUMN public.scheduler_events.ref_type IS
    '目标类型：scene_automation / report_schedule / fleet_command_job 或自由标注；scene 事件固定 scene_automation';
COMMENT ON COLUMN public.scheduler_events.ref_id IS
    '目标实体 ID；scene 事件为场景自动化 ID（服务层校验同租户存在）';
COMMENT ON COLUMN public.scheduler_events.cron IS
    '5/6 段 cron 表达式；rpc 一次性事件为空、直接落 next_run_at';
COMMENT ON COLUMN public.scheduler_events.next_run_at IS
    '下一次触发时刻；scene/report 事件由服务层按 cron 以 UTC 计算并随更新重算，rpc 直接给定';
COMMENT ON COLUMN public.scheduler_events.enabled IS
    '启用开关；scene 事件与同名 id 的 scene_automation_timers 行 enabled 同步';

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/scheduler/events'),
  ('api/v1/scheduler/events/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
-- 仅 SYS_ADMIN / TENANT_ADMIN：调度是租户管理面（聚合可见全部存量调度 + 注册写面），
-- TENANT_USER fail-closed（口径同 131.sql/136.sql）。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/scheduler/events'),
  ('SYS_ADMIN',    'api/v1/scheduler/events/:id'),
  ('TENANT_ADMIN', 'api/v1/scheduler/events'),
  ('TENANT_ADMIN', 'api/v1/scheduler/events/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 管理菜单 (sys_ui_elements，新建 scheduler 顶级目录，对标 136.sql mobile-app 目录行) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /scheduler/calendar 由下一阶段前端日历页四件套注册，菜单行先行就位（NOT EXISTS 守卫）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'f0a1b2c3-9d04-4e56-9f67-8a9b0c1d2e33',
    '0',
    'scheduler', 1, 121, '/scheduler', 'mdi:calendar-clock', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '统一调度', CURRENT_TIMESTAMP,
    'Unified scheduler calendar over scene timers, report schedules and fleet command jobs (TB-48)', 'route.scheduler',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'scheduler'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'f0a1b2c3-9d04-4e56-9f67-8a9b0c1d2e34',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'scheduler'),
    'scheduler_calendar', 3, 1, '/scheduler/calendar', 'mdi:calendar-clock', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '调度日历', CURRENT_TIMESTAMP,
    'Month-grid calendar aggregating scheduled events by source type (TB-48)', 'route.scheduler_calendar',
    'view.scheduler_calendar'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'scheduler_calendar'
);
