-- 98.sql — P3 审计导出路由的 Casbin 登记。
-- POST /api/v1/operation_logs/export:当前租户操作日志 CSV 导出(时间窗必填,
-- request/response 载荷列不导出)。列表 GET 路径已在 63/64.sql 登记。
-- 读与导出同授:导出是列表数据的时间窗子集投影(载荷列更少),不放大暴露面。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/operation_logs/export')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/operation_logs/export'),
  ('TENANT_ADMIN', 'api/v1/operation_logs/export'),
  ('TENANT_USER',  'api/v1/operation_logs/export')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
