-- 78.sql: Phase D3 定时报表——建表 + casbin 受保护路由登记（2026-09-06）。
-- 功能：report_schedules 存储租户级定时报表任务（cron 表达式、收件人、设备/测点作用域、
--       回看时长、启用状态），由后端 cron 每分钟扫描到期任务，导出遥测 CSV 并经 D2 邮件渠道投递。
-- 1) 建表（IF NOT EXISTS 幂等）+ 两个索引；
-- 2) 登记受保护路由 api/v1/report/schedules 与 api/v1/report/schedules/:id 的 g2 资源行与 p 授权行。
--    授权口径：报表属租户级配置，仅 SYS_ADMIN + TENANT_ADMIN 可管理（不向 TENANT_USER 开放）。
-- 幂等性：CREATE TABLE/INDEX IF NOT EXISTS；casbin 行 INSERT 前置 NOT EXISTS 守卫。

CREATE TABLE IF NOT EXISTS report_schedules (
    id             VARCHAR(36) PRIMARY KEY,
    tenant_id      VARCHAR(36) NOT NULL,
    name           VARCHAR(128) NOT NULL,
    cron_expr      VARCHAR(128) NOT NULL,
    recipients     TEXT NOT NULL,
    device_ids     JSONB,
    keys           JSONB,
    lookback_hours INTEGER NOT NULL DEFAULT 24,
    format         VARCHAR(16) NOT NULL DEFAULT 'csv',
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at    TIMESTAMP,
    last_status    VARCHAR(64),
    created_at     TIMESTAMP NOT NULL DEFAULT now(),
    updated_at     TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_report_schedules_tenant ON report_schedules (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_schedules_enabled ON report_schedules (enabled);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/report/schedules'),
  ('api/v1/report/schedules/:id'),
  ('api/v1/report/schedules/:id/run')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/report/schedules'),
  ('TENANT_ADMIN', 'api/v1/report/schedules'),
  ('SYS_ADMIN',    'api/v1/report/schedules/:id'),
  ('TENANT_ADMIN', 'api/v1/report/schedules/:id'),
  ('SYS_ADMIN',    'api/v1/report/schedules/:id/run'),
  ('TENANT_ADMIN', 'api/v1/report/schedules/:id/run')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
