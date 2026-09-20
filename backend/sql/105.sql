-- 105.sql — TB-1（告警规则 2.0）第三片：告警生命周期与显式/自动清除（Clear Alarm & Lifecycle）。
--
-- 背景：
--   对齐 ThingsBoard 4.3 LTS 告警生命周期四态（ACTIVE_UNACK, ACTIVE_ACK, CLEARED_UNACK, CLEARED_ACK）。
--   提供告警清除（Clear Alarm）端点，支持携带清除原因与清除人审计留痕。
--   登记 Casbin 权限，授予 SYS_ADMIN 与 TENANT_ADMIN。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/alarm/info/history/:id/clear')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/alarm/info/history/:id/clear'),
  ('TENANT_ADMIN', 'api/v1/alarm/info/history/:id/clear')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
