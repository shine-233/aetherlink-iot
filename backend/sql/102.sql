-- ROADMAP 收尾：把"前端已实现、但 sys_ui_elements 里没有菜单行"的路由补成可达。
--
-- 背景（2026-09-15 全量扫描，automation_tests/scripts/diag-spa-mount.js）：
--   把 frontend 路由表（typings/elegant-router.d.ts 的 RouteMap，76 条）
--   与 sys_ui_elements 的 param1 做差集，再逐条用真实浏览器验证，
--   结果 **19 条路由返回 403 No Permission**。
--   根因与 100.sql 那批完全一致：本项目 VITE_AUTH_ROUTE_MODE=dynamic，
--   授权路由由菜单驱动；路由表里有条目、菜单里没有 → 守卫跳 403（不是 404）。
--   也就是说这些页面**代码写完了、路由注册了，用户却无处可达**。
--
-- 本批全部以 param3='1' 加入：**授权可达，但不显示到侧边栏**。
--   理由：param3='1' → hideInMenu（见 service/api/management.adapter.ts:249
--   `hideInMenu: item.param3 === '1'`）。这些页面此前从未在真实浏览器里被验证过，
--   直接摆上侧边栏等于把未经验证的界面推给用户。先让它们可达以便逐个取证，
--   验证通过后再由产品决定是否把 param3 改成 '0' 显示出来。
--
-- 只补菜单数据，不改表结构；全部带 NOT EXISTS 守卫，可重复执行。

-- ---------------------------------------------------------------------------
-- 一、新增两个父级（/dashboard、/product 在前端是纯父路由，无 index.vue）
-- ---------------------------------------------------------------------------

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e04',
    '0',
    'dashboard', 1, 110, '/dashboard', 'mdi:view-dashboard-outline', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '仪表盘', CURRENT_TIMESTAMP,
    'Dashboard parent (hidden until pages are verified)', 'route.dashboard',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'dashboard'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0c',
    '0',
    'product', 1, 111, '/product', 'mdi:package-variant-closed', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '产品', CURRENT_TIMESTAMP,
    'Product parent (hidden until pages are verified)', 'route.product',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'product'
);

-- ---------------------------------------------------------------------------
-- 二、叶子页（挂在既有父级下）
-- ---------------------------------------------------------------------------

-- /apply/service
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e01',
    'a2c53126-029f-7138-4d7a-f45491f396da',
    'apply_service', 3, 33, '/apply/service', 'mdi:application-cog', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '服务', CURRENT_TIMESTAMP,
    'Service management under apply', 'route.apply-service',
    'view.apply_service'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'apply_service'
);

-- 规则链：ThingsBoard _rule engine 的对标能力，此前完全不可达
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e02',
    '676e8f33-875a-0473-e9ca-c82fd09fef57',
    'automation_rule-chain', 3, 1144, '/automation/rule-chain', 'mdi:graph-outline', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '规则链', CURRENT_TIMESTAMP,
    'Rule chain console', 'route.automation-rule-chain',
    'view.automation_rule-chain'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'automation_rule-chain'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e03',
    '676e8f33-875a-0473-e9ca-c82fd09fef57',
    'automation_rule-chain-edit', 3, 1145, '/automation/rule-chain/edit', 'mdi:graph-outline', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '规则链编辑', CURRENT_TIMESTAMP,
    'Rule chain editor', 'route.automation-rule-chain-edit',
    'view.automation_rule-chain-edit'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'automation_rule-chain-edit'
);

-- /dashboard 三页
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e05',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'dashboard'),
    'dashboard_rdi-overview', 3, 1, '/dashboard/rdi-overview', 'mdi:view-dashboard', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, 'RDI 总览', CURRENT_TIMESTAMP,
    'RDI dashboard overview', 'route.dashboard-rdi-overview',
    'view.dashboard_rdi-overview'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'dashboard_rdi-overview'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e06',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'dashboard'),
    'dashboard_workbench', 3, 2, '/dashboard/workbench', 'mdi:view-dashboard', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '工作台', CURRENT_TIMESTAMP,
    'Dashboard workbench', 'route.dashboard-workbench',
    'view.dashboard_workbench'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'dashboard_workbench'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e07',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'dashboard'),
    'dashboard_workspace', 3, 3, '/dashboard/workspace', 'mdi:view-dashboard', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '工作区', CURRENT_TIMESTAMP,
    'Dashboard workspace', 'route.dashboard-workspace',
    'view.dashboard_workspace'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'dashboard_workspace'
);

