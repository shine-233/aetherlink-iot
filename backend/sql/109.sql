-- 109.sql — TB-18 通用 Secrets Storage（Universal Secrets Management & Storage，对标 ThingsBoard PE）
--
-- 背景：
--   对标 ThingsBoard Professional Edition (PE) 核心安全能力 Secrets Storage：
--   1. 创建 sys_secrets 表，提供基于 AES-256-GCM 信封加密的通用密钥与敏感凭证集中安全存储；
--   2. 严格按租户隔离（支持系统级公共密钥与租户独立命名空间），联合唯一键约束 (tenant_id, key)；
--   3. 登记 Casbin 路由 (api/v1/secrets 相关) 并按最小特权原则赋权角色；
--   4. 挂载 sys_ui_elements 菜单配置（系统管理 / 密钥管理）。

-- 1. 创建通用密钥保管库主表
CREATE TABLE IF NOT EXISTS sys_secrets (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(36) NOT NULL DEFAULT '',
    key varchar(64) NOT NULL,
    name varchar(128) NOT NULL,
    secret_type varchar(32) NOT NULL DEFAULT 'GENERIC', -- 'GENERIC' | 'API_KEY' | 'TOKEN' | 'PASSWORD' | 'CERTIFICATE' | 'OAUTH2'
    description varchar(255) DEFAULT '',
    encrypted_value text NOT NULL,
    mask_preview varchar(32) NOT NULL DEFAULT '****',
    key_id varchar(32) NOT NULL DEFAULT '',
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_sys_secrets_tenant_key UNIQUE (tenant_id, key)
);

CREATE INDEX IF NOT EXISTS idx_sys_secrets_tenant ON sys_secrets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sys_secrets_type ON sys_secrets(tenant_id, secret_type);

-- 2. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/secrets'),
  ('api/v1/secrets/:id'),
  ('api/v1/secrets/:id/reveal'),
  ('api/v1/secrets/:id/reseal')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 3. 授权角色 (p)
-- SYS_ADMIN: 全量管理与解密/重加密权限
-- TENANT_ADMIN: 租户内管理与解密/重加密权限
-- TENANT_USER: 仅允许查询列表和详情（仅可见脱敏掩码，禁止 reveal 与 reseal）
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/secrets'),
  ('SYS_ADMIN',    'api/v1/secrets/:id'),
  ('SYS_ADMIN',    'api/v1/secrets/:id/reveal'),
  ('SYS_ADMIN',    'api/v1/secrets/:id/reseal'),

  ('TENANT_ADMIN', 'api/v1/secrets'),
  ('TENANT_ADMIN', 'api/v1/secrets/:id'),
  ('TENANT_ADMIN', 'api/v1/secrets/:id/reveal'),
  ('TENANT_ADMIN', 'api/v1/secrets/:id/reseal'),

  ('TENANT_USER',  'api/v1/secrets'),
  ('TENANT_USER',  'api/v1/secrets/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- 4. 挂载前端菜单到系统管理模块
INSERT INTO sys_ui_elements (
    id, parent_id, element_code, element_type, orders,
    param1, param2, param3, authority, description,
    created_at, remark, multilingual, route_path
)
SELECT
    '99e1c4a0-5b12-4cf3-911e-8e4f1a2b3c4d',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_secrets', 3, 47,
    '/management/secrets', 'mdi:key-variant', 'self',
    '["SYS_ADMIN", "TENANT_ADMIN"]'::json,
    'Universal Secrets Storage (TB-18)',
    CURRENT_TIMESTAMP, '',
    'route.management_secrets',
    'view.management_secrets'
WHERE NOT EXISTS (
    SELECT 1 FROM sys_ui_elements WHERE element_code = 'management_secrets'
);
