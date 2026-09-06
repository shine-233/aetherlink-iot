-- 76.sql: 计算字段历史重算路由登记与授权(PHASE-D-D4,2026-09-06)。
-- 新端点:POST/GET api/v1/calcfield/recompute、GET api/v1/calcfield/recompute/:id。
-- g2 资源登记 + p 授权(SYS_ADMIN + TENANT_ADMIN;计算字段属租户管理能力,TENANT_USER 不开放——对齐既有口径)。
-- 幂等性:NOT EXISTS 守卫,重放无副作用。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/calcfield/recompute'),
  ('api/v1/calcfield/recompute/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/calcfield/recompute'),
  ('SYS_ADMIN',    'api/v1/calcfield/recompute/:id'),
  ('TENANT_ADMIN', 'api/v1/calcfield/recompute'),
  ('TENANT_ADMIN', 'api/v1/calcfield/recompute/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path
);
