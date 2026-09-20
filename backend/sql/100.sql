-- ROADMAP P1.6 / P2.2：补齐"前端已实现但后端菜单缺失"的两组路由。
--
-- 背景（2026-09-14 浏览器实测）：
--   本项目的授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic）。
--   views/visualization/anomaly 与 views/market/browse 两个页面在 frontend/src/router
--   里已注册（generatedRoutes 有条目），但 sys_ui_elements 里没有对应菜单行。
--   路由守卫 permission.ts 的判定是：
--     to.name === 'not-found' && getIsAuthRouteExist(path) === true  →  跳 403
--   也就是"路由表里有、但不在你的授权菜单里" → 403（而不是 404）。
--   结果：页面存在、路由已注册，用户仍然无处可达，后端整条闸门是死门。
--
-- 本迁移只补菜单数据，不改表结构；全部带 NOT EXISTS 守卫，可重复执行。
-- 对应前端改动：src/router/elegant/{routes,imports,transform,visualizationRoutes}.ts、
--   src/typings/elegant-router.d.ts、src/locales/langs/*/route.json。

-- 1) 异常检测页（P2.2），挂在既有 visualization 父级下。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'a7f3c1d2-5e84-4b19-9c6a-2d8f0e3b7a41',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'visualization'),
    'visualization_anomaly', 3, 4, '/visualization/anomaly', 'mdi:chart-line-variant', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '异常检测', CURRENT_TIMESTAMP,
    'Telemetry anomaly detection workbench (P2.2)', 'route.visualization-anomaly',
    'view.visualization_anomaly'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'visualization_anomaly'
);

-- 2) 模板市场父级（P1.6）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b8e4d2f3-6f95-4c2a-8d7b-3e9a1f4c8b52',
    '0',
    'market', 1, 116, '/market', 'mdi:storefront-outline', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '模板市场', CURRENT_TIMESTAMP,
    'Device template market (P1.6)', 'route.market',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'market'
);

-- 3) 模板市场浏览 / 打包导入页（P1.6），挂在 market 下。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'c9f5e3a4-7a06-4d3b-9e8c-4fab2a5d9c63',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'market'),
    'market_browse', 3, 1, '/market/browse', 'mdi:package-variant-closed', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '模板市场浏览', CURRENT_TIMESTAMP,
    'Market browse with signed bundle preview / overwrite gate (P1.6)',
    'route.market_browse', 'view.market_browse'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'market_browse'
);

-- 4) 边缘节点控制台（P1.5），挂在既有 management 父级下。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'd1a6f4b5-8b17-4e4c-af9d-5abc3b6ead74',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_edge-nodes', 3, 45, '/management/edge-nodes', 'mdi:lan-connect', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '边缘节点', CURRENT_TIMESTAMP,
    'Edge node registry, heartbeat and reconcile console (P1.5)', 'route.management_edge-nodes',
    'view.management_edge-nodes'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'management_edge-nodes'
);

-- 5) 许可证控制台（P3）。只给 SYS_ADMIN：这是平台级视图，
--    tenant_admin 落到 403 是预期行为（见 e2e/24_p1_console_surfaces.spec.js 注释）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'e2b7a5c6-9c28-4f5d-b0ae-6bcd4c7fbe85',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_license', 3, 46, '/management/license', 'mdi:license', 'self',
    '["SYS_ADMIN"]'::json, '许可证', CURRENT_TIMESTAMP,
    'Offline Ed25519 license status view (P3)', 'route.management_license',
    'view.management_license'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'management_license'
);
