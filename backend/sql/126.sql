-- 126.sql — 告警 SLA 计时与超时升级（TB-27，对标 ThingsBoard 告警 SLA/升级闭环）
--
-- 背景：
--   1. ROADMAP §4.1 TB-27：告警协作的分配/评论已完整，SLA 计时与超时升级缺失；
--   2. 本迁移给告警配置（alarm_config）增加可空的 sla_hours 列（SLA 时限，小时），
--      空（NULL）= 不启用 SLA 超时升级，与 125.sql 的档案级可空列同一约定口径；
--   3. 给告警历史（alarm_history）增加 sla_due_at（触发时刻按 sla_hours 起算的到期时间，
--      可空 = 该条告警未启用 SLA）与 sla_breached（cron 升级标记，默认 FALSE）；
--   4. 超时升级执行体：initialize/croninit/cron.go 每 5 分钟扫描到期未恢复未 breach 的
--      活动告警 → 标记 sla_breached 并把严重度升一档（L→M→H，H 到顶保持，N 不动），
--      remark JSON 追加 sla_escalation 审计（沿用 43.sql text 列 remark 存 JSON 的现状，
--      details 结构化 JSONB 列迁移不在本批次范围）。

-- ---- 1. alarm_config 增加 SLA 时限列 ----
-- INT NULL：NULL=不启用；>0 的整数 = 触发后允许的处理时限（小时）。
-- 服务层（service.normalizeAlarmSlaHours）把 <=0 折叠为 NULL，负数直接参数错误。
ALTER TABLE public.alarm_config
    ADD COLUMN IF NOT EXISTS sla_hours INT;

COMMENT ON COLUMN public.alarm_config.sla_hours IS '告警SLA时限（小时；空=不启用超时升级）';

-- ---- 2. alarm_history 增加 SLA 到期与 breach 标记列 ----
-- sla_due_at TIMESTAMPTZ NULL：告警触发时刻 + sla_hours；NULL = 未启用或恢复(N)行。
-- sla_breached BOOLEAN NOT NULL DEFAULT FALSE：PG11+ 快速默认，锁表风险可忽略。
ALTER TABLE public.alarm_history
    ADD COLUMN IF NOT EXISTS sla_due_at TIMESTAMPTZ;

ALTER TABLE public.alarm_history
    ADD COLUMN IF NOT EXISTS sla_breached BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN public.alarm_history.sla_due_at IS 'SLA到期时间（触发时刻+sla_hours；空=未启用SLA）';
COMMENT ON COLUMN public.alarm_history.sla_breached IS 'SLA是否已超时升级（cron扫描标记；TRUE后不再重复升级）';

-- ---- 3. cron 扫描索引 ----
-- 升级任务的扫描条件是 sla_due_at < now() AND alarm_status IN (H,M,L) AND NOT sla_breached；
-- 活动告警行占比很小，partial index 只覆盖启用 SLA 的行，避免全表扫。
CREATE INDEX IF NOT EXISTS idx_alarm_history_sla_due_at
    ON public.alarm_history (sla_due_at)
    WHERE sla_due_at IS NOT NULL;
