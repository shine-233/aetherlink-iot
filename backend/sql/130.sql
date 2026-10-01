-- 130.sql — TB-45 Integration 实体纳管管线（integrations 统一集成实体）
--
-- 背景：
--   1. 对标 ThingsBoard Integration 实体：一个 Integration = 一条"连接器 + 上下行转换器绑定"，
--      把分散的连接器（OPC UA / SNMP 采集器、插件）纳管为统一实体（ROADMAP §4.1 批 1 首项）；
--   2. converter_uplink_id / converter_downlink_id 可空外键指向 data_converters（120.sql）：
--      上行转换器在采集回填路径执行（OPC UA 采集器首接，失败丢弃不阻断采集）；
--      下行反向下发执行按批次范围明确不做，列先立、语义留白；
--   3. 设备与 Integration 的绑定关系存于 config JSONB 的 device_ids 数组（无独立关联表，
--      单租户内集成实例数量级小，避免 join 复杂度）；connector_type 以 CHECK 约束收敛枚举；
--   4. Casbin 路由登记与赋权（SYS_ADMIN, TENANT_ADMIN）；注册统一集成管理页菜单
--      （sys_ui_elements，前端 /integration/list）。
--
-- 关键注意事项：
--   - converter_* 外键 ON DELETE SET NULL：转换器删除不级联删集成实例，绑定自动解挂，
--     与 service 层"删除转换器不检查引用"的现状兼容；
--   - connector_type 的合法值由 CHECK 约束兜底，业务校验（oneof）仍在 model 层双保险。

-- ---- 1. 统一集成实体表 (integrations) ----
CREATE TABLE IF NOT EXISTS public.integrations (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    connector_type VARCHAR(32) NOT NULL,                  -- opcua / snmp / plugin（CHECK 收敛）
    converter_uplink_id VARCHAR(36) REFERENCES public.data_converters(id) ON DELETE SET NULL,
    converter_downlink_id VARCHAR(36) REFERENCES public.data_converters(id) ON DELETE SET NULL,
    config JSONB NOT NULL DEFAULT '{}',                   -- 自由配置：device_ids 数组即设备绑定关系
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_integrations_connector_type CHECK (connector_type IN ('opcua', 'snmp', 'plugin'))
);

CREATE INDEX IF NOT EXISTS idx_integrations_tenant_connector ON public.integrations(tenant_id, connector_type);
CREATE INDEX IF NOT EXISTS idx_integrations_converter_uplink ON public.integrations(converter_uplink_id);
CREATE INDEX IF NOT EXISTS idx_integrations_converter_downlink ON public.integrations(converter_downlink_id);

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/integrations'),
  ('api/v1/integrations/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/integrations'),
  ('SYS_ADMIN',    'api/v1/integrations/:id'),
  ('TENANT_ADMIN', 'api/v1/integrations'),
  ('TENANT_ADMIN', 'api/v1/integrations/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 统一集成管理菜单 (sys_ui_elements，对标 TB Integrations 顶级菜单) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /integration/list 已注册，必须补菜单行否则路由守卫判定 403（见 100.sql 注释）。
-- 顶层目录行 element_type 1 + route_path 'layout.base'，子页面行 element_type 3 + route_path 'view.integration_list'。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'd4e5f6a7-8b29-4c30-9d41-2e3f4a5b6c72',
    '0',
    'integration', 1, 121, '/integration', 'mdi:plug-socket', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '统一集成', CURRENT_TIMESTAMP,
    'Unified integrations management (TB-45)', 'route.integration',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'integration'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'e5f6a7b8-9c3a-4d41-ae52-3f4a5b6c7d83',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'integration'),
    'integration_list', 3, 1, '/integration/list', 'mdi:plug-socket', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '集成管理', CURRENT_TIMESTAMP,
    'Integration instances with converter binding (TB-45)', 'route.integration_list',
    'view.integration_list'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'integration_list'
);
