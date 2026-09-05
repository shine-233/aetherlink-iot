-- 66.sql: 模板市场路由登记与授权（ROADMAP 模板市场 MVP，2026-09-05）。
-- 新端点（fail-fast 审计要求登记 + 授权双行）：
--   GET  api/v1/device/template/export/:id   按读权限导出模板描述符
--   POST api/v1/device/template/import       导出载荷创建为调用者租户新模板（幂等）
-- 1) g2 资源登记（g2, url, url——受保护路由注册表，缺失则启动审计 fail-fast）；
-- 2) p 授权（SYS_ADMIN + TENANT_ADMIN；市场操作属租户管理能力，TENANT_USER 不开放）。
-- 幂等性：均前置 NOT EXISTS 守卫，重放无副作用。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/template/export/:id'),
  ('api/v1/device/template/import')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',   'api/v1/device/template/export/:id'),
  ('TENANT_ADMIN','api/v1/device/template/export/:id'),
  ('SYS_ADMIN',   'api/v1/device/template/import'),
  ('TENANT_ADMIN','api/v1/device/template/import')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path
);
