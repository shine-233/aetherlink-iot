-- 114.sql — TB-19 解决方案模板引擎：前端菜单登记
--
-- 背景：
--   109.sql 为 TB-18 Secrets 登记了菜单行（element_code=management_secrets），
--   但该页面的路由四件套当时漏配，运行期被 route-adapter 以
--   「skip invalid menu route」跳过——菜单存在而页面不可达（2026-09-19 复核发现，
--   已在前端补齐 imports/systemRoutes/transform/typings 四处并同批修复）。
--   本迁移为 TB-19 行业方案页登记同级菜单行，路由四件套随前端同批交付，
--   避免 Secrets 那样的「菜单有了、页面没挂」再次发生。

-- 1. 挂载前端菜单到系统管理模块（orders 48，紧随 Secrets 的 47）
INSERT INTO sys_ui_elements (
    id, parent_id, element_code, element_type, orders,
    param1, param2, param3, authority, description,
    created_at, remark, multilingual, route_path
)
SELECT
    'a1b2c3d4-11a1-4bb2-9cc3-114d4e5f6a7b',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_solutions', 3, 48,
    '/management/solutions', 'mdi:package-variant-closed', 'self',
    '["SYS_ADMIN", "TENANT_ADMIN"]'::json,
    'Industry Solution Templates (TB-19)',
    CURRENT_TIMESTAMP, '',
    'route.management_solutions',
    'view.management_solutions'
WHERE NOT EXISTS (
    SELECT 1 FROM sys_ui_elements WHERE element_code = 'management_solutions'
);
