-- P0.4 定时器触发持久化：服务重启后调度不丢任务。
--
-- 此前场景自动化只由设备遥测触发，没有任何持久化的定时触发：进程内存里的
-- 定时器随重启全部消失，也没有任何记录能证明"某次触发本应发生"。
-- 本表把"下一次该在什么时候触发"落库，配合租约实现：
--   - 多副本并发只有一个能领走（fenced lease）
--   - 领取后崩溃的定时器在租约到期后可被重新领取（不丢任务）
--   - 触发完成后才推进 next_run_at（不做"先推进后执行"的乐观假设）
--
-- 编号说明：88 已被 P1.3/P1.4（SCADA 与移动端推送）占用，本迁移顺延为 89。

CREATE TABLE IF NOT EXISTS public.scene_automation_timers (
    id                   text PRIMARY KEY,
    tenant_id            text        NOT NULL,
    scene_automation_id  text        NOT NULL,
    cron_expr            text        NOT NULL,
    timezone             text        NOT NULL DEFAULT 'UTC',
    enabled              boolean     NOT NULL DEFAULT true,
    -- 下一次应触发的时刻。时钟推进的唯一依据。
    next_run_at          timestamptz NOT NULL,
    last_run_at          timestamptz NULL,
    -- 租约：非空表示已被某个副本领走且仍在执行。
    lease_owner          text        NULL,
    lease_until          timestamptz NULL,
    -- 连续失败次数；达到上限后停摆等待人工介入，避免无限重试打爆下游。
    consecutive_failures integer     NOT NULL DEFAULT 0,
    last_error           text        NULL,
    created_at           timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at           timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- 领取扫描：按 next_run_at 过滤，必须能走索引。
CREATE INDEX IF NOT EXISTS idx_scene_timers_due
    ON public.scene_automation_timers (next_run_at)
    WHERE enabled = true;

-- 同一场景只允许一条启用的定时器，避免重复触发被放大成多次执行。
CREATE UNIQUE INDEX IF NOT EXISTS uq_scene_timers_automation
    ON public.scene_automation_timers (scene_automation_id)
    WHERE enabled = true;

COMMENT ON TABLE public.scene_automation_timers IS
    'P0.4 持久化的场景定时触发；next_run_at 与租约共同保证重启后不丢任务。';
COMMENT ON COLUMN public.scene_automation_timers.lease_until IS
    '租约到期时刻。超过该时刻仍未被释放，视为领取方已崩溃，可被重新领取。';
