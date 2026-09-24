-- 120.sql — ThingsBoard 核心数据转换器（Data Converters / 载荷解析与编解码引擎）
--
-- 背景：
--   1. 对标 ThingsBoard Integrations 与核心 Data Converter 引擎（Uplink / Downlink 转换器）；
--   2. 支持非标十六进制 Hex、自定义嵌套 JSON 与脚本模式解析设备上报原始载荷；
--   3. 提供转换器配置管理（CRUD）与 Dry-Run 仿真测试调试能力（POST /api/v1/converters/test）；
--   4. Casbin 路由登记与赋权（SYS_ADMIN, TENANT_ADMIN）。

-- ---- 1. 数据转换器表 (data_converters) ----
CREATE TABLE IF NOT EXISTS data_converters (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'UPLINK',
    converter_mode VARCHAR(32) NOT NULL DEFAULT 'SCRIPT',
    debug_mode BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id VARCHAR(36) NOT NULL,
    configuration TEXT NOT NULL DEFAULT '{}',
    script TEXT,
    description VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_data_converters_tenant_type ON data_converters(tenant_id, type);

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/converters'),
  ('api/v1/converters/:id'),
  ('api/v1/converters/test'),
  ('api/v1/data-converters'),
  ('api/v1/data-converters/:id'),
  ('api/v1/data-converters/test')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/converters'),
  ('SYS_ADMIN',    'api/v1/converters/:id'),
  ('SYS_ADMIN',    'api/v1/converters/test'),
  ('SYS_ADMIN',    'api/v1/data-converters'),
  ('SYS_ADMIN',    'api/v1/data-converters/:id'),
  ('SYS_ADMIN',    'api/v1/data-converters/test'),
  ('TENANT_ADMIN', 'api/v1/converters'),
  ('TENANT_ADMIN', 'api/v1/converters/:id'),
  ('TENANT_ADMIN', 'api/v1/converters/test'),
  ('TENANT_ADMIN', 'api/v1/data-converters'),
  ('TENANT_ADMIN', 'api/v1/data-converters/:id'),
  ('TENANT_ADMIN', 'api/v1/data-converters/test')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
