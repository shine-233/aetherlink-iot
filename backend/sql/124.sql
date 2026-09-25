-- 124.sql — 租户配额执法（API 维度）：api_usage_daily 日调用计量 + 配额查询端点 + 前端配额页（TB-17）
--
-- 背景：
--   1. 对标 ThingsBoard PE 的租户 API 配额：117.sql 已在 subscription_plans 落了 max_api_calls_per_day，
--      但 API 维度没有计量落点，也没有按日执法——本迁移补上计量持久化表；
--   2. 计量：middleware.TenantRateLimit 的既有按租户限流路径上同日累加（Redis INCR 实时计数，
--      定期异步落库到本表；计量链路 fail-open，计量失败不阻断请求）；
--   3. 执法：按租户订阅套餐（tenant_subscriptions -> subscription_plans.max_api_calls_per_day）
--      判定当日调用是否超限，超限返回 429 + Retry-After（与 per-tenant 限流同一套 429 语义）；
--   4. 新增配额查询端点 GET /api/v1/billing/api-quota（今日调用数/限额/剩余量）与前端配额页菜单。
--
-- 注意：日窗口以 UTC 日期（usage_date）切分，与后端计量器/执法判定保持同一口径。

-- ---- 1. API 日调用计量表 (api_usage_daily) ----
-- 列名用 usage_date 而非 date：避免与 PG 类型关键字同名带来的引用歧义。
-- api_calls 用 bigint：enterprise 档 5,000,000/日仍留足余量。
CREATE TABLE IF NOT EXISTS public.api_usage_daily (
    tenant_id VARCHAR(36) NOT NULL,
    usage_date DATE NOT NULL,
    api_calls BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_api_usage_daily_tenant_date UNIQUE (tenant_id, usage_date)
);

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/billing/api-quota')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p)：与 117.sql 的 billing 端点授权面一致 ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/billing/api-quota'),
  ('TENANT_ADMIN', 'api/v1/billing/api-quota')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 配额页菜单 (sys_ui_elements)：billing 顶级目录 + api-quota 子页面 ----
-- 本项目的授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），
-- 页面路由 /billing/api-quota 已注册，必须补菜单行否则路由守卫判定 403（见 100.sql 注释）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b3f9d2e1-7a46-4c58-9b0d-8e2f1a3c5d60',
    '0',
    'billing', 1, 118, '/billing', 'mdi:chart-box-outline', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '配额计费', CURRENT_TIMESTAMP,
    'Billing & API quota console (TB-17, ThingsBoard PE per-tenant quotas parity)', 'route.billing',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'billing'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'c4a8e3f2-8b57-4d69-9c1e-9f3a2b4d6e71',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'billing'),
    'billing_api-quota', 3, 1, '/billing/api-quota', 'mdi:gauge', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, 'API 配额', CURRENT_TIMESTAMP,
    'Per-tenant daily API quota usage: calls today / limit / remaining (TB-17)',
    'route.billing_api-quota', 'view.billing_api-quota'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'billing_api-quota'
);
