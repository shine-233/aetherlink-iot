-- 110.sql — Product CRUD & Casbin 权限登记 (TB-15 & P0.5 闭环)
--
-- 背景：
--   1. 开放 Product 增删改查完整路由，解除预注册建档无产品可用的阻断；
--   2. 登记 api/v1/product/:id 路由规则并授予 SYS_ADMIN, TENANT_ADMIN, TENANT_USER；
--   3. 确保多租户隔离在 Casbin 与 DAL 两层双重闭环。

-- 1. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/product/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 2. 授权角色 (p)
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/product/:id'),
  ('TENANT_ADMIN', 'api/v1/product/:id'),
  ('TENANT_USER',  'api/v1/product/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
