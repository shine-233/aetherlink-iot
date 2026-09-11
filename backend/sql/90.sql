-- P0.5 清理执行面：登记 api/v1/device/preRegister/cleanup 路由资源。
--
-- 背景：该路由已在 router/apps/device.go 挂载（POST preRegister/cleanup），
-- 但从未写入 casbin_rule；而 casbin.route-audit-mode 默认 fail-fast，
-- 「CasbinRBAC 之后注册的路由必须登记进资源表」，缺登记会让后端拒绝启动。
-- 同批次的 preRegister 与 preRegister/export 已在 63.sql 登记，只有 cleanup 漏了。
--
-- 两条 INSERT 都带 WHERE NOT EXISTS，可重跑；已存在的记录不会被改写。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/preRegister/cleanup')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 清理会删除设备，只授予管理员角色；不给普通租户用户。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device/preRegister/cleanup'),
  ('TENANT_ADMIN', 'api/v1/device/preRegister/cleanup')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
