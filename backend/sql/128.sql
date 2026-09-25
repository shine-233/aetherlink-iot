-- 128.sql — TB-25 实体版本控制差异对比端点 Casbin 登记（Entity Version Diff）
--
-- 背景：
--   1. 新端点 GET /api/v1/entity_versions/:id/diff/:target_id：递归比较两份
--      entity_versions.snapshot JSONB，返回新增/删除/修改点号路径列表（ROADMAP TB-25）；
--   2. 本迁移只做 Casbin 路由登记，不建表不加列（快照仍存于 58.sql 的 entity_versions 表）；
--   3. 授权口径与 63.sql 对 entity_versions 组一致：三个内置角色全量允许，
--      参数路径由 casbin urlPatternMatch 模式通道命中。
-- 幂等：NOT EXISTS 守卫，可重复执行。
-- 回滚：
--   DELETE FROM casbin_rule WHERE ptype='g2' AND v0='api/v1/entity_versions/:id/diff/:target_id';
--   DELETE FROM casbin_rule WHERE ptype='p' AND v1='api/v1/entity_versions/:id/diff/:target_id';

-- ---- 1. Casbin 路由登记 (g2 资源自引用) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/entity_versions/:id/diff/:target_id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 2. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/entity_versions/:id/diff/:target_id'),
  ('TENANT_ADMIN', 'api/v1/entity_versions/:id/diff/:target_id'),
  ('TENANT_USER',  'api/v1/entity_versions/:id/diff/:target_id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
