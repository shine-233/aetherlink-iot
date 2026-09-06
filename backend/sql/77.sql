-- 77.sql: Phase D 集成修复——补齐 3 条未登记受保护路由的 casbin 登记与授权（2026-09-06）。
-- 触发：全新库部署启动期 casbin route-audit（fail-fast）实测报错 3 条未登记路由，
--       这三条来自 D4/D9/D10 新增端点，其迁移只登记了部分路径或登记了错误路径：
--   - api/v1/calculated_fields/recompute        （D4 实际路由前缀是 calculated_fields，76.sql 误登记为 calcfield）
--   - api/v1/calculated_fields/recompute/:id    （同上，启动审计唯一报出的漏登记项）
--   - api/v1/device/template/market/catalog     （D10 市场分类目录端点漏登记）
--   - api/v1/plugins/:id/token                  （D9 插件接入凭证端点漏登记）
-- 1) 清理 D4 误登记的 calcfield/recompute*（g2 与 p 全清，含 :id）；
-- 2) 按既有口径重新登记 4 条路径的 g2 资源行；
-- 3) p 授权口径：calculated_fields/recompute* 与 plugins/:id/token = SYS_ADMIN + TENANT_ADMIN
--    （重算会触发后台任务、插件 token 属凭证发放，均不向 TENANT_USER 开放，与 76.sql 原意一致）；
--    device/template/market/catalog = SYS_ADMIN + TENANT_ADMIN + TENANT_USER（只读浏览，对齐 market/list）。
-- 幂等性：INSERT 前置 NOT EXISTS 守卫；DELETE 可重放无副作用。

DELETE FROM casbin_rule
WHERE (ptype = 'g2' OR ptype = 'p')
  AND v0 LIKE 'api/v1/calcfield/recompute%';

DELETE FROM casbin_rule
WHERE ptype = 'p'
  AND v1 LIKE 'api/v1/calcfield/recompute%';

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/calculated_fields/recompute'),
  ('api/v1/calculated_fields/recompute/:id'),
  ('api/v1/device/template/market/catalog'),
  ('api/v1/plugins/:id/token')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/calculated_fields/recompute'),
  ('TENANT_ADMIN', 'api/v1/calculated_fields/recompute'),
  ('SYS_ADMIN',    'api/v1/calculated_fields/recompute/:id'),
  ('TENANT_ADMIN', 'api/v1/calculated_fields/recompute/:id'),
  ('SYS_ADMIN',    'api/v1/plugins/:id/token'),
  ('TENANT_ADMIN', 'api/v1/plugins/:id/token'),
  ('SYS_ADMIN',    'api/v1/device/template/market/catalog'),
  ('TENANT_ADMIN', 'api/v1/device/template/market/catalog'),
  ('TENANT_USER',  'api/v1/device/template/market/catalog')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
