-- 99.sql — P1.6 模板升级/回滚：升级历史表 + 新路由 Casbin 登记。
--
-- 设计：升级 = 以新版本导入新行（租户幂等）+ 把旧版本完整导出载荷存入本表；
-- 回滚 = 重放历史里的旧载荷（幂等），**不删除新版本行**——删行不可逆，
-- 重放让版本共存、切换交给引用方。previous_payload 即回滚凭据本身。

CREATE TABLE IF NOT EXISTS public.device_template_upgrade_history (
    id               varchar(36)  PRIMARY KEY,
    tenant_id        varchar(36)  NOT NULL,
    template_name    varchar(255) NOT NULL,
    from_version     varchar(36)  NOT NULL,
    to_version       varchar(36)  NOT NULL,
    previous_payload text         NOT NULL,
    actor_id         varchar(36)  NOT NULL,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT device_template_upgrade_history_tenant_check CHECK (tenant_id <> '')
);

CREATE INDEX IF NOT EXISTS device_template_upgrade_history_name_idx
    ON public.device_template_upgrade_history (tenant_id, template_name, created_at DESC);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/template/upgrade'),
  ('api/v1/device/template/upgrade/:history_id/rollback'),
  ('api/v1/device/template/upgrade/history')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device/template/upgrade'),
  ('TENANT_ADMIN', 'api/v1/device/template/upgrade'),
  ('SYS_ADMIN',    'api/v1/device/template/upgrade/:history_id/rollback'),
  ('TENANT_ADMIN', 'api/v1/device/template/upgrade/:history_id/rollback'),
  ('SYS_ADMIN',    'api/v1/device/template/upgrade/history'),
  ('TENANT_ADMIN', 'api/v1/device/template/upgrade/history'),
  ('TENANT_USER',  'api/v1/device/template/upgrade/history')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
