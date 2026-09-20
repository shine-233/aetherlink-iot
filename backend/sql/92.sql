-- P1.2 规则链输入回放的持久化（回放记录落库）。
--
-- 背景：rule_chain_replay.go 的 recorder 是**刻意默认不接线**的注入点——
-- 回放必须留存输入，这与 trace 的"审计最小化"不是一回事（trace 只记事实、
-- replay 记输入），默认状态不该产生第二份数据副本。
-- 因此本表只是"运维显式开启回放留存时"的落点，建表本身不改变任何默认行为。
--
-- 设计要点：
--   1. 记录按 (execution_id, node_id) 唯一：同一次执行的同一节点只留一条快照，
--      重跑不会产生第二条，回放因此是幂等的。
--   2. payload/metadata 用 jsonb 原样留存；回放必然要留输入，
--      因此本表的敏感性与遥测载荷同级，必须带 tenant_id 隔离。
--   3. 租户隔离由 tenant_id 承担；跨租户读取一律表现为未命中。

CREATE TABLE IF NOT EXISTS public.rule_chain_replay_records (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     text        NOT NULL,
    chain_id      text        NOT NULL,
    execution_id  text        NOT NULL,
    node_id       text        NOT NULL,
    node_type     text        NOT NULL,
    payload       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    metadata      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    pass          boolean     NOT NULL,
    error         text        NULL,
    recorded_at   timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- 回放按执行 ID 取全部节点快照，顺序由记录时间决定。
CREATE INDEX IF NOT EXISTS idx_rule_chain_replay_exec
    ON public.rule_chain_replay_records (tenant_id, execution_id, recorded_at);

-- 同一次执行的同一节点只留一条快照：重跑不产生第二条，保证回放幂等。
CREATE UNIQUE INDEX IF NOT EXISTS uq_rule_chain_replay_node
    ON public.rule_chain_replay_records (execution_id, node_id);

COMMENT ON TABLE public.rule_chain_replay_records IS
    'P1.2 规则链回放输入快照；仅在运维显式开启回放留存时写入，默认不产生数据。';
