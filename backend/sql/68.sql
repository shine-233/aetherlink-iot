-- 68.sql: 规则链节点 trace 查询路由登记与授权（PHASE-D-D1，2026-09-06）。
-- 新端点：GET api/v1/rule-chains/:id/nodes/:nodeId/traces（画布节点调试面板）。
-- 1) g2 资源登记（fail-fast 审计要求）；2) p 授权（SYS_ADMIN + TENANT_ADMIN，
--    规则链编辑属租户管理能力，TENANT_USER 不开放——与 66.sql 口径一致）。
-- 幂等性：均前置 NOT EXISTS 守卫，重放无副作用。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/rule-chains/:id/nodes/:nodeId/traces')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/nodes/:nodeId/traces'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/nodes/:nodeId/traces')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path
);
