-- 75.sql: 模板市场运营化——下载计数列 + 行业打包路由登记（PHASE-D-D10，2026-09-06）。
-- 1) device_templates.download_count：单模板导出（单个/打包）次数计数，市场浏览页展示热度；
-- 2) g2 资源登记 + p 授权（SYS_ADMIN + TENANT_ADMIN，与模板市场导出同口径）。
-- 幂等性：IF NOT EXISTS / NOT EXISTS 守卫，重放无副作用。

ALTER TABLE device_templates ADD COLUMN IF NOT EXISTS download_count BIGINT NOT NULL DEFAULT 0;

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/template/market/bundle')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device/template/market/bundle'),
  ('TENANT_ADMIN', 'api/v1/device/template/market/bundle')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path
);
