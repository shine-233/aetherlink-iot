-- 补登新增路由的 Casbin 资源与授权。
--
-- 背景：router_init.go 中 CasbinRBAC 之后注册的路由必须登记进 casbin_rule，
-- 而 casbin.route-audit-mode 默认 fail-fast——缺登记会让后端在启动期直接拒绝启动
-- （见 router/casbin_audit.go、90.sql 对 preRegister/cleanup 的同类修复）。
--
-- 本批补登：
--   1. api/v1/ota/task/:id/governance-apply
--      P0.3 灰度执行面（POST）。同批的 governance-preview 已在 63.sql 登记，
--      只有执行面漏了——"能预览却不能执行"到这里才真正可用。
--   2. api/v1/entity-relations 系列
--      P1.1 通用实体关系的 CRUD 入口（POST/GET/DELETE）。
--
-- 两组 INSERT 都带 WHERE NOT EXISTS，可重跑；已存在的记录不会被改写。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/ota/task/:id/governance-apply'),
  ('api/v1/entity-relations'),
  ('api/v1/entity-relations/:id'),
  ('api/v1/entity-relations/by-entity')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 角色授权：三个内置角色均可访问。
-- 关系的可见性与越权由服务层的租户 Scope 守卫决定（跨租户一律表现为 not found），
-- 这里不额外做角色区分——否则会出现"路由能通但关系查不到"的假权限。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, t.path, 'allow'
FROM (VALUES ('SYS_ADMIN'), ('TENANT_ADMIN'), ('TENANT_USER')) AS r(role)
CROSS JOIN (VALUES
  ('api/v1/ota/task/:id/governance-apply'),
  ('api/v1/entity-relations'),
  ('api/v1/entity-relations/:id'),
  ('api/v1/entity-relations/by-entity')
) AS t(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
   WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = t.path
);
