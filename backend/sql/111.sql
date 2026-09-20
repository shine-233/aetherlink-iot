-- 111.sql — P1.2 规则链死信队列（DLQ Sink）持久化与端点权限登记
--
-- 背景：
--   1. 节点策略终局失败自动下沉死信队列（遵循审计最小化约定，不落原始业务载荷，仅存租户、链、节点、设备、尝试次数与错误摘要）；
--   2. 登记 DLQ、单消息 Trace 串联、输入回放快照与回放执行 API 路由并授予 SYS_ADMIN, TENANT_ADMIN, TENANT_USER。

CREATE TABLE IF NOT EXISTS public.rule_chain_dead_letters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     text        NOT NULL,
    chain_id      text        NOT NULL,
    exec_id       text        NOT NULL,
    node_id       text        NOT NULL,
    node_type     text        NOT NULL,
    device_id     text        NULL,
    error         text        NULL,
    attempts      integer     NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- 租户维度死信查询（链视图按时间倒序）
CREATE INDEX IF NOT EXISTS idx_rule_chain_dead_letters_tenant_chain
    ON public.rule_chain_dead_letters (tenant_id, chain_id, created_at DESC);

-- 单消息/执行批次关联查询
CREATE INDEX IF NOT EXISTS idx_rule_chain_dead_letters_exec
    ON public.rule_chain_dead_letters (tenant_id, exec_id);

COMMENT ON TABLE public.rule_chain_dead_letters IS
    'P1.2 规则链节点终局失败死信记录（审计最小化，不含业务载荷）';

-- 1. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/rule-chains/:id/dead-letters'),
  ('api/v1/rule-chains/dead-letters'),
  ('api/v1/rule-chains/:id/executions/:execId/traces'),
  ('api/v1/rule-chains/executions/:execId/traces'),
  ('api/v1/rule-chains/:id/executions/:execId/replay-records'),
  ('api/v1/rule-chains/:id/replay')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 2. 授权角色 (p)
-- 查询端点授予管理员与普通租户用户
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/dead-letters'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/dead-letters'),
  ('TENANT_USER',  'api/v1/rule-chains/:id/dead-letters'),
  ('SYS_ADMIN',    'api/v1/rule-chains/dead-letters'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/dead-letters'),
  ('TENANT_USER',  'api/v1/rule-chains/dead-letters'),
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/executions/:execId/traces'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/traces'),
  ('TENANT_USER',  'api/v1/rule-chains/:id/executions/:execId/traces'),
  ('SYS_ADMIN',    'api/v1/rule-chains/executions/:execId/traces'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/executions/:execId/traces'),
  ('TENANT_USER',  'api/v1/rule-chains/executions/:execId/traces'),
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/executions/:execId/replay-records'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/executions/:execId/replay-records'),
  ('TENANT_USER',  'api/v1/rule-chains/:id/executions/:execId/replay-records'),
  -- 回放执行属于改变系统状态操作，仅授予管理员角色
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/replay'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/replay')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
