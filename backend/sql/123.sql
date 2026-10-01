-- 123.sql — 部件库 widget_bundles（ThingsBoard Widget Bundle 对标，ROADMAP TB-04）
--
-- 背景：
--   1. 看板/SCADA 画布已有，但部件均为代码内置（service/scada_mobile_wiring.go 的
--      builtinWidgetDefinitions：gauge/chart/valve/twin3d），全仓此前无 widget 部件库实体；
--   2. 本迁移建立租户级部件库表，部件定义以 JSONB 存储（形状对齐 service.WidgetDefinition：
--      type/version/schema/capabilities/commands），为"内置部件迁移为可管理 bundle"提供落点；
--   3. 支持资源中心打包/验签导入分发（resource_type=widget_bundle）；
--   4. 提供 Casbin 路由登记与多租户权限隔离：
--      - POST /api/v1/widget-bundles (创建部件库)
--      - PUT /api/v1/widget-bundles (更新部件库)
--      - GET /api/v1/widget-bundles (分页拉取租户部件库列表)
--      - GET /api/v1/widget-bundles/:id (获取部件库详情)
--      - DELETE /api/v1/widget-bundles/:id (删除部件库)
--      - GET /api/v1/widget-bundles/builtin (内置四部件定义导出描述)
--      - POST /api/v1/widget-bundles/seed (一键把内置四部件落为租户可管理种子 bundle)
--
-- 关键注意事项：
--   - 除本注释列出的核心列外，额外加 version/type_key 两列：资源中心的冲突预览按
--     (租户, 名称, 版本) 判定覆盖、目录按行业类型聚合，没有这两列分发链路无法闭环。
--   - widgets JSONB 只做结构校验（service 层复用 ValidateWidgetDefinition），
--     schema 编译仍发生在注册表/画布保存路径，本表不存已编译 schema。

-- ---- 1. 部件库主表 (widget_bundles) ----
CREATE TABLE IF NOT EXISTS public.widget_bundles (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    widgets JSONB NOT NULL DEFAULT '[]'::jsonb,
    description VARCHAR(500),
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    type_key VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_widget_bundles_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_widget_bundles_tenant_id ON public.widget_bundles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_widget_bundles_type_key ON public.widget_bundles(type_key);

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/widget-bundles'),
  ('api/v1/widget-bundles/:id'),
  ('api/v1/widget-bundles/builtin'),
  ('api/v1/widget-bundles/seed')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/widget-bundles'),
  ('SYS_ADMIN',    'api/v1/widget-bundles/:id'),
  ('SYS_ADMIN',    'api/v1/widget-bundles/builtin'),
  ('SYS_ADMIN',    'api/v1/widget-bundles/seed'),
  ('TENANT_ADMIN', 'api/v1/widget-bundles'),
  ('TENANT_ADMIN', 'api/v1/widget-bundles/:id'),
  ('TENANT_ADMIN', 'api/v1/widget-bundles/builtin'),
  ('TENANT_ADMIN', 'api/v1/widget-bundles/seed')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 部件库管理菜单 (sys_ui_elements) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /visualization/widget-bundles 已注册，必须补菜单行否则路由守卫判定 403（见 100.sql 注释）。
-- 父级 'visualization' 由 1.sql 提供，此处只挂子页面行。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'd1e6f7a8-4b37-5c48-9ba2-8d3e4f5a6b72',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'visualization'),
    'visualization_widget-bundles', 3, 5, '/visualization/widget-bundles', 'mdi:view-dashboard-edit-outline', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '部件库', CURRENT_TIMESTAMP,
    'Widget bundle library with builtin seed import (TB-04)', 'route.visualization-widget-bundles',
    'view.visualization_widget-bundles'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'visualization_widget-bundles'
);
