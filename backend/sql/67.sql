-- 67.sql: 规则链 2.0 —— 节点级调试 trace 与 flow.checkpoint 落库表（PHASE-D-D1，2026-09-06）。
-- 背景：D1 将规则链扩到 28 种节点（富化/流控/分析/外发），配套两条新表：
--   rule_chain_node_traces   节点执行 trace（输入/输出摘要不落库，仅 pass/错误摘要/耗时——审计最小化）；
--   rule_chain_checkpoints   flow.checkpoint 节点把消息持久化为检查点（断点续跑/下游幂等锚点）。
-- 幂等性：IF NOT EXISTS，重放无副作用；回滚：DROP TABLE 两表即可（trace/checkpoint 均可丢）。

CREATE TABLE IF NOT EXISTS rule_chain_node_traces (
    id          VARCHAR(36) PRIMARY KEY,
    exec_id     VARCHAR(36) NOT NULL,
    chain_id    VARCHAR(36) NOT NULL,
    node_id     VARCHAR(64) NOT NULL,
    node_type   VARCHAR(64) NOT NULL,
    pass        BOOLEAN NOT NULL DEFAULT TRUE,
    error_msg   TEXT,
    elapsed_ms  BIGINT NOT NULL DEFAULT 0,
    tenant_id   VARCHAR(36) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rcnt_chain_node_time
    ON rule_chain_node_traces (chain_id, node_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rcnt_tenant_time
    ON rule_chain_node_traces (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS rule_chain_checkpoints (
    id          VARCHAR(36) PRIMARY KEY,
    exec_id     VARCHAR(36) NOT NULL,
    chain_id    VARCHAR(36) NOT NULL,
    node_id     VARCHAR(64) NOT NULL,
    tenant_id   VARCHAR(36) NOT NULL,
    device_id   VARCHAR(36),
    payload     JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rcc_chain_node_time
    ON rule_chain_checkpoints (chain_id, node_id, created_at DESC);
