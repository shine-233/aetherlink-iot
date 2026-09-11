-- P0.4 场景与 Flow 语义：为场景自动化持久化执行窗口。
-- 此前 ExecutionWindow / FlowEngine.CanRun 虽已实现，但场景自动化表没有任何
-- 窗口列，也没有任何生产调用点，等于"写了但没接线"，窗口语义实际不生效。
-- 三列均为可空：NULL 表示该侧无界（保持既有"始终可执行"行为），
-- 因此本次迁移对存量场景是纯增量、不改变任何既有执行结果。

ALTER TABLE public.scene_automations
    ADD COLUMN IF NOT EXISTS execution_starts_at  timestamptz NULL,
    ADD COLUMN IF NOT EXISTS execution_expires_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS execution_timezone   text NULL;

-- 时区若填写必须是可解析的 IANA 时区名。留空按 UTC 处理（由服务层显式声明），
-- 但填了非法值不能静默降级，故在此加约束挡住明显的空串/纯空白。
ALTER TABLE public.scene_automations
    DROP CONSTRAINT IF EXISTS scene_automations_execution_timezone_not_blank;

ALTER TABLE public.scene_automations
    ADD CONSTRAINT scene_automations_execution_timezone_not_blank
    CHECK (execution_timezone IS NULL OR btrim(execution_timezone) <> '');

COMMENT ON COLUMN public.scene_automations.execution_starts_at IS
    'P0.4 执行窗口下界，NULL 表示无下界；区间为左闭右开 [starts_at, expires_at)。';
COMMENT ON COLUMN public.scene_automations.execution_expires_at IS
    'P0.4 执行窗口上界，NULL 表示无上界；恰好等于该时刻不再执行。';
COMMENT ON COLUMN public.scene_automations.execution_timezone IS
    'P0.4 执行窗口时区（IANA 名）。NULL/空按 UTC；非法值在服务层 fail closed，不静默兜底。';
