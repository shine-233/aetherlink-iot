-- 54.sql: 规则链（ROADMAP B2）——可视化 DAG 编排的底层通用能力。
-- 背景：场景自动化面向高层联动；规则链提供更底层的
--   「触发器 → 过滤 → 转换 → 动作」DAG 编排，节点类型可注册扩展。
-- 处理：
--   1) rule_chains 表：graph 列存 {nodes:[], edges:[]} JSONB；
--      enabled 直接控制是否参与上行执行。
--   2) 幂等：重复执行迁移不会产生重复表。
-- 回滚：DROP TABLE rule_chains;（功能增量，无存量数据依赖）

CREATE TABLE IF NOT EXISTS rule_chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    graph JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rule_chains_tenant_enabled
    ON rule_chains (tenant_id, enabled);

COMMENT ON TABLE rule_chains IS '规则链：DAG 定义（nodes+edges），由上行遥测/设备上线事件驱动执行';
