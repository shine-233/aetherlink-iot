-- 81.sql: Phase D6 边缘计算 2.0——边缘同步任务表 + casbin 受保护路由登记（2026-09-07）。
-- 功能：edge_sync_tasks 记录看板/规则链快照下发与 OTA 经边分发的投递任务
--       （payload 为完整下发快照，状态机 pending→synced|failed，error/attempts 可观测，可 retry 重放）。
--       下行通道：共享 MQTT 发布客户端 → {commands.publish_topic}{gateway_device_number}。
-- casbin：登记 4 条受保护路由（创建/列表/详情/重试/OTA 经边分发——列表与创建同路径）。
--   授权口径：边缘编排属租户级配置，仅 SYS_ADMIN + TENANT_ADMIN 可管理。
-- 幂等性：CREATE TABLE/INDEX IF NOT EXISTS；casbin 行 INSERT 前置 NOT EXISTS 守卫。

CREATE TABLE IF NOT EXISTS edge_sync_tasks (
    id                    VARCHAR(36) PRIMARY KEY,
    tenant_id             VARCHAR(36) NOT NULL,
    gateway_device_id     VARCHAR(36) NOT NULL,
    gateway_device_number VARCHAR(128) NOT NULL,
    resource_type         VARCHAR(32) NOT NULL,
    resource_id           VARCHAR(36) NOT NULL,
    payload               TEXT NOT NULL,
    status                VARCHAR(16) NOT NULL DEFAULT 'pending',
    error                 TEXT,
    attempts              INTEGER NOT NULL DEFAULT 0,
    synced_at             TIMESTAMP,
    created_at            TIMESTAMP NOT NULL DEFAULT now(),
    updated_at            TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_edge_sync_tenant ON edge_sync_tasks (tenant_id);
CREATE INDEX IF NOT EXISTS idx_edge_sync_gateway ON edge_sync_tasks (gateway_device_id);
CREATE INDEX IF NOT EXISTS idx_edge_sync_resource ON edge_sync_tasks (resource_type);
CREATE INDEX IF NOT EXISTS idx_edge_sync_status ON edge_sync_tasks (status);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/edge/sync'),
  ('api/v1/edge/sync/:id'),
  ('api/v1/edge/sync/:id/retry'),
  ('api/v1/edge/ota/distribute')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/edge/sync'),
  ('TENANT_ADMIN', 'api/v1/edge/sync'),
  ('SYS_ADMIN',    'api/v1/edge/sync/:id'),
  ('TENANT_ADMIN', 'api/v1/edge/sync/:id'),
  ('SYS_ADMIN',    'api/v1/edge/sync/:id/retry'),
  ('TENANT_ADMIN', 'api/v1/edge/sync/:id/retry'),
  ('SYS_ADMIN',    'api/v1/edge/ota/distribute'),
  ('TENANT_ADMIN', 'api/v1/edge/ota/distribute')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
