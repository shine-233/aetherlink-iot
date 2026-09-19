-- 116.sql — P3 租户管理与自助开通（Tenant Management & Quota Enforcement）
--
-- 背景：
--   1. ROADMAP P3 商业化闭环：租户配额（max_tenants）在全仓缺少执法点；
--   2. 本次上线独立的租户服务（internal/service/tenant.go），包含：
--      - 平台管理员租户创建与列表（受 license doc.MaxTenants 配额硬性门控）；
--      - 租户管理员层级查询（仅可见 self + 下级子孙租户）；
--      - 客户自助开通开箱入驻（POST /api/v1/tenant/provision，无需登录，受配额约束）；
--   3. Casbin 路由赋权：登记 /api/v1/tenants 与 /api/v1/tenants/:id，授予 SYS_ADMIN 与 TENANT_ADMIN。

-- ---- 1. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/tenants'),
  ('api/v1/tenants/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 2. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/tenants'),
  ('TENANT_ADMIN', 'api/v1/tenants'),
  ('SYS_ADMIN',    'api/v1/tenants/:id'),
  ('TENANT_ADMIN', 'api/v1/tenants/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