-- /device 资产与实体关系
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e08',
    '5373a6a2-1861-af35-eb4c-adfd5ca55ecd',
    'device_asset', 3, 1180, '/device/asset', 'mdi:domain', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '资产', CURRENT_TIMESTAMP,
    'Device asset management', 'route.device-asset',
    'view.device_asset'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'device_asset'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e09',
    '5373a6a2-1861-af35-eb4c-adfd5ca55ecd',
    'device_entity-relation', 3, 1181, '/device/entity-relation', 'mdi:relation-many-to-many', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '实体关系', CURRENT_TIMESTAMP,
    'Entity relation management', 'route.device-entity-relation',
    'view.device_entity-relation'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'device_entity-relation'
);

-- /management 实体版本与角色
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0a',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_entity-version', 3, 1180, '/management/entity-version', 'mdi:source-branch', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '实体版本', CURRENT_TIMESTAMP,
    'Entity version management', 'route.management-entity-version',
    'view.management_entity-version'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'management_entity-version'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0b',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_role', 3, 1181, '/management/role', 'mdi:account-key', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '角色', CURRENT_TIMESTAMP,
    'Role management', 'route.management-role',
    'view.management_role'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'management_role'
);

-- /product 三页（预注册 / OTA 升级 / 升级包）
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0d',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'product'),
    'product_pre-register', 3, 1, '/product/pre-register', 'mdi:barcode', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '预注册', CURRENT_TIMESTAMP,
    'Product pre-registration', 'route.product-pre-register',
    'view.product_pre-register'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'product_pre-register'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0e',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'product'),
    'product_update-ota', 3, 2, '/product/update-ota', 'mdi:update', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, 'OTA 升级', CURRENT_TIMESTAMP,
    'OTA update management', 'route.product-update-ota',
    'view.product_update-ota'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'product_update-ota'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e0f',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'product'),
    'product_update-package', 3, 3, '/product/update-package', 'mdi:package-up', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '升级包', CURRENT_TIMESTAMP,
    'OTA update package management', 'route.product-update-package',
    'view.product_update-package'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'product_update-package'
);

-- /system-management-user/equipment-map（挂在 management 下，与 system-log 同级）
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e10',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'system-management-user_equipment-map', 3, 1172, '/system-management-user/equipment-map', 'mdi:map-marker-radius', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '设备地图', CURRENT_TIMESTAMP,
    'Equipment map', 'route.system-management-user-equipment-map',
    'view.system-management-user_equipment-map'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'system-management-user_equipment-map'
);

-- SCADA：列表与编辑器（组件分别在 @/views/scada、@/views/visualization/scada-editor）
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e11',
    '95e2a961-382b-f4a6-87b3-1898123c95bc',
    'visualization_scada', 3, 5, '/visualization/scada', 'mdi:monitor-dashboard', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, 'SCADA', CURRENT_TIMESTAMP,
    'SCADA board list', 'route.visualization-scada',
    'view.visualization_scada'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'visualization_scada'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b1e1a0c2-3d4e-4f50-8162-7a8b9c0d1e12',
    '95e2a961-382b-f4a6-87b3-1898123c95bc',
    'visualization_scada-editor', 3, 6, '/visualization/scada-editor', 'mdi:monitor-edit', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, 'SCADA 编辑器', CURRENT_TIMESTAMP,
    'SCADA editor (symbol library + drag and drop)', 'route.visualization-scada-editor',
    'view.visualization_scada-editor'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'visualization_scada-editor'
);
