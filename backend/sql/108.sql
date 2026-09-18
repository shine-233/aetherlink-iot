-- 108.sql — TB-9 单位换算全链路闭环（Units Conversion End-to-End）
--
-- 背景：
--   对标 ThingsBoard 4.1.0 头条能力 Units Conversion：
--   1. 登记单位字典查询与原子换算 OpenAPI 的 Casbin 路由；
--   2. 授权 SYS_ADMIN、TENANT_ADMIN 与 TENANT_USER 角色。

-- 1. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/units/registry'),
  ('api/v1/units/convert')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 2. 授权角色 (p)
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/units/registry'),
  ('SYS_ADMIN',    'api/v1/units/convert'),

  ('TENANT_ADMIN', 'api/v1/units/registry'),
  ('TENANT_ADMIN', 'api/v1/units/convert'),

  ('TENANT_USER',  'api/v1/units/registry'),
  ('TENANT_USER',  'api/v1/units/convert')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
