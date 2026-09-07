-- 79.sql: Phase D3 补登记手动触发端点 api/v1/report/schedules/:id/run 的 casbin 资源与授权（2026-09-06）。
-- 触发：78.sql 漏登记手动触发端点 api/v1/report/schedules/:id/run，导致全新库启动期
--       casbin route-audit（fail-fast）报错 1 条未登记路由（"1 protected routes are not registered"）。
--       本迁移在已执行 78.sql 的库上补齐该路径的 g2 资源行与 p 授权行。
-- 幂等性：INSERT 前置 NOT EXISTS 守卫；对 78.sql 已含该路径的全新库为 no-op（不会重复插入）。
-- 授权口径：与列表/详情一致，仅 SYS_ADMIN + TENANT_ADMIN 可手动触发报表投递。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES ('api/v1/report/schedules/:id/run')) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/report/schedules/:id/run'),
  ('TENANT_ADMIN', 'api/v1/report/schedules/:id/run')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
