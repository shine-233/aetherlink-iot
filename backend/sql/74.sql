-- 74.sql: 插件框架 gRPC 网关——插件注册表与管理路由登记（PHASE-D-D9，2026-09-06）。
-- 1) plugin_registries 表：插件登记（token 仅存 sha256 摘要，状态机 online/offline/disabled）；
-- 2) g2 资源登记（fail-fast 审计要求）；
-- 3) p 授权：SYS_ADMIN + TENANT_ADMIN（插件管理属平台/租户管理能力，TENANT_USER 不开放）。
-- 幂等性：IF NOT EXISTS + NOT EXISTS 守卫，重放无副作用。

CREATE TABLE IF NOT EXISTS plugin_registries (
    id             VARCHAR(36) PRIMARY KEY,
    name           VARCHAR(128) NOT NULL,
    version        VARCHAR(64),
    transport      VARCHAR(32) NOT NULL DEFAULT 'grpc',
    token_hash     VARCHAR(128) NOT NULL,
    status         VARCHAR(32) NOT NULL DEFAULT 'disabled',
    last_heartbeat TIMESTAMPTZ,
    description    TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_registries_name ON plugin_registries (name);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/plugins'),
  ('api/v1/plugins/:id'),
  ('api/v1/plugins/:id/enable'),
  ('api/v1/plugins/:id/disable'),
  ('api/v1/plugins/:id/downlink')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/plugins'),
  ('SYS_ADMIN',    'api/v1/plugins/:id'),
  ('SYS_ADMIN',    'api/v1/plugins/:id/enable'),
  ('SYS_ADMIN',    'api/v1/plugins/:id/disable'),
  ('SYS_ADMIN',    'api/v1/plugins/:id/downlink'),
  ('TENANT_ADMIN', 'api/v1/plugins'),
  ('TENANT_ADMIN', 'api/v1/plugins/:id'),
  ('TENANT_ADMIN', 'api/v1/plugins/:id/enable'),
  ('TENANT_ADMIN', 'api/v1/plugins/:id/disable'),
  ('TENANT_ADMIN', 'api/v1/plugins/:id/downlink')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path
);
