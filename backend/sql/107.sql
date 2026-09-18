-- 107.sql — TB-7 队列隔离与限流集群化（Queue Isolation & Clustered Rate Limiting）
--
-- 背景：
--   对标 ThingsBoard 3.6.3+ 队列隔离机制与 ThingsBoard 4.3 LTS 集群多策略限流：
--   1. 创建 tenant_rate_limits 表，支持按租户/设备维度动态自定义配额（格式: "100:1,1000:60"）；
--   2. 登记多队列监控与集群限流管理路由的 Casbin 权限规则。

-- 1. 新建租户/设备限流配额自定义表
CREATE TABLE IF NOT EXISTS tenant_rate_limits (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(36) NOT NULL,
    target_type varchar(20) NOT NULL, -- 'tenant' | 'device'
    target_id varchar(64) NOT NULL,    -- tenant_id or device_id
    limit_type varchar(20) NOT NULL,   -- 'api' | 'tenant_transport' | 'device_transport'
    rate_limits varchar(128) NOT NULL,  -- e.g. '200:1,3000:60'
    enabled boolean DEFAULT true,
    description varchar(255) DEFAULT '',
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_rate_limit UNIQUE (tenant_id, target_type, target_id, limit_type)
);

CREATE INDEX IF NOT EXISTS idx_tenant_rate_limits_tenant ON tenant_rate_limits(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_rate_limits_target ON tenant_rate_limits(target_type, target_id);

-- 2. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/ratelimit/config'),
  ('api/v1/ratelimit/metrics'),
  ('api/v1/ratelimit/overrides'),
  ('api/v1/ratelimit/override'),
  ('api/v1/ratelimit/override/:target_type/:target_id'),
  ('api/v1/queue/stats'),
  ('api/v1/queue/config')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 3. 授权角色 (p)
-- SYS_ADMIN 拥有完整管理、监控与动态覆盖权限
-- TENANT_ADMIN 拥有租户级监控与本租户配额查看/配置权限
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/ratelimit/config'),
  ('SYS_ADMIN',    'api/v1/ratelimit/metrics'),
  ('SYS_ADMIN',    'api/v1/ratelimit/overrides'),
  ('SYS_ADMIN',    'api/v1/ratelimit/override'),
  ('SYS_ADMIN',    'api/v1/ratelimit/override/:target_type/:target_id'),
  ('SYS_ADMIN',    'api/v1/queue/stats'),
  ('SYS_ADMIN',    'api/v1/queue/config'),

  ('TENANT_ADMIN', 'api/v1/ratelimit/config'),
  ('TENANT_ADMIN', 'api/v1/ratelimit/metrics'),
  ('TENANT_ADMIN', 'api/v1/ratelimit/overrides'),
  ('TENANT_ADMIN', 'api/v1/ratelimit/override'),
  ('TENANT_ADMIN', 'api/v1/ratelimit/override/:target_type/:target_id'),
  ('TENANT_ADMIN', 'api/v1/queue/stats'),
  ('TENANT_ADMIN', 'api/v1/queue/config')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
