-- 补登 P2.2 轻量分析路由的 Casbin 资源与授权。
--
-- 背景：router_init.go 中 CasbinRBAC 之后注册的路由必须登记进 casbin_rule，
-- 而 casbin.route-audit-mode 默认 fail-fast——缺登记会让后端在启动期直接拒绝启动
-- （见 router/casbin_audit.go、90.sql / 91.sql 的同类修复）。
--
-- 本批补登：api/v1/telemetry/analysis 与 api/v1/telemetry/analysis/export。
-- 编号说明：93/94/95 已被其它在途改动占用，本迁移顺延为 96。
--
-- 两条 INSERT 都带 WHERE NOT EXISTS，可重跑；已存在的记录不会被改写。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/telemetry/analysis'),
  ('api/v1/telemetry/analysis/export')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, t.path, 'allow'
FROM (VALUES ('SYS_ADMIN'), ('TENANT_ADMIN'), ('TENANT_USER')) AS r(role)
CROSS JOIN (VALUES
  ('api/v1/telemetry/analysis'),
  ('api/v1/telemetry/analysis/export')
) AS t(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
   WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = t.path
);
